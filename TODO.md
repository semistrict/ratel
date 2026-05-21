# Outstanding Ramon-Origin Work

This branch has already ported the core CF actors work, explicit actor syntax, subordinate array encoding, S3-backed cluster storage, and the Bazel 9.1.0 migration. The items below were inspected against `origin/main` and still have code missing from this branch.

## Full Column-Family Removal

Status: ported in this branch.

The core single physical row-group path is now ported through table descriptor allocation, system descriptor collapsing, row/columnar fetch, row writes, catalog decoding, zone/comment decoding, declarative schema-change backfills for stored/default `ADD COLUMN`, stale SQL test cleanup, bootstrap descriptor expectation updates, KV count/key-size expectation updates, and upgrade-test cleanup.

Refs:
- `c1c5836` Remove column family descriptor/runtime APIs
- `2e2822e` Remove remaining column family syntax
- `6ee32f5` Remove stale test shims and old compatibility logic

Remaining:
- Continue the mechanical API cleanup for leftover `Family`-named descriptor surfaces that are now row-group compatibility shims.

## Workers / Workerd / Durable Objects

Status: ported in this branch.

The worker platform is now ported through the sidecar/router/proxy/config code, worker API endpoints, actor storage Cap'n Proto server, worker script catalog plumbing, embedded workerd packaging placeholder, Durable Object KV key helpers, and focused Bazel tests for deploy/list validation plus DO storage behavior that can run without a full external workerd binary.

Refs:
- `0d70f6f` RFC: workers sidecar
- `749e8ef` Deploy JS workers with DO storage
- `eb095b8` Update workerd submodule
- `fb9ab31` Embed workerd binary
- `f31ebc1` Multi-node DO routing
- `fa26fd8` Remove gossip, KV-backed descriptor stores
- `38d0f8c` Remove WASM UDF, JS-only
- `930e965` Fix CI/workerd submodule SSH URL

Remaining:
- Add the `c-deps/workerd` submodule and real `pkg/server/workerd_bin/workerd.zst` artifact if this branch should ship an embedded workerd binary instead of relying on `PATH` / `RATEL_WORKERD_BIN`.
- The broad `//pkg/server:server_test` suite still has unrelated bootstrap/range-accounting failures after the system-table changes; the focused worker tests pass.

## JavaScript UDF Runtime

Status: missing.

This branch has older SQL UDF support, but not the JS/V8 runtime work from `origin/main`.

Refs:
- `4abab6a` Add JS/WASM UDFs
- `b8d1bb5` Cache TxnContext
- `38d0f8c` Remove WASM UDF, JS-only

Expected paths/features:
- `pkg/sql/udfruntime/*`
- `pkg/sql/udf_resolver.go`
- JS UDF integration in function creation/execution
- `pkg/sql/colexec/udf_bench_test.go`
- JS/plv8 UDF logictests
- inproc distributed UDF tests and benches

## Inproc Synctest / Jepsen Coverage

Status: mostly ported.

The base `pkg/testutils/inproc` package, topology helpers, liveness tests, registry tests, chaos smoke tests, and Jepsen bank/comments/register/sequential/sets coverage are ported. The default Bazel target runs the synctest cluster cases one per subprocess because this branch still has background gossip/rangefeed loops that can outlive one synctest bubble and poison the next in the same Go process.

Refs:
- `2e5dce0` Add in-process cluster testing support
- `21da7d7` Add synctest workflow
- `f3b4a84` Add Jepsen-style inproc tests
- `d3d66ea` Add partition nemesis coverage
- `ae2fa73` Add network topology support
- `5b15db4` Add liveness poller tests
- `24e72dd` Add chaos tests
- `884990f` Add sequential/register/set/bank coverage
- `d66adca` Add inproc UDF coverage
- `392db7c` Add inproc benchmark coverage
- `76a0d85` Add synctest helpers
- `bdcfc62` Refine inproc registry tests
- `d3dc5f6` Add/adjust synctest Jepsen workflow
- `874909a` Refine inproc raft/lease behavior
- `592e640` Fix inproc workflow/test wiring
- `6176565` Add CI wiring for inproc tests
- `809bbbf` Refine synctest flakes/timeouts
- `d0f2f63` Add missing inproc test coverage
- `7df0877` Fix inproc/synctest race or timing issue
- `e889015` Refine Jepsen test behavior
- `1a761af` Final inproc/synctest cleanup

Expected missing paths/features:
- distributed UDF tests and UDF benches
- full `TestSyncRatelChaos` stress run in the default target; narrower chaos cases are enabled, but the full churn stress is too slow without further server-background-loop cleanup

## SQL Query UI Page

Status: missing.

The pnpm/build migration is mostly covered here, but the DB Console SQL Query page from `origin/main` is absent.

Refs:
- `c3feaf3` Migrate UI build to pnpm, add SQL Query page
- `e638d34` UI dependency/build follow-up
- `6a3060b` UI dependency/build follow-up
- `2d22211` UI dependency/build follow-up
- `969aa31` UI dependency/build follow-up
- `2c29151` UI dependency/build follow-up

Expected paths/features:
- `pkg/ui/workspaces/db-console/src/views/sqlQuery/sqlQueryPage.tsx`
- Any associated routing, exports, tests, and generated UI dependency updates.

## Deploy / Demo Packaging

Status: partially ported.

Runtime S3 storage, node registry, cluster cert upload/download, and Ratel storage URL paths are present. The deployment/demo packaging from `origin/main` is still missing.

Refs:
- `afc42d5` Add S3-backed cluster storage
- `90fb6fc` Add cluster cert storage support
- `3a3ad5e` Add node registry support
- `57218ba` Add Ratel cluster storage wiring
- `c6a0e67` Add deploy/demo support
- `be4255c` Add fly cluster deployment support
- `96b5149` Add Docker/RustFS demo support
- `3fc8e67` Refine storage CLI behavior
- `ff0ab20` Add deploy documentation
- `dcea9cc` Refine cluster storage behavior
- `a8352b0` Refine ratel CLI flags
- `b7d312b` Add storage/deploy follow-up
- `5890d5d` Add deploy/demo follow-up
- `e8979af` Add Ratel storage follow-up
- `829fc4a` Add S3/deploy follow-up
- `777f3a0` Add storage docs/fixes
- `01f9d4f` Add deploy cleanup
- `54cf02b` Add fly deploy cleanup
- `ac390ce` Add storage/CLI cleanup
- `22775ce` Add deployment cleanup
- `a2332eb` Add deploy/demo cleanup
- `8530a47` Add deploy README updates
- `23db2c2` Add final deploy/storage cleanup

Expected missing paths/features:
- `demo/`
- `deploy/fly-cluster/`
- related README/deploy docs and assets

## Tracing Dependency Cleanup

Status: missing.

Jaeger/Zipkin dependencies and code references are still present in this branch.

Refs:
- `f9053da` Remove Jaeger/Zipkin tracing dependencies

Expected affected areas:
- `DEPS.bzl`
- tracing/export code and tests
- SQL/CLI references to Jaeger output where removed upstream

## Lower Priority / Mostly Non-Functional Follow-Ups

Status: not yet triaged individually.

These are lower-risk outstanding commits from `origin/main` that appear to be docs, release notes, metadata, dependency/security bumps, merge commits, formatting, or CI cleanup. Some are likely superseded by the Bazel/Go migration already on this branch.

Refs to audit before porting:
- README / release note / branding updates from `origin/main`
- CI cleanup around duplicate runs, runner versions, regex anchoring, and workflow naming
- dependency/security bumps not already covered by the latest Bazel/Go update
- merge/gofmt-only commits
