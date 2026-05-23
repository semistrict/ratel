# Ratel Workers Chat Example

This is a Ratel-compatible port of Cloudflare's `workers-chat-demo`. It keeps
the same core model: one Durable Object instance per chat room, WebSocket
clients connected to that room, and recent chat history stored in Durable Object
storage.

The source is split into a Worker entry point, Durable Object class, and an
`assets/` directory containing the browser script, HTML, and CSS. Bazel invokes
esbuild from the repo's pnpm-managed npm dependencies to produce the Worker
module, while `worker.jsonc` tells `ratel deploy` to bind the asset directory as
`env.ASSETS`.

Build the deployable worker script:

```sh
bazel build //examples/workers/chat:chat_worker
```

Deploy it to a running local Ratel node:

```sh
examples/workers/chat/scripts/start.sh
```

Open:

```text
http://localhost:5273/workers/chat/
```

Run the Playwright smoke test against a running local deployment:

```sh
RATEL_CHAT_URL=http://localhost:5273/workers/chat/ PLAYWRIGHT_NODE_PATH=/path/to/node_modules \
  bazel test //examples/workers/chat:playwright_test
```

The test covers the page shell, static assets, and WebSocket readiness.
