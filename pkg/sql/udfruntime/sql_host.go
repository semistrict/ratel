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
	"context"
	"fmt"

	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	v8 "github.com/tommie/v8go"
)

// SQLExecutor is the interface for executing SQL from within a UDF.
// This is satisfied by *InternalExecutor in the sql package.
type SQLExecutor interface {
	QueryBufferedEx(
		ctx context.Context,
		opName string,
		txn interface{},
		override interface{},
		stmt string,
		qargs ...interface{},
	) ([]tree.Datums, []ResultColumn, error)
}

// ResultColumn describes a column in a query result.
type ResultColumn struct {
	Name string
	Typ  interface{}
}

// v8ValueToGo converts a V8 value to a Go value suitable for use as a SQL
// query argument.
func v8ValueToGo(val *v8.Value) interface{} {
	if val.IsInt32() {
		return int64(val.Int32())
	}
	if val.IsNumber() {
		return val.Number()
	}
	if val.IsBoolean() {
		return val.Boolean()
	}
	if val.IsString() {
		return val.String()
	}
	if val.IsNull() || val.IsUndefined() {
		return nil
	}
	if val.IsBigInt() {
		return val.Integer()
	}
	return val.String()
}

// rowsToJSArray converts SQL result rows to a JavaScript array of objects.
func rowsToJSArray(ctx *v8.Context, rows []tree.Datums, cols []ResultColumn) (*v8.Value, error) {
	iso := ctx.Isolate()

	arrayVal, err := ctx.RunScript(fmt.Sprintf("new Array(%d)", len(rows)), "")
	if err != nil {
		return nil, err
	}
	arrayObj, err := arrayVal.AsObject()
	if err != nil {
		return nil, err
	}

	for i, row := range rows {
		rowObjTemplate := v8.NewObjectTemplate(iso)
		rowObj, err := rowObjTemplate.NewInstance(ctx)
		if err != nil {
			return nil, err
		}
		for j, col := range cols {
			goVal := datumToGo(row[j])
			if err := rowObj.Set(col.Name, goVal); err != nil {
				return nil, fmt.Errorf("setting column %q: %w", col.Name, err)
			}
		}
		if err := arrayObj.SetIdx(uint32(i), rowObj); err != nil {
			return nil, err
		}
	}

	return arrayVal, nil
}

// datumToGo converts a SQL Datum to a Go value that v8go can handle.
// Uses float64 for integers because v8go maps int64 to BigInt.
// WARNING: INT values above 2^53 (9007199254740992) will lose precision
// due to float64 representation. This matches plv8's behavior.
func datumToGo(d tree.Datum) interface{} {
	if d == tree.DNull {
		return nil
	}
	switch v := d.(type) {
	case *tree.DInt:
		return float64(int64(*v))
	case *tree.DFloat:
		return float64(*v)
	case *tree.DBool:
		return bool(*v)
	case *tree.DString:
		return string(*v)
	case *tree.DBytes:
		return string(*v)
	default:
		return d.String()
	}
}
