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
	"github.com/semistrict/ratel/pkg/sql/sem/tree"
)

// JSONSelectedPathResultCache memoizes materialized selected-path JSON results
// so JSON and text projections over the same path can share one subtree build.
type JSONSelectedPathResultCache struct {
	cachedJSON tree.Datum
	cachedText tree.Datum
	haveJSON   bool
	haveText   bool
}

// Reset clears all cached selected-path result datums.
func (c *JSONSelectedPathResultCache) Reset() {
	c.cachedJSON = nil
	c.cachedText = nil
	c.haveJSON = false
	c.haveText = false
}

// ResultDatum returns the cached result for kind, materializing from builder if
// necessary. Nil builder means the selected path was absent and yields SQL NULL.
func (c *JSONSelectedPathResultCache) ResultDatum(
	builder *SubordinateJSONBuilder, kind JSONAccessKind,
) (tree.Datum, error) {
	switch kind {
	case JSONAccessFetchJSONPath:
		if c.haveJSON {
			return c.cachedJSON, nil
		}
		if builder == nil {
			c.cachedJSON = tree.DNull
			c.haveJSON = true
			return c.cachedJSON, nil
		}
		d, err := builder.Materialize()
		if err != nil {
			return nil, err
		}
		c.cachedJSON = d
		c.haveJSON = true
		return d, nil
	case JSONAccessFetchTextPath:
		if c.haveText {
			return c.cachedText, nil
		}
		d, err := c.ResultDatum(builder, JSONAccessFetchJSONPath)
		if err != nil {
			return nil, err
		}
		if d == tree.DNull {
			c.cachedText = tree.DNull
			c.haveText = true
			return c.cachedText, nil
		}
		txt, err := d.(*tree.DJSON).JSON.AsText()
		if err != nil {
			return nil, err
		}
		if txt == nil {
			c.cachedText = tree.DNull
			c.haveText = true
			return c.cachedText, nil
		}
		c.cachedText = tree.NewDString(*txt)
		c.haveText = true
		return c.cachedText, nil
	default:
		return nil, errors.AssertionFailedf("unexpected selected JSON result kind %d", kind)
	}
}
