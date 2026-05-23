#!/usr/bin/env bash
set -euo pipefail

if [[ -n "${PLAYWRIGHT_NODE_PATH:-}" ]]; then
  export NODE_PATH="${PLAYWRIGHT_NODE_PATH}${NODE_PATH:+:${NODE_PATH}}"
fi

if [[ -n "${RUNFILES_DIR:-}" ]]; then
  node_modules="$(mktemp -d)"
for repo_root in "$RUNFILES_DIR" "$RUNFILES_DIR/_main"; do
    playwright_repo="$repo_root/npm_playwright"
    playwright_core_repo="$repo_root/npm_playwright_core"
    if [[ ! -f "$playwright_repo/package.tgz" ]]; then
      playwright_repo="$repo_root/aspect_rules_js++npm+npm_playwright"
    fi
    if [[ ! -f "$playwright_core_repo/package.tgz" ]]; then
      playwright_core_repo="$repo_root/aspect_rules_js++npm+npm_playwright_core"
    fi
    playwright_dir="$playwright_repo/package"
    playwright_core_dir="$playwright_core_repo/package"
    if [[ ! -d "$playwright_dir" && -f "$playwright_repo/package.tgz" ]]; then
      mkdir -p "$playwright_dir"
      tar -xzf "$playwright_repo/package.tgz" -C "$(dirname "$playwright_dir")"
    fi
    if [[ ! -d "$playwright_core_dir" && -f "$playwright_core_repo/package.tgz" ]]; then
      mkdir -p "$playwright_core_dir"
      tar -xzf "$playwright_core_repo/package.tgz" -C "$(dirname "$playwright_core_dir")"
    fi
    if [[ -d "$playwright_dir" && -d "$playwright_core_dir" ]]; then
      mkdir -p "$node_modules"
      ln -s "$playwright_dir" "$node_modules/playwright"
      ln -s "$playwright_core_dir" "$node_modules/playwright-core"
      export NODE_PATH="$node_modules${NODE_PATH:+:${NODE_PATH}}"
      break
    fi
  done
fi

script_dir="$(cd "$(dirname "$0")" && pwd)"
if [[ -f "$script_dir/chat.spec.js" ]]; then
  node "$script_dir/chat.spec.js"
else
  node "$script_dir/tests/chat.spec.js"
fi
