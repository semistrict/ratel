// Copyright 2026 The Cockroach Authors.
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

package tracing

// These types mirror the Jaeger JSON upload schema closely enough for our trace
// export paths, without pulling in the full Jaeger module.

type jaegerReferenceType string
type jaegerTraceID string
type jaegerSpanID string
type jaegerProcessID string
type jaegerValueType string

const (
	jaegerChildOf    jaegerReferenceType = "CHILD_OF"
	jaegerStringType jaegerValueType     = "string"
	jaegerInt64Type  jaegerValueType     = "int64"
)

type jaegerTrace struct {
	TraceID   jaegerTraceID                     `json:"traceID"`
	Spans     []jaegerSpan                      `json:"spans"`
	Processes map[jaegerProcessID]jaegerProcess `json:"processes"`
	Warnings  []string                          `json:"warnings"`
}

type jaegerSpan struct {
	TraceID       jaegerTraceID     `json:"traceID"`
	SpanID        jaegerSpanID      `json:"spanID"`
	ParentSpanID  jaegerSpanID      `json:"parentSpanID,omitempty"`
	Flags         uint32            `json:"flags,omitempty"`
	OperationName string            `json:"operationName"`
	References    []jaegerReference `json:"references"`
	StartTime     uint64            `json:"startTime"`
	Duration      uint64            `json:"duration"`
	Tags          []jaegerKeyValue  `json:"tags"`
	Logs          []jaegerLog       `json:"logs"`
	ProcessID     jaegerProcessID   `json:"processID,omitempty"`
	Process       *jaegerProcess    `json:"process,omitempty"`
	Warnings      []string          `json:"warnings"`
}

type jaegerReference struct {
	RefType jaegerReferenceType `json:"refType"`
	TraceID jaegerTraceID       `json:"traceID"`
	SpanID  jaegerSpanID        `json:"spanID"`
}

type jaegerProcess struct {
	ServiceName string           `json:"serviceName"`
	Tags        []jaegerKeyValue `json:"tags"`
}

type jaegerLog struct {
	Timestamp uint64           `json:"timestamp"`
	Fields    []jaegerKeyValue `json:"fields"`
}

type jaegerKeyValue struct {
	Key   string          `json:"key"`
	Type  jaegerValueType `json:"type,omitempty"`
	Value interface{}     `json:"value"`
}
