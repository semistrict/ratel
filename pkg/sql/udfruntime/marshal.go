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

package udfruntime

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/semistrict/ratel/pkg/sql/sem/tree"
	"github.com/semistrict/ratel/pkg/sql/types"
)

var jsonMarshal = json.Marshal

// SQLTypeToValType converts a SQL type to a ValType for marshaling.
func SQLTypeToValType(t *types.T) (ValType, error) {
	switch t.Family() {
	case types.IntFamily:
		return ValI64, nil
	case types.FloatFamily:
		return ValF64, nil
	case types.BoolFamily:
		return ValI32, nil
	case types.StringFamily:
		return ValString, nil
	case types.BytesFamily:
		return ValBytes, nil
	case types.TimestampFamily, types.TimestampTZFamily:
		return ValTimestamp, nil
	case types.JsonFamily:
		return ValJSON, nil
	default:
		return 0, fmt.Errorf("unsupported SQL type for UDF: %s (supported: INT, FLOAT, BOOL, STRING, BYTES, TIMESTAMP, JSONB)", t.SQLString())
	}
}

// ValTypeToSQLType converts a ValType to a SQL type.
func ValTypeToSQLType(v ValType) (*types.T, error) {
	switch v {
	case ValI64:
		return types.Int, nil
	case ValF64:
		return types.Float, nil
	case ValI32:
		return types.Bool, nil
	case ValString:
		return types.String, nil
	case ValBytes:
		return types.Bytes, nil
	case ValTimestamp:
		return types.Timestamp, nil
	case ValJSON:
		return types.Jsonb, nil
	default:
		return nil, fmt.Errorf("unsupported value type: 0x%02x", byte(v))
	}
}

// MarshalDatumToJS converts a SQL Datum to a JavaScript expression string.
func MarshalDatumToJS(d tree.Datum, vt ValType) (string, error) {
	if d == tree.DNull {
		return "", fmt.Errorf("NULL values are not supported in UDF functions")
	}
	switch vt {
	case ValI64:
		v, ok := d.(*tree.DInt)
		if !ok {
			return "", fmt.Errorf("expected INT datum for i64 parameter, got %T", d)
		}
		return fmt.Sprintf("%d", int64(*v)), nil
	case ValF64:
		v, ok := d.(*tree.DFloat)
		if !ok {
			return "", fmt.Errorf("expected FLOAT datum for f64 parameter, got %T", d)
		}
		return fmt.Sprintf("%v", float64(*v)), nil
	case ValI32:
		v, ok := d.(*tree.DBool)
		if !ok {
			return "", fmt.Errorf("expected BOOL datum for i32 parameter, got %T", d)
		}
		if bool(*v) {
			return "1", nil
		}
		return "0", nil
	case ValString:
		switch v := d.(type) {
		case *tree.DString:
			return quoteJSString(string(*v)), nil
		default:
			return "", fmt.Errorf("expected STRING datum, got %T", d)
		}
	case ValBytes:
		switch v := d.(type) {
		case *tree.DBytes:
			return bytesToJSUint8Array([]byte(*v)), nil
		default:
			return "", fmt.Errorf("expected BYTES datum, got %T", d)
		}
	case ValTimestamp:
		switch v := d.(type) {
		case *tree.DTimestamp:
			ms := v.Time.UnixMilli()
			return fmt.Sprintf("new Date(%d)", ms), nil
		case *tree.DTimestampTZ:
			ms := v.Time.UnixMilli()
			return fmt.Sprintf("new Date(%d)", ms), nil
		default:
			return "", fmt.Errorf("expected TIMESTAMP datum, got %T", d)
		}
	case ValJSON:
		switch v := d.(type) {
		case *tree.DJSON:
			return v.JSON.String(), nil
		default:
			return "", fmt.Errorf("expected JSONB datum, got %T", d)
		}
	default:
		return "", fmt.Errorf("unsupported value type: 0x%02x", byte(vt))
	}
}

// quoteJSString produces a JSON-style quoted string safe for JS embedding.
func quoteJSString(s string) string {
	// Use JSON encoding which handles all escaping.
	b, _ := jsonMarshal(s)
	return string(b)
}

// bytesToJSUint8Array converts bytes to a JS Uint8Array expression.
func bytesToJSUint8Array(b []byte) string {
	parts := make([]string, len(b))
	for i, v := range b {
		parts[i] = fmt.Sprintf("%d", v)
	}
	return "new Uint8Array([" + strings.Join(parts, ",") + "])"
}

// UnmarshalJSResult converts a JavaScript numeric result to a SQL Datum.
func UnmarshalJSResult(jsVal int64, vt ValType) (tree.Datum, error) {
	switch vt {
	case ValI64:
		d := tree.DInt(jsVal)
		return &d, nil
	case ValF64:
		// For float results we handle this separately via Float64()
		// This path is for integer-typed results only.
		d := tree.DInt(jsVal)
		return &d, nil
	case ValI32:
		if jsVal != 0 {
			return tree.DBoolTrue, nil
		}
		return tree.DBoolFalse, nil
	default:
		return nil, fmt.Errorf("unsupported value type: 0x%02x", byte(vt))
	}
}
