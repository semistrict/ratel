#!/usr/bin/env bash
set -euo pipefail

sql_addr="${RATEL_SQL_ADDR:-localhost:26257}"
http_addr="${RATEL_HTTP_ADDR:-localhost:5273}"
ratel_bin="${RATEL_BIN:-ratel}"

script_dir="$(cd "$(dirname "$0")" && pwd)"
repo_root="$(cd "$script_dir/../../../.." && pwd)"
worker_js="${RATEL_CHAT_WORKER_JS:-$repo_root/_bazel/bin/examples/workers/chat/chat_worker.js}"

"$script_dir/install_schema.sh"
"$ratel_bin" deploy --config "$repo_root/examples/workers/chat/worker.jsonc" "$http_addr" "$worker_js"

cat <<EOF
Chat worker deployed.
Open: http://$http_addr/workers/chat/
SQL:  $sql_addr
EOF
