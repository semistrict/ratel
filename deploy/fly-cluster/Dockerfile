FROM golang:1.26-bookworm AS builder
RUN apt-get update && apt-get install -y autoconf libedit-dev cmake g++ libncurses-dev && rm -rf /var/lib/apt/lists/*
WORKDIR /src
# Copy everything (use .dockerignore to exclude large dirs if needed)
COPY . .
RUN make buildratel BUILDTYPE=release

FROM gcr.io/distroless/base-debian12
COPY --from=builder /src/ratel /ratel
ENTRYPOINT ["/ratel"]
