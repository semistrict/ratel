# Ratel

A lightweight, distributed SQL database that runs entirely on object storage.

Ratel is a cloud-native SQL database where all persistent state — SSTables, manifests, metadata — lives on object storage (S3 or compatible). Local disk is used only as a temporary working area for WAL and memtables. Nodes are stateless and ephemeral: shut one down, wipe its local directory, restart it pointing at the same bucket — all your data is there.

## How it works

Ratel is built on [Pebble](https://github.com/cockroachdb/pebble) (an LSM storage engine) with its shared storage support configured to write all SSTables to remote object storage. A MANIFEST bundle (zip of Pebble metadata files) is periodically checkpointed to object storage so a node can be reopened from scratch on any machine.

**Node lifecycle:**
- **Start**: download MANIFEST bundle from object storage (if exists) → open Pebble → resume
- **Run**: WAL and memtables on local disk, SSTables flushed directly to object storage
- **Checkpoint**: every 5 minutes, flush and upload MANIFEST bundle (tunable)
- **Shutdown**: final flush → upload MANIFEST bundle → local disk can be discarded

## Quick start

```bash
ratel start-single-node --insecure \
  --store="path=/tmp/ratel-local,remote-storage=file:///tmp/ratel-data" \
  --listen-addr=localhost:26257

# In another terminal
ratel sql --insecure -e "CREATE TABLE hello (id INT PRIMARY KEY, msg STRING)"
ratel sql --insecure -e "INSERT INTO hello VALUES (1, 'world')"
ratel sql --insecure -e "SELECT * FROM hello"
```

The `file://` backend stores data on the local filesystem (useful for development and testing). For production, use `s3://bucket/prefix`.

## Current status

Early development. What works today:

- Full SQL (PostgreSQL wire protocol)
- SSTables on object storage via Pebble's shared storage (`CreateOnSharedAll`)
- MANIFEST bundle upload/download for crash recovery
- Periodic background checkpointing
- `file://` backend for local testing without S3
- `--store remote-storage=<url>` flag to configure object storage

What's coming:

- Multi-node distributed clusters with shared object storage
- Stateless node scaling — add/remove nodes without data migration
- Binary rename from `cockroachshort` to `ratel`

## Heritage

Ratel is a fork of [CockroachDB 22.1](https://github.com/cockroachdb/cockroach) (Apache 2.0). It inherits CockroachDB's distributed SQL engine, Raft consensus, and range-based sharding — and extends it with object-storage-native persistence so that nodes carry no irreplaceable local state.

## License

Apache License, Version 2.0
