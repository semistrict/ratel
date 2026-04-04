FROM golang:1.26-bookworm AS build

RUN apt-get update && apt-get install -y --no-install-recommends \
    cmake autoconf libncurses-dev libedit-dev make gcc g++ && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /src
COPY . .

RUN make buildratel && cp ratel /usr/local/bin/ratel

FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates libedit2 && \
    rm -rf /var/lib/apt/lists/*

COPY --from=build /usr/local/bin/ratel /usr/local/bin/ratel
COPY --from=build /src/lib/libgeos*.so* /usr/lib/

ENTRYPOINT ["ratel"]
