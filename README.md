<p align="center">
  <img src="logo.svg" width="160" alt="Ratel logo">
</p>

# Ratel

A lightweight SQL database that runs entirely on object storage.

Ratel is a single-node SQL database — think SQLite but backed by S3 instead of a local file. Local disk is used only as a temporary working area for WAL and memtables. All persistent state (SSTables, manifest) lives on object storage. Shut down the process, wipe the local directory, restart pointing at the same bucket — all your data is there.

## How it works

Ratel is built on [Pebble](https://github.com/cockroachdb/pebble) (an LSM storage engine) with its shared storage support configured to write all SSTables to remote object storage. A MANIFEST bundle (zip of Pebble metadata files) is periodically checkpointed to object storage so the database can be reopened from scratch on any machine.

**Lifecycle:**
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

- Full SQL (PostgreSQL wire protocol) — inherited from the CockroachDB 22.1 SQL engine
- SSTables on object storage via Pebble's shared storage (`CreateOnSharedAll`)
- MANIFEST bundle upload/download for crash recovery
- Periodic background checkpointing
- `file://` backend for local testing without S3
- `--store remote-storage=<url>` flag to configure object storage

## Heritage

Ratel is a fork of [CockroachDB 22.1](https://github.com/cockroachdb/cockroach) (Apache 2.0), stripped down for single-node object-storage use. The multi-node clustering, Raft consensus, and distributed SQL machinery are still present in the code but unused — the goal is to progressively simplify toward a lean single-node SQL engine on object storage.

## License

Apache License, Version 2.0
