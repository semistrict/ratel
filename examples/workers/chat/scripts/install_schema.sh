#!/usr/bin/env bash
set -euo pipefail

sql_addr="${RATEL_SQL_ADDR:-localhost:26257}"
ratel_bin="${RATEL_BIN:-ratel}"

script_dir="$(cd "$(dirname "$0")" && pwd)"
schema_file="$script_dir/../schema.sql"
if [[ ! -f "$schema_file" && -n "${RUNFILES_DIR:-}" ]]; then
  for root in "$RUNFILES_DIR" "$RUNFILES_DIR/_main"; do
    candidate="$root/examples/workers/chat/schema.sql"
    if [[ -f "$candidate" ]]; then
      schema_file="$candidate"
      break
    fi
  done
fi

if [[ "${RATEL_BIN:-}" == "" && -n "${RUNFILES_DIR:-}" ]]; then
  for root in "$RUNFILES_DIR" "$RUNFILES_DIR/_main"; do
    for candidate in \
      "$root/pkg/cmd/ratel/ratel" \
      "$root/pkg/cmd/ratel/ratel_/ratel" \
      "$root/cockroach/pkg/cmd/ratel/ratel"; do
      if [[ -x "$candidate" ]]; then
        ratel_bin="$candidate"
        break 2
      fi
    done
  done
fi

"$ratel_bin" sql "$sql_addr" -e "$(cat "$schema_file")"
