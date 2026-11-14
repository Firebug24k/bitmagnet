#!/bin/bash
# Go codegen + build + test for the bitmagnet fork, in a golang container on block.
# usage: bm-go.sh [gen|test|all]
set -e
cd /root/bitmagnet
MODE=${1:-all}
docker volume create bm-gomod >/dev/null; docker volume create bm-gocache >/dev/null
run() { docker run --rm -v /root/bitmagnet:/src -v bm-gomod:/go/pkg/mod -v bm-gocache:/root/.cache/go-build -w /src golang:1.23.6 bash -c "$1"; }
if [ "$MODE" = gen ] || [ "$MODE" = all ]; then
  run 'git config --global --add safe.directory /src; go run github.com/99designs/gqlgen generate --config ./internal/gql/gqlgen.yml 2>&1 | tail -20'
fi
if [ "$MODE" = test ] || [ "$MODE" = all ]; then
  run 'go build ./... && go vet ./internal/gql/... ./internal/database/search/... ./internal/peerrefresh/... && go test ./internal/gql/... ./internal/database/search/... ./internal/peerrefresh/... ./internal/model/... 2>&1 | tail -25'
fi
