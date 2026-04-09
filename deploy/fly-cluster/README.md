# Globally Distributed Ratel Cluster on Fly.io

Deploy a multi-region Ratel (CockroachDB) cluster on Fly.io Machines
with Tigris object storage for backups.

## Prerequisites

- [flyctl](https://fly.io/docs/hands-on/install-flyctl/) installed and authenticated
- `FLY_API_TOKEN` set: `export FLY_API_TOKEN=$(fly tokens deploy)`
- Docker image pushed to a registry (or use the pre-built one)

## Quick Start

```bash
# Create a 3-region cluster (US East, London, Singapore)
go run ./deploy/fly-cluster

# Check status
go run ./deploy/fly-cluster -status

# Connect via SQL
go run ./deploy/fly-cluster -sql

# Tear down
go run ./deploy/fly-cluster -destroy
```

## Custom Regions

```bash
# 5-region cluster
go run ./deploy/fly-cluster \
  -regions iad,lhr,sin,nrt,gru \
  -cpus 4 \
  -memory 8192 \
  -disk 50
```

## Tigris Object Storage

Create a Tigris bucket for backups:

```bash
fly storage create -a ratel-cluster -n ratel-cluster-backups
```

This sets `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`,
`AWS_ENDPOINT_URL_S3`, and `BUCKET_NAME` as app secrets.

Then configure CockroachDB to use it:

```sql
BACKUP INTO 's3://ratel-cluster-backups/full?AWS_ENDPOINT=fly.storage.tigris.dev&AWS_ACCESS_KEY_ID=...&AWS_SECRET_ACCESS_KEY=...';
```

## Building the Docker Image

```bash
# From the repo root
docker build -f deploy/fly-cluster/Dockerfile -t ghcr.io/semistrict/ratel:latest .
docker push ghcr.io/semistrict/ratel:latest
```

## Architecture

Each Fly Machine runs a single Ratel node with:
- Persistent volume mounted at `/cockroach/cockroach-data`
- Private IPv6 networking between nodes via `.internal` DNS
- Port 26257 for SQL (TLS terminated by Fly)
- Port 8080 for the admin UI (HTTPS via Fly)
- `--locality=region=<fly-region>` for zone-aware replication
