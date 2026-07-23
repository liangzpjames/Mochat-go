#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-async-file-upload-check}"
GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18096}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26398}"
HTTP_ADDR="${MOCHAT_ASYNC_FILE_SOURCE_ADDR:-127.0.0.1:19078}"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-async-file-upload.XXXXXX")"
GO_BIN="$WORK_DIR/mochat-go"
GO_LOG="$WORK_DIR/go.log"
HTTP_LOG="$WORK_DIR/http.log"
GO_PID=""
HTTP_PID=""

compose() {
  MOCHAT_REDIS_PORT="$REDIS_PORT" docker compose -p "$PROJECT_NAME" -f "$COMPOSE_FILE" "$@"
}

cleanup() {
  if [ -n "$GO_PID" ] && kill -0 "$GO_PID" 2>/dev/null; then
    kill "$GO_PID" 2>/dev/null || true
    wait "$GO_PID" 2>/dev/null || true
  fi
  if [ -n "$HTTP_PID" ] && kill -0 "$HTTP_PID" 2>/dev/null; then
    kill "$HTTP_PID" 2>/dev/null || true
    wait "$HTTP_PID" 2>/dev/null || true
  fi
  rm -rf "$WORK_DIR"
  if [ "${KEEP_STACK:-0}" != "1" ]; then
    compose down -v --remove-orphans >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT INT TERM

assert_port_free() {
  local addr="$1"
  local port="${addr##*:}"
  if lsof -nP -iTCP:"$port" -sTCP:LISTEN >/dev/null 2>&1; then
    echo "port $port is already in use" >&2
    exit 1
  fi
}

wait_url() {
  local url="$1"
  local expected="$2"
  local deadline=$((SECONDS + 45))
  while [ "$SECONDS" -lt "$deadline" ]; do
    local code
    code="$(curl -s -o /dev/null -w '%{http_code}' "$url" || true)"
    if [ "$code" = "$expected" ]; then
      return 0
    fi
    sleep 1
  done
  echo "timed out waiting for $url to return $expected" >&2
  [ -f "$GO_LOG" ] && tail -120 "$GO_LOG" >&2 || true
  [ -f "$HTTP_LOG" ] && tail -80 "$HTTP_LOG" >&2 || true
  exit 1
}

wait_service_healthy() {
  local service="$1"
  local deadline=$((SECONDS + ${MOCHAT_SERVICE_HEALTH_TIMEOUT_SECONDS:-360}))
  while [ "$SECONDS" -lt "$deadline" ]; do
    local status
    status="$(compose ps --format json "$service" 2>/dev/null | python3 -c 'import json,sys; data=sys.stdin.read().strip(); print(json.loads(data).get("Health", "")) if data else print("")' 2>/dev/null || true)"
    if [ "$status" = "healthy" ]; then
      return 0
    fi
    sleep 2
  done
  echo "$service did not become healthy" >&2
  compose ps >&2 || true
  compose logs --tail=120 "$service" >&2 || true
  exit 1
}

redis_scalar() {
  compose exec -T redis redis-cli --raw "$@" | tr -d '\r'
}

wait_redis_scalar() {
  local expected="$1"
  shift
  local deadline=$((SECONDS + 60))
  while [ "$SECONDS" -lt "$deadline" ]; do
    local value
    value="$(redis_scalar "$@" || true)"
    if [ "$value" = "$expected" ]; then
      return 0
    fi
    sleep 1
  done
  echo "timed out waiting for Redis command to return $expected" >&2
  echo "redis-cli $*" >&2
  echo "last value: $(redis_scalar "$@" || true)" >&2
  [ -f "$GO_LOG" ] && tail -120 "$GO_LOG" >&2 || true
  exit 1
}

wait_file_content() {
  local path="$1"
  local expected="$2"
  local deadline=$((SECONDS + 60))
  while [ "$SECONDS" -lt "$deadline" ]; do
    if [ -f "$path" ] && [ "$(cat "$path")" = "$expected" ]; then
      return 0
    fi
    sleep 1
  done
  echo "timed out waiting for file $path" >&2
  [ -f "$path" ] && cat "$path" >&2 || true
  [ -f "$GO_LOG" ] && tail -120 "$GO_LOG" >&2 || true
  exit 1
}

assert_port_free "$GO_ADDR"
assert_port_free "127.0.0.1:$REDIS_PORT"
assert_port_free "$HTTP_ADDR"

mkdir -p "$WORK_DIR/http"
printf "remote payload" >"$WORK_DIR/http/remote.txt"
python3 -m http.server "${HTTP_ADDR##*:}" --bind "${HTTP_ADDR%:*}" --directory "$WORK_DIR/http" >"$HTTP_LOG" 2>&1 &
HTTP_PID="$!"
wait_url "http://$HTTP_ADDR/remote.txt" 200

compose up -d redis
wait_service_healthy redis

env -u GOROOT go build -o "$GO_BIN" ./cmd/mochat-go

LOCAL_SOURCE="$WORK_DIR/local-source.txt"
printf "local payload" >"$LOCAL_SOURCE"

env -u GOROOT \
  MOCHAT_GO_STANDALONE=1 \
  MOCHAT_GO_ADDR="$GO_ADDR" \
  MOCHAT_GO_ENABLE_ASYNC_FILE_UPLOAD_WORKER=1 \
  MOCHAT_GO_WORKER_PROCESSING_TIMEOUT_SECONDS=2 \
  MOCHAT_FILE_STORAGE_ROOT="$WORK_DIR/storage" \
  MOCHAT_REDIS_ADDR="127.0.0.1:$REDIS_PORT" \
  MOCHAT_PHP_UPSTREAM="not-a-valid-upstream-url" \
  MOCHAT_SOURCE_ROOT="$WORK_DIR/should-not-read-php-source" \
  MOCHAT_COMPAT_MANIFEST="$WORK_DIR/should-not-read-manifest.json" \
  "$GO_BIN" >"$GO_LOG" 2>&1 &
GO_PID="$!"

wait_url "http://$GO_ADDR/readyz" 200
curl -sS -f "http://$GO_ADDR/compat/status" >"$WORK_DIR/status.json"
python3 - "$WORK_DIR/status.json" <<'PY'
import json
import pathlib
import sys

status = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
tasks = {task.get("name"): task for task in status.get("background_tasks", [])}
task = tasks.get("async-file-upload")
assert task, status
assert task.get("status") == "running", task
assert task.get("started_at"), task
PY

python3 - "$WORK_DIR/event.json" "$LOCAL_SOURCE" "http://$HTTP_ADDR/remote.txt" <<'PY'
import json
import pathlib
import sys

target = pathlib.Path(sys.argv[1])
local_source = sys.argv[2]
remote_source = sys.argv[3]
payload = [
    [local_source, "queued/local.txt", 1],
    [remote_source, "queued/remote.txt", 0],
]
target.write_text(json.dumps(payload, ensure_ascii=False), encoding="utf-8")
PY

compose exec -T redis redis-cli -x RPUSH mochat-go:async-file-upload <"$WORK_DIR/event.json" >/dev/null

wait_file_content "$WORK_DIR/storage/queued/local.txt" "local payload"
wait_file_content "$WORK_DIR/storage/queued/remote.txt" "remote payload"
if [ -e "$LOCAL_SOURCE" ]; then
  echo "local source was not deleted" >&2
  exit 1
fi
wait_redis_scalar "0" LLEN mochat-go:async-file-upload
wait_redis_scalar "0" LLEN mochat-go:async-file-upload:processing
wait_redis_scalar "0" LLEN mochat-go:async-file-upload:dead
grep -q "go worker enabled: AsyncFileUpload Redis consumer" "$GO_LOG"

echo "async file upload worker smoke passed"
