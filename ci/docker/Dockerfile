ARG GO_VERSION=1.19
ARG DEBIAN_VERSION=11.6-slim
FROM golang:${GO_VERSION}-alpine AS build

ARG MIRRORS_SOURCE="dl-cdn.alpinelinux.org" \
    GOPROXY="https://goproxy.io,direct"

WORKDIR /opt

RUN sed -i "s/dl-cdn.alpinelinux.org/${MIRRORS_SOURCE}/g" /etc/apk/repositories \
 && apk add --update --no-cache \
      make \
      binutils

ADD . /opt

RUN GO111MODULE=on \
    GOPROXY=${GOPROXY} \
    go get -d -v ./...

RUN CGO_ENABLED=0 GOOS=linux GO111MODULE=on \
    go build -a -ldflags '-extldflags "-static"' -o ./csi-s3driver ./cmd/csi-s3driver/main.go

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

COPY --from=build /opt/csi-s3driver /csi-s3driver

ENTRYPOINT [ "/csi-s3driver" ]