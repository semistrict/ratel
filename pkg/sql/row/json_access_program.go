// Copyright 2026 The Ratel Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package row

import (
	"fmt"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/semistrict/ratel/pkg/keys"
	"github.com/semistrict/ratel/pkg/sql/rowenc"
	"github.com/semistrict/ratel/pkg/sql/sem/tree"
	jsonutil "github.com/semistrict/ratel/pkg/util/json"
)

// JSONAccessKind identifies a scan-local JSON access operation.
type JSONAccessKind uint8

const (
	JSONAccessExists JSONAccessKind = iota + 1
	JSONAccessExistsAny
	JSONAccessExistsAll
	JSONAccessFetchJSONPath
	JSONAccessFetchTextPath
)

// JSONAccessSpec configures one scan-local JSON access operation for a fetched
// JSON column.
type JSONAccessSpec struct {
	ColIdx      int
	Kind        JSONAccessKind
	Key         string
	Keys        []string
	Path        []string
	Materialize bool
}

type jsonAccessProgramKind uint8

const (
	jsonAccessExists jsonAccessProgramKind = iota + 1
	jsonAccessExistsAny
	jsonAccessExistsAll
	jsonAccessFetchJSONPath
	jsonAccessFetchTextPath
)

// JSONAccessProgram incrementally computes one derived JSON result directly
// from subordinate JSON KVs for a single source column.
type JSONAccessProgram struct {
	kind jsonAccessProgramKind

	key    string
	keys   []string
	keySet map[string]struct{}
	path   []string

	sawSubordinate bool
	matched        bool
	matchedCount   int
	totalKeys      int
	matcher        *subordinateJSONPathMatcher
}

// JSONContainsProgram incrementally computes a JSON containment predicate over
// a fetched JSON column or JSON path using the subordinate JSON stream
// directly, without materializing the matched subtree for filter-only scans.
type JSONContainsProgram struct {
	matcher     *subordinateJSONContainsMatcher
	right       jsonutil.JSON
	pattern     jsonutil.ContainsPattern
	containedBy bool
	cacheKey    string
}

// SharedJSONAccessProgramKey canonicalizes the per-column work key used to
// share scan-local JSON access state across filters and projections.
func SharedJSONAccessProgramKey(spec JSONAccessSpec) string {
	var b strings.Builder
	b.Grow(32 + len(spec.Key) + len(spec.Keys)*8 + len(spec.Path)*8)
	fmt.Fprintf(&b, "%d/", spec.ColIdx)
	switch spec.Kind {
	case JSONAccessFetchJSONPath, JSONAccessFetchTextPath:
		b.WriteString("path|")
	default:
		fmt.Fprintf(&b, "%d/%s|", spec.Kind, spec.Key)
	}
	for _, key := range spec.Keys {
		b.WriteString(key)
		b.WriteByte(0)
	}
	b.WriteByte('|')
	for _, step := range spec.Path {
		b.WriteString(step)
		b.WriteByte(0)
	}
	return b.String()
}

// SharedJSONSelectedPathKey canonicalizes the per-column path-selection key
// used to share path resolution across scan-local JSON programs.
func SharedJSONSelectedPathKey(colIdx int, path []string) string {
	return SharedJSONAccessProgramKey(JSONAccessSpec{
		ColIdx: colIdx,
		Kind:   JSONAccessFetchJSONPath,
		Path:   path,
	})
}

func newJSONExistsProgram(key string) *JSONAccessProgram {
	return &JSONAccessProgram{
		kind: jsonAccessExists,
		key:  key,
	}
}

func newJSONFetchJSONPathProgram(path []string) *JSONAccessProgram {
	cp := append([]string(nil), path...)
	return &JSONAccessProgram{
		kind:    jsonAccessFetchJSONPath,
		path:    cp,
		matcher: newSubordinateJSONPathMatcher(cp),
	}
}

func newJSONFetchTextPathProgram(path []string) *JSONAccessProgram {
	cp := append([]string(nil), path...)
	return &JSONAccessProgram{
		kind:    jsonAccessFetchTextPath,
		path:    cp,
		matcher: newSubordinateJSONPathMatcher(cp),
	}
}

func newJSONExistsAnyProgram(keys []string) *JSONAccessProgram {
	cp := append([]string(nil), keys...)
	keySet := makeJSONAccessKeySet(cp)
	return &JSONAccessProgram{
		kind:      jsonAccessExistsAny,
		keys:      cp,
		keySet:    keySet,
		totalKeys: len(keySet),
	}
}

func newJSONExistsAllProgram(keys []string) *JSONAccessProgram {
	cp := append([]string(nil), keys...)
	keySet := makeJSONAccessKeySet(cp)
	return &JSONAccessProgram{
		kind:      jsonAccessExistsAll,
		keys:      cp,
		keySet:    keySet,
		totalKeys: len(keySet),
	}
}

func NewJSONAccessProgram(spec JSONAccessSpec) (*JSONAccessProgram, error) {
	switch spec.Kind {
	case JSONAccessExists:
		return newJSONExistsProgram(spec.Key), nil
	case JSONAccessExistsAny:
		return newJSONExistsAnyProgram(spec.Keys), nil
	case JSONAccessExistsAll:
		return newJSONExistsAllProgram(spec.Keys), nil
	case JSONAccessFetchJSONPath:
		return newJSONFetchJSONPathProgram(spec.Path), nil
	case JSONAccessFetchTextPath:
		return newJSONFetchTextPathProgram(spec.Path), nil
	default:
		return nil, errors.AssertionFailedf("unknown JSON access kind %d", spec.Kind)
	}
}

// NewJSONContainsProgram constructs a scan-local containment program over a
// fetched JSON column or JSON path.
func NewJSONContainsProgram(path []string, right jsonutil.JSON, containedBy bool) (*JSONContainsProgram, error) {
	matcher, err := newSubordinateJSONContainsMatcher(path, right, containedBy)
	if err != nil {
		return nil, err
	}
	pattern, err := jsonutil.NewContainsPattern(right)
	if err != nil {
		return nil, err
	}
	return &JSONContainsProgram{
		matcher:     matcher,
		right:       right,
		pattern:     pattern,
		containedBy: containedBy,
		cacheKey:    containmentCacheKey(containedBy, right),
	}, nil
}

func (p *JSONContainsProgram) Reset() {
	p.matcher.Reset()
}

func (p *JSONContainsProgram) Observe(
	path []keys.SubordinatePathSegment,
	kind rowenc.SubordinateJSONNodeKind,
	childCount int,
	j jsonutil.JSON,
) error {
	nodeKind, err := SubordinateJSONNodeKindFromEncoded(kind)
	if err != nil {
		return err
	}
	return p.matcher.Observe(path, nodeKind, childCount, j)
}

func (p *JSONContainsProgram) ObserveSelected(
	relPath []keys.SubordinatePathSegment,
	kind rowenc.SubordinateJSONNodeKind,
	childCount int,
	j jsonutil.JSON,
) error {
	nodeKind, err := SubordinateJSONNodeKindFromEncoded(kind)
	if err != nil {
		return err
	}
	p.matcher.sawSubordinate = true
	return p.matcher.ObserveSelected(relPath, nodeKind, childCount, j)
}

func (p *JSONContainsProgram) SawSubordinate() bool {
	return p.matcher.sawSubordinate
}

func (p *JSONContainsProgram) Passes() (bool, error) {
	return p.matcher.Result()
}

// EvaluateMaterialized applies the containment predicate to a materialized JSON
// subtree. This is used when another scan-local program already built the same
// selected path, so containment can reuse that subtree instead of keeping a
// second streamed matcher in lockstep.
func (p *JSONContainsProgram) EvaluateMaterialized(j jsonutil.JSON) (bool, error) {
	if p.containedBy {
		return p.pattern.Contains(j)
	}
	return jsonutil.ContainsWithPattern(j, p.pattern)
}

func containmentCacheKey(containedBy bool, right jsonutil.JSON) string {
	if containedBy {
		return "contained-by|" + right.String()
	}
	return "contains|" + right.String()
}

func (p *JSONAccessProgram) Reset() {
	p.sawSubordinate = false
	p.matched = false
	p.matchedCount = 0
	if p.totalKeys > 0 {
		p.keySet = makeJSONAccessKeySet(p.keys)
	}
	if p.matcher != nil {
		p.matcher = newSubordinateJSONPathMatcher(p.path)
	}
}

func (p *JSONAccessProgram) Observe(
	path []keys.SubordinatePathSegment,
	kind rowenc.SubordinateJSONNodeKind,
	childCount int,
	j jsonutil.JSON,
) error {
	p.sawSubordinate = true
	switch p.kind {
	case jsonAccessExists:
		return p.observeExists(path, kind, j)
	case jsonAccessExistsAny, jsonAccessExistsAll:
		return p.observeExistsSet(path, kind, j)
	case jsonAccessFetchJSONPath, jsonAccessFetchTextPath:
		return p.matcher.Observe(path, kind, childCount, j)
	default:
		return errors.AssertionFailedf("unknown JSON access program kind %d", p.kind)
	}
}

func (p *JSONAccessProgram) ObserveSelected(
	relPath []keys.SubordinatePathSegment,
	kind rowenc.SubordinateJSONNodeKind,
	childCount int,
	j jsonutil.JSON,
) error {
	p.sawSubordinate = true
	switch p.kind {
	case jsonAccessFetchJSONPath, jsonAccessFetchTextPath:
		return p.matcher.ObserveSelected(relPath, kind, childCount, j)
	default:
		return errors.AssertionFailedf("ObserveSelected called for non-path JSON access kind %d", p.kind)
	}
}

func (p *JSONAccessProgram) NeedsScalarAt(
	path []keys.SubordinatePathSegment, kind rowenc.SubordinateJSONNodeKind,
) bool {
	switch p.kind {
	case jsonAccessExists, jsonAccessExistsAny, jsonAccessExistsAll:
		return subordinateJSONExistsCandidateNeedsScalar(path, kind)
	default:
		return true
	}
}

func (p *JSONAccessProgram) ResultDatum() (tree.Datum, error) {
	switch p.kind {
	case jsonAccessFetchJSONPath:
		return p.ResultDatumForKind(JSONAccessFetchJSONPath)
	case jsonAccessFetchTextPath:
		return p.ResultDatumForKind(JSONAccessFetchTextPath)
	default:
		return p.resultDatumNonPath()
	}
}

func (p *JSONAccessProgram) resultDatumNonPath() (tree.Datum, error) {
	switch p.kind {
	case jsonAccessExists:
		if !p.sawSubordinate {
			return tree.DNull, nil
		}
		if p.matched {
			return tree.DBoolTrue, nil
		}
		return tree.DBoolFalse, nil
	case jsonAccessExistsAny:
		if !p.sawSubordinate {
			return tree.DNull, nil
		}
		if p.matched {
			return tree.DBoolTrue, nil
		}
		return tree.DBoolFalse, nil
	case jsonAccessExistsAll:
		if !p.sawSubordinate {
			return tree.DNull, nil
		}
		if p.matchedCount == p.totalKeys {
			return tree.DBoolTrue, nil
		}
		return tree.DBoolFalse, nil
	default:
		return nil, errors.AssertionFailedf("unknown JSON access program kind %d", p.kind)
	}
}

func (p *JSONAccessProgram) ResultDatumForKind(kind JSONAccessKind) (tree.Datum, error) {
	switch kind {
	case JSONAccessFetchJSONPath:
		j, err := p.matcher.Materialize()
		if err != nil {
			return nil, err
		}
		if j == nil {
			return tree.DNull, nil
		}
		return tree.NewDJSON(*j), nil
	case JSONAccessFetchTextPath:
		txt, err := p.matcher.MaterializeText()
		if err != nil {
			return nil, err
		}
		if txt == nil {
			return tree.DNull, nil
		}
		return tree.NewDString(*txt), nil
	default:
		return p.resultDatumNonPath()
	}
}

func (p *JSONAccessProgram) observeExists(
	path []keys.SubordinatePathSegment, kind rowenc.SubordinateJSONNodeKind, j jsonutil.JSON,
) error {
	candidate, ok, err := subordinateJSONExistsCandidate(path, kind, j)
	if err != nil {
		return err
	}
	if ok && candidate == p.key {
		p.matched = true
	}
	return nil
}

func (p *JSONAccessProgram) observeExistsSet(
	path []keys.SubordinatePathSegment, kind rowenc.SubordinateJSONNodeKind, j jsonutil.JSON,
) error {
	candidate, ok, err := subordinateJSONExistsCandidate(path, kind, j)
	if err != nil || !ok {
		return err
	}
	if _, exists := p.keySet[candidate]; exists {
		delete(p.keySet, candidate)
		p.matchedCount++
		p.matched = true
	}
	return nil
}

func subordinateJSONExistsCandidate(
	path []keys.SubordinatePathSegment, kind rowenc.SubordinateJSONNodeKind, j jsonutil.JSON,
) (string, bool, error) {
	path = normalizeObservedSubordinateJSONPath(path)
	if len(path) == 1 && path[0].Kind == keys.SubordinatePathHeader && kind == rowenc.SubordinateJSONScalar {
		txt, err := j.AsText()
		if err != nil {
			return "", false, err
		}
		if txt != nil {
			return *txt, true, nil
		}
		return "", false, nil
	}
	if len(path) != 1 {
		return "", false, nil
	}
	switch path[0].Kind {
	case keys.SubordinatePathObjectKey:
		return path[0].ObjectKey, true, nil
	case keys.SubordinatePathArrayIndex:
		if kind != rowenc.SubordinateJSONScalar {
			return "", false, nil
		}
		txt, err := j.AsText()
		if err != nil {
			return "", false, err
		}
		if txt != nil {
			return *txt, true, nil
		}
	}
	return "", false, nil
}

func subordinateJSONExistsCandidateNeedsScalar(
	path []keys.SubordinatePathSegment, kind rowenc.SubordinateJSONNodeKind,
) bool {
	path = normalizeObservedSubordinateJSONPath(path)
	if kind != rowenc.SubordinateJSONScalar {
		return false
	}
	if len(path) == 1 && path[0].Kind == keys.SubordinatePathHeader {
		return true
	}
	return len(path) == 1 && path[0].Kind == keys.SubordinatePathArrayIndex
}

func makeJSONAccessKeySet(keys []string) map[string]struct{} {
	if len(keys) == 0 {
		return nil
	}
	m := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		m[key] = struct{}{}
	}
	return m
}
