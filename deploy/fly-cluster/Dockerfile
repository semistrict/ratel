FROM ubuntu:24.04 AS builder
RUN apt-get update && apt-get install -y curl libedit-dev g++ && rm -rf /var/lib/apt/lists/*
RUN curl -sSfL https://go.dev/dl/go1.26.1.linux-amd64.tar.gz | tar -C /usr/local -xz
ENV PATH="/usr/local/go/bin:$PATH"
WORKDIR /src
COPY . .
# Build a stub libproj — ratel doesn't use geospatial features.
RUN gcc -c deploy/fly-cluster/docker/proj_stub.c -o /tmp/proj_stub.o && \
    ar rcs /usr/lib/x86_64-linux-gnu/libproj.a /tmp/proj_stub.o
# Build with stdmalloc (skip jemalloc) and make tag (skip zcgo_flags).
RUN go build -mod=vendor -tags 'stdmalloc make' -o /ratel ./pkg/cmd/ratel/

FROM ubuntu:24.04
RUN apt-get update && apt-get install -y libedit2 ca-certificates && rm -rf /var/lib/apt/lists/*
COPY --from=builder /ratel /ratel
ENTRYPOINT ["/ratel"]
