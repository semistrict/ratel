// Copyright 2019 The Cockroach Authors.
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

package builtins

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/semistrict/ratel/pkg/base"
	"github.com/semistrict/ratel/pkg/kv"
	"github.com/semistrict/ratel/pkg/sql/sem/tree"
	"github.com/semistrict/ratel/pkg/testutils/serverutils"
	jsonutil "github.com/semistrict/ratel/pkg/util/json"
	"github.com/semistrict/ratel/pkg/util/leaktest"
	"github.com/stretchr/testify/require"
)

func TestConcurrentProcessorsReadEpoch(t *testing.T) {
	defer leaktest.AfterTest(t)()
	ctx := context.Background()
	params := base.TestServerArgs{
		Knobs: base.TestingKnobs{
			SQLEvalContext: &tree.EvalContextTestingKnobs{
				CallbackGenerators: map[string]*tree.CallbackValueGenerator{
					"my_callback": tree.NewCallbackValueGenerator(
						func(ctx context.Context, prev int, _ *kv.Txn) (int, error) {
							if prev < 10 {
								return prev + 1, nil
							}
							return -1, nil
						}),
				},
			},
		},
	}
	s, db, _ := serverutils.StartServer(t, params)
	defer s.Stopper().Stop(ctx)

	rows, err := db.Query(` select * from crdb_internal.testing_callback('my_callback')`)
	require.NoError(t, err)
	exp := 1
	for rows.Next() {
		var got int
		require.NoError(t, rows.Scan(&got))
		require.Equal(t, exp, got)
		exp++
	}
}

type maybeDecodeSpy struct {
	jsonutil.JSON
	maybeDecodeCalls int
}

type arrayValueIteratorFunc func(context.Context) (jsonutil.JSON, bool, error)

func (fn arrayValueIteratorFunc) NextValue(ctx context.Context) (jsonutil.JSON, bool, error) {
	return fn(ctx)
}

func (fn arrayValueIteratorFunc) Close(context.Context) {}

func (s *maybeDecodeSpy) MaybeDecode() jsonutil.JSON {
	s.maybeDecodeCalls++
	return s.JSON.MaybeDecode()
}

func TestJSONArrayGeneratorStartDoesNotEagerlyDecodeWholeArray(t *testing.T) {
	defer leaktest.AfterTest(t)()

	parsed, err := jsonutil.ParseJSON(`[{"test":1},{"test":2},{"test":3}]`)
	require.NoError(t, err)

	spy := &maybeDecodeSpy{JSON: parsed}
	genIface, err := makeJSONArrayGenerator(tree.Datums{tree.NewDJSON(spy)}, false /* asText */)
	require.NoError(t, err)

	gen := genIface.(*jsonArrayGenerator)
	require.NoError(t, gen.Start(context.Background(), nil /* txn */))

	require.Zero(t, spy.maybeDecodeCalls,
		"jsonb_array_elements should stream array elements without eagerly MaybeDecode()ing the whole array")
}

func TestJSONArrayGeneratorPrefersStreamingIteratorOverIndexedFetch(t *testing.T) {
	defer leaktest.AfterTest(t)()

	var indexedFetchCalls int
	var iteratorCalls int
	lazy := jsonutil.NewLazyArrayWithIterator(
		3,
		func(_ context.Context, idx int) (jsonutil.JSON, error) {
			indexedFetchCalls++
			return jsonutil.FromInt(idx), nil
		},
		func(context.Context) (jsonutil.JSON, error) {
			return jsonutil.ParseJSON(`[0,1,2]`)
		},
		func() jsonutil.ArrayValueIterator {
			next := 0
			return arrayValueIteratorFunc(func(_ context.Context) (jsonutil.JSON, bool, error) {
				if next >= 3 {
					return nil, false, nil
				}
				iteratorCalls++
				v := next
				next++
				return jsonutil.FromInt(v), true, nil
			})
		},
	)

	genIface, err := makeJSONArrayGenerator(tree.Datums{tree.NewDJSON(lazy)}, false /* asText */)
	require.NoError(t, err)

	gen := genIface.(*jsonArrayStreamGenerator)
	require.NoError(t, gen.Start(context.Background(), nil /* txn */))
	defer gen.Close(context.Background())

	for i := 0; i < 3; i++ {
		ok, err := gen.Next(context.Background())
		require.NoError(t, err)
		require.True(t, ok)
		vals, err := gen.Values()
		require.NoError(t, err)
		require.Equal(t, tree.NewDJSON(jsonutil.FromInt(i)), vals[0])
	}
	ok, err := gen.Next(context.Background())
	require.NoError(t, err)
	require.False(t, ok)

	require.Equal(t, 3, iteratorCalls)
	require.Zero(t, indexedFetchCalls,
		"jsonb_array_elements should use the array's streaming iterator instead of per-index fetches when available")
}

type largeJSONArrayAggregateCounters struct {
	rootDecode   int64
	rootIdxFetch int64
	objectDecode int64
}

type aggregateLazyObjectDriver struct {
	chunk     string
	counters  *largeJSONArrayAggregateCounters
	idx       int
	testValue jsonutil.JSON
	junkValue jsonutil.JSON
}

func (d *aggregateLazyObjectDriver) FetchValKeyContext(
	_ context.Context, key string,
) (jsonutil.JSON, error) {
	switch key {
	case "test":
		return d.testValue, nil
	case "junk":
		return d.junkValue, nil
	case "i":
		return jsonutil.FromInt(d.idx), nil
	default:
		return nil, nil
	}
}

func (*aggregateLazyObjectDriver) FetchValIdxContext(context.Context, int) (jsonutil.JSON, error) {
	return nil, nil
}

func (d *aggregateLazyObjectDriver) DecodeContext(context.Context) (jsonutil.JSON, error) {
	atomic.AddInt64(&d.counters.objectDecode, 1)
	return jsonutil.ParseJSON(fmt.Sprintf(`{"test":1,"junk":"%s","i":%d}`, d.chunk, d.idx))
}

func makeLargeLazyJSONArrayAggregateInput(targetBytes int) (tree.Datum, int, *largeJSONArrayAggregateCounters) {
	const approxElementBytes = 270
	count := targetBytes / approxElementBytes
	if count < 1 {
		count = 1
	}
	chunk := strings.Repeat("x", 240)
	counters := &largeJSONArrayAggregateCounters{}
	testValue := jsonutil.FromInt(1)
	junkValue := jsonutil.FromString(chunk)

	makeObj := func(i int) jsonutil.JSON {
		return jsonutil.NewLazyObject(
			3,
			func(_ context.Context, key string) (jsonutil.JSON, error) {
				switch key {
				case "test":
					return testValue, nil
				case "junk":
					return junkValue, nil
				case "i":
					return jsonutil.FromInt(i), nil
				default:
					return nil, nil
				}
			},
			func(context.Context) (jsonutil.JSON, error) {
				atomic.AddInt64(&counters.objectDecode, 1)
				return jsonutil.ParseJSON(fmt.Sprintf(`{"test":1,"junk":"%s","i":%d}`, chunk, i))
			},
		)
	}
	driver := &aggregateLazyObjectDriver{
		chunk:     chunk,
		counters:  counters,
		testValue: testValue,
		junkValue: junkValue,
	}
	reusableObj := jsonutil.NewMutableLazyNode(driver)

	lazy := jsonutil.NewLazyArrayWithIterator(
		count,
		func(_ context.Context, idx int) (jsonutil.JSON, error) {
			atomic.AddInt64(&counters.rootIdxFetch, 1)
			return makeObj(idx), nil
		},
		func(context.Context) (jsonutil.JSON, error) {
			atomic.AddInt64(&counters.rootDecode, 1)
			var b strings.Builder
			b.Grow(targetBytes + targetBytes/8)
			b.WriteByte('[')
			for i := 0; i < count; i++ {
				if i > 0 {
					b.WriteByte(',')
				}
				fmt.Fprintf(&b, `{"test":1,"junk":"%s","i":%d}`, chunk, i)
			}
			b.WriteByte(']')
			return jsonutil.ParseJSON(b.String())
		},
		func() jsonutil.ArrayValueIterator {
			next := 0
			return arrayValueIteratorFunc(func(_ context.Context) (jsonutil.JSON, bool, error) {
				if next >= count {
					return nil, false, nil
				}
				driver.idx = next
				reusableObj.ResetObject(3)
				next++
				return reusableObj, true, nil
			})
		},
	)
	return tree.NewDJSON(lazy), count, counters
}

func TestJSONArrayGeneratorAggregateFieldAvoidsWholeValueDecodes(t *testing.T) {
	defer leaktest.AfterTest(t)()

	ctx := context.Background()
	for _, tc := range []struct {
		name       string
		targetSize int
	}{
		{name: "1MiB", targetSize: 1 << 20},
		{name: "8MiB", targetSize: 8 << 20},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input, expected, counters := makeLargeLazyJSONArrayAggregateInput(tc.targetSize)
			gen, err := makeJSONArrayGenerator(tree.Datums{input}, false /* asText */)
			require.NoError(t, err)
			require.NoError(t, gen.Start(ctx, nil /* txn */))
			defer gen.Close(ctx)

			sum := 0
			for {
				ok, err := gen.Next(ctx)
				require.NoError(t, err)
				if !ok {
					break
				}
				vals, err := gen.Values()
				require.NoError(t, err)
				j := vals[0].(*tree.DJSON).JSON
				field, err := j.FetchValKey("test")
				require.NoError(t, err)
				require.NotNil(t, field)
				dec, ok := field.AsDecimal()
				require.True(t, ok)
				v, err := dec.Int64()
				require.NoError(t, err)
				sum += int(v)
			}

			require.Equal(t, expected, sum)
			require.Zero(t, atomic.LoadInt64(&counters.rootDecode))
			require.Zero(t, atomic.LoadInt64(&counters.rootIdxFetch))
			require.Zero(t, atomic.LoadInt64(&counters.objectDecode))
		})
	}
}
