// Copyright 2021 The Cockroach Authors.
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

package indexrec_test

import (
	"testing"

	"github.com/cockroachdb/datadriven"
	"github.com/semistrict/ratel/pkg/sql/opt/memo"
	"github.com/semistrict/ratel/pkg/sql/opt/testutils/opttester"
	"github.com/semistrict/ratel/pkg/sql/opt/testutils/testcat"
	"github.com/semistrict/ratel/pkg/testutils"
	"github.com/semistrict/ratel/pkg/util/leaktest"
	"github.com/semistrict/ratel/pkg/util/log"
)

// TestIndexRec tests that index candidates and index recommendations for
// queries are found as expected.
func TestIndexRec(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	const fmtFlags = memo.ExprFmtHideStats | memo.ExprFmtHideRuleProps |
		memo.ExprFmtHideQualifications | memo.ExprFmtHideScalars | memo.ExprFmtHideTypes
	datadriven.Walk(t, testutils.TestDataPath(t), func(t *testing.T, path string) {
		catalog := testcat.New()
		datadriven.RunTest(t, path, func(t *testing.T, d *datadriven.TestData) string {
			tester := opttester.New(catalog, d.Input)
			tester.Flags.ExprFormat = fmtFlags
			return tester.RunCommand(t, d)
		})
	})
}
