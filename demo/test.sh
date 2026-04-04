#!/bin/bash
set -euo pipefail

CLUSTER_URL="s3://ratel/?endpoint=http://rustfs:9000&region=us-east-1"

run_sql() {
    docker compose exec -T ratel-init ratel sql "$CLUSTER_URL" -e "$1"
}

echo "=== Waiting for cluster ==="
for i in $(seq 1 30); do
    if run_sql "SELECT 1" >/dev/null 2>&1; then
        echo "Cluster is ready."
        break
    fi
    if [ "$i" -eq 30 ]; then
        echo "FAIL: cluster not ready after 30 attempts"
        exit 1
    fi
    sleep 1
done

echo ""
echo "=== Creating table ==="
run_sql "DROP TABLE IF EXISTS test_data"
run_sql "CREATE TABLE test_data (id INT PRIMARY KEY, name STRING, value FLOAT)"

echo ""
echo "=== Inserting rows ==="
run_sql "INSERT INTO test_data VALUES (1, 'alpha', 3.14), (2, 'beta', 2.72), (3, 'gamma', 1.62), (4, 'delta', 0.58), (5, 'epsilon', 1.41)"

echo ""
echo "=== SELECT * ==="
run_sql "SELECT * FROM test_data ORDER BY id"

echo ""
echo "=== Aggregate query ==="
run_sql "SELECT count(*), avg(value), min(name), max(name) FROM test_data"

echo ""
echo "=== Filter query ==="
run_sql "SELECT name, value FROM test_data WHERE value > 2.0 ORDER BY value DESC"

echo ""
echo "=== Update ==="
run_sql "UPDATE test_data SET value = value * 2 WHERE id = 1"
run_sql "SELECT id, name, value FROM test_data WHERE id = 1"

echo ""
echo "=== Delete ==="
run_sql "DELETE FROM test_data WHERE id = 5"
run_sql "SELECT count(*) AS remaining FROM test_data"

echo ""
echo "=== Show nodes ==="
run_sql "SELECT node_id, epoch, expiration FROM crdb_internal.gossip_liveness ORDER BY node_id"

echo ""
echo "=== Drop table ==="
run_sql "DROP TABLE test_data"

echo ""
echo "=== ALL TESTS PASSED ==="
