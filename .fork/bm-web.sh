#!/bin/bash
# Web UI codegen + lint + build for the bitmagnet fork (runs on block).
set -e
source ~/.nvm/nvm.sh >/dev/null
cd /root/bitmagnet/webui
npm run -s graphql:codegen 2>&1 | tail -5
npx prettier --write src/app/graphql/generated src/app/torrents src/app/themes src/app/util src/styles.scss >/dev/null
npx tsc -p tsconfig.app.json --noEmit 2>&1 | tail -30
npm run -s lint 2>&1 | tail -30 || true
npm run -s build 2>&1 | grep -v "^\s*$" | tail -25
