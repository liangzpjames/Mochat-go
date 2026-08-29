#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
unset GOROOT
source scripts/lib/migration_inventory_smoke.sh

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-schema-migrate-check}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13331}"
MYSQL_SCHEMA="${MOCHAT_MIGRATION_SMOKE_SCHEMA:-mochat_migration_smoke}"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-schema-migrate.XXXXXX")"
MIGRATE_BIN="$WORK_DIR/mochat-migrate"
IDENTITY_MIGRATE_BIN="$WORK_DIR/mochat-identity-migrate"
AI_INSIGHT_0165_BIN="$WORK_DIR/mochat-ai-insight-0165"

printf '%064d' 2 >"$WORK_DIR/saas-mfa.key"
printf '%064d' 3 >"$WORK_DIR/dashboard-mfa.key"
compose() {
  MOCHAT_MYSQL_PORT="$MYSQL_PORT" \
  MOCHAT_SAAS_ADMIN_MFA_ENCRYPTION_KEY_FILE="$WORK_DIR/saas-mfa.key" MOCHAT_SAAS_ADMIN_MFA_ENCRYPTION_KEY_ID=smoke-saas \
  MOCHAT_DASHBOARD_MFA_ENCRYPTION_KEY_FILE="$WORK_DIR/dashboard-mfa.key" MOCHAT_DASHBOARD_MFA_ENCRYPTION_KEY_ID=smoke-dashboard \
  MOCHAT_SAAS_ADMIN_JWT_SECRET=smoke-saas-jwt MOCHAT_DASHBOARD_JWT_SECRET=smoke-dashboard-jwt \
  docker compose -p "$PROJECT_NAME" -f "$COMPOSE_FILE" "$@"
}
cleanup() {
  if [ "${KEEP_STACK:-0}" != "1" ]; then compose down --remove-orphans >/dev/null 2>&1 || true; fi
  rm -rf "$WORK_DIR"
}
trap cleanup EXIT INT TERM

wait_healthy() {
  local deadline=$((SECONDS + 360)) status
  while [ "$SECONDS" -lt "$deadline" ]; do
    status="$(compose ps --format json mysql 2>/dev/null | python3 -c 'import json,sys; value=sys.stdin.read().strip(); print(json.loads(value).get("Health", "")) if value else print("")' 2>/dev/null || true)"
    [ "$status" = "healthy" ] && return 0
    sleep 2
  done
  compose logs --tail=160 mysql >&2 || true
  return 1
}

if [ "${MOCHAT_REUSE_MIGRATION_STACK:-0}" != "1" ]; then compose up -d mysql; fi
wait_healthy
compose exec -T mysql mariadb -uroot -pmochat_root -e "DROP DATABASE IF EXISTS \`$MYSQL_SCHEMA\`; CREATE DATABASE \`$MYSQL_SCHEMA\` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci; GRANT ALL PRIVILEGES ON \`$MYSQL_SCHEMA\`.* TO 'mochat'@'%'; FLUSH PRIVILEGES;"

mysql_scalar() {
  local schema="$1" query="$2"
  compose exec -T mysql mariadb -umochat -pmochat_pass -N -B "$schema" -e "$query" | tr -d '\r'
}

MYSQL_DSN="mochat:mochat_pass@tcp(127.0.0.1:$MYSQL_PORT)/$MYSQL_SCHEMA?parseTime=true&loc=Local"
run_migration_inventory_smoke
