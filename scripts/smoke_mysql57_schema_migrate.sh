#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
unset GOROOT
source scripts/lib/migration_inventory_smoke.sh

COMPOSE_FILE="deploy/mysql57/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-mysql57-schema-check}"
MYSQL_PORT="${MOCHAT_MYSQL57_PORT:-13332}"
MYSQL_SCHEMA="${MOCHAT_MIGRATION_SMOKE_SCHEMA:-mochat_migration_smoke}"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-mysql57-schema.XXXXXX")"
MIGRATE_BIN="$WORK_DIR/mochat-migrate"
IDENTITY_MIGRATE_BIN="$WORK_DIR/mochat-identity-migrate"
AI_INSIGHT_0165_BIN="$WORK_DIR/mochat-ai-insight-0165"

ARCH="$(uname -m)"
if { [ "$ARCH" = "arm64" ] || [ "$ARCH" = "aarch64" ]; } && [ "${MOCHAT_FORCE_MYSQL57:-0}" != "1" ]; then
  echo "mysql 5.7 migration smoke SKIP on $ARCH: mysql:5.7 requires amd64; set MOCHAT_FORCE_MYSQL57=1 only when emulation is known-good" >&2
  exit 0
fi

compose() { MOCHAT_MYSQL57_PORT="$MYSQL_PORT" docker compose -p "$PROJECT_NAME" -f "$COMPOSE_FILE" "$@"; }
cleanup() {
  rm -rf "$WORK_DIR"
  if [ "${KEEP_STACK:-0}" != "1" ]; then compose down --remove-orphans >/dev/null 2>&1 || true; fi
}
trap cleanup EXIT INT TERM

wait_healthy() {
  local deadline=$((SECONDS + 240)) status
  while [ "$SECONDS" -lt "$deadline" ]; do
    status="$(compose ps --format json mysql57 2>/dev/null | python3 -c 'import json,sys; value=sys.stdin.read().strip(); print(json.loads(value).get("Health", "")) if value else print("")' 2>/dev/null || true)"
    [ "$status" = "healthy" ] && return 0
    sleep 2
  done
  compose logs --tail=160 mysql57 >&2 || true
  return 1
}

if [ "${MOCHAT_REUSE_MIGRATION_STACK:-0}" != "1" ]; then compose up -d mysql57; fi
wait_healthy
compose exec -T mysql57 mysql -uroot -pmochat_root -e "DROP DATABASE IF EXISTS \`$MYSQL_SCHEMA\`; CREATE DATABASE \`$MYSQL_SCHEMA\` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci; GRANT ALL PRIVILEGES ON \`$MYSQL_SCHEMA\`.* TO 'mochat'@'%'; FLUSH PRIVILEGES;"

mysql_scalar() {
  local schema="$1" query="$2"
  compose exec -T mysql57 mysql -umochat -pmochat_pass -N -B "$schema" -e "$query" | tr -d '\r'
}

MYSQL_DSN="mochat:mochat_pass@tcp(127.0.0.1:$MYSQL_PORT)/$MYSQL_SCHEMA?parseTime=true&loc=Local"
test "$(mysql_scalar "$MYSQL_SCHEMA" "SELECT VERSION() LIKE '5.7.%'")" = "1"
run_migration_inventory_smoke
