- Feature Name: Explicit Actors
- Status: draft
- Start Date: 2026-04-11
- Authors: Ramon Medeiros, OpenAI Codex
- RFC PR:
- Cockroach Issue:

# Summary

Ratel should support lightweight, explicitly addressed per-entity **actors**
within a tenant. Each actor has its own isolated table-data keyspace, shares the
tenant's schema, is created implicitly on first write, and is guaranteed to
remain within a single Raft group for its lifetime.

This model is inspired by Cloudflare Durable Objects backed by SQLite, but is
adapted to a distributed SQL system:

1. Schema is shared across all actors in a tenant.
2. Cross-actor transactions are allowed.
3. Cross-actor transactions use Ratel's normal distributed transaction and
   two-phase commit machinery rather than a special actor-only protocol.

The term **actor** is intentional. The storage model in this RFC is designed to
match a future execution model in which user code can run in a Durable Object-
like style against one actor's isolated state. The name should reflect that
broader abstraction. "Partition" is misleading because CockroachDB already uses
it for table and index partitioning, and "database" suggests a transparently
scalable storage abstraction rather than a bounded, single-shard unit.

The high-level design is:

1. Keep schema and shared system metadata in the existing tenant keyspace.
2. Add an actor-scoped prefix only for user table and index data.
3. Expose two complementary SQL interfaces for selecting actors: a session
   variable for single-actor workloads and a table-qualifying syntax for
   explicit cross-actor references.
4. Carry actor identity through the optimizer and DistSQL specs on a per-scan
   basis.
5. Enforce a hard invariant that one actor maps to exactly one Raft group.
6. Reject operations that would require splitting an actor across multiple Raft
   groups.


# Motivation

Ratel needs a data model that can cheaply isolate data per logical entity
without requiring separate tenants, separate SQL schemas, or descriptor-scale
metadata growth. The target workload is a very large number of isolated data
sets, each typically a few GB or less, all sharing application schema.

The desired user model is much closer to "many tiny actors with one shared
schema definition" than to Cockroach's existing table partitioning features.
Cloudflare's Durable Objects provide a useful analogy:

- each logical entity has isolated state
- creation is cheap and implicit
- the common case is single-entity access
- the system should keep one entity's data physically together

Ratel also wants to preserve database capabilities that DO-style systems often
avoid:

- shared SQL schema across entities
- ordinary SQL over that shared schema
- cross-actor transactions when needed

At the same time, the product contract here is stronger than "usually local":
an actor is intended to be a bounded single-shard abstraction. Silent promotion
from "fast single actor" to "multi-range distributed actor" would create a
severe and unpredictable performance cliff. This RFC therefore treats
single-Raft-group placement as a hard invariant, not a best-effort heuristic.


# Technical design

## Terminology

This RFC uses the following terms:

- **Tenant**: the existing top-level SQL and KV isolation boundary.
- **Actor**: a lightweight isolated table-data scope within a tenant.
- **Unscoped mode**: access to the tenant's legacy unscoped data.
- **Actor-scoped mode**: access to exactly one selected actor.

An actor is not an independent PostgreSQL-style database or schema namespace.
It is an isolated per-entity data scope under a shared tenant schema, with an
execution model that is expected to grow into a DO-style actor runtime.

## Relationship to tenants

Actors exist **within** a tenant. They are not an alternative to tenants and do
not introduce a new top-level isolation boundary. This feature should be thought
of as a tenant-internal data model, much like most other SQL and KV features in
the system.

The distinction is important:

- a **tenant** is a heavyweight top-level isolation boundary with its own
  schema, namespace, security context, and system-level metadata
- an **actor** is a lightweight isolated data scope inside one tenant

Actors are expected to be extremely numerous. The intended scale is
millions to billions of actors per tenant. That is precisely why actors cannot
look like miniature tenants.

In particular, actors do **not** have:

- their own schema
- their own descriptor tree
- their own namespace
- their own tenant boundary

Instead, a tenant owns one shared schema, and many actors reuse that schema
while isolating only their table data.

## Goals

- Support tens of millions of actors per tenant.
- Isolate user table and index data by actor while preserving shared schema.
- Allow implicit actor creation on first write.
- Preserve backward compatibility for unscoped access.
- Support session-scoped actor selection for single-actor workloads.
- Support explicit per-table actor selection for cross-actor queries.
- Support cross-actor transactions.
- Use the normal distributed transaction and 2PC machinery for cross-actor
  transactions.
- Guarantee that each actor remains within a single Raft group.
- Use a deterministic fixed-width hash-derived key prefix.

## Non-goals

- Independent schema per actor.
- Independent namespace or descriptor trees per actor.
- Treating actors as separate tenants.
- Transparent automatic scaling of one actor across multiple Raft groups.
- Full admin, backup, and restore support in the first implementation.
- Introducing a full DO-style code execution runtime in the same change. This
  RFC only establishes the storage and transactional substrate that such a
  runtime can build on.

## Data model

Within a tenant, schema is shared across all actors and across unscoped data.

Shared across the tenant:

- descriptors
- namespace metadata
- migrations
- zone/span configuration metadata
- schema changer state
- other shared system tables

Isolated per actor:

- user table rows
- user secondary index entries
- MVCC history for those keys
- scan, lookup, insert, update, and delete access to table data

This is intentionally not "many schemas". It is "one schema, many isolated
actors".

## SQL model

There are two complementary ways to select an actor. They are mutually
exclusive: using both in the same query is an error.

### Session variable: `actor_scope`

Set a session-level default actor for all table references in subsequent
statements:

```sql
SET actor_scope = 'user_123';
SELECT * FROM kv WHERE id = 1;
INSERT INTO kv VALUES (2, 'hello');
SET actor_scope = '';
```

The empty string means unscoped mode (the default). This is the primary
interface for single-actor workloads, analogous to Cloudflare's Durable Object
stub binding.

### Table qualifier: `actor('name').table`

Qualify individual table references with an explicit actor:

```sql
SELECT * FROM actor('user_123').kv WHERE id = 1;
INSERT INTO actor('user_123').kv VALUES (1, 'hello');
```

This syntax works in all table reference positions: SELECT, INSERT, UPDATE,
DELETE, and JOIN. It enables cross-actor queries in a single statement:

```sql
-- Cross-actor JOIN
SELECT a.value, b.value
FROM actor('alice').kv a
JOIN actor('bob').kv b
ON a.id = b.id;

-- Cross-actor transaction
BEGIN;
UPDATE actor('alice').kv SET value = 'a' WHERE id = 1;
UPDATE actor('bob').kv SET value = 'b' WHERE id = 1;
COMMIT;
```

### Mutual exclusion

The two interfaces must not be used together. If `actor_scope` is set to a
non-empty value, any `actor('name').table` reference in a query is an error:

```text
cannot use actor() table qualifier when actor_scope is set;
run SET actor_scope = '' first
```

This avoids ambiguity about which actor applies to unqualified table references
in the same statement.

### Introspection

```sql
actor_id(name STRING) -> BYTES
```

Returns the deterministic 16-byte actor hash for the given name.

### Query examples

Single-actor session:

```sql
SET actor_scope = 'user_123';
SELECT * FROM kv WHERE id = 1;
INSERT INTO kv VALUES (2, 'world');
UPDATE kv SET value = 'updated' WHERE id = 1;
```

Cross-actor read:

```sql
SELECT a.value, b.value
FROM actor('user_123').kv a
JOIN actor('user_456').kv b
ON a.id = b.id;
```

Cross-actor transaction:

```sql
BEGIN;
UPDATE actor('user_123').kv SET value = 'a' WHERE id = 1;
UPDATE actor('user_456').kv SET value = 'b' WHERE id = 1;
COMMIT;
```

Cross-actor transactions are fully supported. They do not require a special
actor-aware commit protocol. They are executed using the normal distributed
transaction path, including the existing two-phase commit machinery when the
transaction spans multiple ranges.

### Rules

- The argument to `actor()` in a table qualifier must be a string constant.
- Different table references in the same statement may name different actors.
- `actor_scope` and `actor().table` are mutually exclusive per query.

## Key encoding

### Unscoped

```text
[TENANT PREFIX][TABLE_ID][INDEX_ID][ROW...][FAMILY]
```

### Actor-scoped

```text
[TENANT PREFIX][ACTOR_SENTINEL][ACTOR_HASH][TABLE_ID][INDEX_ID][ROW...][FAMILY]
```

Constants:

- `ACTOR_SENTINEL`: 1 byte
- `ACTOR_HASH`: 16 bytes
- total per-key overhead: 17 bytes

The actor sentinel is a dedicated keyspace marker, not a synthetic or reserved
table ID. Actor-scoped table data therefore lives in an explicit actor envelope
instead of pretending to be ordinary `/Table/<id>` data. For the system tenant,
this creates a first-class `/Actor/...` region in the SQL keyspace; for
secondary tenants, the equivalent layout is `/Tenant/<id>/Actor/...`.

Hash derivation:

```text
ACTOR_HASH = trunc128(SHA-256(actor_name))
```

The visible name is not stored in each key. It is resolved via a registry table.

## Why a fixed-width hash prefix

The fixed-width hash prefix provides:

- uniform distribution
- stable prefix length
- deterministic name-to-prefix mapping
- cheap prefix comparison
- no variable-length actor names in every key

This gives the same broad shape as DO-style `idFromName`, while keeping the
storage prefix compact and predictable.

## Registry

The system must track actor name-to-hash mappings explicitly.

Add a system table:

```text
system.actors(
  tenant_id,
  actor_name,
  actor_hash,
  created_at
)
```

Constraints:

- unique `(tenant_id, actor_name)`
- unique `(tenant_id, actor_hash)`

This registry is required for:

- collision detection
- implicit creation
- observability and admin tooling
- future lifecycle operations

### Implicit creation

On first successful write to an actor:

1. compute the hash from the supplied name
2. look up or insert the registry entry
3. verify name/hash consistency
4. reject collisions between distinct names with the same hash
5. establish sticky range splits at the actor keyspan boundaries
6. proceed with the write

This work should occur transactionally before the statement emits user-table KV.

## Execution model

The existing tenant `SQLCodec` remains the codec for schema and shared metadata.

This is a critical design constraint. Actor scoping must not be implemented as a
wholesale replacement of tenant prefix semantics in `SQLCodec`, because the
same codec currently participates in both row keys and shared metadata keys.

Actor scoping applies only to user table and index data.

That preserves unscoped behavior for:

- descriptor keys
- namespace keys
- migration keys
- sequence and schema metadata
- zone/span configuration metadata

while still isolating user table data.

## Optimizer and planner

Actor identity must be attached explicitly to scans and mutations.

The optimizer should:

1. extract actor identity from `actor().table` qualifiers or the session
   variable
2. annotate scan and mutation operators with that identity
3. reject ambiguous or unsupported forms (e.g. mixing `actor_scope` with
   `actor().table`)
4. preserve distinct identities for different table references within a
   statement

This requires actor fields on scan and mutation operator privates and
corresponding execution parameters. The source-of-truth definitions for these
must live in the optgen inputs and protobuf sources, not in generated files.

## DistSQL

Session propagation is necessary but not sufficient.

Whole-session single-actor execution can derive actor identity from the session
variable on remote nodes. However, plans containing multiple actor-scoped scans
in one statement require per-scan transport of actor identity.

Physical specs such as `TableReaderSpec`, and any other processor spec that
performs direct table/index KV access, must therefore include an actor field.

Remote processors reconstruct row prefixes from:

- the base tenant codec
- the selected actor name
- the deterministic actor hash

## Row-key and span formation

Actor scoping should be introduced specifically into row and index key formation
paths, not by mutating the meaning of `TenantPrefix()`.

Any path that forms table-data spans or keys must have access to:

- the base tenant codec
- the target table and index ids
- the optional selected actor

This includes:

- scans
- point lookups
- inserts
- updates
- deletes
- delete range over table data
- lookup joins
- index joins
- row fetchers
- fast-path insert
- DistSQL table readers

Any path touching shared metadata remains unscoped.

## Transactions

A transaction may touch:

- only unscoped data
- one actor
- multiple actors

Cross-actor transactions are explicitly supported.

They use the normal Ratel distributed transaction implementation:

- normal transaction record semantics
- normal intent resolution
- normal range coordination
- normal two-phase commit when the transaction spans multiple ranges

This feature does not introduce a special actor-specific commit path. Actors are
an addressing and locality abstraction, not a separate transactional subsystem.

The performance model is intentionally asymmetric:

- single-actor transactions are the common case and should be physically
  favored
- cross-actor transactions are allowed, correct, and sometimes useful, but they
  cross actor boundaries and therefore naturally lose some of the locality
  benefits of single-actor access

## Single-Raft-group invariant

An actor is guaranteed to remain within a single Raft group for its lifetime.

This is a product invariant, not a best-effort optimization. The system must
not silently split one actor into multiple ranges, because that would create a
large and effectively random performance cliff.

The consequence is that actors must be treated as a bounded abstraction with
explicit capacity limits.

### Required enforcement

The system must prevent an actor from being split by:

- size-based splitting
- load-based splitting
- split-at-config-boundary logic
- manual or admin split requests inside the actor span

The system must also prevent merges that would combine two distinct actor spans
into one range.

### Capacity limits

Because actors cannot split, the system must define explicit limits and surface
them clearly. At minimum:

- maximum actor live data size

When an operation would exceed the hard limit, the operation must fail with a
clear user-visible error. It must not trigger automatic repartition of the
actor.

## Span configuration

On first write, the system places sticky range splits at the actor keyspan
boundaries to establish the single-Raft-group invariant.

## DDL semantics

DDL remains tenant-global because schema is shared.

When an actor is active, DDL must be rejected with an error such as:

```text
cannot execute DDL with actor_scope set; run SET actor_scope = '' first
```

This avoids confusing semantics and prevents accidental attempts to write schema
metadata into an actor-scoped data path.

## Actor deletion

Deleting an actor removes its registry entry from `system.actors` and deletes
all actor-scoped KV data via `DeleteRange`. This is a hard delete with no
tombstone or soft-delete semantics.

## Backward compatibility

When no actor is selected:

- the existing unscoped key layout is unchanged
- existing applications continue to work
- legacy unscoped data remains visible

When an actor is selected:

- only that actor's data is visible
- unscoped data is not visible unless the session returns to unscoped mode

This preserves compatibility while allowing incremental adoption.

## Why "actor"

"Actor" is the right term for this feature for two reasons.

First, it accurately communicates the storage contract:

- isolated per-entity state
- strong locality
- bounded single-shard semantics

Second, it matches the intended long-term execution model. Ratel expects to add
DO-style code execution against these isolated units. The storage abstraction
should use the same term that the future execution abstraction will use.

By contrast:

- "partition" is already overloaded in CockroachDB and implies something closer
  to table/index placement partitioning
- "database" suggests a transparently scalable storage abstraction, which is not
  what an actor is if it must stay in one Raft group

## Alternatives

### Separate tenant per actor

Rejected. Too heavy for the target scale and too expensive in metadata and
operations.

### Separate schema per actor

Rejected. This creates too much descriptor and namespace growth and is not a
good fit for tens of millions of isolated entities.

### Reuse table or index partitioning

Rejected. Existing partitioning is schema- and placement-oriented, not an
actor-like isolation model.

### Implement scoping by replacing `SQLCodec`

Rejected. This is too broad. It would risk moving shared metadata writes into
actor-scoped prefixes unless every schema and system-key caller were perfectly
fenced off.

### WHERE-clause actor marker

Rejected. An earlier design used `actor('name')` as a boolean predicate in
WHERE clauses. This had several problems:

- Could not be used for INSERTs (no WHERE clause).
- Using `actor()` inside OR or NOT silently returned wrong results.
- The marker looked like a filter predicate but actually controlled key
  addressing, creating a confusing semantic mismatch.
- Required special extraction logic in the optimizer that was fragile across
  mutation paths.

The `actor('name').table` table-qualifier syntax replaces this approach. It
works uniformly in all table reference positions and avoids the boolean-
predicate footguns.


## Drawbacks

- The actor abstraction has a hard capacity ceiling because it cannot split.
- Cross-actor transactions are fully supported, but naturally cost more than
  single-actor access.
- DistSQL and row-fetch code paths must carry per-scan actor identity, which
  expands the execution surface area.
- The `actor('name').table` syntax requires parser changes to support a function
  call in the schema-qualification position of a table reference.
- The future code-execution meaning of "actor" must stay aligned with the
  storage meaning introduced here.


## Rationale and alternatives

The central design choice in this RFC is to separate **shared schema** from
**isolated actor data**.

That gives the desired user model without multiplying descriptor trees or
introducing separate tenants for each logical entity. It also preserves normal
relational behavior, including cross-actor transactions via the existing
distributed transaction and 2PC machinery, while still giving the system a
strong locality signal for the dominant single-actor case.

The second key design choice is to treat "single actor = single Raft group" as
an invariant. This makes the model honest. Users can reason about actor
performance without wondering whether they have silently crossed a hidden
sharding threshold.

The third key design choice is to provide two complementary SQL interfaces:
`SET actor_scope` for single-actor workloads (the 99% case) and
`actor('name').table` for cross-actor queries. The two are mutually exclusive
to avoid ambiguity. This mirrors how `SET search_path` relates to fully
qualified `schema.table` references in standard SQL.


# Explain it to folk outside of your team

Imagine a tenant that wants one shared schema, but millions of isolated little
stateful objects.

Each object, or actor, has its own data. Most requests interact with just one
actor, so the system keeps that actor's data together in one Raft group. That
gives a predictable locality model that is a good fit for DO-style applications.

You select an actor either by setting a session variable (`SET actor_scope =
'alice'`) or by qualifying table names (`actor('alice').kv`). The first is for
applications that work with one actor at a time. The second is for queries that
need to touch multiple actors in one statement.

Unlike many actor systems, Ratel still allows SQL transactions that touch more
than one actor. Those transactions are not special-cased. They use the same
distributed transaction and two-phase commit machinery the database already
uses.

The term "actor" is chosen because this is not just a storage feature. It is
meant to become the storage foundation for a future Durable Object-like code
execution model.


# Unresolved questions

- How should backup and restore expose actor-scoped spans?
- Do we need explicit overload protection for a single actor that is too hot
  even before it is too large?
