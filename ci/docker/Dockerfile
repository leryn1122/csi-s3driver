ARG GO_VERSION=1.21
ARG DEBIAN_VERSION=buster-slim
FROM golang:${GO_VERSION}-alpine AS build

ARG MIRRORS_SOURCE="dl-cdn.alpinelinux.org"
ARG GOPROXY="https://goproxy.io,direct"

WORKDIR /tmp/project

RUN sed -i "s/dl-cdn.alpinelinux.org/${MIRRORS_SOURCE}/g" /etc/apk/repositories \
 && apk add --update --no-cache \
      bash \
      binutils \
      make

ADD . /tmp/project

RUN make install

RUN make build

FROM debian:${DEBIAN_VERSION} AS runtime

LABEL maintainers="Leryn <leryn1122@gmail.com>"
LABEL description="csi-s3driver"

ARG MIRRORS_SOURCE="deb.debian.org"

RUN sed -i "s/deb.debian.org/${MIRRORS_SOURCE}/g" /etc/apt/sources.list \
 && apt-get update && apt-get install -y \
      curl \
      s3fs \
      rclone \
      unzip \
 && apt-get clean \
 && rm -rf /var/lib/apt/lists/*

COPY --from=build /tmp/project/target/csi-s3driver /csi-s3driver

ENTRYPOINT [ "/csi-s3driver" ]
