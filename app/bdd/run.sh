#!/usr/bin/env bash
# Sobe o stack BDD (wiremock + app + nginx) e roda a suíte Godog contra ele.
# Uso: ./run.sh            (porta padrão 8089; sobrescreva com BDD_PORT)
set -euo pipefail
cd "$(dirname "$0")"

PORT="${BDD_PORT:-8089}"

cleanup() { docker compose down -v --remove-orphans >/dev/null 2>&1 || true; }
trap cleanup EXIT

docker compose up -d --build --force-recreate

echo "[i] Aguardando http://localhost:$PORT/health ..."
for i in $(seq 1 60); do
  if curl -fsS "http://localhost:$PORT/health" >/dev/null 2>&1; then
    break
  fi
  if [ "$i" = 60 ]; then
    echo "[!] App não respondeu — logs:"
    docker compose logs
    exit 1
  fi
  sleep 2
done

echo "[OK] Stack no ar — rodando Godog"
BDD_BASE_URL="http://localhost:$PORT" go test -count=1 -v .
