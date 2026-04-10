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
	"sort"

	"github.com/cockroachdb/errors"
	"github.com/semistrict/ratel/pkg/keys"
	"github.com/semistrict/ratel/pkg/sql/rowenc"
	"github.com/semistrict/ratel/pkg/sql/sem/tree"
	"github.com/semistrict/ratel/pkg/sql/types"
	jsonutil "github.com/semistrict/ratel/pkg/util/json"
)

type SubordinateArrayBuilder struct {
	elemType *types.T
	elems    tree.Datums
}

type SubordinateJSONNodeKind byte

const (
	SubordinateJSONNodeScalar SubordinateJSONNodeKind = iota + 1
	SubordinateJSONNodeObject
	SubordinateJSONNodeArray
)

type subordinateJSONNodeBuilder struct {
	kind     SubordinateJSONNodeKind
	scalar   jsonutil.JSON
	object   map[string]*subordinateJSONNodeBuilder
	array    map[int]*subordinateJSONNodeBuilder
	hasValue bool
}

type SubordinateJSONBuilder struct {
	root subordinateJSONNodeBuilder
}

func NewSubordinateArrayBuilder(elemType *types.T) *SubordinateArrayBuilder {
	return &SubordinateArrayBuilder{elemType: elemType}
}

func (b *SubordinateArrayBuilder) Set(elemIdx int, value tree.Datum) {
	if elemIdx >= len(b.elems) {
		elems := make(tree.Datums, elemIdx+1)
		copy(elems, b.elems)
		b.elems = elems
	}
	b.elems[elemIdx] = value
}

func (b *SubordinateArrayBuilder) Materialize() (*tree.DArray, error) {
	arr := tree.NewDArray(b.elemType)
	for i, elem := range b.elems {
		if elem == nil {
			return nil, errors.AssertionFailedf("missing subordinate array element %d", i)
		}
		if err := arr.Append(elem); err != nil {
			return nil, err
		}
	}
	return arr, nil
}

func (b *SubordinateJSONBuilder) Set(
	path []keys.SubordinatePathSegment, kind SubordinateJSONNodeKind, scalar jsonutil.JSON,
) error {
	node := &b.root
	for _, seg := range path {
		switch seg.Kind {
		case keys.SubordinatePathHeader:
			continue
		case keys.SubordinatePathObjectKey:
			if node.object == nil {
				node.object = make(map[string]*subordinateJSONNodeBuilder)
			}
			child := node.object[seg.ObjectKey]
			if child == nil {
				child = &subordinateJSONNodeBuilder{}
				node.object[seg.ObjectKey] = child
			}
			node = child
		case keys.SubordinatePathArrayIndex:
			if node.array == nil {
				node.array = make(map[int]*subordinateJSONNodeBuilder)
			}
			child := node.array[int(seg.ArrayIdx)]
			if child == nil {
				child = &subordinateJSONNodeBuilder{}
				node.array[int(seg.ArrayIdx)] = child
			}
			node = child
		default:
			return errors.AssertionFailedf("unknown subordinate JSON path segment kind %d", seg.Kind)
		}
	}
	node.kind = kind
	node.scalar = scalar
	node.hasValue = true
	return nil
}

func (n *subordinateJSONNodeBuilder) materialize() (jsonutil.JSON, error) {
	if !n.hasValue {
		return nil, errors.AssertionFailedf("missing subordinate JSON node value")
	}
	switch n.kind {
	case SubordinateJSONNodeScalar:
		if n.scalar == nil {
			return nil, errors.AssertionFailedf("missing subordinate JSON scalar value")
		}
		return n.scalar, nil
	case SubordinateJSONNodeObject:
		keys := make([]string, 0, len(n.object))
		for k := range n.object {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		builder := jsonutil.NewObjectBuilder(len(keys))
		for _, k := range keys {
			child, err := n.object[k].materialize()
			if err != nil {
				return nil, err
			}
			builder.Add(k, child)
		}
		return builder.Build(), nil
	case SubordinateJSONNodeArray:
		maxIdx := -1
		for idx := range n.array {
			if idx > maxIdx {
				maxIdx = idx
			}
		}
		builder := jsonutil.NewArrayBuilder(maxIdx + 1)
		for i := 0; i <= maxIdx; i++ {
			child := n.array[i]
			if child == nil {
				return nil, errors.AssertionFailedf("missing subordinate JSON array element %d", i)
			}
			j, err := child.materialize()
			if err != nil {
				return nil, err
			}
			builder.Add(j)
		}
		return builder.Build(), nil
	default:
		return nil, errors.AssertionFailedf("unknown subordinate JSON node kind %d", n.kind)
	}
}

func (b *SubordinateJSONBuilder) Materialize() (*tree.DJSON, error) {
	j, err := b.root.materialize()
	if err != nil {
		return nil, err
	}
	return tree.NewDJSON(j), nil
}

func SubordinateJSONNodeKindFromEncoded(
	kind rowenc.SubordinateJSONNodeKind,
) (SubordinateJSONNodeKind, error) {
	switch kind {
	case rowenc.SubordinateJSONScalar:
		return SubordinateJSONNodeScalar, nil
	case rowenc.SubordinateJSONObject:
		return SubordinateJSONNodeObject, nil
	case rowenc.SubordinateJSONArray:
		return SubordinateJSONNodeArray, nil
	default:
		return 0, errors.AssertionFailedf("unknown subordinate JSON node kind %d", kind)
	}
}
