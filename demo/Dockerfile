FROM golang:1.26-bookworm AS build

RUN apt-get update && apt-get install -y --no-install-recommends \
    libedit-dev libproj-dev gcc g++ && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /src
COPY go.mod go.sum ./
COPY vendor/ vendor/
COPY c-deps/proj/src/ c-deps/proj/src/
COPY pkg/ pkg/

RUN go build -mod=vendor -tags 'stdmalloc short' -o /usr/local/bin/ratel ./pkg/cmd/ratel

FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates libedit2 libproj25 curl && \
    rm -rf /var/lib/apt/lists/*

COPY --from=build /usr/local/bin/ratel /usr/local/bin/ratel

ENTRYPOINT ["ratel"]
