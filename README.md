# Ratel

> **WARNING: Pre-alpha software. It will lose your data. Do not use for anything you care about.**

A lightweight, distributed SQL database that runs entirely on object storage. Nodes are stateless and ephemeral — all persistent state lives on S3.

A single storage URL is the cluster identity. Certificates, node discovery, and shared storage are all derived from it.

## Quick Start

```bash
# Build
make buildratel

# Initialize a new cluster
ratel init s3://my-bucket/?endpoint=http://localhost:9000&region=us-east-1

# Join additional nodes
ratel join s3://my-bucket/?endpoint=http://localhost:9000&region=us-east-1

# Connect a SQL shell
ratel sql s3://my-bucket/?endpoint=http://localhost:9000&region=us-east-1
```

Local filesystem also works:

```bash
ratel init file:///tmp/my-cluster/
ratel sql  file:///tmp/my-cluster/
```

## How It Works

The storage URL points to an S3 bucket (or local directory) with this layout:

```
s3://my-bucket/                 (or file:///tmp/my-cluster/)
  v1/
    certs/                      CA cert, CA key (optionally encrypted), client certs
    nodes/                      Node registry (JSON files for peer discovery)
    sstables/                   Pebble SSTables (shared storage)
    metadata/                   MANIFEST bundle
```

- **`ratel init <url>`** — Generate CA + client certs, upload to S3, bootstrap the cluster, start serving.
- **`ratel join <url>`** — Download CA from S3, generate a node cert with this host's name, discover peers, join.
- **`ratel sql <url>`** — Download client certs, pick a live node from the registry, connect via PostgreSQL wire protocol.

Each node generates its own TLS certificate signed by the shared CA, with its hostname as a SAN. No shared node certs.

## Security

The storage URL is the trust boundary. Whoever has access to the bucket has full cluster access. The bucket must not be public.

The CA private key can be encrypted with a passphrase:

```bash
# Interactive prompt (with confirmation)
ratel init s3://my-bucket/

# Via environment variable
RATEL_PASSPHRASE=secret ratel join s3://my-bucket/

# Skip encryption entirely
ratel init --no-passphrase s3://my-bucket/
```

## Docker Demo

A 5-node demo with [rustfs](https://rustfs.com) (S3-compatible storage):

```bash
cd demo
docker compose up --build
```

See [demo/README.md](demo/README.md) for details.

## License

Apache License, Version 2.0
