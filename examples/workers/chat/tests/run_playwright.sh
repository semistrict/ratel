#!/usr/bin/env bash
set -euo pipefail

if [[ -n "${PLAYWRIGHT_NODE_PATH:-}" ]]; then
  export NODE_PATH="${PLAYWRIGHT_NODE_PATH}${NODE_PATH:+:${NODE_PATH}}"
fi

script_dir="$(cd "$(dirname "$0")" && pwd)"
node "$script_dir/tests/chat.spec.js"
