# Release Notes

## Unreleased

### Explicit Actors

Ratel now supports **explicit actors** -- lightweight, per-entity isolated data
scopes inspired by Cloudflare Durable Objects. Each actor gets its own keyspace
within the cluster, confined to a single Raft range with automatic sticky splits
and size-based backpressure.

**SQL syntax:**

```sql
-- Session-level scoping for single-actor workloads:
SET actor_scope = 'alice';
INSERT INTO orders VALUES (1, 'widget', 9.99);
SELECT * FROM orders;
SET actor_scope = '';

-- Explicit per-table qualifier (enables cross-actor queries):
SELECT * FROM actor('alice').orders;
INSERT INTO actor('bob').orders VALUES (2, 'gadget', 19.99);
UPDATE actor('alice').orders SET price = 10.99 WHERE id = 1;
DELETE FROM actor('bob').orders WHERE id = 2;

-- Cross-actor transaction:
BEGIN;
UPDATE actor('alice').balance SET amount = amount - 100;
UPDATE actor('bob').balance SET amount = amount + 100;
COMMIT;
```

**Key properties:**

- Each actor is a contiguous key prefix (truncated SHA-256 hash). On first
  write, a sticky split creates a dedicated Raft range for the actor. From that
  point on, the actor cannot be split internally and cannot be merged with
  adjacent actors -- one actor, one range.
- `kv.actor.max_size` cluster setting (default 4 GiB) rejects writes that would
  exceed the limit via KV backpressure.
- The `system.actors` table provides a registry for collision detection and
  enumeration.
- `actor_scope` and `actor('name').table` are mutually exclusive -- using both
  in the same query is an error.
- `crdb_internal.delete_actor('name')` performs hard deletion of all actor data
  and its registry entry.

### Build Changes

- `make all` now builds the `ratel` binary by default (previously built
  `cockroachoss`).
- The ratel binary now includes the web UI.
- Old CockroachDB build targets renamed: `oldbuildoss`, `oldbuildshort`.
