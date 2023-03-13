#!/usr/bin/env bash

export MINIO_ACCESS_KEY=3tDcHgbrFVkxBL0D
export MINIO_SECRET_KEY=YNGOcikxOsWlyjl3

mkdir -p /tmp/minio
minio server /tmp/minio &>/dev/null &
sleep 5

go test ./... -cover