// Copyright 2022 The Cockroach Authors.
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

package span_test

import (
	"context"
	"testing"

	"github.com/semistrict/ratel/pkg/keys"
	"github.com/semistrict/ratel/pkg/roachpb"
	"github.com/semistrict/ratel/pkg/sql/catalog/desctestutils"
	"github.com/semistrict/ratel/pkg/sql/catalog/systemschema"
	"github.com/semistrict/ratel/pkg/sql/span"
	"github.com/semistrict/ratel/pkg/sql/tests"
	"github.com/semistrict/ratel/pkg/testutils/serverutils"
	"github.com/semistrict/ratel/pkg/util"
	"github.com/semistrict/ratel/pkg/util/leaktest"
	"github.com/semistrict/ratel/pkg/util/log"
)

func TestSpanSplitterIsNoop(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	ctx := context.Background()
	params, _ := tests.CreateTestServerParams()
	s, sqlDB, kvDB := serverutils.StartServer(t, params)
	defer s.Stopper().Stop(ctx)

	if _, err := sqlDB.Exec(`CREATE DATABASE t`); err != nil {
		t.Fatal(err)
	}
	if _, err := sqlDB.Exec(`
CREATE TABLE t.u (
	a INT PRIMARY KEY,
	b INT,
	c INT,
	UNIQUE INDEX u_b (b),
	INDEX u_c (c)
)`); err != nil {
		t.Fatal(err)
	}
	desc := desctestutils.TestingGetPublicTableDescriptor(kvDB, keys.SystemSQLCodec, "t", "u")

	testCases := []struct {
		name      string
		indexName string
	}{
		{name: "user-primary", indexName: desc.GetPrimaryIndex().GetName()},
		{name: "user-secondary", indexName: "u_b"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			idx, err := desc.FindIndexWithName(tc.indexName)
			if err != nil {
				t.Fatal(err)
			}
			splitter := span.MakeSplitter(desc, idx, util.MakeFastIntSet(0, 1, 2))
			if !splitter.IsNoop() {
				t.Fatal("expected no-op splitter")
			}
			got := splitter.AppendSpan(nil, roachpb.Span{Key: []byte("k"), EndKey: []byte("ke")}, idx.NumKeyColumns(), false)
			if len(got) != 1 {
				t.Fatalf("expected appended span, got %v", got)
			}
		})
	}

	systemSplitter := span.MakeSplitter(
		systemschema.DescriptorTable,
		systemschema.DescriptorTable.GetPrimaryIndex(),
		util.MakeFastIntSet(0),
	)
	if !systemSplitter.IsNoop() {
		t.Fatal("expected system-table splitter to be a no-op")
	}
	got := systemSplitter.AppendSpan(nil, roachpb.Span{Key: []byte("k"), EndKey: []byte("ke")}, 1, false)
	if len(got) != 1 {
		t.Fatalf("expected appended span, got %v", got)
	}
}
