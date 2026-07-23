#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-queue-idempotency-check}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26394}"

compose() {
  MOCHAT_REDIS_PORT="$REDIS_PORT" docker compose -p "$PROJECT_NAME" -f "$COMPOSE_FILE" "$@"
}

cleanup() {
  if [ "${KEEP_STACK:-0}" != "1" ]; then
    compose down -v --remove-orphans >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT INT TERM

if lsof -nP -iTCP:"$REDIS_PORT" -sTCP:LISTEN >/dev/null 2>&1; then
  echo "port $REDIS_PORT is already in use" >&2
  exit 1
fi

compose up -d redis

deadline=$((SECONDS + 180))
while [ "$SECONDS" -lt "$deadline" ]; do
  status="$(compose ps --format json redis 2>/dev/null | python3 -c 'import json,sys; data=sys.stdin.read().strip(); print(json.loads(data).get("Health", "")) if data else print("")' 2>/dev/null || true)"
  if [ "$status" = "healthy" ]; then
    break
  fi
  sleep 2
done
if [ "${status:-}" != "healthy" ]; then
  echo "redis did not become healthy" >&2
  compose ps >&2 || true
  compose logs --tail=120 redis >&2 || true
  exit 1
fi

MOCHAT_REDIS_ADDR="127.0.0.1:$REDIS_PORT" env -u GOROOT go test ./internal/store -run TestRedisStoreQueueIdempotencyIntegration -count=1

echo "queue idempotency smoke passed"
