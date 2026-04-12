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

package kvserver

import (
	"github.com/semistrict/ratel/pkg/keys"
	"github.com/semistrict/ratel/pkg/roachpb"
)

func actorSpanForRange(desc *roachpb.RangeDescriptor) (roachpb.Span, bool, error) {
	span, ok, err := keys.ActorSpanFromKey(desc.StartKey.AsRawKey())
	if err != nil || !ok {
		return roachpb.Span{}, ok, err
	}
	if !desc.EndKey.AsRawKey().Equal(span.EndKey) {
		return roachpb.Span{}, false, nil
	}
	return span, true, nil
}
