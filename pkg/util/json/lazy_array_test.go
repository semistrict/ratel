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
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLazyArrayFetchValIdxDoesNotDecodeWholeArray(t *testing.T) {
	var decodeCalls int
	var fetchCalls []int
	lazy := NewLazyArray(
		3,
		func(_ context.Context, idx int) (JSON, error) {
			fetchCalls = append(fetchCalls, idx)
			switch idx {
			case 0:
				return FromInt(10), nil
			case 1:
				return FromInt(20), nil
			case 2:
				return FromInt(30), nil
			default:
				t.Fatalf("unexpected index %d", idx)
				return nil, nil
			}
		},
		func(context.Context) (JSON, error) {
			decodeCalls++
			return ParseJSON(`[10,20,30]`)
		},
	)

	got, err := lazy.FetchValIdx(1)
	require.NoError(t, err)
	require.Equal(t, "20", got.String())
	require.Equal(t, []int{1}, fetchCalls)
	require.Zero(t, decodeCalls)

	got, err = lazy.FetchValIdx(-1)
	require.NoError(t, err)
	require.Equal(t, "30", got.String())
	require.Equal(t, []int{1, 2}, fetchCalls)
	require.Zero(t, decodeCalls)
}

func TestLazyArrayFallsBackToDecodeForNonIndexedOperations(t *testing.T) {
	var decodeCalls int
	lazy := NewLazyArray(
		2,
		func(_ context.Context, idx int) (JSON, error) {
			switch idx {
			case 0:
				return FromString("a"), nil
			case 1:
				return FromString("b"), nil
			default:
				return nil, nil
			}
		},
		func(context.Context) (JSON, error) {
			decodeCalls++
			return ParseJSON(`["a","b"]`)
		},
	)

	txt, err := lazy.AsText()
	require.NoError(t, err)
	require.NotNil(t, txt)
	require.Equal(t, `["a", "b"]`, *txt)
	require.Equal(t, 1, decodeCalls)

	_, err = lazy.Compare(lazy)
	require.NoError(t, err)
	require.Equal(t, 1, decodeCalls)
}

type ArrayValueIteratorFunc func(context.Context) (JSON, bool, error)

func (fn ArrayValueIteratorFunc) NextValue(ctx context.Context) (JSON, bool, error) {
	return fn(ctx)
}

func (fn ArrayValueIteratorFunc) Close(context.Context) {}
