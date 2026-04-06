# Build stage: compile ratel from source
FROM golang:1.26-bookworm AS builder

RUN apt-get update && apt-get install -y \
    autoconf libedit-dev cmake g++ libncurses-dev \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /src
COPY . .
RUN make buildshort

# Runtime stage
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y \
    ca-certificates libedit2 \
    && rm -rf /var/lib/apt/lists/*

RUN mkdir -p /cockroach/cockroach-data
COPY --from=builder /src/cockroach-short /cockroach/cockroach
COPY deploy/fly-cluster/entrypoint.sh /cockroach/entrypoint.sh
RUN chmod +x /cockroach/entrypoint.sh

WORKDIR /cockroach
EXPOSE 26257 8080
ENTRYPOINT ["/cockroach/entrypoint.sh"]
