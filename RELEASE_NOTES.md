# Release Notes

## Unreleased

### Explicit Actors

Ratel now supports **explicit actors** -- lightweight, per-entity isolated data
scopes inspired by Cloudflare Durable Objects. Each actor gets its own keyspace
within the cluster, designed to scale to billions of small actors.

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

- Each actor is a contiguous key prefix (truncated SHA-256 hash). Multiple small
  actors share Raft ranges. When a range grows large, the split queue splits it
  at an actor boundary -- never through the middle of an actor's data.
- A single large actor that fills its own range cannot be split further.
  `kv.actor.max_size` (default 4 GiB) rejects writes via KV backpressure once
  an actor occupies a dedicated range and exceeds the limit.
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
