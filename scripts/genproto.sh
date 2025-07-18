#!/usr/bin/env bash
if [ $# -ne 1 ]; then
    echo "Usage: $0 <proto_file>"
    exit 1
fi

protoc \
    --go_out=. \
    --go-grpc_out=. \
    "$1"