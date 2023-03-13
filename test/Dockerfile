ARG GO_VERSION=1.19
ARG DEBIAN_VERSION=11.6-slim
FROM golang:${GO_VERSION}-alpine AS base

ARG MIRRORS_SOURCE="dl-cdn.alpinelinux.org" \
    GOPROXY="https://goproxy.io,direct"

WORKDIR /opt

RUN sed -i "s/dl-cdn.alpinelinux.org/${MIRRORS_SOURCE}/g" /etc/apk/repositories \
 && apk add --update --no-cache \
      make \
      binutils

RUN cd /tmp \
 && wget -q https://dl.min.io/server/minio/release/$(go env GOOS)-$(go env GOARCH)/minio \
 && mv minio /usr/local/bin \
 && chmod +x /usr/local/bin/minio

WORKDIR /opt

COPY go.* .

RUN make install

ADD test/docker-entrypoint.sh /

ENTRYPOINT [ "/docker-entrypoint.sh" ]