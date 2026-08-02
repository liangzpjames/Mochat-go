#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
project="mochat-go-phase32-mysql"
port="${MOCHAT_PHASE32_MYSQL_PORT:-13342}"
compose=(docker compose -p "$project" -f deploy/standalone/docker-compose.yml)
run_compose() { MOCHAT_MYSQL_PORT="$port" "${compose[@]}" "$@"; }
compose_status() { run_compose ps --format json mysql 2>/dev/null; }
cleanup() { "${compose[@]}" down -v --remove-orphans >/dev/null 2>&1 || true; }
trap cleanup EXIT INT TERM
run_compose up -d mysql
for _ in $(seq 1 90); do
  health=$(compose_status | python3 -c 'import json,sys; s=sys.stdin.read().strip(); print(json.loads(s).get("Health", "") if s else "")' 2>/dev/null || true)
  [ "$health" = healthy ] && break
  sleep 2
done
[ "$health" = healthy ] || { run_compose logs --tail=100 mysql >&2; exit 1; }
dsn="mochat:mochat_pass@tcp(127.0.0.1:${port})/mochat?parseTime=true&loc=Local"
temp="$(mktemp -d)"
trap 'rm -rf "$temp"; cleanup' EXIT INT TERM
env -u GOROOT go build -o "$temp/mochat-migrate" ./cmd/mochat-migrate
"$temp/mochat-migrate" -dsn "$dsn" -project-root "$PWD" -action apply
MOCHAT_MYSQL_DSN="$dsn" MOCHAT_REQUIRE_MYSQL_INTEGRATION=1 go test -count=1 ./internal/modules/scrm/adapters/mysql ./internal/store -run Integration
echo "Phase 3.2 isolated MariaDB integration passed."
