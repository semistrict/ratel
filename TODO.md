# Outstanding Ramon-Origin Work

This branch has already ported the core CF actors work, explicit actor syntax, subordinate array encoding, S3-backed cluster storage, Ratel CLI/TLS hardening, Pebble v1.1.5 storage fixes, tracing cleanup, and the Bazel 9.1.0 migration. The items below were inspected against Ramon-authored work in `origin/main` and `../ratel-cockroach`.

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
- full `TestSyncRatelChaos` stress run in the default target; narrower chaos cases are enabled, but the full churn stress is too slow without further server-background-loop cleanup

## Subordinate JSON Storage / Scan Pushdown

Status: missing.

The subordinate array storage work was ported, but the later JSON-specific stack is absent. Reference-only source files such as `pkg/sql/row/json_access_program.go`, `pkg/sql/json_scan_pushdown.go`, `pkg/sql/row/subordinate_json_row_head_fetcher.go`, `pkg/sql/row/subordinate_json_mutation.go`, `pkg/sql/subordinate_json_update.go`, `pkg/util/json/lazy_array.go`, and `pkg/testutils/inproc/subordinate_json_access_test.go` are not present in this branch.

Refs:
- `e38b0f2` Add recursive subordinate JSON scan programs
- `efdbeb9` Add cfetcher inline JSON regression tests
- `fae1956` Broaden subordinate JSON logic coverage
- `48bb05f` Test subordinate JSON rewrite behavior
- `f07f04f` Implement recursive subordinate JSON scan pushdown
- `2f121e1` Optimize recursive JSONB scan locality
- `d61d1d2` Implement Redshift-style JSON dotted access
- `aae1a66` Stream large JSON aggregate paths lazily

Expected missing paths/features:
- recursive JSON subordinate key encoding and access programs
- row/columnar fetcher pushdown for JSON paths and containment
- JSON update/mutation support over subordinate entries
- Redshift-style dotted JSON access parser/type-checking changes
- lazy JSON array/object materialization for large aggregate paths
- logic, row, util/json, and inproc regression coverage

## Tracing Dependency Cleanup

Status: ported in this branch.

Jaeger/Zipkin/OpenTracing exporter dependencies have been removed from dependency
metadata. The local Jaeger JSON trace export used by explain/debug bundles remains
intentionally present.

Refs:
- `f9053da` Remove Jaeger/Zipkin tracing dependencies

Ported areas:
- `DEPS.bzl`
- `go.sum`
- `build/bazelutil/distdir_files.bzl`

## Lower Priority / Mostly Non-Functional Follow-Ups

Status: not yet triaged individually.

These are lower-risk outstanding commits from `origin/main` that appear to be docs, release notes, metadata, dependency/security bumps, merge commits, formatting, or CI cleanup. Some are likely superseded by the Bazel/Go migration already on this branch.

Refs to audit before porting:
- README / release note / branding updates from `origin/main`
- CI cleanup around duplicate runs, runner versions, regex anchoring, and workflow naming
- dependency/security bumps not already covered by the latest Bazel/Go update
- merge/gofmt-only commits
