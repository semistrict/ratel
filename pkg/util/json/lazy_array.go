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

type lazyArray struct {
	length    int
	fetchElem func(ctx context.Context, idx int) (JSON, error)
	decodeAll func(ctx context.Context) (JSON, error)
	iterFn    func() ArrayValueIterator

	mu struct {
		syncutil.Mutex
		decoded JSON
	}
}

// ArrayValueIterator streams JSON array elements without requiring indexed
// lookups or decoding the whole array. Returned JSON values are only valid
// until the next call to NextValue or Close.
type ArrayValueIterator interface {
	NextValue(ctx context.Context) (JSON, bool, error)
	Close(ctx context.Context)
}

// ArrayValueIteratorFactory exposes a streaming iterator over JSON array
// elements when the underlying representation can provide one efficiently.
type ArrayValueIteratorFactory interface {
	ArrayValueIterator() ArrayValueIterator
}

// NewLazyArray constructs a JSON array value that can serve indexed lookups
// without eagerly decoding the whole array. All operations other than indexed
// access fall back to decodeAll on demand.
func NewLazyArray(
	length int,
	fetchElem func(ctx context.Context, idx int) (JSON, error),
	decodeAll func(ctx context.Context) (JSON, error),
) JSON {
	return NewLazyArrayWithIterator(length, fetchElem, decodeAll, nil /* iterFn */)
}

// NewLazyArrayWithIterator constructs a lazy JSON array that can optionally
// stream values sequentially without forcing indexed lookups.
func NewLazyArrayWithIterator(
	length int,
	fetchElem func(ctx context.Context, idx int) (JSON, error),
	decodeAll func(ctx context.Context) (JSON, error),
	iterFn func() ArrayValueIterator,
) JSON {
	return &lazyArray{
		length:    length,
		fetchElem: fetchElem,
		decodeAll: decodeAll,
		iterFn:    iterFn,
	}
}

func (j *lazyArray) decodeWithContext(ctx context.Context) (JSON, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.mu.decoded != nil {
		return j.mu.decoded, nil
	}
	decoded, err := j.decodeAll(ctx)
	if err != nil {
		return nil, err
	}
	if decoded == nil {
		return nil, errors.AssertionFailedf("lazy array decode returned nil")
	}
	if decoded.Type() != ArrayJSONType {
		return nil, errors.AssertionFailedf("lazy array decode returned non-array %s", decoded.Type())
	}
	j.mu.decoded = decoded
	return decoded, nil
}

func (j *lazyArray) decode() (JSON, error) {
	return j.decodeWithContext(context.Background())
}

func (j *lazyArray) MaybeDecode() JSON {
	decoded, err := j.decode()
	if err != nil {
		panic(err)
	}
	return decoded
}

func (j *lazyArray) tryDecode() (JSON, error) {
	return j.decode()
}

func (j *lazyArray) Type() Type {
	return ArrayJSONType
}

func (j *lazyArray) Len() int {
	return j.length
}

func (j *lazyArray) FetchValIdx(idx int) (JSON, error) {
	return j.FetchValIdxContext(context.Background(), idx)
}

// FetchValIdxContext performs indexed array lookup under the provided context
// without eagerly decoding the whole array.
func (j *lazyArray) FetchValIdxContext(ctx context.Context, idx int) (JSON, error) {
	if idx < 0 {
		idx = j.length + idx
	}
	if idx < 0 || idx >= j.length {
		return nil, nil
	}
	return j.fetchElem(ctx, idx)
}

func (j *lazyArray) FetchValKey(string) (JSON, error) {
	return nil, nil
}

func (j *lazyArray) FetchValKeyOrIdx(key string) (JSON, error) {
	idx, err := strconv.Atoi(key)
	if err != nil {
		return nil, nil //nolint:returnerrcheck
	}
	return j.FetchValIdx(idx)
}

func (j *lazyArray) ArrayValueIterator() ArrayValueIterator {
	if j.iterFn == nil {
		return nil
	}
	return j.iterFn()
}

func (j *lazyArray) Compare(other JSON) (int, error) {
	decoded, err := j.decode()
	if err != nil {
		return 0, err
	}
	return decoded.Compare(other)
}

func (j *lazyArray) Format(buf *bytes.Buffer) {
	j.MaybeDecode().Format(buf)
}

func (j *lazyArray) String() string {
	var buf bytes.Buffer
	j.Format(&buf)
	return buf.String()
}

func (j *lazyArray) Size() uintptr {
	decoded, err := j.decode()
	if err != nil {
		panic(err)
	}
	return decoded.Size()
}

func (j *lazyArray) encodeInvertedIndexKeys(b []byte) ([][]byte, error) {
	decoded, err := j.decode()
	if err != nil {
		return nil, err
	}
	return decoded.encodeInvertedIndexKeys(b)
}

func (j *lazyArray) encodeContainingInvertedIndexSpans(
	b []byte, isRoot, isObjectValue bool,
) (inverted.Expression, error) {
	decoded, err := j.decode()
	if err != nil {
		return nil, err
	}
	return decoded.encodeContainingInvertedIndexSpans(b, isRoot, isObjectValue)
}

func (j *lazyArray) encodeContainedInvertedIndexSpans(
	b []byte, isRoot, isObjectValue bool,
) (inverted.Expression, error) {
	decoded, err := j.decode()
	if err != nil {
		return nil, err
	}
	return decoded.encodeContainedInvertedIndexSpans(b, isRoot, isObjectValue)
}

func (j *lazyArray) numInvertedIndexEntries() (int, error) {
	decoded, err := j.decode()
	if err != nil {
		return 0, err
	}
	return decoded.numInvertedIndexEntries()
}

func (j *lazyArray) allPaths() ([]JSON, error) {
	decoded, err := j.decode()
	if err != nil {
		return nil, err
	}
	return decoded.allPaths()
}

func (j *lazyArray) RemoveString(s string) (JSON, bool, error) {
	decoded, err := j.decode()
	if err != nil {
		return nil, false, err
	}
	return decoded.RemoveString(s)
}

func (j *lazyArray) RemoveIndex(idx int) (JSON, bool, error) {
	decoded, err := j.decode()
	if err != nil {
		return nil, false, err
	}
	return decoded.RemoveIndex(idx)
}

func (j *lazyArray) RemovePath(path []string) (JSON, bool, error) {
	decoded, err := j.decode()
	if err != nil {
		return nil, false, err
	}
	return decoded.RemovePath(path)
}

func (j *lazyArray) doRemovePath(path []string) (JSON, bool, error) {
	decoded, err := j.decode()
	if err != nil {
		return nil, false, err
	}
	return decoded.doRemovePath(path)
}

func (j *lazyArray) Concat(other JSON) (JSON, error) {
	decoded, err := j.decode()
	if err != nil {
		return nil, err
	}
	return decoded.Concat(other)
}

func (j *lazyArray) AsText() (*string, error) {
	decoded, err := j.decode()
	if err != nil {
		return nil, err
	}
	return decoded.AsText()
}

func (j *lazyArray) AsDecimal() (*apd.Decimal, bool) {
	decoded, err := j.decode()
	if err != nil {
		panic(err)
	}
	return decoded.AsDecimal()
}

func (j *lazyArray) AsBool() (bool, bool) {
	decoded, err := j.decode()
	if err != nil {
		panic(err)
	}
	return decoded.AsBool()
}

func (j *lazyArray) Exists(key string) (bool, error) {
	decoded, err := j.decode()
	if err != nil {
		return false, err
	}
	return decoded.Exists(key)
}

func (j *lazyArray) StripNulls() (JSON, bool, error) {
	decoded, err := j.decode()
	if err != nil {
		return nil, false, err
	}
	return decoded.StripNulls()
}

func (j *lazyArray) ObjectIter() (*ObjectIterator, error) {
	decoded, err := j.decode()
	if err != nil {
		return nil, err
	}
	return decoded.ObjectIter()
}

func (j *lazyArray) isScalar() bool {
	return false
}

func (j *lazyArray) preprocessForContains() (containsable, error) {
	decoded, err := j.decode()
	if err != nil {
		return nil, err
	}
	return decoded.preprocessForContains()
}

func (j *lazyArray) encode(appendTo []byte) (jEntry, []byte, error) {
	decoded, err := j.decode()
	if err != nil {
		return jEntry{}, nil, err
	}
	return decoded.encode(appendTo)
}

func (j *lazyArray) toGoRepr() (interface{}, error) {
	decoded, err := j.decode()
	if err != nil {
		return nil, err
	}
	return decoded.toGoRepr()
}

func (j *lazyArray) HasContainerLeaf() (bool, error) {
	decoded, err := j.decode()
	if err != nil {
		return false, err
	}
	return decoded.HasContainerLeaf()
}
