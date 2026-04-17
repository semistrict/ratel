FROM ubuntu:22.04

RUN apt-get update \
 && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
      ca-certificates curl git build-essential cmake \
      autoconf automake libtool pkg-config \
      bison libncurses-dev ccache \
      python3 python3-venv \
 && rm -rf /var/lib/apt/lists/*

ARG BAZELISK_VERSION=v1.28.1
ARG TARGETARCH
RUN arch="$(dpkg --print-architecture)" \
 && curl -sSfL -o /usr/local/bin/bazel \
      https://github.com/bazelbuild/bazelisk/releases/download/${BAZELISK_VERSION}/bazelisk-linux-${arch} \
 && chmod +x /usr/local/bin/bazel

RUN useradd -m -u 1000 builder
USER builder
WORKDIR /cockroach
