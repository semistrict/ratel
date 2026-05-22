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
	"strconv"
	"strings"

	"github.com/cockroachdb/cockroach/pkg/keys"
	"github.com/cockroachdb/cockroach/pkg/sql/rowenc"
	jsonutil "github.com/cockroachdb/cockroach/pkg/util/json"
	"github.com/cockroachdb/errors"
)

const (
	jsonPathStepKeyPrefix        = "k:"
	jsonPathStepIndexPrefix      = "i:"
	jsonPathStepKeyOrIndexPrefix = "p:"
)

type subordinateJSONQueryPathStepKind uint8

const (
	subordinateJSONQueryPathKey subordinateJSONQueryPathStepKind = iota + 1
	subordinateJSONQueryPathIndex
	subordinateJSONQueryPathKeyOrIndex
)

type subordinateJSONQueryPathStep struct {
	kind  subordinateJSONQueryPathStepKind
	value string
	index int
}

// subordinateJSONPathMatcher incrementally reconstructs only the JSON subtree
// addressed by a path while subordinate JSON KVs are scanned in key order.
type subordinateJSONPathMatcher struct {
	selector *subordinateJSONPathSelector
	builder  *subordinateJSONBuilder
}

// JSONSelectedPathState incrementally resolves one scan-local JSON path and
// returns subtree-relative paths for matching subordinate JSON KVs.
type JSONSelectedPathState struct {
	encodedPath []string
	selector    *subordinateJSONPathSelector
}

type subordinateJSONPathSelector struct {
	queryPath     []subordinateJSONQueryPathStep
	staticTarget  []keys.SubordinatePathSegment
	targetPrefix  []keys.SubordinatePathSegment
	resolvedSteps int
	sawRoot       bool
	missing       bool
	parseErr      error
}

func newSubordinateJSONPathMatcher(path []string) *subordinateJSONPathMatcher {
	return &subordinateJSONPathMatcher{selector: newSubordinateJSONPathSelector(path)}
}

// NewJSONSelectedPathState constructs shared path-selection state for one JSON
// column/path pair.
func NewJSONSelectedPathState(path []string) *JSONSelectedPathState {
	cp := append([]string(nil), path...)
	return &JSONSelectedPathState{
		encodedPath: cp,
		selector:    newSubordinateJSONPathSelector(cp),
	}
}

// TryStaticSubordinateJSONPath decodes a scan-local JSON path into concrete
// subordinate key segments when that path can be addressed statically from KV
// keys alone. It returns ok=false for ambiguous key-or-index steps and for
// negative indexes, which require runtime container metadata.
func TryStaticSubordinateJSONPath(path []string) ([]keys.SubordinatePathSegment, bool, error) {
	segments := make([]keys.SubordinatePathSegment, 0, len(path))
	for _, enc := range path {
		step, err := decodeSubordinateJSONQueryPathStep(enc)
		if err != nil {
			return nil, false, err
		}
		switch step.kind {
		case subordinateJSONQueryPathKey:
			segments = append(segments, keys.SubordinatePathSegment{
				Kind:      keys.SubordinatePathObjectKey,
				ObjectKey: step.value,
			})
		case subordinateJSONQueryPathIndex:
			if step.index < 0 {
				return nil, false, nil
			}
			segments = append(segments, keys.SubordinatePathSegment{
				Kind:     keys.SubordinatePathArrayIndex,
				ArrayIdx: uint32(step.index),
			})
		case subordinateJSONQueryPathKeyOrIndex:
			if idx, err := strconv.Atoi(step.value); err == nil {
				if idx < 0 {
					return nil, false, nil
				}
				return nil, false, nil
			}
			segments = append(segments, keys.SubordinatePathSegment{
				Kind:      keys.SubordinatePathObjectKey,
				ObjectKey: step.value,
			})
		default:
			return nil, false, errors.AssertionFailedf("unknown subordinate JSON path step kind %d", step.kind)
		}
	}
	return segments, true, nil
}

// LongestStaticSubordinateJSONPathPrefix returns the longest path prefix that
// can be addressed directly from subordinate JSON keys alone. Unlike
// TryStaticSubordinateJSONPath, it stops at the first runtime-dependent step
// and returns the static prefix collected so far.
func LongestStaticSubordinateJSONPathPrefix(
	path []string,
) ([]keys.SubordinatePathSegment, bool, error) {
	segments := make([]keys.SubordinatePathSegment, 0, len(path))
	for _, enc := range path {
		step, err := decodeSubordinateJSONQueryPathStep(enc)
		if err != nil {
			return nil, false, err
		}
		switch step.kind {
		case subordinateJSONQueryPathKey:
			segments = append(segments, keys.SubordinatePathSegment{
				Kind:      keys.SubordinatePathObjectKey,
				ObjectKey: step.value,
			})
		case subordinateJSONQueryPathIndex:
			if step.index < 0 {
				return segments, true, nil
			}
			segments = append(segments, keys.SubordinatePathSegment{
				Kind:     keys.SubordinatePathArrayIndex,
				ArrayIdx: uint32(step.index),
			})
		case subordinateJSONQueryPathKeyOrIndex:
			if idx, err := strconv.Atoi(step.value); err == nil {
				if idx < 0 {
					return segments, true, nil
				}
				return segments, true, nil
			}
			segments = append(segments, keys.SubordinatePathSegment{
				Kind:      keys.SubordinatePathObjectKey,
				ObjectKey: step.value,
			})
		default:
			return nil, false, errors.AssertionFailedf("unknown subordinate JSON query path step kind %d", step.kind)
		}
	}
	return segments, true, nil
}

// Reset prepares the selector for a new row.
func (s *JSONSelectedPathState) Reset() {
	s.selector = newSubordinateJSONPathSelector(s.encodedPath)
}

// Select resolves one subordinate JSON KV against the configured path and, if
// it belongs to the selected subtree, returns the path relative to that
// subtree root.
func (s *JSONSelectedPathState) Select(
	path []keys.SubordinatePathSegment,
	kind rowenc.SubordinateJSONNodeKind,
	childCount int,
	scalar jsonutil.JSON,
) ([]keys.SubordinatePathSegment, bool, error) {
	return s.selector.Observe(path, kind, childCount, scalar)
}

func (s *JSONSelectedPathState) SelectPath(
	path []keys.SubordinatePathSegment, kind rowenc.SubordinateJSONNodeKind, childCount int,
) ([]keys.SubordinatePathSegment, bool, error) {
	return s.selector.Observe(path, kind, childCount, nil)
}

func newSubordinateJSONPathSelector(path []string) *subordinateJSONPathSelector {
	steps := make([]subordinateJSONQueryPathStep, 0, len(path))
	for _, enc := range path {
		step, err := decodeSubordinateJSONQueryPathStep(enc)
		if err != nil {
			return &subordinateJSONPathSelector{parseErr: err}
		}
		steps = append(steps, step)
	}
	selector := &subordinateJSONPathSelector{queryPath: steps}
	staticPath, ok, err := TryStaticSubordinateJSONPath(path)
	if err != nil {
		selector.parseErr = err
		return selector
	}
	if ok {
		selector.staticTarget = staticPath
	}
	return selector
}

func matchObservedSubordinateJSONStep(
	step subordinateJSONQueryPathStep, observed keys.SubordinatePathSegment,
) (keys.SubordinatePathSegment, bool, bool, error) {
	switch step.kind {
	case subordinateJSONQueryPathKey:
		if observed.Kind != keys.SubordinatePathObjectKey || observed.ObjectKey != step.value {
			return keys.SubordinatePathSegment{}, false, false, nil
		}
		return observed, true, false, nil
	case subordinateJSONQueryPathIndex:
		if step.index < 0 {
			return keys.SubordinatePathSegment{}, false, true, nil
		}
		if observed.Kind != keys.SubordinatePathArrayIndex || observed.ArrayIdx != uint32(step.index) {
			return keys.SubordinatePathSegment{}, false, false, nil
		}
		return observed, true, false, nil
	case subordinateJSONQueryPathKeyOrIndex:
		switch observed.Kind {
		case keys.SubordinatePathObjectKey:
			if observed.ObjectKey != step.value {
				return keys.SubordinatePathSegment{}, false, false, nil
			}
			return observed, true, false, nil
		case keys.SubordinatePathArrayIndex:
			idx, err := strconv.Atoi(step.value)
			if err != nil || idx < 0 || observed.ArrayIdx != uint32(idx) {
				return keys.SubordinatePathSegment{}, false, false, nil
			}
			return observed, true, false, nil
		default:
			return keys.SubordinatePathSegment{}, false, false, nil
		}
	default:
		return keys.SubordinatePathSegment{}, false, false,
			errors.AssertionFailedf("unknown subordinate JSON path step kind %d", step.kind)
	}
}

func (s *subordinateJSONPathSelector) tryDirectMatch(
	normalizedPath []keys.SubordinatePathSegment,
) ([]keys.SubordinatePathSegment, bool, error) {
	if len(normalizedPath) == 0 || len(s.queryPath) == 0 {
		return nil, false, nil
	}
	limit := len(normalizedPath)
	if limit > len(s.queryPath) {
		limit = len(s.queryPath)
	}
	resolved := make([]keys.SubordinatePathSegment, 0, limit)
	for i := 0; i < limit; i++ {
		seg, ok, unresolved, err := matchObservedSubordinateJSONStep(s.queryPath[i], normalizedPath[i])
		if err != nil {
			return nil, false, err
		}
		if unresolved || !ok {
			return nil, false, nil
		}
		resolved = append(resolved, seg)
	}
	if len(normalizedPath) >= len(s.queryPath) {
		relPath := []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}}
		if len(normalizedPath) > len(s.queryPath) {
			relPath = append(relPath, normalizedPath[len(s.queryPath):]...)
		}
		return relPath, true, nil
	}
	if len(resolved) > s.resolvedSteps {
		s.targetPrefix = append(s.targetPrefix[:0], resolved...)
		s.resolvedSteps = len(resolved)
	}
	return nil, false, nil
}

func (s *subordinateJSONPathSelector) Observe(
	path []keys.SubordinatePathSegment,
	kind rowenc.SubordinateJSONNodeKind,
	childCount int,
	_ jsonutil.JSON,
) ([]keys.SubordinatePathSegment, bool, error) {
	if s.parseErr != nil {
		return nil, false, s.parseErr
	}
	if s.missing {
		return nil, false, nil
	}

	if len(s.queryPath) == 0 {
		return append([]keys.SubordinatePathSegment(nil), path...), true, nil
	}
	normalizedPath := normalizeObservedSubordinateJSONPath(path)
	if s.staticTarget != nil {
		if !subordinatePathHasPrefix(normalizedPath, s.staticTarget) {
			return nil, false, nil
		}
		relPath := []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}}
		if len(normalizedPath) > len(s.staticTarget) {
			relPath = append(relPath, normalizedPath[len(s.staticTarget):]...)
		}
		return relPath, true, nil
	}
	if relPath, ok, err := s.tryDirectMatch(normalizedPath); err != nil || ok {
		return relPath, ok, err
	}

	if len(normalizedPath) == 1 && normalizedPath[0].Kind == keys.SubordinatePathHeader {
		s.sawRoot = true
		if s.resolvedSteps == 0 {
			seg, ok, err := resolveSubordinateJSONPathStep(kind, childCount, s.queryPath[0])
			if err != nil {
				return nil, false, err
			}
			if !ok {
				s.missing = true
				return nil, false, nil
			}
			s.targetPrefix = []keys.SubordinatePathSegment{seg}
			s.resolvedSteps = 1
		}
		return nil, false, nil
	}
	if s.resolvedSteps == 0 {
		return nil, false, nil
	}

	if s.resolvedSteps < len(s.queryPath) && subordinatePathEqual(normalizedPath, s.targetPrefix) {
		seg, ok, err := resolveSubordinateJSONPathStep(kind, childCount, s.queryPath[s.resolvedSteps])
		if err != nil {
			return nil, false, err
		}
		if !ok {
			s.missing = true
			return nil, false, nil
		}
		s.targetPrefix = append(s.targetPrefix[:len(s.targetPrefix):len(s.targetPrefix)], seg)
		s.resolvedSteps++
	}

	if s.resolvedSteps != len(s.queryPath) || !subordinatePathHasPrefix(normalizedPath, s.targetPrefix) {
		return nil, false, nil
	}
	relPath := []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}}
	if len(normalizedPath) > len(s.targetPrefix) {
		relPath = append(relPath, normalizedPath[len(s.targetPrefix):]...)
	}
	return relPath, true, nil
}

func (m *subordinateJSONPathMatcher) Observe(
	path []keys.SubordinatePathSegment,
	kind rowenc.SubordinateJSONNodeKind,
	childCount int,
	scalar jsonutil.JSON,
) error {
	relPath, ok, err := m.selector.Observe(path, kind, childCount, scalar)
	if err != nil || !ok {
		return err
	}
	return m.ObserveSelected(relPath, kind, childCount, scalar)
}

func (m *subordinateJSONPathMatcher) ObserveSelected(
	relPath []keys.SubordinatePathSegment,
	kind rowenc.SubordinateJSONNodeKind,
	_ int,
	scalar jsonutil.JSON,
) error {
	nodeKind, err := SubordinateJSONNodeKindFromEncoded(kind)
	if err != nil {
		return err
	}
	if m.builder == nil {
		m.builder = &subordinateJSONBuilder{}
	}
	return m.builder.Set(relPath, nodeKind, scalar)
}

func (m *subordinateJSONPathMatcher) Materialize() (*jsonutil.JSON, error) {
	if m.builder == nil {
		return nil, nil
	}
	j, err := m.builder.root.materialize()
	if err != nil {
		return nil, err
	}
	return &j, nil
}

func (m *subordinateJSONPathMatcher) MaterializeText() (*string, error) {
	j, err := m.Materialize()
	if err != nil || j == nil {
		return nil, err
	}
	return (*j).AsText()
}

func resolveSubordinateJSONPathStep(
	containerKind rowenc.SubordinateJSONNodeKind,
	childCount int,
	step subordinateJSONQueryPathStep,
) (keys.SubordinatePathSegment, bool, error) {
	switch containerKind {
	case rowenc.SubordinateJSONObject:
		if step.kind == subordinateJSONQueryPathIndex {
			return keys.SubordinatePathSegment{}, false, nil
		}
		return keys.SubordinatePathSegment{
			Kind:      keys.SubordinatePathObjectKey,
			ObjectKey: step.value,
		}, true, nil
	case rowenc.SubordinateJSONArray:
		if step.kind == subordinateJSONQueryPathKey {
			return keys.SubordinatePathSegment{}, false, nil
		}
		idx := step.index
		if idx < 0 {
			idx = childCount + idx
		}
		if idx < 0 || idx >= childCount {
			return keys.SubordinatePathSegment{}, false, nil
		}
		return keys.SubordinatePathSegment{
			Kind:     keys.SubordinatePathArrayIndex,
			ArrayIdx: uint32(idx),
		}, true, nil
	case rowenc.SubordinateJSONScalar:
		return keys.SubordinatePathSegment{}, false, nil
	default:
		return keys.SubordinatePathSegment{}, false, errors.AssertionFailedf("unknown subordinate JSON container kind %d", containerKind)
	}
}

func decodeSubordinateJSONQueryPathStep(enc string) (subordinateJSONQueryPathStep, error) {
	switch {
	case strings.HasPrefix(enc, jsonPathStepKeyPrefix):
		value, err := strconv.Unquote(strings.TrimPrefix(enc, jsonPathStepKeyPrefix))
		if err != nil {
			return subordinateJSONQueryPathStep{}, err
		}
		return subordinateJSONQueryPathStep{
			kind:  subordinateJSONQueryPathKey,
			value: value,
		}, nil
	case strings.HasPrefix(enc, jsonPathStepIndexPrefix):
		idx, err := strconv.Atoi(strings.TrimPrefix(enc, jsonPathStepIndexPrefix))
		if err != nil {
			return subordinateJSONQueryPathStep{}, err
		}
		return subordinateJSONQueryPathStep{
			kind:  subordinateJSONQueryPathIndex,
			index: idx,
		}, nil
	case strings.HasPrefix(enc, jsonPathStepKeyOrIndexPrefix):
		value, err := strconv.Unquote(strings.TrimPrefix(enc, jsonPathStepKeyOrIndexPrefix))
		if err != nil {
			return subordinateJSONQueryPathStep{}, err
		}
		step := subordinateJSONQueryPathStep{
			kind:  subordinateJSONQueryPathKeyOrIndex,
			value: value,
		}
		if idx, err := strconv.Atoi(value); err == nil {
			step.index = idx
		}
		return step, nil
	default:
		return subordinateJSONQueryPathStep{}, errors.AssertionFailedf("unknown subordinate JSON path step encoding %q", enc)
	}
}

func subordinatePathEqual(a, b []keys.SubordinatePathSegment) bool {
	if len(a) != len(b) {
		return false
	}
	return subordinatePathHasPrefix(a, b)
}

func subordinatePathHasPrefix(path, prefix []keys.SubordinatePathSegment) bool {
	if len(prefix) > len(path) {
		return false
	}
	for i := range prefix {
		if path[i].Kind != prefix[i].Kind ||
			path[i].ArrayIdx != prefix[i].ArrayIdx ||
			path[i].ObjectKey != prefix[i].ObjectKey {
			return false
		}
	}
	return true
}

func normalizeObservedSubordinateJSONPath(path []keys.SubordinatePathSegment) []keys.SubordinatePathSegment {
	if len(path) > 1 && path[0].Kind == keys.SubordinatePathHeader {
		return path[1:]
	}
	return path
}
