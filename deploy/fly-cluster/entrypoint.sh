#!/bin/bash
set -e

# If COCKROACH_ARGS is set (by the deploy script), use those.
# Otherwise fall back to default single-node mode for local testing.
if [ -n "$COCKROACH_ARGS" ]; then
    exec /cockroach/cockroach $COCKROACH_ARGS
else
    exec /cockroach/cockroach start-single-node \
        --insecure \
        --store=/cockroach/cockroach-data \
        --listen-addr=0.0.0.0:26257 \
        --http-addr=0.0.0.0:8080
fi
