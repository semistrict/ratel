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

	"github.com/cockroachdb/apd/v3"
	"github.com/cockroachdb/errors"
	"github.com/semistrict/ratel/pkg/sql/inverted"
	"github.com/semistrict/ratel/pkg/util/syncutil"
)

type lazyObject struct {
	length    int
	fetchKey  func(ctx context.Context, key string) (JSON, error)
	decodeAll func(ctx context.Context) (JSON, error)

	mu struct {
		syncutil.Mutex
		decoded JSON
	}
}

// NewLazyObject constructs a JSON object value that can serve keyed lookups
// without eagerly decoding the whole object. All operations other than keyed
// access fall back to decodeAll on demand.
func NewLazyObject(
	length int,
	fetchKey func(ctx context.Context, key string) (JSON, error),
	decodeAll func(ctx context.Context) (JSON, error),
) JSON {
	return &lazyObject{
		length:    length,
		fetchKey:  fetchKey,
		decodeAll: decodeAll,
	}
}

func (j *lazyObject) decodeWithContext(ctx context.Context) (JSON, error) {
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
		return nil, errors.AssertionFailedf("lazy object decode returned nil")
	}
	if decoded.Type() != ObjectJSONType {
		return nil, errors.AssertionFailedf("lazy object decode returned non-object %s", decoded.Type())
	}
	j.mu.decoded = decoded
	return decoded, nil
}

func (j *lazyObject) decode() (JSON, error) {
	return j.decodeWithContext(context.Background())
}

func (j *lazyObject) MaybeDecode() JSON {
	decoded, err := j.decode()
	if err != nil {
		panic(err)
	}
	return decoded
}

func (j *lazyObject) tryDecode() (JSON, error) {
	return j.decode()
}

func (j *lazyObject) Type() Type {
	return ObjectJSONType
}

func (j *lazyObject) Len() int {
	return j.length
}

func (j *lazyObject) FetchValKey(key string) (JSON, error) {
	return j.fetchKey(context.Background(), key)
}

func (j *lazyObject) FetchValIdx(int) (JSON, error) {
	return nil, nil
}

func (j *lazyObject) FetchValKeyOrIdx(key string) (JSON, error) {
	return j.FetchValKey(key)
}

func (j *lazyObject) Compare(other JSON) (int, error) {
	decoded, err := j.decode()
	if err != nil {
		return 0, err
	}
	return decoded.Compare(other)
}

func (j *lazyObject) Format(buf *bytes.Buffer) {
	j.MaybeDecode().Format(buf)
}

func (j *lazyObject) String() string {
	var buf bytes.Buffer
	j.Format(&buf)
	return buf.String()
}

func (j *lazyObject) Size() uintptr {
	decoded, err := j.decode()
	if err != nil {
		panic(err)
	}
	return decoded.Size()
}

func (j *lazyObject) encodeInvertedIndexKeys(b []byte) ([][]byte, error) {
	decoded, err := j.decode()
	if err != nil {
		return nil, err
	}
	return decoded.encodeInvertedIndexKeys(b)
}

func (j *lazyObject) encodeContainingInvertedIndexSpans(
	b []byte, isRoot, isObjectValue bool,
) (inverted.Expression, error) {
	decoded, err := j.decode()
	if err != nil {
		return nil, err
	}
	return decoded.encodeContainingInvertedIndexSpans(b, isRoot, isObjectValue)
}

func (j *lazyObject) encodeContainedInvertedIndexSpans(
	b []byte, isRoot, isObjectValue bool,
) (inverted.Expression, error) {
	decoded, err := j.decode()
	if err != nil {
		return nil, err
	}
	return decoded.encodeContainedInvertedIndexSpans(b, isRoot, isObjectValue)
}

func (j *lazyObject) numInvertedIndexEntries() (int, error) {
	decoded, err := j.decode()
	if err != nil {
		return 0, err
	}
	return decoded.numInvertedIndexEntries()
}

func (j *lazyObject) allPaths() ([]JSON, error) {
	decoded, err := j.decode()
	if err != nil {
		return nil, err
	}
	return decoded.allPaths()
}

func (j *lazyObject) RemoveString(s string) (JSON, bool, error) {
	decoded, err := j.decode()
	if err != nil {
		return nil, false, err
	}
	return decoded.RemoveString(s)
}

func (j *lazyObject) RemoveIndex(idx int) (JSON, bool, error) {
	decoded, err := j.decode()
	if err != nil {
		return nil, false, err
	}
	return decoded.RemoveIndex(idx)
}

func (j *lazyObject) RemovePath(path []string) (JSON, bool, error) {
	decoded, err := j.decode()
	if err != nil {
		return nil, false, err
	}
	return decoded.RemovePath(path)
}

func (j *lazyObject) doRemovePath(path []string) (JSON, bool, error) {
	decoded, err := j.decode()
	if err != nil {
		return nil, false, err
	}
	return decoded.doRemovePath(path)
}

func (j *lazyObject) Concat(other JSON) (JSON, error) {
	decoded, err := j.decode()
	if err != nil {
		return nil, err
	}
	return decoded.Concat(other)
}

func (j *lazyObject) AsText() (*string, error) {
	decoded, err := j.decode()
	if err != nil {
		return nil, err
	}
	return decoded.AsText()
}

func (j *lazyObject) AsDecimal() (*apd.Decimal, bool) {
	decoded, err := j.decode()
	if err != nil {
		panic(err)
	}
	return decoded.AsDecimal()
}

func (j *lazyObject) AsBool() (bool, bool) {
	decoded, err := j.decode()
	if err != nil {
		panic(err)
	}
	return decoded.AsBool()
}

func (j *lazyObject) Exists(key string) (bool, error) {
	val, err := j.fetchKey(context.Background(), key)
	return val != nil, err
}

func (j *lazyObject) StripNulls() (JSON, bool, error) {
	decoded, err := j.decode()
	if err != nil {
		return nil, false, err
	}
	return decoded.StripNulls()
}

func (j *lazyObject) ObjectIter() (*ObjectIterator, error) {
	decoded, err := j.decode()
	if err != nil {
		return nil, err
	}
	return decoded.ObjectIter()
}

func (j *lazyObject) isScalar() bool {
	return false
}

func (j *lazyObject) preprocessForContains() (containsable, error) {
	decoded, err := j.decode()
	if err != nil {
		return nil, err
	}
	return decoded.preprocessForContains()
}

func (j *lazyObject) encode(appendTo []byte) (jEntry, []byte, error) {
	decoded, err := j.decode()
	if err != nil {
		return jEntry{}, nil, err
	}
	return decoded.encode(appendTo)
}

func (j *lazyObject) toGoRepr() (interface{}, error) {
	decoded, err := j.decode()
	if err != nil {
		return nil, err
	}
	return decoded.toGoRepr()
}

func (j *lazyObject) HasContainerLeaf() (bool, error) {
	decoded, err := j.decode()
	if err != nil {
		return false, err
	}
	return decoded.HasContainerLeaf()
}
