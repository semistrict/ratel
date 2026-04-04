// Copyright 2024 The Cockroach Authors.
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

package rateltest

import (
	gosql "database/sql"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestMultiNodeWriteRead verifies that data written through one node is
// readable from another node. This catches issues like:
//   - CreatorID collisions (all nodes using the same Pebble CreatorID)
//   - Shared metadata corruption (multiple nodes writing the same MANIFEST)
//   - SSTable filename collisions on shared storage
func TestMultiNodeWriteRead(t *testing.T) {
	tc, ss := StartCluster(t, 3)
	defer tc.Stopper().Stop(t.Context())
	defer ss.Close()

	db0 := tc.ServerConn(0)
	db1 := tc.ServerConn(1)
	db2 := tc.ServerConn(2)

	// Create table via node 0.
	ExecSQL(t, db0, "CREATE TABLE test_shared (id INT PRIMARY KEY, origin INT, value STRING)")

	// Insert via each node.
	for i, db := range []*gosql.DB{db0, db1, db2} {
		ExecSQL(t, db, "INSERT INTO test_shared VALUES ($1, $2, $3)", i+1, i, fmt.Sprintf("from-node-%d", i))
	}

	// Read from each node — all should see all 3 rows.
	for i, db := range []*gosql.DB{db0, db1, db2} {
		count := QueryCountSQL(t, db, "test_shared")
		require.Equal(t, 3, count, "node %d should see 3 rows", i)
	}

	// Verify specific values across nodes.
	var value string
	QueryRowSQL(t, db2, "SELECT value FROM test_shared WHERE id = 1", &value)
	require.Equal(t, "from-node-0", value, "node 2 should read data written by node 0")

	QueryRowSQL(t, db0, "SELECT value FROM test_shared WHERE id = 3", &value)
	require.Equal(t, "from-node-2", value, "node 0 should read data written by node 2")
}

// TestMultiNodeAggregation verifies that SQL aggregation works correctly
// across data written by multiple nodes.
func TestMultiNodeAggregation(t *testing.T) {
	tc, ss := StartCluster(t, 3)
	defer tc.Stopper().Stop(t.Context())
	defer ss.Close()

	db0 := tc.ServerConn(0)
	db1 := tc.ServerConn(1)
	db2 := tc.ServerConn(2)

	ExecSQL(t, db0, "CREATE TABLE test_agg (id INT PRIMARY KEY, amount FLOAT)")

	// Each node inserts different data.
	ExecSQL(t, db0, "INSERT INTO test_agg VALUES (1, 10.0), (2, 20.0)")
	ExecSQL(t, db1, "INSERT INTO test_agg VALUES (3, 30.0), (4, 40.0)")
	ExecSQL(t, db2, "INSERT INTO test_agg VALUES (5, 50.0)")

	// Aggregate from a different node than any writer.
	var sum float64
	var count int
	QueryRowSQL(t, db1, "SELECT count(*), sum(amount) FROM test_agg", &count, &sum)
	require.Equal(t, 5, count)
	require.InDelta(t, 150.0, sum, 0.01)
}

// TestMultiNodeSchemaChange verifies that DDL changes propagate across nodes.
func TestMultiNodeSchemaChange(t *testing.T) {
	tc, ss := StartCluster(t, 3)
	defer tc.Stopper().Stop(t.Context())
	defer ss.Close()

	db0 := tc.ServerConn(0)
	db1 := tc.ServerConn(1)
	db2 := tc.ServerConn(2)

	// Create and populate via node 0.
	ExecSQL(t, db0, "CREATE TABLE test_schema (id INT PRIMARY KEY, name STRING)")
	ExecSQL(t, db0, "INSERT INTO test_schema VALUES (1, 'alice')")

	// Add column via node 1.
	ExecSQL(t, db1, "ALTER TABLE test_schema ADD COLUMN age INT DEFAULT 0")

	// Insert with new column via node 2.
	ExecSQL(t, db2, "INSERT INTO test_schema VALUES (2, 'bob', 30)")

	// Read from node 0 — should see both rows with the new column.
	var name string
	var age int
	QueryRowSQL(t, db0, "SELECT name, age FROM test_schema WHERE id = 2", &name, &age)
	require.Equal(t, "bob", name)
	require.Equal(t, 30, age)

	// Original row should have default value.
	QueryRowSQL(t, db0, "SELECT age FROM test_schema WHERE id = 1", &age)
	require.Equal(t, 0, age)
}
