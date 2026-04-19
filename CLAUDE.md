# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Licensing — OPEN SOURCE ONLY

ONLY use open source software, code, and dependencies. Never pull in, depend on, or suggest proprietary, source-available, or non-OSI-approved licensed code.

Before adding any new dependency, tool, library, fork, vendored code, or submodule, verify the license is OSI-approved (Apache-2.0, MIT, BSD, MPL-2.0, GPL, LGPL, etc.). If you're unsure about a license, ASK before adding it.

This rule takes precedence over convenience. If the only way to build/test something requires a proprietary or non-OSI component, STOP and explain the blocker — do not proceed.

### License headers

**New files** in this repo must use the Apache-2.0 header attributed to **The Ratel Authors** (template below). Do not carry forward the upstream BSL/CRL header on new files.

**Existing files**: leave the header as-is. Do not rewrite BSL headers just because you're editing the file — that creates noisy churn. A global relicense pass will handle existing files separately.

Template for new files:

```go
// Copyright <year> The Ratel Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.
```

Use the appropriate comment syntax for the language (`//` for Go/C++/TS/JS, `#` for Python/shell/YAML, etc.) but keep the wording identical.

## Building

Use the reproducible OSS-only Docker builder. It wraps `./dev` (Bazel) inside `ubuntu:22.04` with only open-source tools (bazelisk, gcc, cmake, etc.):

```
build/docker/build-in-docker.sh build short
build/docker/build-in-docker.sh test //pkg/util/log:log_test
```

The image spec lives at `build/docker/Dockerfile`. Do not use the legacy `build/builder/` image from upstream — it's amd64-only and requires emulation on Apple Silicon.

Local Bazel builds on macOS 15+ (Sequoia/Tahoe) fail because the `wrapped_clang` toolchain binary shipped with the CRDB-vendored Bazel 6.2.1 lacks `LC_UUID`, which modern dyld rejects. Always build inside the Docker image instead.
