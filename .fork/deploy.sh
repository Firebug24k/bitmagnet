#!/bin/bash
# Rebuild + redeploy the Firebug24k bitmagnet fork on block.
#   1. web UI codegen/lint/build  2. Go gqlgen/tests  3. docker image  4. recreate the `bitmagnet`
#   service in /home/firebug/bitmagnet (crawler + queue + UI, behind bitmagnet-gluetun, :3333).
# Rollback: docker tag bitmagnet-fb:<previous-tag> bitmagnet-fb:current && rerun step 4,
#   or restore /home/firebug/bitmagnet/docker-compose.yaml.bak-20261113-upstream (upstream image).
set -e
cd /root/bitmagnet
bash .fork/bm-web.sh
bash .fork/bm-go.sh test
TAG=$(git describe --tags --always --dirty)
docker build -q -t "bitmagnet-fb:$TAG" . >/dev/null
docker tag "bitmagnet-fb:$TAG" bitmagnet-fb:current
cd /home/firebug/bitmagnet && docker compose up -d --no-deps bitmagnet 2>&1 | tail -2
sleep 10
docker logs --tail 4 bitmagnet 2>&1
curl -s -o /dev/null -w "webui %{http_code} ($TAG)\n" http://127.0.0.1:3333/webui/
