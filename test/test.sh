#!/usr/bin/env bash

export MINIO_ACCESS_KEY=MOYmRRlL45feb3Xb
export MINIO_SECRET_KEY=xyiMaa0cAaSulzQ4

mkdir -p /tmp/minio
minio server /tmp/minio &>/dev/null &
sleep 5

go test github.com/leryn1122/csi-s3/pkg/s3 -cover