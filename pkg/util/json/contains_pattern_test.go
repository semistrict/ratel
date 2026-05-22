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
	"testing"

	"github.com/stretchr/testify/require"
)

func TestContainsPatternMatchesContains(t *testing.T) {
	right, err := ParseJSON(`{"a":{"b":[20]}}`)
	require.NoError(t, err)
	pattern, err := NewContainsPattern(right)
	require.NoError(t, err)

	left, err := ParseJSON(`{"a":{"b":[10,20]},"z":7}`)
	require.NoError(t, err)
	got, err := ContainsWithPattern(left, pattern)
	require.NoError(t, err)
	want, err := Contains(left, right)
	require.NoError(t, err)
	require.Equal(t, want, got)
}

func TestContainsPatternContainsMatchesContains(t *testing.T) {
	left, err := ParseJSON(`{"a":{"b":[10,20]},"z":7}`)
	require.NoError(t, err)
	pattern, err := NewContainsPattern(left)
	require.NoError(t, err)

	right, err := ParseJSON(`{"a":{"b":[20]}}`)
	require.NoError(t, err)
	got, err := pattern.Contains(right)
	require.NoError(t, err)
	want, err := Contains(left, right)
	require.NoError(t, err)
	require.Equal(t, want, got)
}

func TestContainsPatternMatchesContainsNullAndEmpty(t *testing.T) {
	testCases := []struct {
		name  string
		left  string
		right string
	}{
		{name: "json null", left: `null`, right: `null`},
		{name: "empty object", left: `{}`, right: `{}`},
		{name: "empty array", left: `[]`, right: `[]`},
		{name: "array contains empty array", left: `[1,2,3]`, right: `[]`},
		{name: "object contains empty object", left: `{"a":1}`, right: `{}`},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			left, err := ParseJSON(tc.left)
			require.NoError(t, err)
			right, err := ParseJSON(tc.right)
			require.NoError(t, err)
			pattern, err := NewContainsPattern(right)
			require.NoError(t, err)

			got, err := ContainsWithPattern(left, pattern)
			require.NoError(t, err)
			want, err := Contains(left, right)
			require.NoError(t, err)
			require.Equal(t, want, got)
		})
	}
}

func TestContainsPatternMatchesContainsDuplicateArrayScalars(t *testing.T) {
	testCases := []struct {
		name  string
		left  string
		right string
		want  bool
	}{
		{name: "contains duplicate scalars", left: `[1,1,2]`, right: `[1,1]`, want: true},
		{name: "duplicate scalar is set-like", left: `[1,2]`, right: `[1,1]`, want: true},
		{name: "cannot contain extra scalar", left: `[1,1]`, right: `[1,1,2]`, want: false},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			left, err := ParseJSON(tc.left)
			require.NoError(t, err)
			right, err := ParseJSON(tc.right)
			require.NoError(t, err)
			pattern, err := NewContainsPattern(right)
			require.NoError(t, err)

			got, err := ContainsWithPattern(left, pattern)
			require.NoError(t, err)
			require.Equal(t, tc.want, got)

			got, err = Contains(left, right)
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}
