# Workers Platform

- Status: Draft
- Date: 2026-04-12

## Summary

Ratel embeds Cloudflare's open-source workerd runtime as a sidecar process to
provide a Workers + Durable Objects platform. Workers are stateless JavaScript
handlers that run on any node. Durable Objects are stateful actors that run on
the leaseholder node for their data, with local KV access through Ratel's actor
key encoding.

## Architecture

```
                 Client HTTP
                      |
                      v
               Any Ratel Node
          +-----------------------+
          |  Ratel (Go)           |
          |  - HTTP ingress       |    <-- routes by Host/path to worker name
          |  - Worker registry    |    <-- system.worker_scripts
          |  - DO routing         |    <-- actor hash -> leaseholder lookup
          |                       |
          |       | Cap'n Proto   |
          |       v               |
          |  workerd (C++ sidecar)|
          |  - V8 isolate pool    |    <-- executes JS
          |  - Worker fetch()     |
          |  - DO instantiation   |
          |       |               |
          |       | Cap'n Proto   |
          |       v               |
          |  Ratel KV (local)     |    <-- actor-storage.capnp Operations
          +-----------------------+
```

Each Ratel node runs one workerd sidecar. Communication between Ratel and
workerd uses Cap'n Proto RPC over a Unix domain socket.

## Worker scripts

Stored in Ratel's system catalog:

```sql
CREATE TABLE system.worker_scripts (
    name         STRING NOT NULL,
    version      INT NOT NULL DEFAULT 1,
    script       BYTES NOT NULL,
    compat_date  STRING NOT NULL,
    bindings     JSONB,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (name, version)
);
```

Deploy via HTTP:

```
PUT /v1/workers/{name}
Content-Type: application/javascript

export default {
  async fetch(request, env) {
    return new Response("hello");
  }
}
```

The deploy handler inserts a new version row. Previous versions are retained.
On deploy, Ratel notifies all nodes (via gossip or rangefeed) to reload the
worker definition.

## HTTP ingress

Each Ratel node listens on a workers HTTP port (separate from the SQL and admin
ports). Routing from request to worker:

1. Match `Host` header against a routing table (stored in
   `system.worker_routes`)
2. Look up the latest version of the worker script
3. Forward the request to the local workerd sidecar via Cap'n Proto RPC
4. workerd executes the worker's `fetch()` handler in a V8 isolate
5. Return the Response to the client

Workers are stateless. Any node can serve any worker.

## Durable Object routing

When a worker calls `env.MY_DO.get(id).fetch(request)`:

1. **Hash the DO ID** to get the actor key prefix (same as Ratel's actor hash)
2. **Find the leaseholder** for the range containing that actor key (via
   DistSender's range cache)
3. **If local**: dispatch directly to the local workerd sidecar
4. **If remote**: RPC the request to the leaseholder node, which dispatches to
   its local workerd sidecar

The leaseholder node instantiates the DO class (loading the script from
`system.worker_scripts` if not cached) and calls its `fetch()` method.

### DO ID mapping

The actor identity includes the DO class name. This gives each class its own
isolated keyspace per instance, matching Cloudflare's model where each class
has a separate storage namespace.

The 16-byte actor hash is split into two parts to promote co-location of
actors sharing the same entity name across different classes:

```
actor_hash = trunc96(SHA256(name)) ++ trunc32(SHA256(class + ":" + name))
             ---- 12 bytes ----       ---- 4 bytes ----
             range placement          class discriminator
```

- The first 12 bytes are derived from the **name alone**.
  `OrderActor.idFromName("alice")` and `ChatActor.idFromName("alice")` share
  this prefix and will land in the same Raft range.
- The last 4 bytes are derived from **class+name**, giving each class its own
  keyspace. The two actors have independent data despite sharing a range.
- `newUniqueId()` generates a random hash within the caller's range (see
  below). The class name is recorded in `system.actors` for observability.

```
idFromName("alice") in OrderActor  →  SHA256("alice")[0:12] ++ SHA256("OrderActor:alice")[0:4]
idFromName("alice") in ChatActor   →  SHA256("alice")[0:12] ++ SHA256("ChatActor:alice")[0:4]
                                      ^^^^ same prefix ^^^^    ^^^^ different suffix ^^^^
```

This means all of "alice"'s DOs — orders, chat, inventory — are co-located
on the same node. Cross-DO calls between them are local.

## Differences from Cloudflare Workers

### PostgreSQL SQL syntax

Ratel uses PostgreSQL-compatible SQL (CockroachDB dialect). DOs that use the
SQL storage API (`storage.sql`) execute queries in PostgreSQL syntax, not
SQLite syntax. This affects:

- String concatenation: `||` (same in both)
- Type system: PostgreSQL types (INT8, STRING, TIMESTAMPTZ, etc.)
- JSON: `JSONB` type with PostgreSQL operators (`->`, `->>`, `@>`)
- Upsert: `INSERT ... ON CONFLICT DO UPDATE` (same in both)
- No `PRAGMA`, `AUTOINCREMENT`, or SQLite-specific features
- Window functions, CTEs, and set operations follow PostgreSQL behavior

Workers ported from Cloudflare that use `storage.sql` may need query
adjustments. Workers that use only the KV API (`storage.get/put/delete/list`)
are fully compatible.

### Schema defined outside the worker

On Cloudflare, each DO manages its own schema — the DO constructor typically
runs `CREATE TABLE IF NOT EXISTS` on first instantiation. Tables are private
to each DO instance (separate SQLite databases).

On Ratel, schema is defined at the database level, outside and independent of
any worker or DO:

```sql
-- Admin creates tables via psql / SQL client
CREATE TABLE orders (id INT PRIMARY KEY, item STRING, price DECIMAL);
CREATE TABLE inventory (sku STRING PRIMARY KEY, qty INT);
```

All actors (DOs) of a given class share the same table schema. Each actor sees
only its own rows (isolated by the actor key prefix), but the columns and
indexes are the same for all actors. This is the same as Ratel's existing
actor model.

Consequences:

- **No DDL in DO code.** Workers cannot CREATE/ALTER/DROP tables. Schema
  changes are an admin operation, applied once and shared by all actors.
- **Schema is tenant-global.** All actors see the same tables, indexes, and
  constraints. An actor cannot have a table that other actors don't have.
- **Migrations are simpler.** One `ALTER TABLE` applies to all actors. No need
  to run migrations inside each DO instance.
- **DOs query real tables, not KV.** Instead of `storage.sql("SELECT * FROM
  my_table")` creating an ad-hoc SQLite table per DO, the DO queries
  Ratel tables that were defined by an admin. The actor scoping is implicit.

Example DO accessing actor-scoped data:

```javascript
export class OrderActor {
  constructor(state, env) {
    this.state = state;
    this.sql = state.storage.sql;
  }

  async fetch(request) {
    // This query is scoped to the actor automatically.
    // The 'orders' table was created by an admin via SQL.
    const orders = this.sql`SELECT * FROM orders`;
    return Response.json(orders);
  }
}
```

### newUniqueId() is range-local

On Cloudflare, `newUniqueId()` produces a globally random ID. The new DO
can land on any machine.

On Ratel, `newUniqueId()` called from within a DO generates an ID whose
actor hash falls within the caller's Raft range:

```javascript
// Inside a DO handler:
const id = env.MY_DO.newUniqueId();
const helper = env.MY_DO.get(id);
await helper.fetch(req);  // same node, no network hop
```

Implementation: look up the range containing the current actor, determine
the actor hash space it covers, generate a random hash within that space.

This means DOs that create other DOs get locality by default. No opt-in
required. The co-location holds as long as the range hasn't split between
the two actors. For small actors this is durable in practice.

`idFromName(name)` is deterministic and maps to a specific hash anywhere
in the cluster. Use it when you need a well-known global identity.

## DO storage: local KV via actor-storage.capnp

workerd already defines a Cap'n Proto RPC interface for DO storage
(`src/workerd/io/actor-storage.capnp`). In the open-source release, this is
backed by local SQLite. In our platform, we implement it backed by the local
Ratel KV store.

### Implementation

Ratel runs a Cap'n Proto RPC server on a Unix domain socket. For each active
DO, it provides an `Operations` capability:

```
Operations.get(key)        -> Ratel KV Get on actor-scoped key
Operations.put(entries)    -> Ratel KV Put batch
Operations.delete(keys)    -> Ratel KV Delete batch
Operations.list(...)       -> Ratel KV Scan with actor prefix
Operations.transaction()   -> Ratel KV Txn
Operations.getAlarm()      -> Read from actor metadata
Operations.setAlarm(time)  -> Write to actor metadata + schedule
```

The key encoding for DO KV entries within an actor:

```
[TENANT PREFIX][0xfb][ACTOR HASH 16B][KV_TABLE_ID][INDEX_ID][user_key]
```

A synthetic table ID is reserved for DO KV storage. The user key from
`storage.put('mykey', value)` becomes the row key.

### Data locality

The DO is instantiated on the leaseholder for its actor's range. All KV
operations go to the local Ratel store. No network hops for reads or writes.

workerd's in-memory cache (`ActorCacheOps`) further reduces KV traffic:
- Reads hit the cache first
- Consecutive writes without `await` are batched into one transaction
- Most operations never reach Ratel

### Data path

```
DO JS: await this.state.storage.put('key', 'value')
  -> workerd ActorCacheOps (in-memory batch)
  -> Cap'n Proto RPC (Unix domain socket, ~microseconds)
  -> Ratel KV Put (local leaseholder, no network)
  -> Raft commit (replicated to followers)
  -> Ack back to JS
```

## DO lifecycle

### Instantiation

On-demand when a request arrives. The leaseholder node:

1. Checks if a workerd isolate is already running for this DO
2. If not, loads the worker script (cached locally after first fetch from
   `system.worker_scripts`)
3. Creates an `Operations` capability connected to the local KV store for
   this actor
4. Starts the isolate, passes the capability
5. Calls the DO constructor with `DurableObjectState`

### Eviction

After an idle timeout (configurable, default 60s), the DO is evicted:

1. Drain in-flight requests
2. Flush the write cache
3. Destroy the isolate
4. Release the `Operations` capability

### Lease transfer

If the leaseholder moves (rebalancing, node failure):

1. Old leaseholder detects lease loss
2. Drains the DO (same as eviction)
3. New leaseholder re-instantiates on next request
4. workerd's cache is cold but the data is in Ratel (replicated)

In-flight requests to the old leaseholder get a retriable error. The client
(the worker that called `stub.fetch()`) retries, and the routing layer finds
the new leaseholder.

## Sidecar management

Ratel manages the workerd sidecar process:

- Starts workerd on node startup
- Monitors health (restart on crash)
- Passes configuration via Cap'n Proto config file
- Communicates via Unix domain socket
- Shuts down on node shutdown

workerd config is generated by Ratel and includes:

- The Cap'n Proto RPC socket address for storage
- V8 flags and memory limits
- Compatibility settings

Worker scripts are NOT baked into the workerd config. They are loaded
dynamically via a Ratel-provided `WorkerLoader` binding (workerd supports
dynamic worker loading).

## What needs building

### Phase 1: Foundation
- [ ] Build workerd from source, package as sidecar binary
- [ ] Implement `actor-storage.capnp` Operations backed by Ratel KV
- [ ] Sidecar process management (start, monitor, restart)
- [ ] Unix domain socket Cap'n Proto RPC between Ratel and workerd

### Phase 2: Worker management
- [ ] `system.worker_scripts` table and deploy HTTP API
- [ ] Dynamic worker loading in workerd from Ratel
- [ ] HTTP ingress routing (Host/path -> worker)

### Phase 3: Durable Objects
- [ ] DO routing: actor hash -> leaseholder -> RPC
- [ ] DO instantiation and eviction on workerd sidecar
- [ ] DO alarm scheduling integration
- [ ] Lease transfer drain and re-instantiation

### Phase 4: Production readiness
- [ ] Worker script size limits and validation
- [ ] Resource limits (CPU time, memory per isolate)
- [ ] Observability (request logs, DO storage metrics)
- [ ] `system.worker_routes` for HTTP routing configuration
