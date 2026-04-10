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

	"github.com/cockroachdb/errors"
	"github.com/semistrict/ratel/pkg/keys"
	"github.com/semistrict/ratel/pkg/sql/rowenc"
	jsonutil "github.com/semistrict/ratel/pkg/util/json"
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

func newSubordinateJSONPathSelector(path []string) *subordinateJSONPathSelector {
	steps := make([]subordinateJSONQueryPathStep, 0, len(path))
	for _, enc := range path {
		step, err := decodeSubordinateJSONQueryPathStep(enc)
		if err != nil {
			return &subordinateJSONPathSelector{parseErr: err}
		}
		steps = append(steps, step)
	}
	return &subordinateJSONPathSelector{queryPath: steps}
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

	if len(path) == 1 && path[0].Kind == keys.SubordinatePathHeader {
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
	if !s.sawRoot {
		return nil, false, errors.AssertionFailedf("subordinate JSON child encountered before root header")
	}

	if s.resolvedSteps < len(s.queryPath) && subordinatePathEqual(path, s.targetPrefix) {
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

	if s.resolvedSteps != len(s.queryPath) || !subordinatePathHasPrefix(path, s.targetPrefix) {
		return nil, false, nil
	}
	relPath := []keys.SubordinatePathSegment{{Kind: keys.SubordinatePathHeader}}
	if len(path) > len(s.targetPrefix) {
		relPath = append(relPath, path[len(s.targetPrefix):]...)
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
