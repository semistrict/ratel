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

func TestLazyObjectFetchValKeyDoesNotDecodeWholeObject(t *testing.T) {
	var decodeCalls int
	var fetchCalls []string
	lazy := NewLazyObject(
		2,
		func(_ context.Context, key string) (JSON, error) {
			fetchCalls = append(fetchCalls, key)
			switch key {
			case "a":
				return FromInt(10), nil
			case "b":
				return FromString("x"), nil
			default:
				return nil, nil
			}
		},
		func(context.Context) (JSON, error) {
			decodeCalls++
			return ParseJSON(`{"a":10,"b":"x"}`)
		},
	)

	got, err := lazy.FetchValKey("b")
	require.NoError(t, err)
	require.Equal(t, `"x"`, got.String())
	require.Equal(t, []string{"b"}, fetchCalls)
	require.Zero(t, decodeCalls)

	got, err = lazy.FetchValKey("missing")
	require.NoError(t, err)
	require.Nil(t, got)
	require.Equal(t, []string{"b", "missing"}, fetchCalls)
	require.Zero(t, decodeCalls)
}

func TestLazyObjectFallsBackToDecodeForNonKeyedOperations(t *testing.T) {
	var decodeCalls int
	lazy := NewLazyObject(
		1,
		func(_ context.Context, key string) (JSON, error) {
			if key == "a" {
				return FromInt(10), nil
			}
			return nil, nil
		},
		func(context.Context) (JSON, error) {
			decodeCalls++
			return ParseJSON(`{"a":10}`)
		},
	)

	_, err := lazy.AsText()
	require.NoError(t, err)
	require.Equal(t, 1, decodeCalls)

	_, err = lazy.Compare(lazy)
	require.NoError(t, err)
	require.Equal(t, 1, decodeCalls)
}
