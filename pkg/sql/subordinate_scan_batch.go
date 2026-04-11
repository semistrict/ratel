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

package sql

import (
	"github.com/semistrict/ratel/pkg/sql/catalog/descpb"
	"github.com/semistrict/ratel/pkg/sql/rowinfra"
	"github.com/semistrict/ratel/pkg/sql/types"
)

// subordinateScanBatchBytesLimit keeps scans over subordinate-encoded values
// streaming in small chunks instead of letting a single large row arrive in one
// oversized KV batch. This bounds executor heap usage for tiny-path reads over
// very large JSON/array rows.
const subordinateScanBatchBytesLimit rowinfra.BytesLimit = 128 << 10 // 128 KiB

func scanFetchSpecHasSubordinateColumns(spec descpb.IndexFetchSpec) bool {
	for i := range spec.FetchedColumns {
		switch spec.FetchedColumns[i].Type.Family() {
		case types.ArrayFamily, types.JsonFamily:
			return true
		}
	}
	return false
}

func defaultScanBatchBytesLimit(spec descpb.IndexFetchSpec) rowinfra.BytesLimit {
	if scanFetchSpecHasSubordinateColumns(spec) {
		return subordinateScanBatchBytesLimit
	}
	return rowinfra.DefaultBatchBytesLimit
}
