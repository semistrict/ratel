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

package wasmruntime

import (
	"fmt"
	"math"

	"github.com/semistrict/ratel/pkg/sql/sem/tree"
	"github.com/semistrict/ratel/pkg/sql/types"
)

// SQLTypeToValType converts a SQL type to a WASM value type.
// Only numeric types are supported in v1.
func SQLTypeToValType(t *types.T) (ValType, error) {
	switch t.Family() {
	case types.IntFamily:
		return ValI64, nil
	case types.FloatFamily:
		return ValF64, nil
	case types.BoolFamily:
		return ValI32, nil
	default:
		return 0, fmt.Errorf("unsupported SQL type for WASM: %s (supported: INT, FLOAT, BOOL)", t.SQLString())
	}
}

// ValTypeToSQLType converts a WASM value type to a SQL type.
func ValTypeToSQLType(v ValType) (*types.T, error) {
	switch v {
	case ValI64:
		return types.Int, nil
	case ValF64:
		return types.Float, nil
	case ValI32:
		return types.Bool, nil
	default:
		return nil, fmt.Errorf("unsupported WASM value type: 0x%02x", byte(v))
	}
}

// MarshalDatum converts a SQL Datum to a WASM uint64 value.
// This is the raw bit representation that wazero uses for function arguments.
func MarshalDatum(d tree.Datum, vt ValType) (uint64, error) {
	if d == tree.DNull {
		return 0, fmt.Errorf("NULL values are not supported in WASM functions")
	}
	switch vt {
	case ValI64:
		switch v := d.(type) {
		case *tree.DInt:
			return uint64(int64(*v)), nil
		default:
			return 0, fmt.Errorf("expected INT datum for i64 parameter, got %T", d)
		}
	case ValF64:
		switch v := d.(type) {
		case *tree.DFloat:
			return math.Float64bits(float64(*v)), nil
		default:
			return 0, fmt.Errorf("expected FLOAT datum for f64 parameter, got %T", d)
		}
	case ValI32:
		switch v := d.(type) {
		case *tree.DBool:
			if bool(*v) {
				return 1, nil
			}
			return 0, nil
		default:
			return 0, fmt.Errorf("expected BOOL datum for i32 parameter, got %T", d)
		}
	default:
		return 0, fmt.Errorf("unsupported WASM value type: 0x%02x", byte(vt))
	}
}

// UnmarshalDatum converts a WASM uint64 result to a SQL Datum.
func UnmarshalDatum(val uint64, vt ValType) (tree.Datum, error) {
	switch vt {
	case ValI64:
		d := tree.DInt(int64(val))
		return &d, nil
	case ValF64:
		d := tree.DFloat(math.Float64frombits(val))
		return &d, nil
	case ValI32:
		if val != 0 {
			return tree.DBoolTrue, nil
		}
		return tree.DBoolFalse, nil
	default:
		return nil, fmt.Errorf("unsupported WASM value type: 0x%02x", byte(vt))
	}
}
