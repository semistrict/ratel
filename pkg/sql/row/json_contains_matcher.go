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
	"github.com/cockroachdb/errors"
	"github.com/semistrict/ratel/pkg/keys"
	"github.com/semistrict/ratel/pkg/sql/rowenc"
	jsonutil "github.com/semistrict/ratel/pkg/util/json"
)

type jsonContainmentMode uint8

const (
	jsonContainmentModeContains jsonContainmentMode = iota + 1
	jsonContainmentModeContainedBy
)

type jsonContainmentPatternKind uint8

const (
	jsonContainmentPatternScalar jsonContainmentPatternKind = iota + 1
	jsonContainmentPatternObject
	jsonContainmentPatternArray
)

type jsonContainmentPattern struct {
	kind jsonContainmentPatternKind

	scalar jsonutil.JSON

	objectKeys []string
	object     map[string]*jsonContainmentPattern

	arrayScalars []jsonutil.JSON
	arrayComplex []*jsonContainmentPattern
}

func newJSONContainmentPattern(j jsonutil.JSON) (*jsonContainmentPattern, error) {
	switch j.Type() {
	case jsonutil.ObjectJSONType:
		iter, err := j.ObjectIter()
		if err != nil {
			return nil, err
		}
		p := &jsonContainmentPattern{
			kind:   jsonContainmentPatternObject,
			object: make(map[string]*jsonContainmentPattern, j.Len()),
		}
		for iter.Next() {
			child, err := newJSONContainmentPattern(iter.Value())
			if err != nil {
				return nil, err
			}
			k := iter.Key()
			p.objectKeys = append(p.objectKeys, k)
			p.object[k] = child
		}
		return p, nil
	case jsonutil.ArrayJSONType:
		p := &jsonContainmentPattern{kind: jsonContainmentPatternArray}
		for i := 0; i < j.Len(); i++ {
			child, err := j.FetchValIdx(i)
			if err != nil {
				return nil, err
			}
			if child == nil {
				return nil, errors.AssertionFailedf("missing JSON array element %d", i)
			}
			if child.Type() == jsonutil.ArrayJSONType || child.Type() == jsonutil.ObjectJSONType {
				childPattern, err := newJSONContainmentPattern(child)
				if err != nil {
					return nil, err
				}
				p.arrayComplex = append(p.arrayComplex, childPattern)
			} else {
				p.arrayScalars = append(p.arrayScalars, child)
			}
		}
		return p, nil
	default:
		return &jsonContainmentPattern{
			kind:   jsonContainmentPatternScalar,
			scalar: j,
		}, nil
	}
}

type subordinateJSONContainsMatcher struct {
	encodedPath []string
	selector    *subordinateJSONPathSelector
	mode        jsonContainmentMode
	pattern     *jsonContainmentPattern

	sawSubordinate bool
	stack          []*subordinateJSONContainmentFrame
	result         bool
}

type subordinateJSONContainmentFrame struct {
	path       []keys.SubordinatePathSegment
	parent     *subordinateJSONContainmentFrame
	parentMeta any
	node       subordinateJSONContainmentNode
}

type subordinateJSONContainmentSpawn struct {
	pattern *jsonContainmentPattern
	mode    jsonContainmentMode
	meta    any
}

type subordinateJSONContainmentNode interface {
	observeSelf(kind subordinateJSONNodeKind, scalar jsonutil.JSON) error
	observeDescendant(
		path []keys.SubordinatePathSegment, kind subordinateJSONNodeKind, scalar jsonutil.JSON,
	) ([]subordinateJSONContainmentSpawn, error)
	onChildResult(meta any, matched bool) error
	finish() (bool, error)
}

func subordinateJSONDirectObjectKey(path []keys.SubordinatePathSegment) (string, bool) {
	if len(path) != 2 || path[1].Kind != keys.SubordinatePathObjectKey {
		return "", false
	}
	return path[1].ObjectKey, true
}

func subordinateJSONIsDirectArrayElement(path []keys.SubordinatePathSegment) bool {
	return len(path) == 2 && path[1].Kind == keys.SubordinatePathArrayIndex
}

func newSubordinateJSONContainsMatcher(
	path []string, right jsonutil.JSON, containedBy bool,
) (*subordinateJSONContainsMatcher, error) {
	pattern, err := newJSONContainmentPattern(right)
	if err != nil {
		return nil, err
	}
	mode := jsonContainmentModeContains
	if containedBy {
		mode = jsonContainmentModeContainedBy
	}
	return &subordinateJSONContainsMatcher{
		encodedPath: append([]string(nil), path...),
		selector:    newSubordinateJSONPathSelector(path),
		mode:        mode,
		pattern:     pattern,
	}, nil
}

func (m *subordinateJSONContainsMatcher) Reset() {
	m.sawSubordinate = false
	m.stack = m.stack[:0]
	m.result = false
	m.selector = newSubordinateJSONPathSelector(m.encodedPath)
}

func (m *subordinateJSONContainsMatcher) Observe(
	path []keys.SubordinatePathSegment,
	kind subordinateJSONNodeKind,
	childCount int,
	scalar jsonutil.JSON,
) error {
	m.sawSubordinate = true
	relPath, ok, err := m.selector.Observe(path, encodedSubordinateJSONNodeKind(kind), childCount, scalar)
	if err != nil || !ok {
		return err
	}
	return m.ObserveSelected(relPath, kind, childCount, scalar)
}

func (m *subordinateJSONContainsMatcher) ObserveSelected(
	relPath []keys.SubordinatePathSegment,
	kind subordinateJSONNodeKind,
	_ int,
	scalar jsonutil.JSON,
) error {
	for len(m.stack) > 0 && !subordinatePathHasPrefix(relPath, m.stack[len(m.stack)-1].path) {
		if err := m.finishTopFrame(); err != nil {
			return err
		}
	}
	if len(m.stack) == 0 {
		frame, err := newSubordinateJSONContainmentFrame(m.pattern, m.mode, relPath, kind, scalar, nil, nil)
		if err != nil {
			return err
		}
		m.stack = append(m.stack, frame)
		return nil
	}
	var spawned []*subordinateJSONContainmentFrame
	for _, frame := range m.stack {
		if !subordinatePathHasPrefix(relPath, frame.path) || subordinatePathEqual(relPath, frame.path) {
			continue
		}
		subPath := make([]keys.SubordinatePathSegment, 1, 1+len(relPath)-len(frame.path))
		subPath[0] = keys.SubordinatePathSegment{Kind: keys.SubordinatePathHeader}
		subPath = append(subPath, relPath[len(frame.path):]...)
		specs, err := frame.node.observeDescendant(subPath, kind, scalar)
		if err != nil {
			return err
		}
		for _, spec := range specs {
			child, err := newSubordinateJSONContainmentFrame(spec.pattern, spec.mode, relPath, kind, scalar, frame, spec.meta)
			if err != nil {
				return err
			}
			spawned = append(spawned, child)
		}
	}
	m.stack = append(m.stack, spawned...)
	return nil
}

func (m *subordinateJSONContainsMatcher) Result() (bool, error) {
	for len(m.stack) > 0 {
		if err := m.finishTopFrame(); err != nil {
			return false, err
		}
	}
	return m.result, nil
}

func (m *subordinateJSONContainsMatcher) finishTopFrame() error {
	top := m.stack[len(m.stack)-1]
	m.stack = m.stack[:len(m.stack)-1]
	matched, err := top.node.finish()
	if err != nil {
		return err
	}
	if top.parent == nil {
		m.result = matched
		return nil
	}
	return top.parent.node.onChildResult(top.parentMeta, matched)
}

func newSubordinateJSONContainmentFrame(
	pattern *jsonContainmentPattern,
	mode jsonContainmentMode,
	path []keys.SubordinatePathSegment,
	kind subordinateJSONNodeKind,
	scalar jsonutil.JSON,
	parent *subordinateJSONContainmentFrame,
	parentMeta any,
) (*subordinateJSONContainmentFrame, error) {
	node := newSubordinateJSONContainmentNode(pattern, mode)
	if err := node.observeSelf(kind, scalar); err != nil {
		return nil, err
	}
	framePath := append([]keys.SubordinatePathSegment(nil), path...)
	return &subordinateJSONContainmentFrame{
		path:       framePath,
		parent:     parent,
		parentMeta: parentMeta,
		node:       node,
	}, nil
}

func newSubordinateJSONContainmentNode(
	pattern *jsonContainmentPattern, mode jsonContainmentMode,
) subordinateJSONContainmentNode {
	switch mode {
	case jsonContainmentModeContains:
		switch pattern.kind {
		case jsonContainmentPatternScalar:
			return &subordinateJSONContainsScalarNode{pattern: pattern}
		case jsonContainmentPatternObject:
			return &subordinateJSONContainsObjectNode{
				children: map[string]*jsonContainmentPattern(pattern.object),
				matched:  make(map[string]bool, len(pattern.object)),
			}
		case jsonContainmentPatternArray:
			keys, err := subordinateJSONScalarKeys(pattern.arrayScalars)
			if err != nil {
				panic(err)
			}
			objectIdxs, arrayIdxs := subordinateJSONComplexPatternIndexes(pattern.arrayComplex)
			return &subordinateJSONContainsArrayNode{
				remainingScalarKeys: keys,
				remainingScalars:    len(keys),
				complex:             append([]*jsonContainmentPattern(nil), pattern.arrayComplex...),
				complexMatched:      make([]bool, len(pattern.arrayComplex)),
				objectIdxs:          objectIdxs,
				arrayIdxs:           arrayIdxs,
			}
		}
	case jsonContainmentModeContainedBy:
		switch pattern.kind {
		case jsonContainmentPatternScalar:
			return &subordinateJSONContainsScalarNode{pattern: pattern}
		case jsonContainmentPatternObject:
			return &subordinateJSONContainedByObjectNode{
				children: map[string]*jsonContainmentPattern(pattern.object),
			}
		case jsonContainmentPatternArray:
			set, err := subordinateJSONScalarSet(pattern.arrayScalars)
			if err != nil {
				panic(err)
			}
			objects, arrays := subordinateJSONComplexPatternBuckets(pattern.arrayComplex)
			return &subordinateJSONContainedByArrayNode{
				scalarSet: set,
				complex:   append([]*jsonContainmentPattern(nil), pattern.arrayComplex...),
				objects:   objects,
				arrays:    arrays,
				pending:   make(map[int]int),
				matched:   make(map[int]bool),
			}
		}
	}
	panic(errors.AssertionFailedf("unknown subordinate JSON containment mode %d / pattern kind %d", mode, pattern.kind))
}

type subordinateJSONContainsScalarNode struct {
	pattern *jsonContainmentPattern
	matched bool
}

func (n *subordinateJSONContainsScalarNode) observeSelf(kind subordinateJSONNodeKind, scalar jsonutil.JSON) error {
	if kind != subordinateJSONNodeScalar {
		n.matched = false
		return nil
	}
	eq, err := jsonEqual(scalar, n.pattern.scalar)
	if err != nil {
		return err
	}
	n.matched = eq
	return nil
}

func (n *subordinateJSONContainsScalarNode) observeDescendant(
	_ []keys.SubordinatePathSegment, _ subordinateJSONNodeKind, _ jsonutil.JSON,
) ([]subordinateJSONContainmentSpawn, error) {
	return nil, nil
}

func (n *subordinateJSONContainsScalarNode) onChildResult(_ any, _ bool) error { return nil }
func (n *subordinateJSONContainsScalarNode) finish() (bool, error)             { return n.matched, nil }

type subordinateJSONContainsObjectNode struct {
	rootOK   bool
	children map[string]*jsonContainmentPattern
	matched  map[string]bool
}

func (n *subordinateJSONContainsObjectNode) observeSelf(kind subordinateJSONNodeKind, _ jsonutil.JSON) error {
	n.rootOK = kind == subordinateJSONNodeObject
	return nil
}

func (n *subordinateJSONContainsObjectNode) observeDescendant(
	path []keys.SubordinatePathSegment, kind subordinateJSONNodeKind, scalar jsonutil.JSON,
) ([]subordinateJSONContainmentSpawn, error) {
	key, ok := subordinateJSONDirectObjectKey(path)
	if !n.rootOK || !ok {
		return nil, nil
	}
	child, ok := n.children[key]
	if !ok {
		return nil, nil
	}
	return []subordinateJSONContainmentSpawn{{
		pattern: child,
		mode:    jsonContainmentModeContains,
		meta:    key,
	}}, nil
}

func (n *subordinateJSONContainsObjectNode) onChildResult(meta any, matched bool) error {
	if matched {
		n.matched[meta.(string)] = true
	}
	return nil
}

func (n *subordinateJSONContainsObjectNode) finish() (bool, error) {
	if !n.rootOK {
		return false, nil
	}
	for key := range n.children {
		if !n.matched[key] {
			return false, nil
		}
	}
	return true, nil
}

type subordinateJSONContainedByObjectNode struct {
	rootOK   bool
	valid    bool
	children map[string]*jsonContainmentPattern
}

func (n *subordinateJSONContainedByObjectNode) observeSelf(kind subordinateJSONNodeKind, _ jsonutil.JSON) error {
	n.rootOK = kind == subordinateJSONNodeObject
	n.valid = n.rootOK
	return nil
}

func (n *subordinateJSONContainedByObjectNode) observeDescendant(
	path []keys.SubordinatePathSegment, _ subordinateJSONNodeKind, _ jsonutil.JSON,
) ([]subordinateJSONContainmentSpawn, error) {
	key, ok := subordinateJSONDirectObjectKey(path)
	if !n.valid || !ok {
		return nil, nil
	}
	child, ok := n.children[key]
	if !ok {
		n.valid = false
		return nil, nil
	}
	return []subordinateJSONContainmentSpawn{{
		pattern: child,
		mode:    jsonContainmentModeContainedBy,
		meta:    nil,
	}}, nil
}

func (n *subordinateJSONContainedByObjectNode) onChildResult(_ any, matched bool) error {
	if !matched {
		n.valid = false
	}
	return nil
}

func (n *subordinateJSONContainedByObjectNode) finish() (bool, error) {
	return n.rootOK && n.valid, nil
}

type subordinateJSONContainsArrayNode struct {
	rootOK              bool
	remainingScalarKeys map[string]bool
	remainingScalars    int
	complex             []*jsonContainmentPattern
	complexMatched      []bool
	objectIdxs          []int
	arrayIdxs           []int
}

func (n *subordinateJSONContainsArrayNode) observeSelf(kind subordinateJSONNodeKind, _ jsonutil.JSON) error {
	n.rootOK = kind == subordinateJSONNodeArray
	return nil
}

func (n *subordinateJSONContainsArrayNode) observeDescendant(
	path []keys.SubordinatePathSegment, kind subordinateJSONNodeKind, scalar jsonutil.JSON,
) ([]subordinateJSONContainmentSpawn, error) {
	if !n.rootOK || !subordinateJSONIsDirectArrayElement(path) {
		return nil, nil
	}
	if kind == subordinateJSONNodeScalar {
		key, err := subordinateJSONScalarKey(scalar)
		if err != nil {
			return nil, err
		}
		if n.remainingScalarKeys[key] {
			delete(n.remainingScalarKeys, key)
			n.remainingScalars--
		}
		return nil, nil
	}
	idxs := n.objectIdxs
	if kind == subordinateJSONNodeArray {
		idxs = n.arrayIdxs
	}
	spawns := make([]subordinateJSONContainmentSpawn, 0, len(idxs))
	for _, i := range idxs {
		if n.complexMatched[i] {
			continue
		}
		candidate := n.complex[i]
		spawns = append(spawns, subordinateJSONContainmentSpawn{
			pattern: candidate,
			mode:    jsonContainmentModeContains,
			meta:    i,
		})
	}
	return spawns, nil
}

func (n *subordinateJSONContainsArrayNode) onChildResult(meta any, matched bool) error {
	if matched {
		n.complexMatched[meta.(int)] = true
	}
	return nil
}

func (n *subordinateJSONContainsArrayNode) finish() (bool, error) {
	if !n.rootOK {
		return false, nil
	}
	if n.remainingScalars != 0 {
		return false, nil
	}
	for _, matched := range n.complexMatched {
		if !matched {
			return false, nil
		}
	}
	return true, nil
}

type subordinateJSONContainedByArrayNode struct {
	rootOK      bool
	valid       bool
	nextChildID int
	scalarSet   map[string]struct{}
	complex     []*jsonContainmentPattern
	objects     []*jsonContainmentPattern
	arrays      []*jsonContainmentPattern
	pending     map[int]int
	matched     map[int]bool
}

func (n *subordinateJSONContainedByArrayNode) observeSelf(kind subordinateJSONNodeKind, _ jsonutil.JSON) error {
	n.rootOK = kind == subordinateJSONNodeArray
	n.valid = n.rootOK
	return nil
}

func (n *subordinateJSONContainedByArrayNode) observeDescendant(
	path []keys.SubordinatePathSegment, kind subordinateJSONNodeKind, scalar jsonutil.JSON,
) ([]subordinateJSONContainmentSpawn, error) {
	if !n.valid || !subordinateJSONIsDirectArrayElement(path) {
		return nil, nil
	}
	if kind == subordinateJSONNodeScalar {
		key, err := subordinateJSONScalarKey(scalar)
		if err != nil {
			return nil, err
		}
		if _, ok := n.scalarSet[key]; ok {
			return nil, nil
		}
		n.valid = false
		return nil, nil
	}
	childID := n.nextChildID
	n.nextChildID++
	candidates := n.objects
	if kind == subordinateJSONNodeArray {
		candidates = n.arrays
	}
	spawns := make([]subordinateJSONContainmentSpawn, 0, len(candidates))
	for _, candidate := range candidates {
		spawns = append(spawns, subordinateJSONContainmentSpawn{
			pattern: candidate,
			mode:    jsonContainmentModeContainedBy,
			meta:    childID,
		})
	}
	if len(spawns) == 0 {
		n.valid = false
		return nil, nil
	}
	n.pending[childID] = len(spawns)
	return spawns, nil
}

func (n *subordinateJSONContainedByArrayNode) onChildResult(meta any, matched bool) error {
	childID := meta.(int)
	if matched {
		n.matched[childID] = true
	}
	n.pending[childID]--
	if n.pending[childID] == 0 {
		delete(n.pending, childID)
		if !n.matched[childID] {
			n.valid = false
		}
		delete(n.matched, childID)
	}
	return nil
}

func (n *subordinateJSONContainedByArrayNode) finish() (bool, error) {
	return n.rootOK && n.valid, nil
}

func patternMatchesNodeKind(pattern *jsonContainmentPattern, kind subordinateJSONNodeKind) bool {
	switch pattern.kind {
	case jsonContainmentPatternScalar:
		return kind == subordinateJSONNodeScalar
	case jsonContainmentPatternObject:
		return kind == subordinateJSONNodeObject
	case jsonContainmentPatternArray:
		return kind == subordinateJSONNodeArray
	default:
		return false
	}
}

func subordinateJSONScalarKey(j jsonutil.JSON) (string, error) {
	if j.Type() == jsonutil.ArrayJSONType || j.Type() == jsonutil.ObjectJSONType {
		return "", errors.AssertionFailedf("non-scalar JSON used as scalar containment key: %s", j.Type())
	}
	return j.String(), nil
}

func subordinateJSONScalarKeys(vals []jsonutil.JSON) (map[string]bool, error) {
	if len(vals) == 0 {
		return nil, nil
	}
	keys := make(map[string]bool, len(vals))
	for _, v := range vals {
		key, err := subordinateJSONScalarKey(v)
		if err != nil {
			return nil, err
		}
		keys[key] = true
	}
	return keys, nil
}

func subordinateJSONScalarSet(vals []jsonutil.JSON) (map[string]struct{}, error) {
	if len(vals) == 0 {
		return nil, nil
	}
	set := make(map[string]struct{}, len(vals))
	for _, v := range vals {
		key, err := subordinateJSONScalarKey(v)
		if err != nil {
			return nil, err
		}
		set[key] = struct{}{}
	}
	return set, nil
}

func subordinateJSONComplexPatternIndexes(patterns []*jsonContainmentPattern) ([]int, []int) {
	var objects []int
	var arrays []int
	for i, pattern := range patterns {
		switch pattern.kind {
		case jsonContainmentPatternObject:
			objects = append(objects, i)
		case jsonContainmentPatternArray:
			arrays = append(arrays, i)
		}
	}
	return objects, arrays
}

func subordinateJSONComplexPatternBuckets(
	patterns []*jsonContainmentPattern,
) ([]*jsonContainmentPattern, []*jsonContainmentPattern) {
	var objects []*jsonContainmentPattern
	var arrays []*jsonContainmentPattern
	for _, pattern := range patterns {
		switch pattern.kind {
		case jsonContainmentPatternObject:
			objects = append(objects, pattern)
		case jsonContainmentPatternArray:
			arrays = append(arrays, pattern)
		}
	}
	return objects, arrays
}

func jsonEqual(a, b jsonutil.JSON) (bool, error) {
	cmp, err := a.Compare(b)
	if err != nil {
		return false, err
	}
	return cmp == 0, nil
}

func encodedSubordinateJSONNodeKind(kind subordinateJSONNodeKind) rowenc.SubordinateJSONNodeKind {
	switch kind {
	case subordinateJSONNodeScalar:
		return rowenc.SubordinateJSONScalar
	case subordinateJSONNodeObject:
		return rowenc.SubordinateJSONObject
	case subordinateJSONNodeArray:
		return rowenc.SubordinateJSONArray
	default:
		panic(errors.AssertionFailedf("unknown subordinate JSON node kind %d", kind))
	}
}
