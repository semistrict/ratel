// Copyright 2024 Oxide Computer Company
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

package sql

import (
	"strings"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/cockroach/pkg/sql/types"
	"github.com/cockroachdb/cockroach/pkg/sql/udfruntime"
)

func TestPrepareJavaScriptBodyWrapsBareBodyAsync(t *testing.T) {
	body := prepareJavaScriptBody([]tree.FuncParam{
		{Name: "x", Type: types.Int},
		{Type: types.String},
	}, "return Promise.resolve(x + $2.length);")

	if !strings.HasPrefix(body, "async function invoke(x, $2)") {
		t.Fatalf("expected async invoke wrapper, got %q", body)
	}
	if !strings.Contains(body, "Promise.resolve") {
		t.Fatalf("expected original body to be preserved, got %q", body)
	}
}

func TestPrepareJavaScriptBodyPreservesExplicitInvoke(t *testing.T) {
	body := "function invoke(x) { return x * 2; }"
	if got := prepareJavaScriptBody(nil, body); got != body {
		t.Fatalf("expected explicit invoke body to be preserved, got %q", got)
	}
}

func TestParsePersistedUDFVolatility(t *testing.T) {
	testCases := []struct {
		name     string
		language udfruntime.Language
		stored   string
		expected tree.Volatility
	}{
		{name: "stored stable", language: udfruntime.LangJavaScript, stored: "stable", expected: tree.VolatilityStable},
		{name: "stored volatile", language: udfruntime.LangJavaScript, stored: "volatile", expected: tree.VolatilityVolatile},
		{name: "stored immutable", language: udfruntime.LangWasm, stored: "immutable", expected: tree.VolatilityImmutable},
		{name: "legacy js fallback", language: udfruntime.LangJavaScript, stored: "", expected: tree.VolatilityStable},
		{name: "legacy wasm fallback", language: udfruntime.LangWasm, stored: "", expected: tree.VolatilityImmutable},
		{name: "unknown fallback", language: udfruntime.LangJavaScript, stored: "bogus", expected: tree.VolatilityImmutable},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := parsePersistedUDFVolatility(tc.language, tc.stored); got != tc.expected {
				t.Fatalf("expected %v, got %v", tc.expected, got)
			}
		})
	}
}
