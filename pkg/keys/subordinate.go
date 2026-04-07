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

	"github.com/cockroachdb/cockroach/pkg/util/encoding"
	"github.com/cockroachdb/errors"
)

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
	// rowKey ends with the family-0 sentinel, a single-byte uvarint(0).
	// pk_prefix length = len(rowKey) - 1.
	pkPrefixLen := len(rowKey) - 1

	// Start with a copy of rowKey (which already contains pk_prefix + family-0 sentinel).
	key := make([]byte, len(rowKey), len(rowKey)+6)
	copy(key, rowKey)

	key = encoding.EncodeUvarintAscending(key, uint64(colID))
	key = encoding.EncodeUvarintAscending(key, uint64(elemIdx))

	// suffix_len = number of bytes from family sentinel through elem_idx.
	suffixLen := len(key) - pkPrefixLen
	key = encoding.EncodeUvarintAscending(key, uint64(suffixLen))
	return key
}

// DecodeSubordinateKey parses a subordinate key, returning the row prefix
// (pk_prefix without the family sentinel), the column ID, and the element
// index.
func DecodeSubordinateKey(key []byte) (rowPrefix []byte, colID uint32, elemIdx uint32, err error) {
	n, err := GetRowPrefixLength(key)
	if err != nil {
		return nil, 0, 0, err
	}
	rowPrefix = key[:n]
	suffix := key[n:]

	// Decode family-0 sentinel.
	suffix, famID, err := encoding.DecodeUvarintAscending(suffix)
	if err != nil {
		return nil, 0, 0, errors.Wrap(err, "decoding subordinate key family sentinel")
	}
	if famID != 0 {
		return nil, 0, 0, errors.Errorf("not a subordinate key: expected family 0, got %d", famID)
	}

	// Decode column ID.
	suffix, col, err := encoding.DecodeUvarintAscending(suffix)
	if err != nil {
		return nil, 0, 0, errors.Wrap(err, "decoding subordinate key column ID")
	}
	if col > math.MaxUint32 {
		return nil, 0, 0, errors.Errorf("subordinate key column ID overflow: %d", col)
	}

	// Decode element index.
	suffix, idx, err := encoding.DecodeUvarintAscending(suffix)
	if err != nil {
		return nil, 0, 0, errors.Wrap(err, "decoding subordinate key element index")
	}
	if idx > math.MaxUint32 {
		return nil, 0, 0, errors.Errorf("subordinate key element index overflow: %d", idx)
	}

	// Remaining bytes are the suffix length uvarint (already used by
	// GetRowPrefixLength). We verify it's present but don't need its value.
	if len(suffix) == 0 {
		return nil, 0, 0, errors.New("subordinate key missing suffix length")
	}

	return rowPrefix, uint32(col), uint32(idx), nil
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
