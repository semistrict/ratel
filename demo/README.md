# Ratel 5-Node Demo

A 5-node ratel cluster backed by rustfs (S3-compatible object storage).

## Prerequisites

- Docker and Docker Compose

## Run

```bash
cd demo
docker compose up --build
```

This starts:
- **rustfs** — S3-compatible storage on port 9000 (console on 9001)
- **ratel-init** — bootstraps the cluster, becomes node 1
- **ratel-2 through ratel-5** — join the cluster after node 1 is healthy

## Connect

```bash
# From the host (port 26257 is forwarded from ratel-init)
docker compose exec ratel-init ratel sql "s3://ratel/?endpoint=http://rustfs:9000&region=us-east-1"

# Or with psql directly
psql "postgresql://root@localhost:26257/defaultdb?sslmode=disable"
```

## Browse

- RustFS console: http://localhost:9001 (rustfsadmin / rustfsadmin)
- CockroachDB admin UI: http://localhost:8080

## Tear down

```bash
docker compose down -v
```
