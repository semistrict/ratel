// Copyright 2017 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.

package row

import (
	"bytes"
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/semistrict/ratel/pkg/keys"
	"github.com/semistrict/ratel/pkg/kv"
	"github.com/semistrict/ratel/pkg/multitenant"
	"github.com/semistrict/ratel/pkg/roachpb"
	"github.com/semistrict/ratel/pkg/sql/catalog"
	"github.com/semistrict/ratel/pkg/sql/catalog/catpb"
	"github.com/semistrict/ratel/pkg/sql/catalog/colinfo"
	"github.com/semistrict/ratel/pkg/sql/catalog/descpb"
	"github.com/semistrict/ratel/pkg/sql/opt/exec"
	"github.com/semistrict/ratel/pkg/sql/rowenc"
	"github.com/semistrict/ratel/pkg/sql/rowenc/valueside"
	"github.com/semistrict/ratel/pkg/sql/rowinfra"
	"github.com/semistrict/ratel/pkg/sql/scrub"
	"github.com/semistrict/ratel/pkg/sql/sem/tree"
	"github.com/semistrict/ratel/pkg/sql/sessiondatapb"
	"github.com/semistrict/ratel/pkg/sql/types"
	"github.com/semistrict/ratel/pkg/util"
	"github.com/semistrict/ratel/pkg/util/admission"
	"github.com/semistrict/ratel/pkg/util/encoding"
	"github.com/semistrict/ratel/pkg/util/hlc"
	jsonutil "github.com/semistrict/ratel/pkg/util/json"
	"github.com/semistrict/ratel/pkg/util/log"
	"github.com/semistrict/ratel/pkg/util/mon"
	"github.com/semistrict/ratel/pkg/util/timeutil"
)

// DebugRowFetch can be used to turn on some low-level debugging logs. We use
// this to avoid using log.V in the hot path.
const DebugRowFetch = false

// noOutputColumn is a sentinel value to denote that a system column is not
// part of the output.
const noOutputColumn = -1

func newTxnWithSteppingEnabled(
	ctx context.Context, db *kv.DB, gatewayNodeID roachpb.NodeID, qualityOfService sessiondatapb.QoSLevel,
) *kv.Txn {
	source := roachpb.AdmissionHeader_FROM_SQL
	if multitenant.HasTenantCostControlExemption(ctx) {
		source = roachpb.AdmissionHeader_OTHER
	}
	txn := kv.NewTxnWithAdmissionControl(
		ctx, db, gatewayNodeID, source, admission.WorkPriority(qualityOfService),
	)
	_ = txn.ConfigureStepping(ctx, kv.SteppingEnabled)
	return txn
}

// KVBatchFetcher abstracts the logic of fetching KVs in batches.
type KVBatchFetcher interface {
	// nextBatch returns the next batch of rows. Returns false in the first
	// parameter if there are no more keys in the scan. May return either a slice
	// of KeyValues or a batchResponse, numKvs pair, depending on the server
	// version - both must be handled by calling code.
	nextBatch(ctx context.Context) (ok bool, kvs []roachpb.KeyValue, batchResponse []byte, err error)

	close(ctx context.Context)
}

type tableInfo struct {
	// -- Fields initialized once --
	spec descpb.IndexFetchSpec

	// The set of indexes into spec.FetchedColumns that are required for columns
	// in the value part.
	neededValueColsByIdx util.FastIntSet

	// The number of needed columns from the value part of the row. Once we've
	// seen this number of value columns for a particular row, we can stop
	// decoding values in that row.
	neededValueCols int

	// Map used to get the index for columns in spec.FetchedColumns.
	colIdxMap catalog.TableColMap

	// One value per column that is part of the key; each value is a column index
	// (into spec.FetchedColumns); -1 if we don't need the value for that column.
	indexColIdx []int

	// -- Fields updated during a scan --

	keyVals    []rowenc.EncDatum
	extraVals  []rowenc.EncDatum
	row        rowenc.EncDatumRow
	decodedRow tree.Datums

	// The following fields contain MVCC metadata for each row and may be
	// returned to users of Fetcher immediately after NextRow returns.
	//
	// rowLastModified is the timestamp of the last time any family in the row
	// was modified in any way.
	rowLastModified hlc.Timestamp
	// timestampOutputIdx controls at what row ordinal to write the timestamp.
	timestampOutputIdx int

	// Fields for outputting the tableoid system column.
	tableOid     tree.Datum
	oidOutputIdx int

	// rowIsDeleted is true when the row has been deleted. This is only
	// meaningful when kv deletion tombstones are returned by the KVBatchFetcher,
	// which the one used by `StartScan` (the common case) doesnt. Notably,
	// changefeeds use this by providing raw kvs with tombstones unfiltered via
	// `StartScanFrom`.
	rowIsDeleted bool
}

// Fetcher handles fetching kvs and forming table rows for a single table.
// Usage:
//
//	var rf Fetcher
//	err := rf.Init(..)
//	// Handle err
//	err := rf.StartScan(..)
//	// Handle err
//	for {
//	   res, err := rf.NextRow()
//	   // Handle err
//	   if res.row == nil {
//	      // Done
//	      break
//	   }
//	   // Process res.row
//	}
type Fetcher struct {
	table tableInfo

	// reverse denotes whether or not the spans should be read in reverse
	// or not when StartScan is invoked.
	reverse bool

	// True if the index key must be decoded. This is only false if there are no
	// needed columns.
	mustDecodeIndexKey bool

	// lockStrength represents the row-level locking mode to use when fetching
	// rows.
	lockStrength descpb.ScanLockingStrength

	// lockWaitPolicy represents the policy to be used for handling conflicting
	// locks held by other active transactions.
	lockWaitPolicy descpb.ScanLockingWaitPolicy

	// lockTimeout specifies the maximum amount of time that the fetcher will
	// wait while attempting to acquire a lock on a key or while blocking on an
	// existing lock in order to perform a non-locking read on a key.
	lockTimeout time.Duration

	// traceKV indicates whether or not session tracing is enabled. It is set
	// when beginning a new scan.
	traceKV bool

	// mvccDecodeStrategy controls whether or not MVCC timestamps should
	// be decoded from KV's fetched.
	mvccDecodeStrategy MVCCDecodingStrategy

	// -- Fields updated during a scan --

	kvFetcher *KVFetcher
	// indexKey stores the index key of the current row, up to (and not including)
	// any family ID.
	indexKey       []byte
	prettyValueBuf *bytes.Buffer

	valueColsFound int // how many needed cols we've found so far in the value

	// hasSubordinateColumns is true when the table has columns that use
	// subordinate key encoding.
	hasSubordinateColumns bool

	// subordinateArrays accumulates array elements from subordinate keys
	// during row assembly. Keyed by index into spec.FetchedColumns.
	// Cleared when starting a new row; finalized in finalizeRow.
	subordinateArrays map[int]*subordinateArrayBuilder

	// subordinateJSONBuilders accumulates recursive JSON nodes from subordinate
	// keys during row assembly. Keyed by index into spec.FetchedColumns.
	subordinateJSONBuilders map[int]*subordinateJSONBuilder

	// arrayEqualsAnyFilter, when set, evaluates a filter of the form
	//   left = ANY(array_col)
	// as array elements are scanned. This is only used when the array column is
	// fetched for scan-local filtering rather than output materialization.
	arrayEqualsAnyFilter *arrayEqualsAnyFilterState

	// lastRowPassesArrayEqualsAnyFilter records the filter result for the most
	// recently finalized row. It is true when no scan-local array filter is set.
	lastRowPassesArrayEqualsAnyFilter bool

	// jsonExistsFilter, when set, evaluates a filter of the form
	//   json_col ? 'key'
	//   json_col ?| array['k1', ...]
	//   json_col ?& array['k1', ...]
	// as subordinate JSON nodes are scanned.
	jsonExistsFilter *jsonExistsFilterState

	// lastRowPassesJSONExistsFilter records the filter result for the most
	// recently finalized row. It is true when no scan-local JSON filter is set.
	lastRowPassesJSONExistsFilter bool

	// jsonAccessPrograms incrementally compute derived results directly from
	// subordinate JSON KVs for the current row.
	jsonAccessPrograms []*jsonAccessProgramState

	// jsonPathCompareFilter, when set, evaluates an equality predicate over a
	// scan-local JSON path access result.
	jsonPathCompareFilter *jsonPathCompareFilterState

	// lastRowPassesJSONPathCompareFilter records the filter result for the most
	// recently finalized row. It is true when no scan-local JSON path compare
	// filter is set.
	lastRowPassesJSONPathCompareFilter bool

	// jsonContainsFilters evaluate containment predicates over scan-local JSON
	// access results.
	jsonContainsFilters []*jsonContainsFilterState
	jsonContainsByCol   map[int][]*jsonContainsFilterState

	// lastRowPassesJSONContainsFilter records whether the most recently
	// finalized row satisfied all configured scan-local JSON containment
	// filters. It is true when no such filters are set.
	lastRowPassesJSONContainsFilter bool

	// lastRowJSONAccessProgramResults stores finalized per-row results for the
	// configured JSON access programs in configuration order.
	lastRowJSONAccessProgramResults []tree.Datum

	// jsonSharedAccessPrograms de-duplicates identical scan-local JSON access
	// programs so multiple consumers can share one matcher state per row.
	jsonSharedAccessPrograms []*sharedJSONAccessProgramState
	jsonSharedAccessByCol    map[int][]*sharedJSONAccessProgramState

	// jsonSharedSelectedPaths de-duplicates scan-local JSON path selection so
	// path-based access and containment programs on the same source subtree only
	// resolve the path once per subordinate KV.
	jsonSharedSelectedPaths []*sharedJSONSelectedPathState
	jsonSharedSelectedByCol map[int][]*sharedJSONSelectedPathState

	// The current key/value, unless kvEnd is true.
	kv                roachpb.KeyValue
	keyRemainingBytes []byte
	kvEnd             bool

	// IgnoreUnexpectedNulls allows Fetcher to return null values for non-nullable
	// columns and is only used for decoding for error messages or debugging.
	IgnoreUnexpectedNulls bool

	// Buffered allocation of decoded datums.
	alloc *tree.DatumAlloc

	// Memory monitor and memory account for the bytes fetched by this fetcher.
	mon             *mon.BytesMonitor
	kvFetcherMemAcc *mon.BoundAccount
}

type arrayEqualsAnyFilterState struct {
	evalCtx     *tree.EvalContext
	colIdx      int
	left        tree.Datum
	materialize bool

	matched        bool
	sawNull        bool
	sawSubordinate bool
}

type jsonExistsFilterState struct {
	kind   JSONAccessKind
	shared *sharedJSONAccessProgramState
}

type jsonAccessProgramState struct {
	kind   JSONAccessKind
	shared *sharedJSONAccessProgramState
}

type jsonPathCompareFilterState struct {
	evalCtx *tree.EvalContext
	kind    JSONAccessKind
	mode    exec.JSONPathFilterMode
	right   tree.Datum
	shared  *sharedJSONAccessProgramState
}

type jsonContainsFilterState struct {
	colIdx      int
	materialize bool
	containedBy bool
	program     *JSONContainsProgram
	selected    *sharedJSONSelectedPathState
}

type sharedJSONAccessProgramState struct {
	key            string
	kind           JSONAccessKind
	colIdx         int
	materialize    bool
	sawSubordinate bool
	program        *JSONAccessProgram
	selected       *sharedJSONSelectedPathState
	existsKeys     []string
	cachedResult   tree.Datum
	haveCached     bool
}

type sharedJSONSelectedPathState struct {
	key         string
	colIdx      int
	encodedPath []string
	materialize bool
	selector    *JSONSelectedPathState
	builder     *SubordinateJSONBuilder
	cache       JSONSelectedPathResultCache
	access      []*sharedJSONAccessProgramState
	contains    []*jsonContainsFilterState
}

type subordinateArrayBuilder = SubordinateArrayBuilder

type subordinateJSONNodeKind = SubordinateJSONNodeKind

const (
	subordinateJSONNodeScalar = SubordinateJSONNodeScalar
	subordinateJSONNodeObject = SubordinateJSONNodeObject
	subordinateJSONNodeArray  = SubordinateJSONNodeArray
)

type subordinateJSONBuilder = SubordinateJSONBuilder

func newSubordinateArrayBuilder(elemType *types.T) *subordinateArrayBuilder {
	return NewSubordinateArrayBuilder(elemType)
}

func (rf *Fetcher) registerJSONAccessProgram(
	spec JSONAccessSpec,
) (*sharedJSONAccessProgramState, error) {
	key := SharedJSONAccessProgramKey(spec)
	for _, shared := range rf.jsonSharedAccessPrograms {
		if shared.key != key {
			continue
		}
		shared.materialize = shared.materialize || spec.Materialize
		return shared, nil
	}
	prog, err := NewJSONAccessProgram(spec)
	if err != nil {
		return nil, err
	}
	shared := &sharedJSONAccessProgramState{
		key:         key,
		kind:        spec.Kind,
		colIdx:      spec.ColIdx,
		materialize: spec.Materialize,
		program:     prog,
	}
	switch spec.Kind {
	case JSONAccessExists:
		shared.existsKeys = []string{spec.Key}
	case JSONAccessExistsAny, JSONAccessExistsAll:
		shared.existsKeys = append([]string(nil), spec.Keys...)
	}
	if spec.Kind == JSONAccessFetchJSONPath || spec.Kind == JSONAccessFetchTextPath {
		selected := rf.registerJSONSelectedPath(spec.ColIdx, spec.Path)
		selected.materialize = selected.materialize || spec.Materialize
		selected.access = append(selected.access, shared)
		shared.selected = selected
	}
	rf.jsonSharedAccessPrograms = append(rf.jsonSharedAccessPrograms, shared)
	if rf.jsonSharedAccessByCol == nil {
		rf.jsonSharedAccessByCol = make(map[int][]*sharedJSONAccessProgramState)
	}
	rf.jsonSharedAccessByCol[spec.ColIdx] = append(rf.jsonSharedAccessByCol[spec.ColIdx], shared)
	return shared, nil
}

func (rf *Fetcher) registerJSONSelectedPath(
	colIdx int, path []string,
) *sharedJSONSelectedPathState {
	key := SharedJSONSelectedPathKey(colIdx, path)
	for _, shared := range rf.jsonSharedSelectedPaths {
		if shared.key == key {
			return shared
		}
	}
	shared := &sharedJSONSelectedPathState{
		key:         key,
		colIdx:      colIdx,
		encodedPath: append([]string(nil), path...),
		selector:    NewJSONSelectedPathState(path),
	}
	rf.jsonSharedSelectedPaths = append(rf.jsonSharedSelectedPaths, shared)
	if rf.jsonSharedSelectedByCol == nil {
		rf.jsonSharedSelectedByCol = make(map[int][]*sharedJSONSelectedPathState)
	}
	rf.jsonSharedSelectedByCol[colIdx] = append(rf.jsonSharedSelectedByCol[colIdx], shared)
	return shared
}

func (s *sharedJSONAccessProgramState) reset() {
	s.sawSubordinate = false
	s.program.Reset()
	if s.selected != nil {
		return
	}
	s.cachedResult = nil
	s.haveCached = false
}

func (s *sharedJSONAccessProgramState) observe(
	path []keys.SubordinatePathSegment,
	kind rowenc.SubordinateJSONNodeKind,
	childCount int,
	j jsonutil.JSON,
) error {
	s.sawSubordinate = true
	return s.program.Observe(path, kind, childCount, j)
}

func (s *sharedJSONAccessProgramState) resultDatum(kind JSONAccessKind) (tree.Datum, error) {
	if s.selected != nil {
		return s.selected.resultDatum(kind)
	}
	if s.haveCached {
		return s.cachedResult, nil
	}
	d, err := s.program.ResultDatumForKind(kind)
	if err != nil {
		return nil, err
	}
	s.cachedResult = d
	s.haveCached = true
	return d, nil
}

func (s *sharedJSONSelectedPathState) reset() {
	s.selector.Reset()
	s.builder = nil
	s.cache.Reset()
}

func (s *sharedJSONSelectedPathState) observe(
	path []keys.SubordinatePathSegment,
	kind rowenc.SubordinateJSONNodeKind,
	childCount int,
	j jsonutil.JSON,
) error {
	relPath, ok, err := s.selector.Select(path, kind, childCount, j)
	if err != nil || !ok {
		return err
	}
	return s.observeSelected(relPath, kind, childCount, j)
}

func (s *sharedJSONSelectedPathState) selectPath(
	path []keys.SubordinatePathSegment, kind rowenc.SubordinateJSONNodeKind, childCount int,
) ([]keys.SubordinatePathSegment, bool, error) {
	return s.selector.SelectPath(path, kind, childCount)
}

func (s *sharedJSONSelectedPathState) observeSelected(
	relPath []keys.SubordinatePathSegment,
	kind rowenc.SubordinateJSONNodeKind,
	childCount int,
	j jsonutil.JSON,
) error {
	if s.materialize || len(s.access) > 0 {
		nodeKind, err := SubordinateJSONNodeKindFromEncoded(kind)
		if err != nil {
			return err
		}
		if s.builder == nil {
			s.builder = &SubordinateJSONBuilder{}
		}
		if err := s.builder.Set(relPath, nodeKind, j); err != nil {
			return err
		}
	}
	for _, access := range s.access {
		access.sawSubordinate = true
	}
	if !s.materialize && len(s.access) == 0 {
		for _, contains := range s.contains {
			if err := contains.program.ObserveSelected(relPath, kind, childCount, j); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *sharedJSONSelectedPathState) resultDatum(kind JSONAccessKind) (tree.Datum, error) {
	return s.cache.ResultDatum(s.builder, kind)
}

func (s *sharedJSONSelectedPathState) containsResult(program *JSONContainsProgram) (bool, error) {
	return s.cache.ContainsResult(s.builder, program)
}

// Reset resets this Fetcher, preserving the memory capacity that was used
// for the tables slice, and the slices within each of the tableInfo objects
// within tables. This permits reuse of this objects without forcing total
// reallocation of all of those slice fields.
func (rf *Fetcher) Reset() {
	*rf = Fetcher{
		table: rf.table,
	}
}

// ConfigureArrayEqualsAnyFilter enables scan-local evaluation of
//
//	left = ANY(array_col)
//
// for the fetched column at colIdx.
func (rf *Fetcher) ConfigureArrayEqualsAnyFilter(
	evalCtx *tree.EvalContext, colIdx int, left tree.Datum, materialize bool,
) {
	rf.arrayEqualsAnyFilter = &arrayEqualsAnyFilterState{
		evalCtx:     evalCtx,
		colIdx:      colIdx,
		left:        left,
		materialize: materialize,
	}
	rf.lastRowPassesArrayEqualsAnyFilter = true
}

// ConfigureJSONExistsFilter enables scan-local evaluation of JSON top-level key
// predicates for the fetched column at colIdx.
func (rf *Fetcher) ConfigureJSONExistsFilter(
	colIdx int, kind JSONAccessKind, key string, keys []string, materialize bool,
) error {
	shared, err := rf.registerJSONAccessProgram(JSONAccessSpec{
		ColIdx:      colIdx,
		Kind:        kind,
		Key:         key,
		Keys:        keys,
		Materialize: materialize,
	})
	if err != nil {
		return err
	}
	rf.jsonExistsFilter = &jsonExistsFilterState{
		kind:   kind,
		shared: shared,
	}
	rf.lastRowPassesJSONExistsFilter = true
	return nil
}

// ConfigureJSONPathCompareFilter enables scan-local evaluation of comparison
// predicates over JSON path accesses on the fetched JSON column at colIdx.
func (rf *Fetcher) ConfigureJSONPathCompareFilter(
	evalCtx *tree.EvalContext,
	colIdx int,
	kind JSONAccessKind,
	path []string,
	mode exec.JSONPathFilterMode,
	right tree.Datum,
	materialize bool,
) error {
	shared, err := rf.registerJSONAccessProgram(JSONAccessSpec{
		ColIdx:      colIdx,
		Kind:        kind,
		Path:        path,
		Materialize: materialize,
	})
	if err != nil {
		return err
	}
	rf.jsonPathCompareFilter = &jsonPathCompareFilterState{
		evalCtx: evalCtx,
		kind:    kind,
		mode:    mode,
		right:   right,
		shared:  shared,
	}
	rf.lastRowPassesJSONPathCompareFilter = true
	return nil
}

// ConfigureJSONContainsFilter enables scan-local evaluation of JSON
// containment predicates over a fetched JSON column or JSON path access.
func (rf *Fetcher) ConfigureJSONContainsFilter(
	colIdx int, path []string, containedBy bool, right jsonutil.JSON, materialize bool,
) error {
	prog, err := NewJSONContainsProgram(path, right, containedBy)
	if err != nil {
		return err
	}
	filter := &jsonContainsFilterState{
		colIdx:      colIdx,
		materialize: materialize,
		containedBy: containedBy,
		program:     prog,
	}
	if rf.jsonContainsByCol == nil {
		rf.jsonContainsByCol = make(map[int][]*jsonContainsFilterState)
	}
	selected := rf.registerJSONSelectedPath(colIdx, path)
	selected.materialize = selected.materialize || materialize
	selected.contains = append(selected.contains, filter)
	filter.selected = selected
	rf.jsonContainsFilters = append(rf.jsonContainsFilters, filter)
	rf.jsonContainsByCol[colIdx] = append(rf.jsonContainsByCol[colIdx], filter)
	rf.lastRowPassesJSONContainsFilter = true
	return nil
}

// ConfigureJSONAccessPrograms enables scan-local JSON access evaluation for
// the fetched JSON columns referenced by specs.
func (rf *Fetcher) ConfigureJSONAccessPrograms(specs []JSONAccessSpec) error {
	rf.jsonAccessPrograms = rf.jsonAccessPrograms[:0]
	rf.lastRowJSONAccessProgramResults = rf.lastRowJSONAccessProgramResults[:0]
	if len(specs) == 0 {
		return nil
	}
	for _, spec := range specs {
		shared, err := rf.registerJSONAccessProgram(spec)
		if err != nil {
			return err
		}
		rf.jsonAccessPrograms = append(rf.jsonAccessPrograms, &jsonAccessProgramState{
			kind:   spec.Kind,
			shared: shared,
		})
		rf.lastRowJSONAccessProgramResults = append(rf.lastRowJSONAccessProgramResults, nil)
	}
	return nil
}

// RowPassesArrayEqualsAnyFilter reports whether the most recently returned row
// satisfied the configured scan-local array filter. It returns true when no
// such filter is configured.
func (rf *Fetcher) RowPassesArrayEqualsAnyFilter() bool {
	return rf.lastRowPassesArrayEqualsAnyFilter
}

// RowPassesJSONExistsFilter reports whether the most recently returned row
// satisfied the configured scan-local JSON filter. It returns true when no
// such filter is configured.
func (rf *Fetcher) RowPassesJSONExistsFilter() bool {
	return rf.lastRowPassesJSONExistsFilter
}

// RowPassesJSONPathCompareFilter reports whether the most recently returned row
// satisfied the configured scan-local JSON path compare filter. It returns true
// when no such filter is configured.
func (rf *Fetcher) RowPassesJSONPathCompareFilter() bool {
	return rf.lastRowPassesJSONPathCompareFilter
}

// RowPassesJSONContainsFilter reports whether the most recently returned row
// satisfied the configured scan-local JSON containment filter. It returns true
// when no such filter is configured.
func (rf *Fetcher) RowPassesJSONContainsFilter() bool {
	return rf.lastRowPassesJSONContainsFilter
}

// JSONAccessProgramResults reports finalized per-row results for the
// configured scan-local JSON access programs in configuration order.
func (rf *Fetcher) JSONAccessProgramResults() []tree.Datum {
	return rf.lastRowJSONAccessProgramResults
}

// Close releases resources held by this fetcher.
func (rf *Fetcher) Close(ctx context.Context) {
	if rf.kvFetcher != nil {
		rf.kvFetcher.Close(ctx)
	}
	if rf.mon != nil {
		rf.kvFetcherMemAcc.Close(ctx)
		rf.mon.Stop(ctx)
	}
}

// Init sets up a Fetcher for a given table and index. If we are using a
// non-primary index, tables.ValNeededForCol can only refer to columns in the
// index.
func (rf *Fetcher) Init(
	ctx context.Context,
	reverse bool,
	lockStrength descpb.ScanLockingStrength,
	lockWaitPolicy descpb.ScanLockingWaitPolicy,
	lockTimeout time.Duration,
	alloc *tree.DatumAlloc,
	memMonitor *mon.BytesMonitor,
	spec *descpb.IndexFetchSpec,
) error {
	if spec.Version != descpb.IndexFetchSpecVersionInitial {
		return errors.Newf("unsupported IndexFetchSpec version %d", spec.Version)
	}
	rf.reverse = reverse
	rf.lockStrength = lockStrength
	rf.lockWaitPolicy = lockWaitPolicy
	rf.lockTimeout = lockTimeout
	rf.alloc = alloc

	if memMonitor != nil {
		rf.mon = mon.NewMonitorInheritWithLimit("fetcher-mem", 0 /* limit */, memMonitor)
		rf.mon.Start(ctx, memMonitor, mon.BoundAccount{})
		memAcc := rf.mon.MakeBoundAccount()
		rf.kvFetcherMemAcc = &memAcc
	}

	table := &rf.table
	*table = tableInfo{
		spec:       *spec,
		row:        make(rowenc.EncDatumRow, len(spec.FetchedColumns)),
		decodedRow: make(tree.Datums, len(spec.FetchedColumns)),

		// These slice fields might get re-allocated below, so reslice them from
		// the old table here in case they've got enough capacity already.
		indexColIdx:        rf.table.indexColIdx[:0],
		keyVals:            rf.table.keyVals[:0],
		extraVals:          rf.table.extraVals[:0],
		timestampOutputIdx: noOutputColumn,
		oidOutputIdx:       noOutputColumn,
	}

	for idx := range spec.FetchedColumns {
		colID := spec.FetchedColumns[idx].ColumnID
		table.colIdxMap.Set(colID, idx)
		if colinfo.IsColIDSystemColumn(colID) {
			switch colinfo.GetSystemColumnKindFromColumnID(colID) {
			case catpb.SystemColumnKind_MVCCTIMESTAMP:
				table.timestampOutputIdx = idx
				rf.mvccDecodeStrategy = MVCCDecodingRequired

			case catpb.SystemColumnKind_TABLEOID:
				table.oidOutputIdx = idx
				table.tableOid = tree.NewDOid(tree.DInt(spec.TableID))
			}
		}
	}

	if len(spec.FetchedColumns) > 0 {
		table.neededValueColsByIdx.AddRange(0, len(spec.FetchedColumns)-1)
	}

	nExtraCols := 0
	// Unique secondary indexes have extra columns to decode from the value (namely
	// the primary index columns).
	if table.spec.IsSecondaryIndex && table.spec.IsUniqueIndex {
		nExtraCols = int(table.spec.NumKeySuffixColumns)
	}
	nIndexCols := len(spec.KeyAndSuffixColumns) - nExtraCols

	neededIndexCols := 0
	compositeIndexCols := 0
	if cap(table.indexColIdx) >= nIndexCols {
		table.indexColIdx = table.indexColIdx[:nIndexCols]
	} else {
		table.indexColIdx = make([]int, nIndexCols)
	}
	for i := 0; i < nIndexCols; i++ {
		id := spec.KeyAndSuffixColumns[i].ColumnID
		colIdx, ok := table.colIdxMap.Get(id)
		if ok {
			table.indexColIdx[i] = colIdx
			neededIndexCols++
			table.neededValueColsByIdx.Remove(colIdx)
		} else {
			table.indexColIdx[i] = -1
		}
		if spec.KeyAndSuffixColumns[i].IsComposite {
			compositeIndexCols++
		}
	}

	// If there are needed columns from the index key, we need to read it;
	// otherwise, we can completely avoid decoding the index key.
	rf.mustDecodeIndexKey = neededIndexCols > 0

	// The number of columns we need to read from the value part of the key.
	// It's the total number of needed columns minus the ones we read from the
	// index key, except for composite columns.
	table.neededValueCols = len(spec.FetchedColumns) - neededIndexCols + compositeIndexCols

	if cap(table.keyVals) >= nIndexCols {
		table.keyVals = table.keyVals[:nIndexCols]
	} else {
		table.keyVals = make([]rowenc.EncDatum, nIndexCols)
	}

	if nExtraCols > 0 {
		// Unique secondary indexes have a value that is the
		// primary index key.
		// Primary indexes only contain ascendingly-encoded
		// values. If this ever changes, we'll probably have to
		// figure out the directions here too.
		if cap(table.extraVals) >= nExtraCols {
			table.extraVals = table.extraVals[:nExtraCols]
		} else {
			table.extraVals = make([]rowenc.EncDatum, nExtraCols)
		}
	}

	// Check for columns that use subordinate key encoding.
	if spec.MaxKeysPerRow == 0 {
		rf.hasSubordinateColumns = true
	}
	for i := range spec.FetchedColumns {
		switch spec.FetchedColumns[i].Type.Family() {
		case types.ArrayFamily, types.JsonFamily:
			rf.hasSubordinateColumns = true
			break
		}
	}

	return nil
}

// StartScan initializes and starts the key-value scan. Can be used multiple
// times.
//
// batchBytesLimit controls whether bytes limits are placed on the batches. If
// set, bytes limits will be used to protect against running out of memory (on
// both this client node, and on the server).
//
// If batchBytesLimit is set, rowLimitHint can also be set to control the number of
// rows that will be scanned by the first batch. If set, subsequent batches (if
// any) will have progressively higher limits (up to a fixed max). The idea with
// row limits is to make the execution of LIMIT queries efficient: if the caller
// has some idea about how many rows need to be read to ultimately satisfy the
// query, the Fetcher uses it. Even if this hint proves insufficient, the
// Fetcher continues to set row limits (in addition to bytes limits) on the
// argument that some number of rows will eventually satisfy the query and we
// likely don't need to scan `spans` fully. The bytes limit, on the other hand,
// is simply intended to protect against OOMs.
func (rf *Fetcher) StartScan(
	ctx context.Context,
	txn *kv.Txn,
	spans roachpb.Spans,
	batchBytesLimit rowinfra.BytesLimit,
	rowLimitHint rowinfra.RowLimit,
	traceKV bool,
	forceProductionKVBatchSize bool,
) error {
	if len(spans) == 0 {
		return errors.AssertionFailedf("no spans")
	}

	if lookups, ok, err := rf.subordinateJSONRowHeadLookupSpecs(); err != nil {
		return err
	} else if ok {
		f, err := NewSubordinateJSONRowHeadKVFetcher(
			txn,
			spans,
			rf.reverse,
			rf.lockStrength,
			rf.lockWaitPolicy,
			rf.lockTimeout,
			lookups,
		)
		if err != nil {
			return err
		}
		return rf.StartScanFrom(ctx, f, traceKV)
	}

	f, err := makeKVBatchFetcher(
		ctx,
		makeKVBatchFetcherDefaultSendFunc(txn),
		spans,
		rf.reverse,
		batchBytesLimit,
		rf.rowLimitToKeyLimit(rowLimitHint),
		rf.lockStrength,
		rf.lockWaitPolicy,
		rf.lockTimeout,
		rf.kvFetcherMemAcc,
		forceProductionKVBatchSize,
		txn.AdmissionHeader(),
		txn.DB().SQLKVResponseAdmissionQ,
	)
	if err != nil {
		return err
	}
	return rf.StartScanFrom(ctx, &f, traceKV)
}

func (rf *Fetcher) subordinateJSONRowHeadLookupSpecs() ([]SubordinateJSONRowLookupSpec, bool, error) {
	if !rf.hasSubordinateColumns {
		return nil, false, nil
	}
	type lookupPaths struct {
		paths      [][]keys.SubordinatePathSegment
		existsKeys []string
	}
	byCol := make(map[int]*lookupPaths)
	for i := range rf.jsonSharedSelectedPaths {
		selected := rf.jsonSharedSelectedPaths[i]
		prefix, ok, err := LongestStaticSubordinateJSONPathPrefix(selected.encodedPath)
		if err != nil {
			return nil, false, err
		}
		if !ok || len(prefix) == 0 {
			return nil, false, nil
		}
		entry := byCol[selected.colIdx]
		if entry == nil {
			entry = &lookupPaths{}
			byCol[selected.colIdx] = entry
		}
		entry.paths = append(entry.paths, prefix)
	}
	for i := range rf.jsonSharedAccessPrograms {
		shared := rf.jsonSharedAccessPrograms[i]
		if shared.selected != nil {
			continue
		}
		switch shared.kind {
		case JSONAccessExists, JSONAccessExistsAny, JSONAccessExistsAll:
		default:
			return nil, false, nil
		}
		entry := byCol[shared.colIdx]
		if entry == nil {
			entry = &lookupPaths{}
			byCol[shared.colIdx] = entry
		}
		entry.existsKeys = append(entry.existsKeys, shared.existsKeys...)
	}
	for i := range rf.jsonSharedAccessPrograms {
		if rf.jsonSharedAccessPrograms[i].materialize {
			return nil, false, nil
		}
	}
	for i := range rf.jsonSharedSelectedPaths {
		if rf.jsonSharedSelectedPaths[i].materialize {
			return nil, false, nil
		}
	}
	if len(byCol) == 0 {
		supported := true
		rf.table.neededValueColsByIdx.ForEach(func(colIdx int) {
			if !supported {
				return
			}
			if colIdx < 0 || colIdx >= len(rf.table.spec.FetchedColumns) {
				supported = false
				return
			}
			switch rf.table.spec.FetchedColumns[colIdx].Type.Family() {
			case types.ArrayFamily, types.JsonFamily:
				supported = false
			}
		})
		if !supported {
			return nil, false, nil
		}
		return nil, true, nil
	}
	if rf.table.spec.MaxKeysPerRow > 1 {
		if len(rf.jsonAccessPrograms) > 0 || len(rf.jsonContainsFilters) > 0 {
			return nil, false, nil
		}
		supported := true
		rf.table.neededValueColsByIdx.ForEach(func(colIdx int) {
			if !supported {
				return
			}
			if colIdx < 0 || colIdx >= len(rf.table.spec.FetchedColumns) {
				supported = false
				return
			}
			if rf.table.spec.FetchedColumns[colIdx].Type.Family() != types.JsonFamily {
				supported = false
				return
			}
			if _, ok := byCol[colIdx]; !ok {
				supported = false
			}
		})
		if !supported {
			return nil, false, nil
		}
	}
	lookups := make([]SubordinateJSONRowLookupSpec, 0, len(byCol))
	for colIdx, entry := range byCol {
		sort.Strings(entry.existsKeys)
		entry.existsKeys = slices.Compact(entry.existsKeys)
		lookups = append(lookups, SubordinateJSONRowLookupSpec{
			ColID:         rf.table.spec.FetchedColumns[colIdx].ColumnID,
			SelectedPaths: entry.paths,
			ExistsKeys:    entry.existsKeys,
		})
	}
	return lookups, true, nil
}

// TestingInconsistentScanSleep introduces a sleep inside the fetcher after
// every KV batch (for inconsistent scans, currently used only for table
// statistics collection).
// TODO(radu): consolidate with forceProductionKVBatchSize into a
// FetcherTestingKnobs struct.
var TestingInconsistentScanSleep time.Duration

// StartInconsistentScan initializes and starts an inconsistent scan, where each
// KV batch can be read at a different historical timestamp.
//
// The scan uses the initial timestamp, until it becomes older than
// maxTimestampAge; at this time the timestamp is bumped by the amount of time
// that has passed. See the documentation for TableReaderSpec for more
// details.
//
// Can be used multiple times.
func (rf *Fetcher) StartInconsistentScan(
	ctx context.Context,
	db *kv.DB,
	initialTimestamp hlc.Timestamp,
	maxTimestampAge time.Duration,
	spans roachpb.Spans,
	batchBytesLimit rowinfra.BytesLimit,
	rowLimitHint rowinfra.RowLimit,
	traceKV bool,
	forceProductionKVBatchSize bool,
	qualityOfService sessiondatapb.QoSLevel,
) error {
	if len(spans) == 0 {
		return errors.AssertionFailedf("no spans")
	}

	txnTimestamp := initialTimestamp
	txnStartTime := timeutil.Now()
	if txnStartTime.Sub(txnTimestamp.GoTime()) >= maxTimestampAge {
		return errors.Errorf(
			"AS OF SYSTEM TIME: cannot specify timestamp older than %s for this operation",
			maxTimestampAge,
		)
	}
	txn := newTxnWithSteppingEnabled(ctx, db, 0 /* gatewayNodeID */, qualityOfService)
	if err := txn.SetFixedTimestamp(ctx, txnTimestamp); err != nil {
		return err
	}
	if log.V(1) {
		log.Infof(ctx, "starting inconsistent scan at timestamp %v", txnTimestamp)
	}

	sendFn := func(ctx context.Context, ba roachpb.BatchRequest) (*roachpb.BatchResponse, error) {
		if now := timeutil.Now(); now.Sub(txnTimestamp.GoTime()) >= maxTimestampAge {
			// Time to bump the transaction. First commit the old one (should be a no-op).
			if err := txn.Commit(ctx); err != nil {
				return nil, err
			}
			// Advance the timestamp by the time that passed.
			txnTimestamp = txnTimestamp.Add(now.Sub(txnStartTime).Nanoseconds(), 0 /* logical */)
			txnStartTime = now
			txn = newTxnWithSteppingEnabled(ctx, db, 0 /* gatewayNodeID */, qualityOfService)
			if err := txn.SetFixedTimestamp(ctx, txnTimestamp); err != nil {
				return nil, err
			}

			if log.V(1) {
				log.Infof(ctx, "bumped inconsistent scan timestamp to %v", txnTimestamp)
			}
		}

		res, err := txn.Send(ctx, ba)
		if err != nil {
			return nil, err.GoError()
		}
		if TestingInconsistentScanSleep != 0 {
			time.Sleep(TestingInconsistentScanSleep)
		}
		return res, nil
	}

	// TODO(radu): we should commit the last txn. Right now the commit is a no-op
	// on read transactions, but perhaps one day it will release some resources.

	f, err := makeKVBatchFetcher(
		ctx,
		sendFunc(sendFn),
		spans,
		rf.reverse,
		batchBytesLimit,
		rf.rowLimitToKeyLimit(rowLimitHint),
		rf.lockStrength,
		rf.lockWaitPolicy,
		rf.lockTimeout,
		rf.kvFetcherMemAcc,
		forceProductionKVBatchSize,
		txn.AdmissionHeader(),
		txn.DB().SQLKVResponseAdmissionQ,
	)
	if err != nil {
		return err
	}
	return rf.StartScanFrom(ctx, &f, traceKV)
}

func (rf *Fetcher) rowLimitToKeyLimit(rowLimitHint rowinfra.RowLimit) rowinfra.KeyLimit {
	if rowLimitHint == 0 {
		return 0
	}
	if rf.hasSubordinateColumns || rf.table.spec.MaxKeysPerRow == 0 {
		// Subordinate keys make the number of KVs per row unbounded, so
		// a KV limit derived from row counts can truncate a row mid-scan.
		return 0
	}
	// If we have a limit hint, we limit the first batch size. Subsequent
	// batches get larger to avoid making things too slow (e.g. in case we have
	// a very restrictive filter and actually have to retrieve a lot of rows).
	// The rowLimitHint is a row limit, but each row could be made up of more than
	// one key. We take the maximum possible keys per row out of all the table
	// rows we could potentially scan over.
	//
	// We add an extra key to make sure we form the last row.
	return rowinfra.KeyLimit(int64(rowLimitHint)*int64(rf.table.spec.MaxKeysPerRow) + 1)
}

// StartScanFrom initializes and starts a scan from the given KVBatchFetcher. Can be
// used multiple times.
func (rf *Fetcher) StartScanFrom(ctx context.Context, f KVBatchFetcher, traceKV bool) error {
	rf.traceKV = traceKV
	rf.indexKey = nil
	if rf.kvFetcher != nil {
		rf.kvFetcher.Close(ctx)
	}
	rf.kvFetcher = newKVFetcher(f)
	rf.kvEnd = false
	// Retrieve the first key.
	_, err := rf.nextKey(ctx)
	return err
}

// setNextKV sets the next KV to process to the input KV. needsCopy, if true,
// causes the input kv to be deep copied. needsCopy should be set to true if
// the input KV is pointing to the last KV of a batch, so that the batch can
// be garbage collected before fetching the next one.
// gcassert:inline
func (rf *Fetcher) setNextKV(kv roachpb.KeyValue, needsCopy bool) {
	if !needsCopy {
		rf.kv = kv
		return
	}

	// If we've made it to the very last key in the batch, copy out the key
	// so that the GC can reclaim the large backing slice before we call
	// NextKV() again.
	kvCopy := roachpb.KeyValue{}
	kvCopy.Key = make(roachpb.Key, len(kv.Key))
	copy(kvCopy.Key, kv.Key)
	kvCopy.Value.RawBytes = make([]byte, len(kv.Value.RawBytes))
	copy(kvCopy.Value.RawBytes, kv.Value.RawBytes)
	kvCopy.Value.Timestamp = kv.Value.Timestamp
	rf.kv = kvCopy
}

// nextKey retrieves the next key/value and sets kv/kvEnd. In the single-family
// layout every KV begins a new row.
func (rf *Fetcher) nextKey(ctx context.Context) (newRow bool, _ error) {
	ok, kv, finalReferenceToBatch, err := rf.kvFetcher.NextKV(ctx, rf.mvccDecodeStrategy)
	if err != nil {
		return false, ConvertFetchError(&rf.table.spec, err)
	}
	rf.setNextKV(kv, finalReferenceToBatch)

	if !ok {
		// No more keys in the scan.
		rf.kvEnd = true
		return true, nil
	}
	// unchangedPrefix will be set to true if the current KV belongs to the same
	// row as the previous KV (i.e. the last and current keys have identical
	// prefix). In this case, we can skip decoding the index key completely.
	// Check if the current key shares the row prefix. For single-row-group
	// tables with subordinate columns, subordinate keys produce additional KVs under the
	// same row prefix that also need grouping.
	unchangedPrefix := (rf.table.spec.MaxKeysPerRow != 1 || rf.hasSubordinateColumns) && rf.indexKey != nil && bytes.HasPrefix(rf.kv.Key, rf.indexKey)
	if unchangedPrefix {
		// Skip decoding!
		rf.keyRemainingBytes = rf.kv.Key[len(rf.indexKey):]
		return false, nil
	}

	// The current key belongs to a new row.
	if rf.mustDecodeIndexKey {
		rf.keyRemainingBytes, _, err = rf.DecodeIndexKey(rf.kv.Key)
		if err != nil {
			return false, err
		}
	} else {
		// Consume the key suffix after the row prefix before decoding the value.
		prefixLen, err := keys.GetRowPrefixLength(rf.kv.Key)
		if err != nil {
			return false, err
		}

		rf.keyRemainingBytes = rf.kv.Key[prefixLen:]
	}

	rf.indexKey = nil
	return true, nil
}

func (rf *Fetcher) prettyKeyDatums(
	cols []descpb.IndexFetchSpec_KeyColumn, vals []rowenc.EncDatum,
) string {
	var buf strings.Builder
	for i, v := range vals {
		buf.WriteByte('/')
		if err := v.EnsureDecoded(cols[i].Type, rf.alloc); err != nil {
			buf.WriteByte('?')
		} else {
			buf.WriteString(v.Datum.String())
		}
	}
	return buf.String()
}

// DecodeIndexKey decodes an index key and returns the remaining key and whether
// it encountered a null while decoding.
func (rf *Fetcher) DecodeIndexKey(key roachpb.Key) (remaining []byte, foundNull bool, err error) {
	key = key[rf.table.spec.KeyPrefixLength:]
	return rowenc.DecodeKeyValsUsingSpec(rf.table.spec.KeyAndSuffixColumns, key, rf.table.keyVals)
}

// processKV processes the given key/value, setting values in the row
// accordingly. If debugStrings is true, returns pretty printed key and value
// information in prettyKey/prettyValue (otherwise they are empty strings).
func (rf *Fetcher) processKV(
	ctx context.Context, kv roachpb.KeyValue,
) (prettyKey string, prettyValue string, err error) {
	table := &rf.table

	if rf.traceKV {
		prettyKey = fmt.Sprintf(
			"/%s/%s%s",
			table.spec.TableName,
			table.spec.IndexName,
			rf.prettyKeyDatums(table.spec.KeyAndSuffixColumns, table.keyVals),
		)
	}

	// Either this is the first key of the fetch or the first key of a new
	// row.
	if rf.indexKey == nil {
		// This is the first key for the row.
		rf.indexKey = []byte(kv.Key[:len(kv.Key)-len(rf.keyRemainingBytes)])

		// Reset the row to nil; it will get filled in with the column
		// values as we decode the key-value pairs for the row.
		// We only need to reset the needed columns in the value component, because
		// non-needed columns are never set and key columns are unconditionally set
		// below.
		for idx, ok := table.neededValueColsByIdx.Next(0); ok; idx, ok = table.neededValueColsByIdx.Next(idx + 1) {
			table.row[idx].UnsetDatum()
		}
		// Clear subordinate array accumulators from the previous row.
		for k := range rf.subordinateArrays {
			delete(rf.subordinateArrays, k)
		}
		for k := range rf.subordinateJSONBuilders {
			delete(rf.subordinateJSONBuilders, k)
		}
		if rf.arrayEqualsAnyFilter != nil {
			rf.arrayEqualsAnyFilter.matched = false
			rf.arrayEqualsAnyFilter.sawNull = false
			rf.arrayEqualsAnyFilter.sawSubordinate = false
			rf.lastRowPassesArrayEqualsAnyFilter = true
		}
		for i := range rf.jsonSharedAccessPrograms {
			rf.jsonSharedAccessPrograms[i].reset()
		}
		for i := range rf.jsonSharedSelectedPaths {
			rf.jsonSharedSelectedPaths[i].reset()
		}
		if rf.jsonExistsFilter != nil {
			rf.lastRowPassesJSONExistsFilter = true
		}
		if rf.jsonPathCompareFilter != nil {
			rf.lastRowPassesJSONPathCompareFilter = true
		}
		for i := range rf.jsonContainsFilters {
			rf.jsonContainsFilters[i].program.Reset()
		}
		if len(rf.jsonContainsFilters) > 0 {
			rf.lastRowPassesJSONContainsFilter = true
		}
		for i := range rf.jsonAccessPrograms {
			rf.lastRowJSONAccessProgramResults[i] = nil
		}

		// Fill in the column values that are part of the index key.
		for i := range table.keyVals {
			if idx := table.indexColIdx[i]; idx != -1 {
				table.row[idx] = table.keyVals[i]
			}
		}

		rf.valueColsFound = 0

		// Reset the MVCC metadata for the next row.

		// set rowLastModified to a sentinel that's before any real timestamp.
		// As kvs are iterated for this row, it keeps track of the greatest
		// timestamp seen.
		table.rowLastModified = hlc.Timestamp{}
		// A row is present if its primary-family KV is present, even if the row
		// is otherwise all NULLs. Thus, a row is deleted if and only if the first
		// KV in it is a tombstone (RawBytes is empty).
		table.rowIsDeleted = len(kv.Value.RawBytes) == 0
	}

	if table.rowLastModified.Less(kv.Value.Timestamp) {
		table.rowLastModified = kv.Value.Timestamp
	}

	if len(table.spec.FetchedColumns) == 0 {
		// We don't need to decode any values.
		if rf.traceKV {
			prettyValue = "<undecoded>"
		}
		return prettyKey, prettyValue, nil
	}

	// For covering secondary indexes, allow for decoding as a primary key.
	if table.spec.EncodingType == descpb.PrimaryIndexEncoding &&
		len(rf.keyRemainingBytes) > 0 {
		// kv.Value contains values for composite key columns. These columns
		// already have a table.row value assigned above, but that value
		// (obtained from the key encoding) might not be correct (e.g. for
		// decimals, it might not contain the right number of trailing 0s; for
		// collated strings, it is one of potentially many strings with the same
		// collation key).
		//
		// In these cases, the correct value is present in the row value and the
		// table.row value gets overwritten.

		// Check for subordinate keys before inspecting the value tag.
		{
			var remaining []byte
			var famID uint64
			remaining, famID, err = encoding.DecodeUvarintAscending(rf.keyRemainingBytes)
			if err == nil && famID == 0 && len(remaining) > 0 {
				prettyKey, prettyValue, err = rf.processSubordinateKV(ctx, table, kv, remaining, prettyKey)
				if err != nil {
					return "", "", scrub.WrapError(scrub.IndexValueDecodingError, err)
				}
				return prettyKey, prettyValue, nil
			}
		}

		switch kv.Value.GetTag() {
		case roachpb.ValueType_TUPLE:
			// The ValueType_TUPLE encoding includes the column ID with every
			// encoded column value.
			var tupleBytes []byte
			tupleBytes, err = kv.Value.GetTuple()
			if err != nil {
				break
			}
			prettyKey, prettyValue, err = rf.processValueBytes(ctx, table, kv, tupleBytes, prettyKey)
		default:
			if table.spec.DefaultColumnID != 0 {
				prettyKey, prettyValue, err = rf.processValueSingle(ctx, table, table.spec.DefaultColumnID, kv, prettyKey)
			}
		}
		if err != nil {
			return "", "", scrub.WrapError(scrub.IndexValueDecodingError, err)
		}
	} else {
		tag := kv.Value.GetTag()
		var valueBytes []byte
		switch tag {
		case roachpb.ValueType_BYTES:
			// Secondary index ValueType_BYTES values store extra primary key
			// columns when they are present, so decode them here.
			valueBytes, err = kv.Value.GetBytes()
			if err != nil {
				return "", "", scrub.WrapError(scrub.IndexValueDecodingError, err)
			}
			if len(table.extraVals) > 0 {
				extraCols := table.spec.KeySuffixColumns()
				// This is a unique secondary index; decode the extra
				// column values from the value.
				var err error
				valueBytes, _, err = rowenc.DecodeKeyValsUsingSpec(
					extraCols,
					valueBytes,
					table.extraVals,
				)
				if err != nil {
					return "", "", scrub.WrapError(scrub.SecondaryIndexKeyExtraValueDecodingError, err)
				}
				for i := range extraCols {
					if idx, ok := table.colIdxMap.Get(extraCols[i].ColumnID); ok {
						table.row[idx] = table.extraVals[i]
					}
				}
				if rf.traceKV {
					prettyValue = rf.prettyKeyDatums(extraCols, table.extraVals)
				}
			}
		case roachpb.ValueType_TUPLE:
			valueBytes, err = kv.Value.GetTuple()
			if err != nil {
				return "", "", scrub.WrapError(scrub.IndexValueDecodingError, err)
			}
		}

		if len(valueBytes) > 0 {
			prettyKey, prettyValue, err = rf.processValueBytes(
				ctx, table, kv, valueBytes, prettyKey,
			)
			if err != nil {
				return "", "", scrub.WrapError(scrub.IndexValueDecodingError, err)
			}
		}
	}

	if rf.traceKV && prettyValue == "" {
		prettyValue = "<undecoded>"
	}

	return prettyKey, prettyValue, nil
}

// processValueSingle processes the given value (of column colID), setting
// values in table.row accordingly. The key is only used for logging.
func (rf *Fetcher) processValueSingle(
	ctx context.Context,
	table *tableInfo,
	colID descpb.ColumnID,
	kv roachpb.KeyValue,
	prettyKeyPrefix string,
) (prettyKey string, prettyValue string, err error) {
	prettyKey = prettyKeyPrefix
	idx, ok := table.colIdxMap.Get(colID)
	if !ok {
		// No need to unmarshal the column value. Either the column was part of
		// the index key or it isn't needed.
		if DebugRowFetch {
			log.Infof(ctx, "Scan %s -> [%d] (skipped)", kv.Key, colID)
		}
		return prettyKey, "", nil
	}

	if rf.traceKV {
		prettyKey = fmt.Sprintf("%s/%s", prettyKey, table.spec.FetchedColumns[idx].Name)
	}
	if len(kv.Value.RawBytes) == 0 {
		return prettyKey, "", nil
	}
	typ := table.spec.FetchedColumns[idx].Type
	// Array and JSON columns must use subordinate key encoding. If we find one
	// stored as a single family value, the data was written by an incompatible
	// version.
	if typ.Family() == types.ArrayFamily || typ.Family() == types.JsonFamily {
		return "", "", errors.AssertionFailedf(
			"column %q (id=%d) has subordinate-encoded type stored as single-column family value; "+
				"this data was written by an incompatible CockroachDB version "+
				"that does not use subordinate key encoding for %s",
			table.spec.FetchedColumns[idx].Name, colID,
			typ.Family(),
		)
	}
	// TODO(arjun): The value is a directly marshaled single value, so we
	// unmarshal it eagerly here. This can potentially be optimized out,
	// although that would require changing UnmarshalColumnValue to operate
	// on bytes, and for Encode/DecodeTableValue to operate on marshaled
	// single values.
	value, err := valueside.UnmarshalLegacy(rf.alloc, typ, kv.Value)
	if err != nil {
		return "", "", err
	}
	if rf.traceKV {
		prettyValue = value.String()
	}
	table.row[idx] = rowenc.DatumToEncDatum(typ, value)
	if DebugRowFetch {
		log.Infof(ctx, "Scan %s -> %v", kv.Key, value)
	}
	return prettyKey, prettyValue, nil
}

func (rf *Fetcher) processValueBytes(
	ctx context.Context,
	table *tableInfo,
	kv roachpb.KeyValue,
	valueBytes []byte,
	prettyKeyPrefix string,
) (prettyKey string, prettyValue string, err error) {
	prettyKey = prettyKeyPrefix
	if rf.traceKV {
		if rf.prettyValueBuf == nil {
			rf.prettyValueBuf = &bytes.Buffer{}
		}
		rf.prettyValueBuf.Reset()
	}

	var colIDDiff uint32
	var lastColID descpb.ColumnID
	var typeOffset, dataOffset int
	var typ encoding.Type
	for len(valueBytes) > 0 && rf.valueColsFound < table.neededValueCols {
		typeOffset, dataOffset, colIDDiff, typ, err = encoding.DecodeValueTag(valueBytes)
		if err != nil {
			return "", "", err
		}
		colID := lastColID + descpb.ColumnID(colIDDiff)
		lastColID = colID
		idx, ok := table.colIdxMap.Get(colID)
		if !ok {
			// This column wasn't requested, so read its length and skip it.
			numBytes, err := encoding.PeekValueLengthWithOffsetsAndType(valueBytes, dataOffset, typ)
			if err != nil {
				return "", "", err
			}
			valueBytes = valueBytes[numBytes:]
			if DebugRowFetch {
				log.Infof(ctx, "Scan %s -> [%d] (skipped)", kv.Key, colID)
			}
			continue
		}

		if rf.traceKV {
			prettyKey = fmt.Sprintf("%s/%s", prettyKey, table.spec.FetchedColumns[idx].Name)
		}
		if table.spec.FetchedColumns[idx].Type.Family() == types.JsonFamily {
			return "", "", errors.AssertionFailedf(
				"column %q (id=%d) has JSON type encoded inline in the row value; "+
					"this data was written by an incompatible version that does not use subordinate key encoding for JSON",
				table.spec.FetchedColumns[idx].Name, colID,
			)
		}

		var encValue rowenc.EncDatum
		encValue, valueBytes, err = rowenc.EncDatumValueFromBufferWithOffsetsAndType(valueBytes, typeOffset,
			dataOffset, typ)
		if err != nil {
			return "", "", err
		}
		if rf.traceKV {
			err := encValue.EnsureDecoded(table.spec.FetchedColumns[idx].Type, rf.alloc)
			if err != nil {
				return "", "", err
			}
			fmt.Fprintf(rf.prettyValueBuf, "/%v", encValue.Datum)
		}
		table.row[idx] = encValue
		rf.valueColsFound++
		if DebugRowFetch {
			log.Infof(ctx, "Scan %d -> %v", idx, encValue)
		}
	}
	if rf.traceKV {
		prettyValue = rf.prettyValueBuf.String()
	}
	return prettyKey, prettyValue, nil
}

// processSubordinateKV handles a subordinate key stored under the row's
// family-0 sentinel.
func (rf *Fetcher) processSubordinateKV(
	ctx context.Context,
	table *tableInfo,
	kv roachpb.KeyValue,
	remaining []byte,
	prettyKeyPrefix string,
) (prettyKey string, prettyValue string, err error) {
	prettyKey = prettyKeyPrefix
	_ = remaining

	_, colID64, path, err := keys.DecodeSubordinatePathKey(kv.Key)
	if err != nil {
		return "", "", err
	}
	colID := descpb.ColumnID(colID64)

	// Look up the column.
	idx, ok := table.colIdxMap.Get(colID)
	if !ok {
		// Column not requested — skip.
		if DebugRowFetch {
			log.Infof(ctx, "Scan %s -> subordinate col %d (skipped)", kv.Key, colID)
		}
		return prettyKey, "", nil
	}

	colSpec := &table.spec.FetchedColumns[idx]
	switch colSpec.Type.Family() {
	case types.ArrayFamily:
		if len(path) != 1 || path[0].Kind != keys.SubordinatePathArrayIndex {
			return "", "", errors.AssertionFailedf(
				"invalid subordinate array path for column %q: %+v",
				colSpec.Name, path,
			)
		}
		elemIdx := int(path[0].ArrayIdx)
		elemType := colSpec.Type.ArrayContents()

		// Unmarshal the element value.
		var value tree.Datum
		if rowenc.IsSubordinateNull(kv.Value) {
			value = tree.DNull
		} else {
			var err error
			value, err = valueside.UnmarshalLegacy(rf.alloc, elemType, kv.Value)
			if err != nil {
				return "", "", errors.Wrapf(err, "decoding subordinate key value for column %d", colID)
			}
		}

		filterCol := rf.arrayEqualsAnyFilter != nil && rf.arrayEqualsAnyFilter.colIdx == idx
		if filterCol {
			rf.arrayEqualsAnyFilter.sawSubordinate = true
			if value == tree.DNull || rf.arrayEqualsAnyFilter.left == tree.DNull {
				rf.arrayEqualsAnyFilter.sawNull = true
			} else if rf.arrayEqualsAnyFilter.left.Compare(rf.arrayEqualsAnyFilter.evalCtx, value) == 0 {
				rf.arrayEqualsAnyFilter.matched = true
			}
			if !rf.arrayEqualsAnyFilter.materialize {
				if rf.traceKV {
					prettyKey = fmt.Sprintf("%s/%s[*]", prettyKey, colSpec.Name)
					prettyValue = value.String()
				}
				if DebugRowFetch {
					log.Infof(ctx, "Scan %s -> subordinate predicate %s = %v", kv.Key, colSpec.Name, value)
				}
				return prettyKey, prettyValue, nil
			}
		}

		// Accumulate into the DArray for this column.
		if rf.subordinateArrays == nil {
			rf.subordinateArrays = make(map[int]*subordinateArrayBuilder)
		}
		arr, exists := rf.subordinateArrays[idx]
		if !exists {
			arr = newSubordinateArrayBuilder(elemType)
			rf.subordinateArrays[idx] = arr
		}
		arr.Set(elemIdx, value)

		if rf.traceKV {
			prettyKey = fmt.Sprintf("%s/%s[%d]", prettyKey, colSpec.Name, elemIdx)
			prettyValue = value.String()
		}
		if DebugRowFetch {
			log.Infof(ctx, "Scan %s -> subordinate %s[%d] = %v", kv.Key, colSpec.Name, elemIdx, value)
		}
		return prettyKey, prettyValue, nil

	case types.JsonFamily:
		nodeKind, childCount, scalarRaw, err := rowenc.PeekSubordinateJSONValueMetadata(kv.Value)
		if err != nil {
			return "", "", errors.Wrapf(err, "decoding subordinate JSON value for column %d", colID)
		}
		var (
			j           jsonutil.JSON
			decodedJSON bool
		)
		decodeScalar := func() (jsonutil.JSON, error) {
			if nodeKind != rowenc.SubordinateJSONScalar {
				return nil, nil
			}
			if decodedJSON {
				return j, nil
			}
			decodedJSON = true
			j, err = rowenc.DecodeSubordinateJSONScalarBytes(scalarRaw)
			if err != nil {
				return nil, err
			}
			return j, nil
		}

		filterCol := rf.jsonExistsFilter != nil && rf.jsonExistsFilter.shared.colIdx == idx
		compareCol := rf.jsonPathCompareFilter != nil && rf.jsonPathCompareFilter.shared.colIdx == idx
		containsFilters := rf.jsonContainsByCol[idx]
		containsCol := len(containsFilters) > 0
		needMaterialize := false
		for _, filter := range containsFilters {
			if filter.selected != nil {
				continue
			}
			if nodeKind == rowenc.SubordinateJSONScalar {
				if j, err = decodeScalar(); err != nil {
					return "", "", err
				}
			}
			if err := filter.program.Observe(path, nodeKind, childCount, j); err != nil {
				return "", "", err
			}
			needMaterialize = needMaterialize || filter.materialize
		}
		hasSelectedPathProgram := len(rf.jsonSharedSelectedByCol[idx]) > 0
		for _, shared := range rf.jsonSharedSelectedByCol[idx] {
			relPath, ok, err := shared.selectPath(path, nodeKind, childCount)
			if err != nil {
				return "", "", err
			}
			if !ok {
				continue
			}
			if nodeKind == rowenc.SubordinateJSONScalar {
				if j, err = decodeScalar(); err != nil {
					return "", "", err
				}
			}
			if err := shared.observeSelected(relPath, nodeKind, childCount, j); err != nil {
				return "", "", err
			}
			needMaterialize = needMaterialize || shared.materialize
		}
		sawJSONProgram := false
		for _, shared := range rf.jsonSharedAccessByCol[idx] {
			if shared.selected != nil {
				continue
			}
			sawJSONProgram = true
			if nodeKind == rowenc.SubordinateJSONScalar && shared.program.NeedsScalarAt(path, nodeKind) {
				if j, err = decodeScalar(); err != nil {
					return "", "", err
				}
			}
			if err := shared.observe(path, nodeKind, childCount, j); err != nil {
				return "", "", err
			}
			needMaterialize = needMaterialize || shared.materialize
		}
		if (filterCol || compareCol || containsCol || sawJSONProgram || hasSelectedPathProgram) && !needMaterialize {
			if rf.traceKV {
				prettyKey = fmt.Sprintf("%s/%s", prettyKey, colSpec.Name)
				prettyValue = "json-access"
			}
			return prettyKey, prettyValue, nil
		}

		kind, err := SubordinateJSONNodeKindFromEncoded(nodeKind)
		if err != nil {
			return "", "", err
		}

		if rf.subordinateJSONBuilders == nil {
			rf.subordinateJSONBuilders = make(map[int]*subordinateJSONBuilder)
		}
		builder := rf.subordinateJSONBuilders[idx]
		if builder == nil {
			builder = &subordinateJSONBuilder{}
			rf.subordinateJSONBuilders[idx] = builder
		}
		if nodeKind == rowenc.SubordinateJSONScalar {
			if j, err = decodeScalar(); err != nil {
				return "", "", err
			}
		}
		if err := builder.Set(path, kind, j); err != nil {
			return "", "", err
		}

		if rf.traceKV {
			prettyKey = fmt.Sprintf("%s/%s", prettyKey, colSpec.Name)
			if len(path) > 1 {
				for _, seg := range path[1:] {
					switch seg.Kind {
					case keys.SubordinatePathObjectKey:
						prettyKey = fmt.Sprintf("%s.%s", prettyKey, seg.ObjectKey)
					case keys.SubordinatePathArrayIndex:
						prettyKey = fmt.Sprintf("%s[%d]", prettyKey, seg.ArrayIdx)
					}
				}
			}
			if j != nil {
				prettyValue = j.String()
			}
		}
		return prettyKey, prettyValue, nil

	default:
		return "", "", errors.AssertionFailedf(
			"column %q has unsupported subordinate-encoded type %s",
			colSpec.Name, colSpec.Type.Family(),
		)
	}
}

// NextRow processes keys until we complete one row, which is returned as an
// EncDatumRow. The row contains one value per IndexFetchSpec.FetchedColumns.
//
// The EncDatumRow should not be modified and is only valid until the next call.
//
// When there are no more rows, the EncDatumRow is nil.
func (rf *Fetcher) NextRow(ctx context.Context) (row rowenc.EncDatumRow, err error) {
	if rf.kvEnd {
		return nil, nil
	}

	// All of the columns for a particular row will be grouped together. We
	// loop over the key/value pairs and decode the key to extract the
	// columns encoded within the key and the column ID. We use the column
	// ID to lookup the column and decode the value. All of these values go
	// into a map keyed by column name. When the index key changes we
	// output a row containing the current values.
	for {
		prettyKey, prettyVal, err := rf.processKV(ctx, rf.kv)
		if err != nil {
			return nil, err
		}
		if rf.traceKV {
			log.VEventf(ctx, 2, "fetched: %s -> %s", prettyKey, prettyVal)
		}

		rowDone, err := rf.nextKey(ctx)
		if err != nil {
			return nil, err
		}
		if rowDone {
			err := rf.finalizeRow()
			return rf.table.row, err
		}
	}
}

// NextRowInto calls NextRow and copies the results into the given EncDatumRow
// slice according to the given column map.
//
// Values for columns that are not in the map are ignored. EncDatums in
// destination that don't correspond to a fetcher column are not modified.
//
// If there are no more rows, returns ok=false.
func (rf *Fetcher) NextRowInto(
	ctx context.Context, destination rowenc.EncDatumRow, colIdxMap catalog.TableColMap,
) (ok bool, err error) {
	row, err := rf.NextRow(ctx)
	if err != nil {
		return false, err
	}
	if row == nil {
		return false, nil
	}

	for i := range rf.table.spec.FetchedColumns {
		if ord, ok := colIdxMap.Get(rf.table.spec.FetchedColumns[i].ColumnID); ok {
			destination[ord] = row[i]
		}
	}
	return true, nil
}

// NextRowDecoded calls NextRow and decodes the EncDatumRow into a Datums.
// The Datums should not be modified and is only valid until the next call.
// When there are no more rows, the Datums is nil.
func (rf *Fetcher) NextRowDecoded(ctx context.Context) (datums tree.Datums, err error) {
	row, err := rf.NextRow(ctx)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, nil
	}

	for i, encDatum := range row {
		if encDatum.IsUnset() {
			rf.table.decodedRow[i] = tree.DNull
			continue
		}
		if err := encDatum.EnsureDecoded(rf.table.spec.FetchedColumns[i].Type, rf.alloc); err != nil {
			return nil, err
		}
		rf.table.decodedRow[i] = encDatum.Datum
	}

	return rf.table.decodedRow, nil
}

// NextRowDecodedInto calls NextRow and decodes the EncDatumRow into Datums,
// storing the results in the given destination slice according to the column
// mapping.
//
// Values for columns that are not in the map are ignored. Datums in
// destination that don't correspond to a fetcher column are not modified.
//
// If there are no more rows, returns ok=false.
func (rf *Fetcher) NextRowDecodedInto(
	ctx context.Context, destination tree.Datums, colIdxMap catalog.TableColMap,
) (ok bool, err error) {
	row, err := rf.NextRow(ctx)
	if err != nil {
		return false, err
	}
	if row == nil {
		return false, nil
	}

	for i := range rf.table.spec.FetchedColumns {
		col := &rf.table.spec.FetchedColumns[i]
		ord, ok := colIdxMap.Get(col.ColumnID)
		if !ok {
			// Column not in map, ignore.
			continue
		}
		encDatum := row[i]
		if encDatum.IsUnset() {
			destination[ord] = tree.DNull
			continue
		}
		if err := encDatum.EnsureDecoded(col.Type, rf.alloc); err != nil {
			return false, err
		}
		destination[ord] = encDatum.Datum
	}

	return true, nil
}

// RowLastModified may only be called after NextRow has returned a non-nil row
// and returns the timestamp of the last modification to that row.
func (rf *Fetcher) RowLastModified() hlc.Timestamp {
	return rf.table.rowLastModified
}

// RowIsDeleted may only be called after NextRow has returned a non-nil row and
// returns true if that row was most recently deleted. This method is only
// meaningful when the configured KVBatchFetcher returns deletion tombstones, which
// the normal one (via `StartScan`) does not.
func (rf *Fetcher) RowIsDeleted() bool {
	return rf.table.rowIsDeleted
}

func (rf *Fetcher) finishArrayEqualsAnyFilter() error {
	if rf.arrayEqualsAnyFilter == nil {
		rf.lastRowPassesArrayEqualsAnyFilter = true
		return nil
	}
	f := rf.arrayEqualsAnyFilter
	if !f.sawSubordinate {
		encDatum := rf.table.row[f.colIdx]
		if encDatum.IsUnset() {
			rf.lastRowPassesArrayEqualsAnyFilter = false
			return nil
		}
		if err := encDatum.EnsureDecoded(rf.table.spec.FetchedColumns[f.colIdx].Type, rf.alloc); err != nil {
			return err
		}
		d := encDatum.Datum
		if d == tree.DNull || f.left == tree.DNull {
			f.sawNull = true
		} else if arr, ok := tree.AsDArray(d); ok {
			// Non-empty arrays must be reconstructed from subordinate keys.
			// Inline arrays are only valid for the empty-array sentinel.
			if arr.Len() > 0 {
				return errors.AssertionFailedf(
					"non-empty inline array encountered in array filter fallback for column %q",
					rf.table.spec.FetchedColumns[f.colIdx].Name,
				)
			}
		}
	}
	rf.lastRowPassesArrayEqualsAnyFilter = f.matched
	return nil
}

func (rf *Fetcher) finishJSONExistsFilter() error {
	if rf.jsonExistsFilter == nil {
		rf.lastRowPassesJSONExistsFilter = true
		return nil
	}
	f := rf.jsonExistsFilter
	if !f.shared.sawSubordinate {
		encDatum := rf.table.row[f.shared.colIdx]
		if encDatum.IsUnset() {
			rf.lastRowPassesJSONExistsFilter = false
			return nil
		}
		if err := encDatum.EnsureDecoded(rf.table.spec.FetchedColumns[f.shared.colIdx].Type, rf.alloc); err != nil {
			return err
		}
		return errors.AssertionFailedf(
			"inline JSON encountered in JSON existence filter fallback for column %q",
			rf.table.spec.FetchedColumns[f.shared.colIdx].Name,
		)
	}
	d, err := f.shared.resultDatum(f.kind)
	if err != nil {
		return err
	}
	rf.lastRowPassesJSONExistsFilter = d == tree.DBoolTrue
	return nil
}

func (rf *Fetcher) finishJSONPathCompareFilter() error {
	if rf.jsonPathCompareFilter == nil {
		rf.lastRowPassesJSONPathCompareFilter = true
		return nil
	}
	f := rf.jsonPathCompareFilter
	if !f.shared.sawSubordinate {
		encDatum := rf.table.row[f.shared.colIdx]
		if encDatum.IsUnset() {
			switch f.mode {
			case exec.JSONPathFilterEq:
				rf.lastRowPassesJSONPathCompareFilter = false
			case exec.JSONPathFilterIsNull:
				rf.lastRowPassesJSONPathCompareFilter = true
			case exec.JSONPathFilterIsNotNull:
				rf.lastRowPassesJSONPathCompareFilter = false
			default:
				return errors.AssertionFailedf("unknown JSON path filter mode %d", f.mode)
			}
			return nil
		}
		if err := encDatum.EnsureDecoded(rf.table.spec.FetchedColumns[f.shared.colIdx].Type, rf.alloc); err != nil {
			return err
		}
		return errors.AssertionFailedf(
			"inline JSON encountered in JSON path compare filter fallback for column %q",
			rf.table.spec.FetchedColumns[f.shared.colIdx].Name,
		)
	}
	d, err := f.shared.resultDatum(f.kind)
	if err != nil {
		return err
	}
	rf.lastRowPassesJSONPathCompareFilter, err = EvalJSONPathFilterDatum(f.evalCtx, f.mode, d, f.right)
	if err != nil {
		return err
	}
	return nil
}

func (rf *Fetcher) finishJSONContainsFilter() error {
	if len(rf.jsonContainsFilters) == 0 {
		rf.lastRowPassesJSONContainsFilter = true
		return nil
	}
	for _, f := range rf.jsonContainsFilters {
		if f.selected != nil && (f.selected.materialize || len(f.selected.access) > 0) {
			contains, err := f.selected.containsResult(f.program)
			if err != nil {
				return err
			}
			if !contains {
				rf.lastRowPassesJSONContainsFilter = false
				return nil
			}
			continue
		}
		if !f.program.SawSubordinate() {
			encDatum := rf.table.row[f.colIdx]
			if encDatum.IsUnset() {
				rf.lastRowPassesJSONContainsFilter = false
				return nil
			}
			if err := encDatum.EnsureDecoded(rf.table.spec.FetchedColumns[f.colIdx].Type, rf.alloc); err != nil {
				return err
			}
			return errors.AssertionFailedf(
				"inline JSON encountered in JSON contains filter fallback for column %q",
				rf.table.spec.FetchedColumns[f.colIdx].Name,
			)
		}
		contains, err := f.program.Passes()
		if err != nil {
			return err
		}
		if !contains {
			rf.lastRowPassesJSONContainsFilter = false
			return nil
		}
	}
	rf.lastRowPassesJSONContainsFilter = true
	return nil
}

func (rf *Fetcher) finishJSONAccessPrograms() error {
	for i := range rf.jsonAccessPrograms {
		state := rf.jsonAccessPrograms[i]
		if !state.shared.sawSubordinate {
			encDatum := rf.table.row[state.shared.colIdx]
			if encDatum.IsUnset() {
				rf.lastRowJSONAccessProgramResults[i] = tree.DNull
				continue
			}
			if err := encDatum.EnsureDecoded(rf.table.spec.FetchedColumns[state.shared.colIdx].Type, rf.alloc); err != nil {
				return err
			}
			return errors.AssertionFailedf(
				"inline JSON encountered in JSON access fallback for column %q",
				rf.table.spec.FetchedColumns[state.shared.colIdx].Name,
			)
		}
		d, err := state.shared.resultDatum(state.kind)
		if err != nil {
			return err
		}
		rf.lastRowJSONAccessProgramResults[i] = d
	}
	return nil
}

func (rf *Fetcher) finalizeRow() error {
	table := &rf.table

	// Finalize subordinate arrays: convert accumulated DArrays into EncDatums.
	for idx, arrBuilder := range rf.subordinateArrays {
		arr, err := arrBuilder.Materialize()
		if err != nil {
			return err
		}
		table.row[idx] = rowenc.EncDatum{Datum: arr}
		rf.valueColsFound++
	}
	for idx, jsonBuilder := range rf.subordinateJSONBuilders {
		dJSON, err := jsonBuilder.Materialize()
		if err != nil {
			return err
		}
		table.row[idx] = rowenc.EncDatum{Datum: dJSON}
		rf.valueColsFound++
	}
	if err := rf.finishArrayEqualsAnyFilter(); err != nil {
		return err
	}
	if err := rf.finishJSONExistsFilter(); err != nil {
		return err
	}
	if err := rf.finishJSONPathCompareFilter(); err != nil {
		return err
	}
	if err := rf.finishJSONContainsFilter(); err != nil {
		return err
	}
	if err := rf.finishJSONAccessPrograms(); err != nil {
		return err
	}

	// Fill in any system columns if requested.
	if table.timestampOutputIdx != noOutputColumn {
		// TODO (rohany): Datums are immutable, so we can't store a DDecimal on the
		//  fetcher and change its contents with each row. If that assumption gets
		//  lifted, then we can avoid an allocation of a new decimal datum here.
		dec := rf.alloc.NewDDecimal(tree.DDecimal{Decimal: tree.TimestampToDecimal(rf.RowLastModified())})
		table.row[table.timestampOutputIdx] = rowenc.EncDatum{Datum: dec}
	}
	if table.oidOutputIdx != noOutputColumn {
		table.row[table.oidOutputIdx] = rowenc.EncDatum{Datum: table.tableOid}
	}

	// Fill in any missing values with NULLs
	for i := range table.spec.FetchedColumns {
		col := &table.spec.FetchedColumns[i]
		if rf.valueColsFound == table.neededValueCols {
			// Found all cols - done!
			return nil
		}
		if table.row[i].IsUnset() {
			if rf.arrayEqualsAnyFilter != nil &&
				rf.arrayEqualsAnyFilter.colIdx == i &&
				!rf.arrayEqualsAnyFilter.materialize {
				continue
			}
			if rf.jsonExistsFilter != nil &&
				rf.jsonExistsFilter.shared.colIdx == i &&
				!rf.jsonExistsFilter.shared.materialize {
				continue
			}
			// If the row was deleted, we'll be missing any non-primary key
			// columns, including nullable ones, but this is expected. If the column
			// is not yet active, we can also expect NULLs.
			if col.IsNonNullable && !table.rowIsDeleted && !rf.IgnoreUnexpectedNulls {
				var indexColValues []string
				for _, idx := range table.indexColIdx {
					if idx != -1 {
						indexColValues = append(indexColValues, table.row[idx].String(table.spec.FetchedColumns[idx].Type))
					} else {
						indexColValues = append(indexColValues, "?")
					}
				}
				var indexColNames []string
				for i := range table.spec.KeyFullColumns() {
					indexColNames = append(indexColNames, table.spec.KeyAndSuffixColumns[i].Name)
				}
				return errors.AssertionFailedf(
					"Non-nullable column \"%s:%s\" with no value! Index scanned was %q with the index key columns (%s) and the values (%s)",
					table.spec.TableName, col.Name, table.spec.IndexName,
					strings.Join(indexColNames, ","), strings.Join(indexColValues, ","))
			}
			table.row[i] = rowenc.EncDatum{
				Datum: tree.DNull,
			}
			// We've set valueColsFound to the number of present columns in the row
			// already, in processValueBytes. Now, we're filling in columns that have
			// no encoded values with NULL - so we increment valueColsFound to permit
			// early exit from this loop once all needed columns are filled in.
			rf.valueColsFound++
		}
	}
	return nil
}

// Key returns the next key (the key that follows the last returned row).
// Key returns nil when there are no more rows.
func (rf *Fetcher) Key() roachpb.Key {
	return rf.kv.Key
}

// PartialKey returns a partial slice of the next key (the key that follows the
// last returned row) containing nCols columns, without the ending column
// family. Returns nil when there are no more rows.
func (rf *Fetcher) PartialKey(nCols int) (roachpb.Key, error) {
	if rf.kv.Key == nil {
		return nil, nil
	}
	partialKeyLength := int(rf.table.spec.KeyPrefixLength)
	for consumedCols := 0; consumedCols < nCols; consumedCols++ {
		l, err := encoding.PeekLength(rf.kv.Key[partialKeyLength:])
		if err != nil {
			return nil, err
		}
		partialKeyLength += l
	}
	return rf.kv.Key[:partialKeyLength], nil
}

// GetBytesRead returns total number of bytes read by the underlying KVFetcher.
func (rf *Fetcher) GetBytesRead() int64 {
	return rf.kvFetcher.GetBytesRead()
}
