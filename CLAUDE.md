# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Licensing — OPEN SOURCE ONLY

ONLY use open source software, code, and dependencies. Never pull in, depend on, or suggest proprietary, source-available, or non-OSI-approved licensed code.

Before adding any new dependency, tool, library, fork, vendored code, or submodule, verify the license is OSI-approved (Apache-2.0, MIT, BSD, MPL-2.0, GPL, LGPL, etc.). If you're unsure about a license, ASK before adding it.

This rule takes precedence over convenience. If the only way to build/test something requires a proprietary or non-OSI component, STOP and explain the blocker — do not proceed.

## Building

Use the reproducible OSS-only Docker builder. It wraps `./dev` (Bazel) inside `ubuntu:22.04` with only open-source tools (bazelisk, gcc, cmake, etc.):

```
build/docker/build-in-docker.sh build short
build/docker/build-in-docker.sh test //pkg/util/log:log_test
```

The image spec lives at `build/docker/Dockerfile`. Do not use the legacy `build/builder/` image from upstream — it's amd64-only and requires emulation on Apple Silicon.

Local Bazel builds on macOS 15+ (Sequoia/Tahoe) fail because the `wrapped_clang` toolchain binary shipped with the CRDB-vendored Bazel 6.2.1 lacks `LC_UUID`, which modern dyld rejects. Always build inside the Docker image instead.
