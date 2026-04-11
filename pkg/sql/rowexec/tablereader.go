// Copyright 2016 The Cockroach Authors.
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

package rowexec

import (
	"context"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/semistrict/ratel/pkg/sql/catalog/typedesc"
	"github.com/semistrict/ratel/pkg/sql/execinfra"
	"github.com/semistrict/ratel/pkg/sql/execinfrapb"
	"github.com/semistrict/ratel/pkg/sql/opt/exec"
	"github.com/semistrict/ratel/pkg/sql/row"
	"github.com/semistrict/ratel/pkg/sql/rowenc"
	"github.com/semistrict/ratel/pkg/sql/rowinfra"
	"github.com/semistrict/ratel/pkg/sql/sem/tree"
	"github.com/semistrict/ratel/pkg/sql/types"
	"github.com/semistrict/ratel/pkg/util/log"
	"github.com/semistrict/ratel/pkg/util/optional"
)

// tableReader is the start of a computation flow; it performs KV operations to
// retrieve rows for a table, runs a filter expression, and passes rows with the
// desired column values to an output RowReceiver.
// See docs/RFCS/distributed_sql.md
type tableReader struct {
	execinfra.ProcessorBase
	execinfra.SpansWithCopy

	limitHint       rowinfra.RowLimit
	parallelize     bool
	batchBytesLimit rowinfra.BytesLimit

	scanStarted bool

	// See TableReaderSpec.MaxTimestampAgeNanos.
	maxTimestampAge time.Duration

	ignoreMisplannedRanges bool

	// fetcher wraps a row.Fetcher, allowing the tableReader to add a stat
	// collection layer.
	fetcher rowFetcher
	alloc   tree.DatumAlloc

	scanStats execinfra.ScanStats

	// rowsRead is the number of rows read and is tracked unconditionally.
	rowsRead int64

	numJSONAccesses int
}

func postProcessOutputsFetchedColumn(
	post *execinfrapb.PostProcessSpec, fetchedCols int, fetchedColIdx int,
) bool {
	if fetchedColIdx < 0 || fetchedColIdx >= fetchedCols {
		return false
	}
	if !post.Projection {
		return true
	}
	for _, outCol := range post.OutputColumns {
		if int(outCol) == fetchedColIdx {
			return true
		}
	}
	return false
}

var _ execinfra.Processor = &tableReader{}
var _ execinfra.RowSource = &tableReader{}
var _ execinfra.Releasable = &tableReader{}
var _ execinfra.OpNode = &tableReader{}

const tableReaderProcName = "table reader"

var trPool = sync.Pool{
	New: func() interface{} {
		return &tableReader{}
	},
}

// newTableReader creates a tableReader.
func newTableReader(
	flowCtx *execinfra.FlowCtx,
	processorID int32,
	spec *execinfrapb.TableReaderSpec,
	post *execinfrapb.PostProcessSpec,
	output execinfra.RowReceiver,
) (*tableReader, error) {
	// NB: we hit this with a zero NodeID (but !ok) with multi-tenancy.
	if nodeID, ok := flowCtx.NodeID.OptionalNodeID(); ok && nodeID == 0 {
		return nil, errors.Errorf("attempting to create a tableReader with uninitialized NodeID")
	}

	if spec.LimitHint > 0 || spec.BatchBytesLimit > 0 {
		// Parallelize shouldn't be set when there's a limit hint, but double-check
		// just in case.
		spec.Parallelize = false
	}
	var batchBytesLimit rowinfra.BytesLimit
	if !spec.Parallelize {
		batchBytesLimit = rowinfra.BytesLimit(spec.BatchBytesLimit)
		if batchBytesLimit == 0 {
			batchBytesLimit = rowinfra.DefaultBatchBytesLimit
		}
	}

	tr := trPool.Get().(*tableReader)

	tr.limitHint = rowinfra.RowLimit(execinfra.LimitHint(spec.LimitHint, post))
	tr.parallelize = spec.Parallelize
	tr.batchBytesLimit = batchBytesLimit
	tr.maxTimestampAge = time.Duration(spec.MaxTimestampAgeNanos)

	// Make sure the key column types are hydrated. The fetched column types
	// will be hydrated in ProcessorBase.Init below.
	resolver := flowCtx.NewTypeResolver(flowCtx.Txn)
	for i := range spec.FetchSpec.KeyAndSuffixColumns {
		if err := typedesc.EnsureTypeIsHydrated(
			flowCtx.EvalCtx.Ctx(), spec.FetchSpec.KeyAndSuffixColumns[i].Type, &resolver,
		); err != nil {
			return nil, err
		}
	}

	resultTypes := make([]*types.T, 0, len(spec.FetchSpec.FetchedColumns)+len(spec.JsonAccesses))
	for i := range spec.FetchSpec.FetchedColumns {
		resultTypes = append(resultTypes, spec.FetchSpec.FetchedColumns[i].Type)
	}
	for i := range spec.JsonAccesses {
		switch row.JSONAccessKind(spec.JsonAccesses[i].Kind) {
		case row.JSONAccessExists:
			resultTypes = append(resultTypes, types.Bool)
		case row.JSONAccessExistsAny:
			resultTypes = append(resultTypes, types.Bool)
		case row.JSONAccessExistsAll:
			resultTypes = append(resultTypes, types.Bool)
		case row.JSONAccessFetchJSONPath:
			resultTypes = append(resultTypes, types.Jsonb)
		case row.JSONAccessFetchTextPath:
			resultTypes = append(resultTypes, types.String)
		default:
			return nil, errors.AssertionFailedf("unknown JSON access kind %d", spec.JsonAccesses[i].Kind)
		}
	}

	tr.ignoreMisplannedRanges = flowCtx.Local
	if err := tr.Init(
		tr,
		post,
		resultTypes,
		flowCtx,
		processorID,
		output,
		nil, /* memMonitor */
		execinfra.ProcStateOpts{
			// We don't pass tr.input as an inputToDrain; tr.input is just an adapter
			// on top of a Fetcher; draining doesn't apply to it. Moreover, Andrei
			// doesn't trust that the adapter will do the right thing on a Next() call
			// after it had previously returned an error.
			InputsToDrain:        nil,
			TrailingMetaCallback: tr.generateTrailingMeta,
		},
	); err != nil {
		return nil, err
	}

	var fetcher row.Fetcher
	if err := fetcher.Init(
		flowCtx.EvalCtx.Context,
		spec.Reverse,
		spec.LockingStrength,
		spec.LockingWaitPolicy,
		flowCtx.EvalCtx.SessionData().LockTimeout,
		&tr.alloc,
		flowCtx.EvalCtx.Mon,
		&spec.FetchSpec,
	); err != nil {
		return nil, err
	}
	if spec.ArrayEqualsAnyFilter != nil {
		leftExpr := spec.ArrayEqualsAnyFilter.Left.LocalExpr
		if leftExpr == nil {
			var err error
			leftExpr, err = execinfrapb.DeserializeExpr(
				spec.ArrayEqualsAnyFilter.Left.Expr, &tr.SemaCtx, tr.EvalCtx, &tree.IndexedVarHelper{},
			)
			if err != nil {
				return nil, err
			}
		}
		leftDatum, err := leftExpr.Eval(tr.EvalCtx)
		if err != nil {
			return nil, err
		}
		fetcher.ConfigureArrayEqualsAnyFilter(
			tr.EvalCtx, int(spec.ArrayEqualsAnyFilter.ArrayColIdx), leftDatum, false, /* materialize */
		)
	}
	if spec.JsonExistsFilter != nil {
		if err := fetcher.ConfigureJSONExistsFilter(
			int(spec.JsonExistsFilter.SourceColIdx),
			row.JSONAccessKind(spec.JsonExistsFilter.Kind),
			spec.JsonExistsFilter.Key,
			append([]string(nil), spec.JsonExistsFilter.Keys...),
			postProcessOutputsFetchedColumn(post, len(spec.FetchSpec.FetchedColumns), int(spec.JsonExistsFilter.SourceColIdx)),
		); err != nil {
			return nil, err
		}
	}
	if spec.JsonPathCompareFilter != nil {
		var rightDatum tree.Datum
		if mode := exec.JSONPathFilterMode(spec.JsonPathCompareFilter.Mode); mode != exec.JSONPathFilterIsNull && mode != exec.JSONPathFilterIsNotNull {
			rightExpr := spec.JsonPathCompareFilter.Right.LocalExpr
			if rightExpr == nil {
				var err error
				rightExpr, err = execinfrapb.DeserializeExpr(
					spec.JsonPathCompareFilter.Right.Expr, &tr.SemaCtx, tr.EvalCtx, &tree.IndexedVarHelper{},
				)
				if err != nil {
					return nil, err
				}
			}
			var err error
			rightDatum, err = rightExpr.Eval(tr.EvalCtx)
			if err != nil {
				return nil, err
			}
		}
		if err := fetcher.ConfigureJSONPathCompareFilter(
			tr.EvalCtx,
			int(spec.JsonPathCompareFilter.SourceColIdx),
			row.JSONAccessKind(spec.JsonPathCompareFilter.Kind),
			append([]string(nil), spec.JsonPathCompareFilter.Path...),
			exec.JSONPathFilterMode(spec.JsonPathCompareFilter.Mode),
			rightDatum,
			postProcessOutputsFetchedColumn(post, len(spec.FetchSpec.FetchedColumns), int(spec.JsonPathCompareFilter.SourceColIdx)),
		); err != nil {
			return nil, err
		}
	}
	for i := range spec.JsonContainsFilters {
		filterSpec := &spec.JsonContainsFilters[i]
		rightExpr := filterSpec.Right.LocalExpr
		if rightExpr == nil {
			var err error
			rightExpr, err = execinfrapb.DeserializeExpr(
				filterSpec.Right.Expr, &tr.SemaCtx, tr.EvalCtx, &tree.IndexedVarHelper{},
			)
			if err != nil {
				return nil, err
			}
		}
		rightDatum, err := rightExpr.Eval(tr.EvalCtx)
		if err != nil {
			return nil, err
		}
		rightJSON, ok := rightDatum.(*tree.DJSON)
		if !ok {
			return nil, errors.AssertionFailedf("JSON contains filter right datum has type %T", rightDatum)
		}
		if err := fetcher.ConfigureJSONContainsFilter(
			int(filterSpec.SourceColIdx),
			append([]string(nil), filterSpec.Path...),
			filterSpec.ContainedBy,
			rightJSON.JSON,
			postProcessOutputsFetchedColumn(post, len(spec.FetchSpec.FetchedColumns), int(filterSpec.SourceColIdx)),
		); err != nil {
			return nil, err
		}
	}
	if len(spec.JsonAccesses) > 0 {
		programs := make([]row.JSONAccessSpec, len(spec.JsonAccesses))
		for i := range spec.JsonAccesses {
			programs[i] = row.JSONAccessSpec{
				ColIdx: int(spec.JsonAccesses[i].SourceColIdx),
				Kind:   row.JSONAccessKind(spec.JsonAccesses[i].Kind),
				Key:    spec.JsonAccesses[i].Key,
				Keys:   append([]string(nil), spec.JsonAccesses[i].Keys...),
				Path:   append([]string(nil), spec.JsonAccesses[i].Path...),
			}
		}
		if err := fetcher.ConfigureJSONAccessPrograms(programs); err != nil {
			return nil, err
		}
		tr.numJSONAccesses = len(programs)
	}

	tr.Spans = spec.Spans
	if !tr.ignoreMisplannedRanges {
		// Make a copy of the spans so that we could get the misplanned ranges
		// info.
		tr.MakeSpansCopy()
	}

	if execinfra.ShouldCollectStats(flowCtx.EvalCtx.Ctx(), flowCtx) {
		tr.fetcher = newRowFetcherStatCollector(&fetcher)
		tr.ExecStatsForTrace = tr.execStatsForTrace
	} else {
		tr.fetcher = &fetcher
	}

	return tr, nil
}

func (tr *tableReader) generateTrailingMeta() []execinfrapb.ProducerMetadata {
	// We need to generate metadata before closing the processor because
	// InternalClose() updates tr.Ctx to the "original" context.
	trailingMeta := tr.generateMeta()
	tr.close()
	return trailingMeta
}

// Start is part of the RowSource interface.
func (tr *tableReader) Start(ctx context.Context) {
	if tr.FlowCtx.Txn == nil {
		log.Fatalf(ctx, "tableReader outside of txn")
	}

	// Keep ctx assignment so we remember StartInternal can make a new one.
	ctx = tr.StartInternal(ctx, tableReaderProcName)
	// Appease the linter.
	_ = ctx
}

func (tr *tableReader) startScan(ctx context.Context) error {
	limitBatches := !tr.parallelize
	var bytesLimit rowinfra.BytesLimit
	if !limitBatches {
		bytesLimit = rowinfra.NoBytesLimit
	} else {
		bytesLimit = tr.batchBytesLimit
	}
	log.VEventf(ctx, 1, "starting scan with limitBatches %t", limitBatches)
	var err error
	if tr.maxTimestampAge == 0 {
		err = tr.fetcher.StartScan(
			ctx, tr.FlowCtx.Txn, tr.Spans, bytesLimit, tr.limitHint,
			tr.FlowCtx.TraceKV,
			tr.EvalCtx.TestingKnobs.ForceProductionBatchSizes,
		)
	} else {
		initialTS := tr.FlowCtx.Txn.ReadTimestamp()
		err = tr.fetcher.StartInconsistentScan(
			ctx, tr.FlowCtx.Cfg.DB, initialTS, tr.maxTimestampAge, tr.Spans,
			bytesLimit, tr.limitHint, tr.FlowCtx.TraceKV,
			tr.EvalCtx.TestingKnobs.ForceProductionBatchSizes,
			tr.EvalCtx.QualityOfService(),
		)
	}
	tr.scanStarted = true
	return err
}

// Release releases this tableReader back to the pool.
func (tr *tableReader) Release() {
	tr.ProcessorBase.Reset()
	tr.fetcher.Reset()
	// Deeply reset the spans so that we don't hold onto the keys of the spans.
	tr.SpansWithCopy.Reset()
	*tr = tableReader{
		ProcessorBase: tr.ProcessorBase,
		SpansWithCopy: tr.SpansWithCopy,
		fetcher:       tr.fetcher,
		rowsRead:      0,
	}
	trPool.Put(tr)
}

var tableReaderProgressFrequency int64 = 5000

// TestingSetScannedRowProgressFrequency changes the frequency at which
// row-scanned progress metadata is emitted by table readers.
func TestingSetScannedRowProgressFrequency(val int64) func() {
	oldVal := tableReaderProgressFrequency
	tableReaderProgressFrequency = val
	return func() { tableReaderProgressFrequency = oldVal }
}

// Next is part of the RowSource interface.
func (tr *tableReader) Next() (rowenc.EncDatumRow, *execinfrapb.ProducerMetadata) {
	for tr.State == execinfra.StateRunning {
		if !tr.scanStarted {
			err := tr.startScan(tr.Ctx())
			if err != nil {
				tr.MoveToDraining(err)
				break
			}
		}
		// Check if it is time to emit a progress update.
		if tr.rowsRead >= tableReaderProgressFrequency {
			meta := execinfrapb.GetProducerMeta()
			meta.Metrics = execinfrapb.GetMetricsMeta()
			meta.Metrics.RowsRead = tr.rowsRead
			tr.rowsRead = 0
			return nil, meta
		}

		row, err := tr.fetcher.NextRow(tr.Ctx())
		if row == nil || err != nil {
			tr.MoveToDraining(err)
			break
		}

		// When tracing is enabled, number of rows read is tracked twice (once
		// here, and once through InputStats). This is done so that non-tracing
		// case can avoid tracking of the stall time which gives a noticeable
		// performance hit.
		tr.rowsRead++
		if !tr.fetcher.RowPassesArrayEqualsAnyFilter() {
			continue
		}
		if !tr.fetcher.RowPassesJSONExistsFilter() {
			continue
		}
		if !tr.fetcher.RowPassesJSONPathCompareFilter() {
			continue
		}
		if !tr.fetcher.RowPassesJSONContainsFilter() {
			continue
		}
		if tr.numJSONAccesses > 0 {
			results := tr.fetcher.JSONAccessProgramResults()
			augmented := make(rowenc.EncDatumRow, len(row)+len(results))
			copy(augmented, row)
			for i := range results {
				augmented[len(row)+i] = rowenc.EncDatum{Datum: results[i]}
			}
			row = augmented
		}
		if outRow := tr.ProcessRowHelper(row); outRow != nil {
			return outRow, nil
		}
	}
	return nil, tr.DrainHelper()
}

func (tr *tableReader) close() {
	if tr.InternalClose() {
		if tr.fetcher != nil {
			tr.fetcher.Close(tr.Ctx())
		}
	}
}

// ConsumerClosed is part of the RowSource interface.
func (tr *tableReader) ConsumerClosed() {
	tr.close()
}

// execStatsForTrace implements ProcessorBase.ExecStatsForTrace.
func (tr *tableReader) execStatsForTrace() *execinfrapb.ComponentStats {
	is, ok := getFetcherInputStats(tr.fetcher)
	if !ok {
		return nil
	}
	tr.scanStats = execinfra.GetScanStats(tr.Ctx(), tr.ExecStatsTrace)
	ret := &execinfrapb.ComponentStats{
		KV: execinfrapb.KVStats{
			BytesRead:      optional.MakeUint(uint64(tr.fetcher.GetBytesRead())),
			TuplesRead:     is.NumTuples,
			KVTime:         is.WaitTime,
			ContentionTime: optional.MakeTimeValue(execinfra.GetCumulativeContentionTime(tr.Ctx(), tr.ExecStatsTrace)),
		},
		Output: tr.OutputHelper.Stats(),
	}
	execinfra.PopulateKVMVCCStats(&ret.KV, &tr.scanStats)
	return ret
}

func (tr *tableReader) generateMeta() []execinfrapb.ProducerMetadata {
	var trailingMeta []execinfrapb.ProducerMetadata
	if !tr.ignoreMisplannedRanges {
		nodeID, ok := tr.FlowCtx.NodeID.OptionalNodeID()
		if ok {
			ranges := execinfra.MisplannedRanges(tr.Ctx(), tr.SpansCopy, nodeID, tr.FlowCtx.Cfg.RangeCache)
			if ranges != nil {
				trailingMeta = append(trailingMeta, execinfrapb.ProducerMetadata{Ranges: ranges})
			}
		}
	}
	if tfs := execinfra.GetLeafTxnFinalState(tr.Ctx(), tr.FlowCtx.Txn); tfs != nil {
		trailingMeta = append(trailingMeta, execinfrapb.ProducerMetadata{LeafTxnFinalState: tfs})
	}

	meta := execinfrapb.GetProducerMeta()
	meta.Metrics = execinfrapb.GetMetricsMeta()
	meta.Metrics.BytesRead = tr.fetcher.GetBytesRead()
	meta.Metrics.RowsRead = tr.rowsRead
	return append(trailingMeta, *meta)
}

// ChildCount is part of the execinfra.OpNode interface.
func (tr *tableReader) ChildCount(bool) int {
	return 0
}

// Child is part of the execinfra.OpNode interface.
func (tr *tableReader) Child(nth int, _ bool) execinfra.OpNode {
	panic(errors.AssertionFailedf("invalid index %d", nth))
}
