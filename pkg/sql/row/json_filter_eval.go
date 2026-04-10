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

package row

import (
	"github.com/cockroachdb/errors"
	"github.com/semistrict/ratel/pkg/sql/opt/exec"
	"github.com/semistrict/ratel/pkg/sql/sem/tree"
)

// EvalJSONPathFilterDatum applies one scan-local JSON path filter mode to an
// already-decoded derived path result datum.
func EvalJSONPathFilterDatum(
	evalCtx *tree.EvalContext, mode exec.JSONPathFilterMode, left tree.Datum, right tree.Datum,
) (bool, error) {
	switch mode {
	case exec.JSONPathFilterEq:
		if left == tree.DNull || right == tree.DNull {
			return false, nil
		}
		return left.Compare(evalCtx, right) == 0, nil
	case exec.JSONPathFilterNe:
		if left == tree.DNull || right == tree.DNull {
			return false, nil
		}
		return left.Compare(evalCtx, right) != 0, nil
	case exec.JSONPathFilterLt:
		if left == tree.DNull || right == tree.DNull {
			return false, nil
		}
		return left.Compare(evalCtx, right) < 0, nil
	case exec.JSONPathFilterLe:
		if left == tree.DNull || right == tree.DNull {
			return false, nil
		}
		return left.Compare(evalCtx, right) <= 0, nil
	case exec.JSONPathFilterGt:
		if left == tree.DNull || right == tree.DNull {
			return false, nil
		}
		return left.Compare(evalCtx, right) > 0, nil
	case exec.JSONPathFilterGe:
		if left == tree.DNull || right == tree.DNull {
			return false, nil
		}
		return left.Compare(evalCtx, right) >= 0, nil
	case exec.JSONPathFilterIsNull:
		return left == tree.DNull, nil
	case exec.JSONPathFilterIsNotNull:
		return left != tree.DNull, nil
	default:
		return false, errors.AssertionFailedf("unknown JSON path filter mode %d", mode)
	}
}
