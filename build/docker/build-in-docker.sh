#!/usr/bin/env bash
# Build CockroachDB inside a reproducible OSS-only Linux container.
#
# Usage:
#   build/docker/build-in-docker.sh [dev-args...]
#
# Examples:
#   build/docker/build-in-docker.sh build short
#   build/docker/build-in-docker.sh test //pkg/util/log:log_test

set -euo pipefail

IMAGE=crdb-builder:local
REPO_ROOT=$(cd "$(dirname "$0")/../.." && pwd)

# (Re)build the image if missing.
if ! docker image inspect "$IMAGE" >/dev/null 2>&1; then
  echo "Building $IMAGE..."
  docker build -t "$IMAGE" "$REPO_ROOT/build/docker"
fi

# Host arch picks the matching platform; defaults to native.
case "$(uname -m)" in
  arm64|aarch64) PLATFORM=linux/arm64 ;;
  x86_64|amd64)  PLATFORM=linux/amd64 ;;
  *) echo "unsupported host arch: $(uname -m)" >&2; exit 1 ;;
esac

exec docker run --rm -it \
  --platform "$PLATFORM" \
  -v "$REPO_ROOT:/cockroach" \
  -w /cockroach \
  "$IMAGE" \
  ./dev "$@"
