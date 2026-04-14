// Copyright 2018 The Cockroach Authors.
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

package gossip

// Minimal stubs for types referenced by gossip.pb.go.
// The gossip protocol has been removed; these exist only for proto compat.

import (
	"time"

	"github.com/cockroachdb/redact"
)

func roundSecs(d time.Duration) time.Duration {
	return d.Truncate(time.Second)
}

func (m MetricSnap) String() string          { return redact.StringWithoutMarkers(m) }
func (c OutgoingConnStatus) String() string  { return redact.StringWithoutMarkers(c) }
func (c ClientStatus) String() string        { return redact.StringWithoutMarkers(c) }
func (c ConnStatus) String() string          { return redact.StringWithoutMarkers(c) }
func (s ServerStatus) String() string        { return redact.StringWithoutMarkers(s) }
func (c Connectivity) String() string        { return redact.StringWithoutMarkers(c) }

// SafeFormat implements the redact.SafeFormatter interface.
func (m MetricSnap) SafeFormat(w redact.SafePrinter, _ rune) {
	w.Printf("infos %d/%d sent/received, bytes %dB/%dB sent/received",
		m.InfosSent, m.InfosReceived, m.BytesSent, m.BytesReceived)
}

// SafeFormat implements the redact.SafeFormatter interface.
func (c OutgoingConnStatus) SafeFormat(w redact.SafePrinter, _ rune) {
	w.Printf("%d: %s (%s: %s)", c.NodeID, c.Address,
		roundSecs(time.Duration(c.AgeNanos)), c.MetricSnap)
}

// SafeFormat implements the redact.SafeFormatter interface.
func (c ClientStatus) SafeFormat(w redact.SafePrinter, _ rune) {
	w.Printf("gossip client (%d/%d cur/max conns)\n", len(c.ConnStatus), c.MaxConns)
}

// SafeFormat implements the redact.SafeFormatter interface.
func (c ConnStatus) SafeFormat(w redact.SafePrinter, _ rune) {
	w.Printf("%d: %s (%s)", c.NodeID, c.Address, roundSecs(time.Duration(c.AgeNanos)))
}

// SafeFormat implements the redact.SafeFormatter interface.
func (s ServerStatus) SafeFormat(w redact.SafePrinter, _ rune) {
	w.Printf("gossip server (%d/%d cur/max conns)", len(s.ConnStatus), s.MaxConns)
}

// SafeFormat implements the redact.SafeFormatter interface.
func (c Connectivity) SafeFormat(w redact.SafePrinter, _ rune) {
	w.Printf("gossip connectivity")
}
