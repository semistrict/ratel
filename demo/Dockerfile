FROM golang:1.26-bookworm AS build

RUN apt-get update && apt-get install -y --no-install-recommends \
    libedit-dev gcc g++ cmake && \
    rm -rf /var/lib/apt/lists/*

# Build vendored PROJ 4 from source (static library).
COPY c-deps/proj/ /tmp/proj-src/
RUN mkdir /tmp/proj-build && cd /tmp/proj-build && \
    cmake /tmp/proj-src -DCMAKE_BUILD_TYPE=Release -DBUILD_LIBPROJ_SHARED=OFF && \
    make -j$(nproc) proj && \
    cp lib/libproj.a /usr/local/lib/ && \
    cp /tmp/proj-src/src/*.h /usr/local/include/

WORKDIR /src
COPY go.mod go.sum ./
COPY vendor/ vendor/
COPY pkg/ pkg/

# Override CGO flags to use system-installed PROJ and skip generated zcgo_flags.
ENV CGO_CPPFLAGS="-I/usr/local/include"
ENV CGO_LDFLAGS="-L/usr/local/lib"

# Remove generated zcgo_flags files that hardcode macOS paths.
RUN rm -f pkg/geo/geoproj/zcgo_flags*.go

RUN go build -mod=vendor -tags 'stdmalloc short make' -o /usr/local/bin/ratel ./pkg/cmd/ratel

FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates libedit2 curl && \
    rm -rf /var/lib/apt/lists/*

COPY --from=build /usr/local/bin/ratel /usr/local/bin/ratel

ENTRYPOINT ["ratel"]
