// Copyright 2026 The Ratel Authors.
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

package keys

import (
	"bytes"
	"math"

	"github.com/cockroachdb/errors"
	"github.com/semistrict/ratel/pkg/util/encoding"
)

// SubordinatePathSegmentKind distinguishes subordinate path segment encodings.
type SubordinatePathSegmentKind uint64

const (
	// SubordinatePathHeader addresses the root/header entry for a subordinate
	// column.
	SubordinatePathHeader SubordinatePathSegmentKind = iota
	// SubordinatePathArrayIndex addresses an array element by index.
	SubordinatePathArrayIndex
	// SubordinatePathObjectKey addresses an object member by key.
	SubordinatePathObjectKey
)

// SubordinatePathSegment is one component in a subordinate key path.
type SubordinatePathSegment struct {
	Kind      SubordinatePathSegmentKind
	ArrayIdx  uint32
	ObjectKey string
}

// MakeSubordinatePathKey builds a subordinate key for a single subordinate
// value under rowKey. rowKey must be a family-0 key (that is, the primary-key
// prefix with the family-0 sentinel appended via MakeFamilyKey(pk, 0)).
//
// The resulting key layout is:
//
//	<pk_prefix>/<family_0:uvarint(0)>/<col_id:uvarint>/<path...>/<end:uvarint(0)>/<suffix_len:uvarint>
//
// Path segments are encoded as:
//
//	<header>     => <kind:uvarint(0)>
//	<array idx>  => <kind:uvarint(1)><elem_idx:uvarint>
//	<object key> => <kind:uvarint(2)><key:bytes>
//
// The extra trailing header/end marker makes an exact ancestor key sort before
// all of its descendants while still allowing MakeSubordinatePathPrefix to
// describe subtree spans over the logical path alone.
//
// suffix_len is the number of bytes from the family-0 sentinel through the end
// marker. This makes GetRowPrefixLength work correctly: the final uvarint tells
// it how many preceding bytes to strip (plus the length byte itself) to recover
// the pk_prefix.
func MakeSubordinatePathKey(rowKey []byte, colID uint32, path []SubordinatePathSegment) []byte {
	// rowKey ends with the family-0 sentinel, a single-byte uvarint(0).
	pkPrefixLen := len(rowKey) - 1
	key := MakeSubordinatePathPrefix(rowKey, colID, path)
	key = encoding.EncodeUvarintAscending(key, uint64(SubordinatePathHeader))

	suffixLen := len(key) - pkPrefixLen
	key = encoding.EncodeUvarintAscending(key, uint64(suffixLen))
	return key
}

// MakeSubordinatePathPrefix builds the sortable key prefix for a subordinate
// JSON/array path without the trailing suffix-length uvarint. It can be used
// to construct subtree spans like [prefix, prefix.PrefixEnd()).
func MakeSubordinatePathPrefix(rowKey []byte, colID uint32, path []SubordinatePathSegment) []byte {
	key := make([]byte, len(rowKey), len(rowKey)+16)
	copy(key, rowKey)

	key = encoding.EncodeUvarintAscending(key, uint64(colID))
	for _, seg := range path {
		key = encoding.EncodeUvarintAscending(key, uint64(seg.Kind))
		switch seg.Kind {
		case SubordinatePathHeader:
			// No payload.
		case SubordinatePathArrayIndex:
			key = encoding.EncodeUvarintAscending(key, uint64(seg.ArrayIdx))
		case SubordinatePathObjectKey:
			key = encoding.EncodeBytesAscending(key, []byte(seg.ObjectKey))
		default:
			panic(errors.AssertionFailedf("unknown subordinate path segment kind %d", seg.Kind))
		}
	}
	return key
}

// MakeSubordinateKey builds a subordinate key for a single array element. The
// rowKey must be a family-0 key (i.e., the primary key prefix with the family-0
// sentinel appended via MakeFamilyKey(pk, 0)).
//
// The resulting key layout is:
//
//	<pk_prefix>/<family_0:uvarint(0)>/<col_id:uvarint>/<elem_idx:uvarint>/<suffix_len:uvarint>
//
// where suffix_len is the number of bytes from the family-0 sentinel through
// elem_idx (inclusive). This makes GetRowPrefixLength work correctly: the last
// uvarint tells it how many preceding bytes to strip (plus the length byte
// itself) to recover the pk_prefix.
//
// Subordinate keys sort after the row sentinel (family 0) and before any
// non-zero family keys, because uvarint(0) < uvarint(1).
func MakeSubordinateKey(rowKey []byte, colID uint32, elemIdx uint32) []byte {
	return MakeSubordinatePathKey(rowKey, colID, []SubordinatePathSegment{{
		Kind:     SubordinatePathArrayIndex,
		ArrayIdx: elemIdx,
	}})
}

// DecodeSubordinatePathKey parses a subordinate key, returning the row prefix
// (pk_prefix without the family sentinel), the column ID, and the subordinate
// path.
func DecodeSubordinatePathKey(key []byte) (rowPrefix []byte, colID uint32, path []SubordinatePathSegment, err error) {
	n, err := GetRowPrefixLength(key)
	if err != nil {
		return nil, 0, nil, err
	}
	rowPrefix = key[:n]
	suffix := key[n:]

	suffix, famID, err := encoding.DecodeUvarintAscending(suffix)
	if err != nil {
		return nil, 0, nil, errors.Wrap(err, "decoding subordinate key family sentinel")
	}
	if famID != 0 {
		return nil, 0, nil, errors.Errorf("not a subordinate key: expected family 0, got %d", famID)
	}

	suffix, col, err := encoding.DecodeUvarintAscending(suffix)
	if err != nil {
		return nil, 0, nil, errors.Wrap(err, "decoding subordinate key column ID")
	}
	if col > math.MaxUint32 {
		return nil, 0, nil, errors.Errorf("subordinate key column ID overflow: %d", col)
	}

	for len(suffix) > 0 {
		rest, kind, err := encoding.DecodeUvarintAscending(suffix)
		if err != nil {
			return nil, 0, nil, errors.Wrap(err, "decoding subordinate path segment kind")
		}

		seg := SubordinatePathSegment{Kind: SubordinatePathSegmentKind(kind)}
		switch seg.Kind {
		case SubordinatePathHeader:
			path = append(path, seg)
			suffix = rest
		case SubordinatePathArrayIndex:
			var idx uint64
			rest, idx, err = encoding.DecodeUvarintAscending(rest)
			if err != nil {
				return nil, 0, nil, errors.Wrap(err, "decoding subordinate array index")
			}
			if idx > math.MaxUint32 {
				return nil, 0, nil, errors.Errorf("subordinate key element index overflow: %d", idx)
			}
			seg.ArrayIdx = uint32(idx)
			path = append(path, seg)
			suffix = rest
		case SubordinatePathObjectKey:
			var keyBytes []byte
			rest, keyBytes, err = encoding.DecodeBytesAscending(rest, nil)
			if err != nil {
				return nil, 0, nil, errors.Wrap(err, "decoding subordinate object key")
			}
			seg.ObjectKey = string(keyBytes)
			path = append(path, seg)
			suffix = rest
		default:
			// The only remaining bytes should be the trailing suffix_len uvarint.
			if len(path) == 0 {
				return nil, 0, nil, errors.Errorf("unknown subordinate path segment kind %d", seg.Kind)
			}
			if n := len(path); n > 0 && path[n-1].Kind == SubordinatePathHeader {
				path = path[:n-1]
			}
			return rowPrefix, uint32(col), path, nil
		}
	}

	if n := len(path); n > 0 && path[n-1].Kind == SubordinatePathHeader {
		path = path[:n-1]
	}
	return rowPrefix, uint32(col), path, nil
}

// DecodeSubordinateKey parses a subordinate key, returning the row prefix
// (pk_prefix without the family sentinel), the column ID, and the element
// index.
func DecodeSubordinateKey(key []byte) (rowPrefix []byte, colID uint32, elemIdx uint32, err error) {
	rowPrefix, colID, path, err := DecodeSubordinatePathKey(key)
	if err != nil {
		return nil, 0, 0, err
	}
	if len(path) != 1 || path[0].Kind != SubordinatePathArrayIndex {
		return nil, 0, 0, errors.New("subordinate key is not an array element key")
	}
	return rowPrefix, colID, path[0].ArrayIdx, nil
}

// IsSubordinateKey returns true if key is a subordinate key under the given row
// prefix. A subordinate key shares the row prefix and has a suffix longer than
// the single-byte family-0 sentinel, starting with uvarint(0).
func IsSubordinateKey(key []byte, rowPrefix []byte) bool {
	if !bytes.HasPrefix(key, rowPrefix) {
		return false
	}
	suffix := key[len(rowPrefix):]
	// A regular family-0 key has a 1-byte suffix (just the uvarint(0) sentinel).
	// Subordinate keys have at least 4 bytes: family sentinel + col_id + elem_idx + suffix_len.
	if len(suffix) < 4 {
		return false
	}
	// The first byte must be the uvarint(0) encoding, i.e. the family-0 sentinel.
	_, famID, err := encoding.DecodeUvarintAscending(suffix)
	return err == nil && famID == 0
}
