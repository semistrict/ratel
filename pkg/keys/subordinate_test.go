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
	"testing"

	"github.com/semistrict/ratel/pkg/roachpb"
	"github.com/semistrict/ratel/pkg/util/encoding"
	"github.com/semistrict/ratel/pkg/util/leaktest"
	"github.com/stretchr/testify/require"
)

func TestSubordinateKeyRoundTrip(t *testing.T) {
	defer leaktest.AfterTest(t)()

	codec := SystemSQLCodec
	tests := []struct {
		name    string
		colID   uint32
		elemIdx uint32
	}{
		{"zero/zero", 0, 0},
		{"small col and idx", 5, 10},
		{"large col ID", 500, 0},
		{"large elem idx", 1, 100000},
		{"both large", 10000, 50000},
		{"max uint32", 4294967295, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build a family-0 row key: /Table/42/Index/1/PK=7/Fam=0
			pkPrefix := encoding.EncodeUvarintAscending(codec.IndexPrefix(42, 1), 7)
			rowKey := MakeFamilyKey(pkPrefix, 0)

			subKey := MakeSubordinateKey(rowKey, tt.colID, tt.elemIdx)

			rowPrefix, gotColID, gotElemIdx, err := DecodeSubordinateKey(subKey)
			require.NoError(t, err)
			require.Equal(t, tt.colID, gotColID)
			require.Equal(t, tt.elemIdx, gotElemIdx)
			require.True(t, bytes.Equal(pkPrefix, rowPrefix),
				"expected row prefix %x, got %x", pkPrefix, rowPrefix)
		})
	}
}

func TestSubordinateKeySortOrder(t *testing.T) {
	defer leaktest.AfterTest(t)()

	codec := SystemSQLCodec
	pkPrefix := encoding.EncodeUvarintAscending(codec.IndexPrefix(42, 1), 7)
	// Bound capacity to prevent slice aliasing between pkPrefix, rowKey, and
	// family keys sharing the same backing array.
	pkPrefix = pkPrefix[:len(pkPrefix):len(pkPrefix)]
	rowKey := MakeFamilyKey(pkPrefix, 0)

	// Row sentinel (family 0).
	sentinel := append(roachpb.Key(nil), rowKey...)

	// Subordinate keys.
	sub_c1_e0 := MakeSubordinateKey(rowKey, 1, 0)
	sub_c1_e1 := MakeSubordinateKey(rowKey, 1, 1)
	sub_c1_e2 := MakeSubordinateKey(rowKey, 1, 2)
	sub_c2_e0 := MakeSubordinateKey(rowKey, 2, 0)
	sub_c2_e5 := MakeSubordinateKey(rowKey, 2, 5)

	// Non-zero family keys. Use fresh copies of pkPrefix to avoid aliasing.
	family1 := MakeFamilyKey(append([]byte(nil), pkPrefix...), 1)
	family2 := MakeFamilyKey(append([]byte(nil), pkPrefix...), 2)

	// Expected sort order:
	// sentinel < sub(1,0) < sub(1,1) < sub(1,2) < sub(2,0) < sub(2,5) < family1 < family2
	ordered := []roachpb.Key{
		sentinel,
		sub_c1_e0,
		sub_c1_e1,
		sub_c1_e2,
		sub_c2_e0,
		sub_c2_e5,
		family1,
		family2,
	}

	for i := 0; i < len(ordered)-1; i++ {
		require.True(t, bytes.Compare(ordered[i], ordered[i+1]) < 0,
			"expected %x < %x (index %d vs %d)", ordered[i], ordered[i+1], i, i+1)
	}
}

func TestSubordinateKeyRowPrefix(t *testing.T) {
	defer leaktest.AfterTest(t)()

	codec := SystemSQLCodec
	pkPrefix := encoding.EncodeUvarintAscending(codec.IndexPrefix(42, 1), 7)
	rowKey := MakeFamilyKey(pkPrefix, 0)

	// GetRowPrefixLength on the sentinel key should match pkPrefix.
	sentinelPrefixLen, err := GetRowPrefixLength(rowKey)
	require.NoError(t, err)
	require.Equal(t, len(pkPrefix), sentinelPrefixLen)

	// GetRowPrefixLength on a subordinate key should return the same prefix.
	subKey := MakeSubordinateKey(rowKey, 5, 100)
	subPrefixLen, err := GetRowPrefixLength(subKey)
	require.NoError(t, err)
	require.Equal(t, len(pkPrefix), subPrefixLen)

	// The actual prefix bytes should be identical.
	require.True(t, bytes.Equal(rowKey[:sentinelPrefixLen], subKey[:subPrefixLen]))
}

func TestSubordinateKeyIsSubordinate(t *testing.T) {
	defer leaktest.AfterTest(t)()

	codec := SystemSQLCodec
	pkPrefix := encoding.EncodeUvarintAscending(codec.IndexPrefix(42, 1), 7)
	rowKey := MakeFamilyKey(pkPrefix, 0)

	subKey := MakeSubordinateKey(rowKey, 3, 7)

	// Subordinate key is recognized as subordinate.
	require.True(t, IsSubordinateKey(subKey, pkPrefix))

	// Row sentinel is NOT a subordinate key.
	require.False(t, IsSubordinateKey(rowKey, pkPrefix))

	// Non-zero family key is NOT a subordinate key.
	fam1Key := MakeFamilyKey(pkPrefix, 1)
	require.False(t, IsSubordinateKey(fam1Key, pkPrefix))

	// Key from a different row is NOT a subordinate.
	otherPK := encoding.EncodeUvarintAscending(codec.IndexPrefix(42, 1), 8)
	otherRow := MakeFamilyKey(otherPK, 0)
	otherSub := MakeSubordinateKey(otherRow, 3, 7)
	require.False(t, IsSubordinateKey(otherSub, pkPrefix))
}

func TestEnsureSafeSplitKeySubordinate(t *testing.T) {
	defer leaktest.AfterTest(t)()

	codec := SystemSQLCodec
	pkPrefix := encoding.EncodeUvarintAscending(codec.IndexPrefix(42, 1), 7)
	rowKey := MakeFamilyKey(pkPrefix, 0)

	// EnsureSafeSplitKey on a subordinate key returns the pk prefix.
	subKey := MakeSubordinateKey(rowKey, 5, 100)
	safe, err := EnsureSafeSplitKey(subKey)
	require.NoError(t, err)
	require.True(t, bytes.Equal(pkPrefix, safe),
		"expected %x, got %x", pkPrefix, safe)

	// Same result as EnsureSafeSplitKey on the sentinel.
	safeSentinel, err := EnsureSafeSplitKey(rowKey)
	require.NoError(t, err)
	require.True(t, bytes.Equal(safeSentinel, safe))
}

func TestSubordinateKeyTenantPrefix(t *testing.T) {
	defer leaktest.AfterTest(t)()

	// Verify subordinate keys work correctly with tenant prefixes.
	tenantCodec := MakeSQLCodec(roachpb.MakeTenantID(5))
	pkPrefix := encoding.EncodeUvarintAscending(tenantCodec.IndexPrefix(42, 1), 7)
	rowKey := MakeFamilyKey(pkPrefix, 0)

	subKey := MakeSubordinateKey(rowKey, 3, 10)

	// Round-trip.
	rowPrefix, colID, elemIdx, err := DecodeSubordinateKey(subKey)
	require.NoError(t, err)
	require.Equal(t, uint32(3), colID)
	require.Equal(t, uint32(10), elemIdx)
	require.True(t, bytes.Equal(pkPrefix, rowPrefix))

	// GetRowPrefixLength should still work.
	prefixLen, err := GetRowPrefixLength(subKey)
	require.NoError(t, err)
	require.Equal(t, len(pkPrefix), prefixLen)

	// EnsureSafeSplitKey should return the pk prefix.
	safe, err := EnsureSafeSplitKey(subKey)
	require.NoError(t, err)
	require.True(t, bytes.Equal(pkPrefix, safe))
}

func TestDecodeSubordinateKeyErrors(t *testing.T) {
	defer leaktest.AfterTest(t)()

	// A regular family key is not a subordinate key.
	codec := SystemSQLCodec
	pkPrefix := encoding.EncodeUvarintAscending(codec.IndexPrefix(42, 1), 7)
	rowKey := MakeFamilyKey(pkPrefix, 0)

	_, _, _, err := DecodeSubordinateKey(rowKey)
	require.Error(t, err)
	require.Contains(t, err.Error(), "decoding subordinate key column ID")

	// A non-zero family key fails the family sentinel check.
	fam1Key := MakeFamilyKey(pkPrefix, 1)
	_, _, _, err = DecodeSubordinateKey(fam1Key)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not a subordinate key")
}
