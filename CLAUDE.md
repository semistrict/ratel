# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Oxide's long-term maintenance fork of CockroachDB 22.1, licensed under Apache 2.0. Used for control plane data storage on the Oxide Cloud Computer (illumos/Helios). Builds target illumos, Linux, and macOS. Enterprise (CCL) features have been removed. The Bazel build system has been removed; only GNU Make is used.

Module path: `github.com/cockroachdb/cockroach`

Main branch for PRs: `release-22.1-oxide`

## Build Commands

**CRITICAL: Always build and test with vendor mode.** Direct `go build` / `go test` without `-mod=vendor` will use the module cache instead of the vendored (and patched) dependencies, causing C compilation errors and missing patches.

```bash
# ALWAYS use the Makefile or pass -mod=vendor:
make buildshort              # Fast build without admin UI (pkg/cmd/cockroach-short)
make buildoss                # Full build with admin UI
make build BUILDTARGET=./pkg/cmd/some-tool  # Build a specific binary

# If running go directly, ALWAYS use -mod=vendor:
go test -mod=vendor -run TestFoo ./pkg/some/package/
go build -mod=vendor ./pkg/some/package/

# Set up vendor directory (required after go.mod changes):
make vendor/modules.txt      # Runs go mod vendor + applies patches from patches/
```

Dependencies are vendored (`-mod=vendor`). Patches in `patches/` are applied to the vendor directory by `make vendor/modules.txt`. C dependencies (jemalloc, GEOS, PROJ) are in `c-deps/` and built automatically.

On illumos, `stdmalloc` build tag is used automatically instead of jemalloc.

## Testing

Tests require `PKG` when specifying `TESTS` or `BENCHES`. **Always use the Makefile or `-mod=vendor`:**

```bash
make test PKG=./pkg/sql                           # All tests in a package
make test PKG=./pkg/sql/parser TESTS=TestParse     # Single test
make testshort                                     # All tests with -short flag
make testrace PKG=./pkg/kv TESTS=TestFoo           # With race detector
make bench PKG=./pkg/sql/parser BENCHES=BenchmarkParse

# SQL logic tests
make testlogic                                     # All logic tests (base + opt)
make testbaselogic FILES='prepare|fk'              # Filter by file name
make testlogic FILES=fk SUBTESTS='20042|20045'     # Filter by subtest
make testlogic TESTCONFIG=local                    # Specific cluster config

# Stress testing
make stress PKG=./pkg/sql TESTS=TestFoo
make stressrace PKG=./pkg/sql TESTS=TestFoo

# Fuzz testing
make fuzz PKG=./pkg/sql/sem/tree TESTS=Decimal TESTTIMEOUT=1m
```

Default timeouts: tests 60m, race 45m, bench 5m, lint 30m.

Extra flags: `TESTFLAGS="-v --vmodule=raft=1"`

## Code Generation and Linting

```bash
make generate     # Regenerate all generated code (protobuf, execgen, optgen, parser, etc.)
make lint         # Full linting (slow, uses lint build tag)
make lintshort    # Faster linting subset
```

Protobuf `.pb.go` files are generated and committed. Proto definitions are spread across packages (roachpb, kvserver, storage, etc.).

Custom code generators: `execgen` (vectorized execution), `optgen` (SQL optimizer), `langgen` (parser).

## Architecture

```
CLI (pkg/cli/)  -->  Server (pkg/server/)  -->  SQL (pkg/sql/)  -->  KV (pkg/kv/)  -->  KVServer (pkg/kv/kvserver/)  -->  Storage (pkg/storage/)  -->  Pebble
```

### Layer Breakdown

- **pkg/cli/**: Cobra-based CLI. Entry point is `cli.Main()` called from `pkg/cmd/cockroach*/main.go`.
- **pkg/server/**: HTTP/gRPC server, node lifecycle, admin API, status monitoring.
- **pkg/sql/**: SQL engine. Parser, cost-based optimizer (`pkg/sql/opt/`), columnar/vectorized execution (`pkg/sql/colexec/`), distributed execution (`pkg/sql/distsql/`). Schema management via descriptors in `pkg/sql/catalog/`.
- **pkg/kv/**: KV client API. `pkg/kv/kvclient/kvcoord/` handles transaction coordination and range routing via DistSender.
- **pkg/kv/kvserver/**: Raft consensus, replica state machine (`replica_*.go`), range allocation, concurrency control (lock tables), lease management.
- **pkg/storage/**: MVCC layer over Pebble (LSM-based storage engine). Engine abstraction in this package.
- **pkg/roachpb/**: Core protobuf definitions (requests, responses, transactions, values, errors, metadata). Used across all layers for RPC serialization.

### Cross-Cutting Systems

- **pkg/gossip/**: Cluster-wide peer communication protocol.
- **pkg/jobs/**: Background job execution (schema changes, backups, imports).
- **pkg/settings/**: Cluster-wide runtime settings.
- **pkg/ts/**: Time series database for internal metrics.
- **pkg/rpc/**: gRPC transport between nodes.
- **pkg/util/**: Shared utilities (logging, tracing, encoding, etc.).

### Data Flow

SQL queries parse into AST -> optimize via cost-based optimizer -> execute as KV batch operations -> DistSender routes to range leaseholders -> KVServer processes via Raft consensus -> MVCC writes to Pebble.

## Key Conventions

- All Go dependencies are vendored in `vendor/`. Tools install to local `bin/` directory.
- Build variables can be persisted in `customenv.mk` at the repo root.
- `build/defs.mk` caches computed values; deleted automatically when stale.
- Git submodules are used and initialized automatically on first build.
