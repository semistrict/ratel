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
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.

package udfruntime

// ValType identifies the JavaScript value type for UDF parameter/result marshaling.
type ValType byte

const (
	ValI32 ValType = 0x7F
	ValI64 ValType = 0x7E
	ValF64 ValType = 0x7C

	ValString    ValType = 0x01
	ValBytes     ValType = 0x02
	ValTimestamp ValType = 0x03
	ValJSON      ValType = 0x04
)
