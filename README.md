# Ratel

> **WARNING: Pre-alpha software. It will lose your data. Do not use for anything you care about.**

A lightweight, distributed SQL database that runs entirely on object storage. Nodes are stateless and ephemeral — all persistent state lives on S3.

A single storage URL is the cluster identity. Certificates, node discovery, and shared storage are all derived from it.

## Quick Start

```bash
# Build
make buildratel

# Initialize a new cluster (generates certs, bootstraps, starts first node)
ratel init file:///tmp/my-cluster/

# Join additional nodes
ratel join file:///tmp/my-cluster/ --listen-addr=localhost:26258 --http-addr=localhost:8081

# Connect a SQL shell
ratel sql file:///tmp/my-cluster/
```

## How It Works

The storage URL points to a directory (local filesystem or S3 bucket) with this layout:

```
file:///tmp/my-cluster/     (or s3://my-bucket/prefix/)
  certs/                    TLS certificates (generated on init)
  nodes/                    Node registry (JSON files for peer discovery)
  sstables/                 Pebble SSTables (shared storage)
  metadata/                 MANIFEST bundle
```

- **`ratel init <url>`** — Generate TLS certs, bootstrap the cluster, register node 1, start serving.
- **`ratel join <url>`** — Download certs, discover peers from `nodes/`, join the cluster, register this node.
- **`ratel sql <url>`** — Download client certs, pick a node from `nodes/`, connect via PostgreSQL wire protocol.

No `--store`, `--join`, or `--init` flags. The URL is the only required argument.

## Security

The storage URL is the trust boundary. Whoever has access to the bucket has full cluster access — certs are stored alongside data. The bucket must not be public.

## License

Apache License, Version 2.0
