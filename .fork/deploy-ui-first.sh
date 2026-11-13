#!/bin/bash
set -e
mkdir -p /opt/bitmagnet-ui && chmod 700 /opt/bitmagnet-ui
cd /opt/bitmagnet-ui
NET=$(docker inspect bitmagnet-postgres --format '{{range $k,$v := .NetworkSettings.Networks}}{{$k}}{{end}}')
PW=$(docker inspect bitmagnet --format '{{range .Config.Env}}{{println .}}{{end}}' | grep '^POSTGRES_PASSWORD=' | cut -d= -f2-)
umask 077
printf 'POSTGRES_HOST=postgres\nPOSTGRES_PASSWORD=%s\nLOG_FILE_ROTATOR_ENABLED=false\n' "$PW" > ui.env
cat > docker-compose.yml <<EOF
# UI-only bitmagnet from the Firebug24k fork (filters / saved searches / Midnight theme).
# Shares the main bitmagnet Postgres; crawler stays on the upstream image in /home/firebug/bitmagnet.
# Source: /root/bitmagnet (github.com/Firebug24k/bitmagnet). Rebuild: /root/bitmagnet/.fork/deploy.sh
services:
  bitmagnet-ui:
    container_name: bitmagnet-ui
    image: bitmagnet-fb:dev
    restart: unless-stopped
    env_file: ui.env
    volumes:
      - /home/firebug/bitmagnet/config:/root/.config/bitmagnet:ro
    ports:
      - "3335:3333"
    labels:
      - com.centurylinklabs.watchtower.enable=false
    networks: [bm]
    command: [worker, run, --keys=http_server]
networks:
  bm:
    name: $NET
    external: true
EOF
docker compose up -d 2>&1 | tail -3
sleep 8
docker ps --filter name=bitmagnet-ui --format '{{.Names}} {{.Status}} {{.Ports}}'
docker logs --tail 15 bitmagnet-ui 2>&1
curl -s -o /dev/null -w 'webui %{http_code}\n' http://127.0.0.1:3335/webui/
