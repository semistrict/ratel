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

// Package gossip is a minimal stub retained only for protobuf type
// compatibility. The gossip protocol has been removed from Ratel and replaced
// by rangefeed-backed descriptor stores, a liveness rangefeed, and S3-based
// node discovery.
package gossip

// Gossip is a deprecated stub type. It exists only so that code referencing
// *gossip.Gossip (e.g. test server interfaces) continues to compile.
// All fields and methods have been removed.
type Gossip struct{}
