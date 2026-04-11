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

package json

import (
	"bytes"
	"context"
	"strconv"

	"github.com/cockroachdb/apd/v3"
	"github.com/cockroachdb/errors"
	"github.com/semistrict/ratel/pkg/sql/inverted"
	"github.com/semistrict/ratel/pkg/util/syncutil"
)

// MutableLazyNodeDriver supplies keyed/indexed access and full decode for a
// MutableLazyNode. Implementations may mutate their backing state between
// iterator steps; callers must not retain values returned from a
// MutableLazyNode-backed iterator beyond the next iterator advance.
type MutableLazyNodeDriver interface {
	FetchValKeyContext(ctx context.Context, key string) (JSON, error)
	FetchValIdxContext(ctx context.Context, idx int) (JSON, error)
	DecodeContext(ctx context.Context) (JSON, error)
}

// MutableLazyNode is a reusable JSON wrapper whose logical value can be reset
// between iterator steps without reallocating a new object/array wrapper each
// time.
type MutableLazyNode struct {
	typ    Type
	length int
	scalar JSON
	driver MutableLazyNodeDriver

	mu struct {
		syncutil.Mutex
		decoded JSON
	}
}

// NewMutableLazyNode constructs a reusable lazy JSON wrapper driven by d.
func NewMutableLazyNode(d MutableLazyNodeDriver) *MutableLazyNode {
	return &MutableLazyNode{driver: d}
}

// ResetScalar updates the node to represent the provided scalar JSON value.
func (j *MutableLazyNode) ResetScalar(v JSON) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.scalar = v
	j.typ = v.Type()
	j.length = 0
	j.mu.decoded = nil
}

// ResetArray updates the node to represent a JSON array of the provided length.
func (j *MutableLazyNode) ResetArray(length int) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.scalar = nil
	j.typ = ArrayJSONType
	j.length = length
	j.mu.decoded = nil
}

// ResetObject updates the node to represent a JSON object of the provided length.
func (j *MutableLazyNode) ResetObject(length int) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.scalar = nil
	j.typ = ObjectJSONType
	j.length = length
	j.mu.decoded = nil
}

func (j *MutableLazyNode) decodeWithContext(ctx context.Context) (JSON, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.scalar != nil {
		return j.scalar, nil
	}
	if j.mu.decoded != nil {
		return j.mu.decoded, nil
	}
	decoded, err := j.driver.DecodeContext(ctx)
	if err != nil {
		return nil, err
	}
	if decoded == nil {
		return nil, errors.AssertionFailedf("mutable lazy node decode returned nil")
	}
	if decoded.Type() != j.typ {
		return nil, errors.AssertionFailedf("mutable lazy node decode returned %s, expected %s", decoded.Type(), j.typ)
	}
	j.mu.decoded = decoded
	return decoded, nil
}

func (j *MutableLazyNode) decode() (JSON, error) {
	return j.decodeWithContext(context.Background())
}

func (j *MutableLazyNode) MaybeDecode() JSON {
	decoded, err := j.decode()
	if err != nil {
		panic(err)
	}
	return decoded
}

func (j *MutableLazyNode) tryDecode() (JSON, error) {
	return j.decode()
}

func (j *MutableLazyNode) Type() Type {
	return j.typ
}

func (j *MutableLazyNode) Len() int {
	switch j.typ {
	case ArrayJSONType, ObjectJSONType:
		return j.length
	default:
		return 0
	}
}

func (j *MutableLazyNode) FetchValKey(key string) (JSON, error) {
	if j.typ != ObjectJSONType {
		return nil, nil
	}
	return j.driver.FetchValKeyContext(context.Background(), key)
}

func (j *MutableLazyNode) FetchValIdx(idx int) (JSON, error) {
	if j.typ != ArrayJSONType {
		return nil, nil
	}
	if idx < 0 {
		idx = j.length + idx
	}
	if idx < 0 || idx >= j.length {
		return nil, nil
	}
	return j.driver.FetchValIdxContext(context.Background(), idx)
}

func (j *MutableLazyNode) FetchValKeyOrIdx(key string) (JSON, error) {
	switch j.typ {
	case ObjectJSONType:
		return j.FetchValKey(key)
	case ArrayJSONType:
		idx, err := strconv.Atoi(key)
		if err != nil {
			return nil, nil //nolint:returnerrcheck
		}
		return j.FetchValIdx(idx)
	default:
		return nil, nil
	}
}

func (j *MutableLazyNode) Compare(other JSON) (int, error) {
	decoded, err := j.decode()
	if err != nil {
		return 0, err
	}
	return decoded.Compare(other)
}

func (j *MutableLazyNode) Format(buf *bytes.Buffer) {
	j.MaybeDecode().Format(buf)
}

func (j *MutableLazyNode) String() string {
	var buf bytes.Buffer
	j.Format(&buf)
	return buf.String()
}

func (j *MutableLazyNode) Size() uintptr {
	decoded, err := j.decode()
	if err != nil {
		panic(err)
	}
	return decoded.Size()
}

func (j *MutableLazyNode) encodeInvertedIndexKeys(b []byte) ([][]byte, error) {
	decoded, err := j.decode()
	if err != nil {
		return nil, err
	}
	return decoded.encodeInvertedIndexKeys(b)
}

func (j *MutableLazyNode) encodeContainingInvertedIndexSpans(
	b []byte, isRoot, isObjectValue bool,
) (inverted.Expression, error) {
	decoded, err := j.decode()
	if err != nil {
		return nil, err
	}
	return decoded.encodeContainingInvertedIndexSpans(b, isRoot, isObjectValue)
}

func (j *MutableLazyNode) encodeContainedInvertedIndexSpans(
	b []byte, isRoot, isObjectValue bool,
) (inverted.Expression, error) {
	decoded, err := j.decode()
	if err != nil {
		return nil, err
	}
	return decoded.encodeContainedInvertedIndexSpans(b, isRoot, isObjectValue)
}

func (j *MutableLazyNode) numInvertedIndexEntries() (int, error) {
	decoded, err := j.decode()
	if err != nil {
		return 0, err
	}
	return decoded.numInvertedIndexEntries()
}

func (j *MutableLazyNode) allPaths() ([]JSON, error) {
	decoded, err := j.decode()
	if err != nil {
		return nil, err
	}
	return decoded.allPaths()
}

func (j *MutableLazyNode) RemoveString(s string) (JSON, bool, error) {
	decoded, err := j.decode()
	if err != nil {
		return nil, false, err
	}
	return decoded.RemoveString(s)
}

func (j *MutableLazyNode) RemoveIndex(idx int) (JSON, bool, error) {
	decoded, err := j.decode()
	if err != nil {
		return nil, false, err
	}
	return decoded.RemoveIndex(idx)
}

func (j *MutableLazyNode) RemovePath(path []string) (JSON, bool, error) {
	decoded, err := j.decode()
	if err != nil {
		return nil, false, err
	}
	return decoded.RemovePath(path)
}

func (j *MutableLazyNode) doRemovePath(path []string) (JSON, bool, error) {
	decoded, err := j.decode()
	if err != nil {
		return nil, false, err
	}
	return decoded.doRemovePath(path)
}

func (j *MutableLazyNode) Concat(other JSON) (JSON, error) {
	decoded, err := j.decode()
	if err != nil {
		return nil, err
	}
	return decoded.Concat(other)
}

func (j *MutableLazyNode) AsText() (*string, error) {
	decoded, err := j.decode()
	if err != nil {
		return nil, err
	}
	return decoded.AsText()
}

func (j *MutableLazyNode) AsDecimal() (*apd.Decimal, bool) {
	decoded, err := j.decode()
	if err != nil {
		panic(err)
	}
	return decoded.AsDecimal()
}

func (j *MutableLazyNode) AsBool() (bool, bool) {
	decoded, err := j.decode()
	if err != nil {
		panic(err)
	}
	return decoded.AsBool()
}

func (j *MutableLazyNode) Exists(key string) (bool, error) {
	if j.typ == ObjectJSONType {
		val, err := j.FetchValKey(key)
		return val != nil, err
	}
	decoded, err := j.decode()
	if err != nil {
		return false, err
	}
	return decoded.Exists(key)
}

func (j *MutableLazyNode) StripNulls() (JSON, bool, error) {
	decoded, err := j.decode()
	if err != nil {
		return nil, false, err
	}
	return decoded.StripNulls()
}

func (j *MutableLazyNode) ObjectIter() (*ObjectIterator, error) {
	decoded, err := j.decode()
	if err != nil {
		return nil, err
	}
	return decoded.ObjectIter()
}

func (j *MutableLazyNode) isScalar() bool {
	return j.typ != ArrayJSONType && j.typ != ObjectJSONType
}

func (j *MutableLazyNode) preprocessForContains() (containsable, error) {
	decoded, err := j.decode()
	if err != nil {
		return nil, err
	}
	return decoded.preprocessForContains()
}

func (j *MutableLazyNode) encode(appendTo []byte) (jEntry, []byte, error) {
	decoded, err := j.decode()
	if err != nil {
		return jEntry{}, nil, err
	}
	return decoded.encode(appendTo)
}

func (j *MutableLazyNode) toGoRepr() (interface{}, error) {
	decoded, err := j.decode()
	if err != nil {
		return nil, err
	}
	return decoded.toGoRepr()
}

func (j *MutableLazyNode) HasContainerLeaf() (bool, error) {
	decoded, err := j.decode()
	if err != nil {
		return false, err
	}
	return decoded.HasContainerLeaf()
}
