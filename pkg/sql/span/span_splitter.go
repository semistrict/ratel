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

package span

import (
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/descpb"
	"github.com/cockroachdb/cockroach/pkg/sql/opt/exec"
	"github.com/cockroachdb/cockroach/pkg/util/intsets"
)

// Splitter is a helper for splitting single-row spans into more specific spans.
//
// In the single-family layout this is always a no-op, but the type remains in
// place so planner and execution code do not need a wide signature change.
type Splitter struct{}

// NoopSplitter returns a splitter that never splits spans.
func NoopSplitter() Splitter {
	return Splitter{}
}

// MakeSplitter returns a no-op splitter. Column-family-specific span splitting
// is no longer used.
func MakeSplitter(
	table catalog.TableDescriptor, index catalog.Index, neededColOrdinals intsets.Fast,
) Splitter {
	_ = table
	_ = index
	_ = neededColOrdinals
	return NoopSplitter()
}

// MakeSplitterWithFamilyIDs returns a no-op splitter for compatibility with
// callers that still pass column-family IDs.
func MakeSplitterWithFamilyIDs(numKeyCols int, familyIDs []descpb.FamilyID) Splitter {
	_ = numKeyCols
	_ = familyIDs
	return NoopSplitter()
}

// MakeSplitterForDelete returns a no-op splitter for delete-range fast paths.
func MakeSplitterForDelete(
	table catalog.TableDescriptor,
	index catalog.Index,
	needed exec.TableColumnOrdinalSet,
	forDelete bool,
) Splitter {
	_ = table
	_ = index
	_ = needed
	_ = forDelete
	return NoopSplitter()
}

// FamilyIDs returns no family IDs in the single-row-group layout.
func (s *Splitter) FamilyIDs() []descpb.FamilyID {
	return nil
}

// IsNoop returns true if this instance will never split spans.
func (s *Splitter) IsNoop() bool {
	return true
}

// AppendSpan appends the input span unchanged.
//
// prefixLen is the number of index columns encoded in the span.
// containsNull indicates if one of the encoded columns was NULL.
//
// The function accepts a slice of spans to append to.
func (s *Splitter) AppendSpan(
	appendTo roachpb.Spans, span roachpb.Span, prefixLen int, containsNull bool,
) roachpb.Spans {
	_ = prefixLen
	_ = containsNull
	return append(appendTo, span)
}

// MaybeSplitSpanIntoSeparateFamilies appends the input span unchanged.
func (s *Splitter) MaybeSplitSpanIntoSeparateFamilies(
	appendTo roachpb.Spans, span roachpb.Span, prefixLen int, containsNull bool,
) roachpb.Spans {
	return s.AppendSpan(appendTo, span, prefixLen, containsNull)
}

// ExistenceCheckSpan returns the span used to check whether a row exists.
func (s *Splitter) ExistenceCheckSpan(
	span roachpb.Span, prefixLen int, containsNull bool,
) roachpb.Span {
	_ = prefixLen
	_ = containsNull
	return span
}
