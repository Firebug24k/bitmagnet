#!/bin/bash
# Rebuild + redeploy the Firebug24k bitmagnet fork UI on block.
#   1. web UI codegen/lint/build  2. Go codegen/test  3. docker image  4. recreate bitmagnet-ui (:3335)
# The crawler (/home/firebug/bitmagnet, upstream image) is never touched.
set -e
cd /root/bitmagnet
bash .fork/bm-web.sh
bash .fork/bm-go.sh test
docker build -q -t bitmagnet-fb:dev . >/dev/null
docker tag bitmagnet-fb:dev "bitmagnet-fb:$(git describe --tags --always --dirty)"
cd /opt/bitmagnet-ui && docker compose up -d --force-recreate 2>&1 | tail -2
sleep 6
docker logs --tail 3 bitmagnet-ui 2>&1
curl -s -o /dev/null -w 'webui %{http_code}\n' http://127.0.0.1:3335/webui/
