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

### Workers Platform

Ratel now includes a **workers platform** that runs JavaScript workers with
Durable Object storage, using Cloudflare's open-source
[workerd](https://github.com/cloudflare/workerd) runtime as a sidecar process.

**Deploy and invoke workers:**

```bash
# Deploy a worker
curl -X PUT http://localhost:26257/api/v2/workers/hello/ \
  -d 'export default { async fetch() { return new Response("hello"); } };'

# Invoke it
curl http://localhost:26257/workers/hello/
# → hello
```

**Durable Objects with persistent storage:**

```bash
# Deploy a worker with a Durable Object class
curl -X PUT http://localhost:26257/api/v2/workers/counter/ \
  -H 'X-Bindings: {"durable_objects": [{"class_name": "Counter"}]}' \
  -d '
export class Counter {
  constructor(state) { this.state = state; }
  async fetch() {
    let val = await this.state.storage.get("count") || 0;
    val++;
    await this.state.storage.put("count", val);
    return new Response(String(val));
  }
}
export default {
  async fetch(request, env) {
    const id = env.Counter.idFromName("singleton");
    return env.Counter.get(id).fetch(request);
  }
};'

curl http://localhost:26257/workers/counter/  # → 1
curl http://localhost:26257/workers/counter/  # → 2
curl http://localhost:26257/workers/counter/  # → 3
```

**Architecture:**

- Workers run inside a workerd sidecar process, managed automatically by Ratel.
- Worker scripts are stored in the `system.worker_scripts` system table and
  survive node restarts.
- HTTP requests arrive on the same port as PostgreSQL (protocol sniffing via
  cmux). Paths under `/workers/<name>/` are reverse-proxied to workerd.
- Durable Object storage uses **Cap'n Proto RPC** over a Unix socketpair between
  workerd and Ratel. Each DO gets an actor-scoped key prefix in Ratel's KV layer,
  with full Raft consensus for writes.
- DO storage operations (get/put/delete/list/deleteAll) are first-class KV
  operations with the same durability guarantees as SQL data.

**Performance (single node, Apple Silicon):**

| Path | Latency |
|------|---------|
| DO storage get (capnp → KV) | 21 us |
| DO storage put (capnp → KV) | 32 us |
| Full e2e (HTTP → workerd → DO → KV) | 473 us |

### Build Changes

- `make all` now builds the `ratel` binary by default (previously built
  `cockroachoss`).
- The ratel binary now includes the web UI.
- Old CockroachDB build targets renamed: `oldbuildoss`, `oldbuildshort`.
