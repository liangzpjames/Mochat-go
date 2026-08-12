#!/usr/bin/env bash
set -Eeuo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-saas-service-account-check}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13392}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26442}"
GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18166}"
DATABASE="mochat_saas_service_accounts"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-saas-service-accounts.XXXXXX")"
MIGRATE_BIN="$WORK_DIR/mochat-migrate"
BOOTSTRAP_BIN="$WORK_DIR/mochat-bootstrap"
GO_BIN="$WORK_DIR/mochat-go"
MAINTENANCE_BIN="$WORK_DIR/mochat-saas-maintenance"
GO_LOG="$WORK_DIR/go.log"
GO_PID=""
SECRET="${MOCHAT_SIMPLE_JWT_SECRET:-saas-service-account-jwt-pepper-secret}"
PEPPER_KEY="${MOCHAT_GO_SAAS_SERVICE_ACCOUNT_KEY_PEPPER:-0707070707070707070707070707070707070707070707070707070707070707}"
PEPPER_ID="${MOCHAT_GO_SAAS_SERVICE_ACCOUNT_KEY_PEPPER_ID:-service-account-2026-q3}"

compose() {
  MOCHAT_MYSQL_PORT="$MYSQL_PORT" MOCHAT_REDIS_PORT="$REDIS_PORT" docker compose -p "$PROJECT_NAME" -f "$COMPOSE_FILE" "$@"
}

cleanup() {
  if [ -n "$GO_PID" ] && kill -0 "$GO_PID" 2>/dev/null; then
    kill "$GO_PID" 2>/dev/null || true
    wait "$GO_PID" 2>/dev/null || true
  fi
  rm -rf "$WORK_DIR"
  if [ "${KEEP_STACK:-0}" != "1" ]; then
    compose down -v --remove-orphans >/dev/null 2>&1 || true
  fi
}
trap 'echo "SaaS service account smoke failed at line $LINENO" >&2; tail -180 "$GO_LOG" >&2 || true' ERR
trap cleanup EXIT INT TERM

assert_port_free() {
  local port="$1"
  if lsof -nP -iTCP:"$port" -sTCP:LISTEN >/dev/null 2>&1; then
    echo "port $port is already in use" >&2
    exit 1
  fi
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
  compose ps >&2 || true
  compose logs --tail=120 "$service" >&2 || true
  exit 1
}

wait_url() {
  local url="$1" expected="$2"
  local deadline=$((SECONDS + 60))
  while [ "$SECONDS" -lt "$deadline" ]; do
    local code
    code="$(curl -s -o /dev/null -w '%{http_code}' "$url" || true)"
    if [ "$code" = "$expected" ]; then
      return 0
    fi
    sleep 1
  done
  return 1
}

mysql_root() {
  compose exec -T mysql mariadb -uroot -pmochat_root "$@"
}

mysql_scalar() {
  compose exec -T mysql mariadb -umochat -pmochat_pass -N -B "$DATABASE" -e "$1" </dev/null | tr -d '\r'
}

wait_mysql_at_least() {
  local query="$1" minimum="$2" deadline=$((SECONDS + 60)) value
  while [ "$SECONDS" -lt "$deadline" ]; do
    value="$(mysql_scalar "$query" || true)"
    if [[ "$value" =~ ^[0-9]+$ ]] && [ "$value" -ge "$minimum" ]; then return 0; fi
    sleep 1
  done
  echo "timed out waiting for MySQL value >= $minimum: $query" >&2
  return 1
}

login_token() {
  local phone="$1" password="$2" output="$3"
  curl -sS -f -H "Content-Type: application/json" -d "{\"phone\":\"$phone\",\"password\":\"$password\"}" "http://$GO_ADDR/dashboard/user/auth" >"$output"
  python3 - "$output" <<'PY'
import json, pathlib, sys
payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
assert payload.get("code") == 200, payload
print(payload["data"]["token"])
PY
}

admin_get() {
  local token="$1" path="$2" output="$3" expected="${4:-200}"
  local status
  status="$(curl -sS -o "$output" -w '%{http_code}' -H "Authorization: Bearer $token" "http://$GO_ADDR$path")"
  test "$status" = "$expected" || { echo "GET $path returned $status" >&2; cat "$output" >&2; return 1; }
}

admin_post() {
  local token="$1" path="$2" body="$3" output="$4" expected="${5:-200}"
  local status
  status="$(curl -sS -D "$output.headers" -o "$output" -w '%{http_code}' -H "Authorization: Bearer $token" -H "Content-Type: application/json" -d "$body" "http://$GO_ADDR$path")"
  test "$status" = "$expected" || { echo "POST $path returned $status" >&2; cat "$output" >&2; return 1; }
}

admin_put() {
  local token="$1" path="$2" body="$3" output="$4" expected="${5:-200}"
  local status
  status="$(curl -sS -D "$output.headers" -o "$output" -w '%{http_code}' -X PUT -H "Authorization: Bearer $token" -H "Content-Type: application/json" -d "$body" "http://$GO_ADDR$path")"
  test "$status" = "$expected" || { echo "PUT $path returned $status" >&2; cat "$output" >&2; return 1; }
}

key_get() {
  local key="$1" path="$2" output="$3" expected="${4:-200}" forwarded_ip="${5:-}"
  local status
  local args=(-sS -D "$output.headers" -o "$output" -w '%{http_code}' -H "Authorization: Bearer $key")
  [ -n "$forwarded_ip" ] && args+=(-H "X-Forwarded-For: $forwarded_ip")
  status="$(curl "${args[@]}" "http://$GO_ADDR$path")"
  test "$status" = "$expected" || { echo "API key GET $path returned $status" >&2; cat "$output" >&2; return 1; }
  if [ "$expected" = "200" ] && [ ! -s "$output" ]; then
    echo "API key GET $path returned an empty body" >&2
    cat "$output.headers" >&2
    return 1
  fi
  if [ "$expected" = "200" ] && ! python3 - "$output" <<'PY'
import json, pathlib, sys
json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
PY
  then
    echo "API key GET $path returned a non-JSON body" >&2
    cat "$output.headers" >&2
    sed -n '1,20p' "$output" >&2
    return 1
  fi
}

json_value() {
  local file="$1" expression="$2"
  python3 - "$file" "$expression" <<'PY'
import json, pathlib, sys
value = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
for part in sys.argv[2].split('.'):
    value = value[int(part)] if part.isdigit() else value[part]
print(value)
PY
}

approve_service_account_create() {
  local payload="$1" output_prefix="$2"
  local request_body approval_id approval_version decision_body execute_body plain_text_key
  request_body="$(jq -cn --argjson payload "$payload" --arg key "service-account-create-$output_prefix" '{actionType:"service_account.create",payload:$payload,reason:"smoke 双人复核创建服务账号",idempotencyKey:$key}')"
  admin_post "$OPERATIONS_TOKEN" "/dashboard/saasAdmin/approvalRequest" "$request_body" "$WORK_DIR/$output_prefix-request.json"
  approval_id="$(json_value "$WORK_DIR/$output_prefix-request.json" data.approval.id)"
  approval_version="$(json_value "$WORK_DIR/$output_prefix-request.json" data.approval.version)"
  jq -e '.data.approval.actionType == "service_account.create" and .data.approval.requiredApprovals == 2 and .data.approval.status == "pending" and .data.approval.targetId == "961:data_sync"' "$WORK_DIR/$output_prefix-request.json" >/dev/null
  test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approvals WHERE id = $approval_id AND JSON_EXTRACT(request_json, '$.key') IS NULL AND JSON_EXTRACT(request_json, '$.Key') IS NULL AND LOCATE('mch_live_', request_json) = 0")" = "1"
  test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_service_accounts WHERE tenant_id = 961 AND code = 'data_sync'")" = "0"

  decision_body="{\"approvalId\":$approval_id,\"decision\":\"approve\",\"reason\":\"平台超管一审通过\",\"expectedVersion\":$approval_version}"
  admin_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/approvalDecision" "$decision_body" "$WORK_DIR/$output_prefix-decision-one.json"
  approval_version="$(json_value "$WORK_DIR/$output_prefix-decision-one.json" data.approval.version)"
  jq -e '.data.approval.status == "pending" and .data.approval.approvalCount == 1' "$WORK_DIR/$output_prefix-decision-one.json" >/dev/null

  decision_body="{\"approvalId\":$approval_id,\"decision\":\"approve\",\"reason\":\"独立审批人二审通过\",\"expectedVersion\":$approval_version}"
  admin_post "$APPROVER_TOKEN" "/dashboard/saasAdmin/approvalDecision" "$decision_body" "$WORK_DIR/$output_prefix-decision-two.json"
  approval_version="$(json_value "$WORK_DIR/$output_prefix-decision-two.json" data.approval.version)"
  jq -e '.data.approval.status == "approved" and .data.approval.approvalCount == 2' "$WORK_DIR/$output_prefix-decision-two.json" >/dev/null

  execute_body="{\"approvalId\":$approval_id,\"expectedVersion\":$approval_version}"
  admin_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/approvalExecute" "$execute_body" "$WORK_DIR/$output_prefix-execute.json"
  plain_text_key="$(json_value "$WORK_DIR/$output_prefix-execute.json" data.result.plainTextKey)"
  jq -e '.data.approval.status == "executed" and .data.approval.effectOperationId > 0 and .data.result.initialKey == true and (.data.result.plainTextKey | startswith("mch_live_")) and .data.result.account.tenantId == 961 and .data.result.account.code == "data_sync" and .data.result.key.status == "active" and .data.result.operationId == .data.approval.effectOperationId' "$WORK_DIR/$output_prefix-execute.json" >/dev/null
  grep -qi '^Cache-Control: no-store' "$WORK_DIR/$output_prefix-execute.json.headers"
  test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approvals WHERE id = $approval_id AND action_type = 'service_account.create' AND status = 'executed' AND approval_count = 2 AND effect_applied_at IS NOT NULL AND effect_operation_id > 0 AND JSON_EXTRACT(result_json, '$.plainTextKey') IS NULL AND JSON_EXTRACT(result_json, '$.plainTextKeyDelivered') = TRUE AND LOCATE('$plain_text_key', request_json) = 0 AND LOCATE('$plain_text_key', result_json) = 0")" = "1"
}

approve_service_account_key_revoke() {
  local key_id="$1" key_version="$2" output_prefix="$3"
  local request_body approval_id approval_version decision_body execute_body
  request_body="{\"actionType\":\"service_account.key.revoke\",\"payload\":{\"serviceAccountId\":$ACCOUNT_ID,\"keyId\":$key_id,\"expectedVersion\":$key_version},\"reason\":\"smoke 双人复核吊销 API Key #$key_id\",\"idempotencyKey\":\"service-account-key-revoke-$key_id\"}"
  admin_post "$OPERATIONS_TOKEN" "/dashboard/saasAdmin/approvalRequest" "$request_body" "$WORK_DIR/$output_prefix-request.json"
  approval_id="$(json_value "$WORK_DIR/$output_prefix-request.json" data.approval.id)"
  approval_version="$(json_value "$WORK_DIR/$output_prefix-request.json" data.approval.version)"
  jq -e '.data.approval.actionType == "service_account.key.revoke" and .data.approval.requiredApprovals == 2 and .data.approval.status == "pending"' "$WORK_DIR/$output_prefix-request.json" >/dev/null

  decision_body="{\"approvalId\":$approval_id,\"decision\":\"approve\",\"reason\":\"平台超管一审通过\",\"expectedVersion\":$approval_version}"
  admin_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/approvalDecision" "$decision_body" "$WORK_DIR/$output_prefix-decision-one.json"
  approval_version="$(json_value "$WORK_DIR/$output_prefix-decision-one.json" data.approval.version)"
  jq -e '.data.approval.status == "pending" and .data.approval.approvalCount == 1' "$WORK_DIR/$output_prefix-decision-one.json" >/dev/null

  decision_body="{\"approvalId\":$approval_id,\"decision\":\"approve\",\"reason\":\"独立审批人二审通过\",\"expectedVersion\":$approval_version}"
  admin_post "$APPROVER_TOKEN" "/dashboard/saasAdmin/approvalDecision" "$decision_body" "$WORK_DIR/$output_prefix-decision-two.json"
  approval_version="$(json_value "$WORK_DIR/$output_prefix-decision-two.json" data.approval.version)"
  jq -e '.data.approval.status == "approved" and .data.approval.approvalCount == 2' "$WORK_DIR/$output_prefix-decision-two.json" >/dev/null

  execute_body="{\"approvalId\":$approval_id,\"expectedVersion\":$approval_version}"
  admin_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/approvalExecute" "$execute_body" "$WORK_DIR/$output_prefix-execute.json"
  jq -e --argjson key_id "$key_id" '.data.approval.status == "executed" and .data.approval.effectOperationId > 0 and .data.result.key.id == $key_id and .data.result.key.status == "revoked" and .data.result.operationId == .data.approval.effectOperationId' "$WORK_DIR/$output_prefix-execute.json" >/dev/null
  test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approvals WHERE id = $approval_id AND action_type = 'service_account.key.revoke' AND status = 'executed' AND approval_count = 2 AND effect_applied_at IS NOT NULL AND effect_operation_id > 0")" = "1"
}

approve_service_account_update() {
  local payload="$1" output_prefix="$2"
  local request_body approval_id approval_version decision_body execute_body
  request_body="{\"actionType\":\"service_account.update\",\"payload\":$payload,\"reason\":\"smoke 双人复核变更服务账号\",\"idempotencyKey\":\"service-account-update-$output_prefix\"}"
  admin_post "$OPERATIONS_TOKEN" "/dashboard/saasAdmin/approvalRequest" "$request_body" "$WORK_DIR/$output_prefix-request.json"
  approval_id="$(json_value "$WORK_DIR/$output_prefix-request.json" data.approval.id)"
  approval_version="$(json_value "$WORK_DIR/$output_prefix-request.json" data.approval.version)"
  jq -e --argjson account_id "$ACCOUNT_ID" '.data.approval.actionType == "service_account.update" and .data.approval.requiredApprovals == 2 and .data.approval.status == "pending" and (.data.approval.targetId | tonumber) == $account_id' "$WORK_DIR/$output_prefix-request.json" >/dev/null

  decision_body="{\"approvalId\":$approval_id,\"decision\":\"approve\",\"reason\":\"平台超管一审通过\",\"expectedVersion\":$approval_version}"
  admin_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/approvalDecision" "$decision_body" "$WORK_DIR/$output_prefix-decision-one.json"
  approval_version="$(json_value "$WORK_DIR/$output_prefix-decision-one.json" data.approval.version)"
  jq -e '.data.approval.status == "pending" and .data.approval.approvalCount == 1' "$WORK_DIR/$output_prefix-decision-one.json" >/dev/null

  decision_body="{\"approvalId\":$approval_id,\"decision\":\"approve\",\"reason\":\"独立审批人二审通过\",\"expectedVersion\":$approval_version}"
  admin_post "$APPROVER_TOKEN" "/dashboard/saasAdmin/approvalDecision" "$decision_body" "$WORK_DIR/$output_prefix-decision-two.json"
  approval_version="$(json_value "$WORK_DIR/$output_prefix-decision-two.json" data.approval.version)"
  jq -e '.data.approval.status == "approved" and .data.approval.approvalCount == 2' "$WORK_DIR/$output_prefix-decision-two.json" >/dev/null

  execute_body="{\"approvalId\":$approval_id,\"expectedVersion\":$approval_version}"
  admin_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/approvalExecute" "$execute_body" "$WORK_DIR/$output_prefix-execute.json"
  jq -e --argjson account_id "$ACCOUNT_ID" '.data.approval.status == "executed" and .data.approval.effectOperationId > 0 and .data.result.account.id == $account_id and .data.result.operationId == .data.approval.effectOperationId' "$WORK_DIR/$output_prefix-execute.json" >/dev/null
  test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approvals WHERE id = $approval_id AND action_type = 'service_account.update' AND status = 'executed' AND approval_count = 2 AND effect_applied_at IS NOT NULL AND effect_operation_id > 0")" = "1"
}

approve_service_account_key_rotate() {
  local payload="$1" output_prefix="$2"
  local request_body approval_id approval_version decision_body execute_body plain_text_key
  request_body="{\"actionType\":\"service_account.key.rotate\",\"payload\":$payload,\"reason\":\"smoke 双人复核轮换服务账号 API Key\",\"idempotencyKey\":\"service-account-key-rotate-$output_prefix\"}"
  admin_post "$OPERATIONS_TOKEN" "/dashboard/saasAdmin/approvalRequest" "$request_body" "$WORK_DIR/$output_prefix-request.json"
  approval_id="$(json_value "$WORK_DIR/$output_prefix-request.json" data.approval.id)"
  approval_version="$(json_value "$WORK_DIR/$output_prefix-request.json" data.approval.version)"
  jq -e --argjson account_id "$ACCOUNT_ID" '.data.approval.actionType == "service_account.key.rotate" and .data.approval.requiredApprovals == 2 and .data.approval.status == "pending" and (.data.approval.targetId | tonumber) == $account_id' "$WORK_DIR/$output_prefix-request.json" >/dev/null
  test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approvals WHERE id = $approval_id AND JSON_EXTRACT(request_json, '$.key') IS NULL AND JSON_EXTRACT(request_json, '$.Key') IS NULL")" = "1"

  decision_body="{\"approvalId\":$approval_id,\"decision\":\"approve\",\"reason\":\"平台超管一审通过\",\"expectedVersion\":$approval_version}"
  admin_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/approvalDecision" "$decision_body" "$WORK_DIR/$output_prefix-decision-one.json"
  approval_version="$(json_value "$WORK_DIR/$output_prefix-decision-one.json" data.approval.version)"
  jq -e '.data.approval.status == "pending" and .data.approval.approvalCount == 1' "$WORK_DIR/$output_prefix-decision-one.json" >/dev/null

  decision_body="{\"approvalId\":$approval_id,\"decision\":\"approve\",\"reason\":\"独立审批人二审通过\",\"expectedVersion\":$approval_version}"
  admin_post "$APPROVER_TOKEN" "/dashboard/saasAdmin/approvalDecision" "$decision_body" "$WORK_DIR/$output_prefix-decision-two.json"
  approval_version="$(json_value "$WORK_DIR/$output_prefix-decision-two.json" data.approval.version)"
  jq -e '.data.approval.status == "approved" and .data.approval.approvalCount == 2' "$WORK_DIR/$output_prefix-decision-two.json" >/dev/null

  execute_body="{\"approvalId\":$approval_id,\"expectedVersion\":$approval_version}"
  admin_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/approvalExecute" "$execute_body" "$WORK_DIR/$output_prefix-execute.json"
  plain_text_key="$(json_value "$WORK_DIR/$output_prefix-execute.json" data.result.plainTextKey)"
  jq -e '.data.approval.status == "executed" and .data.approval.effectOperationId > 0 and (.data.result.plainTextKey | startswith("mch_live_")) and .data.result.key.status == "active" and .data.result.operationId == .data.approval.effectOperationId' "$WORK_DIR/$output_prefix-execute.json" >/dev/null
  grep -qi '^Cache-Control: no-store' "$WORK_DIR/$output_prefix-execute.json.headers"
  test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approvals WHERE id = $approval_id AND action_type = 'service_account.key.rotate' AND status = 'executed' AND approval_count = 2 AND effect_applied_at IS NOT NULL AND effect_operation_id > 0 AND JSON_EXTRACT(result_json, '$.plainTextKey') IS NULL AND JSON_EXTRACT(result_json, '$.plainTextKeyDelivered') = TRUE AND LOCATE('$plain_text_key', request_json) = 0 AND LOCATE('$plain_text_key', result_json) = 0")" = "1"
}

stop_go() {
  if [ -n "$GO_PID" ] && kill -0 "$GO_PID" 2>/dev/null; then
    kill "$GO_PID" 2>/dev/null || true
    wait "$GO_PID" 2>/dev/null || true
  fi
  GO_PID=""
}

start_go() {
  local allow_legacy="$1" run_on_start="${2:-0}"
  env -u GOROOT \
    MOCHAT_GO_STANDALONE=1 \
    MOCHAT_GO_ADDR="$GO_ADDR" \
    MOCHAT_MYSQL_DSN="$DSN" \
    MOCHAT_REDIS_ADDR="127.0.0.1:$REDIS_PORT" \
    MOCHAT_SIMPLE_JWT_SECRET="$SECRET" \
    MOCHAT_GO_MIGRATE_AUTH=1 \
    MOCHAT_GO_MIGRATE_LOGIN_SHOW=1 \
    MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1 \
    MOCHAT_GO_SAAS_ADMIN_APPROVAL_REQUIRED=1 \
    MOCHAT_GO_SAAS_PLATFORM_ADMIN_TENANT_ID=1 \
    MOCHAT_GO_SAAS_SERVICE_ACCOUNT_KEY_PEPPER="$PEPPER_KEY" \
    MOCHAT_GO_SAAS_SERVICE_ACCOUNT_KEY_PEPPER_ID="$PEPPER_ID" \
    MOCHAT_GO_SAAS_SERVICE_ACCOUNT_ALLOW_LEGACY_JWT_PEPPER="$allow_legacy" \
    MOCHAT_GO_SAAS_SERVICE_ACCOUNT_REQUIRE_DEDICATED_PEPPER=1 \
    MOCHAT_GO_SAAS_SERVICE_ACCOUNT_TRUST_PROXY_HEADERS=1 \
    MOCHAT_GO_SAAS_TRUSTED_PROXY_CIDRS=127.0.0.1/32 \
    MOCHAT_GO_ENABLE_SAAS_SERVICE_ACCOUNT_USAGE_ALERT_CRON=1 \
    MOCHAT_GO_SAAS_SERVICE_ACCOUNT_USAGE_ALERT_CRON_INTERVAL_SECONDS=3600 \
    MOCHAT_GO_SAAS_SERVICE_ACCOUNT_USAGE_ALERT_CRON_RUN_ON_START="$run_on_start" \
    MOCHAT_GO_SAAS_SERVICE_ACCOUNT_USAGE_ALERT_LIMIT=100 \
    MOCHAT_GO_ENABLE_SAAS_SERVICE_ACCOUNT_USAGE_CLEANUP_CRON=1 \
    MOCHAT_GO_SAAS_SERVICE_ACCOUNT_USAGE_CLEANUP_CRON_INTERVAL_SECONDS=3600 \
    MOCHAT_GO_SAAS_SERVICE_ACCOUNT_USAGE_CLEANUP_CRON_RUN_ON_START="$run_on_start" \
    MOCHAT_GO_SAAS_SERVICE_ACCOUNT_USAGE_CLEANUP_LIMIT=100 \
    "$GO_BIN" >>"$GO_LOG" 2>&1 &
  GO_PID="$!"
  wait_url "http://$GO_ADDR/readyz" 200
}

assert_port_free "$MYSQL_PORT"
assert_port_free "$REDIS_PORT"
assert_port_free "${GO_ADDR##*:}"

compose up -d mysql redis
wait_service_healthy mysql
wait_service_healthy redis
mysql_root <<SQL
DROP DATABASE IF EXISTS $DATABASE;
CREATE DATABASE $DATABASE CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
GRANT ALL PRIVILEGES ON $DATABASE.* TO 'mochat'@'%';
FLUSH PRIVILEGES;
SQL

env -u GOROOT go build -o "$MIGRATE_BIN" ./cmd/mochat-migrate
env -u GOROOT go build -o "$BOOTSTRAP_BIN" ./cmd/mochat-bootstrap
env -u GOROOT go build -o "$GO_BIN" ./cmd/mochat-go
env -u GOROOT go build -o "$MAINTENANCE_BIN" ./cmd/mochat-saas-maintenance
DSN="mochat:mochat_pass@tcp(127.0.0.1:$MYSQL_PORT)/$DATABASE?parseTime=true&loc=Local"
"$MIGRATE_BIN" -dsn "$DSN" -project-root "$PWD" -action apply >"$WORK_DIR/migrate.out"
grep -q $'0049_saas_service_accounts\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0060_saas_service_account_usage_retention\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0064_saas_audit_anchor_remote_immutability\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0068_wechat_open_credential_encryption\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0079_saas_service_account_key_revoke_guard\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0080_saas_service_account_update_guard\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0081_saas_service_account_key_rotate_guard\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0082_saas_service_account_create_guard\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0083_saas_identity_mfa_reset_guard\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0085_saas_tenant_package_assignment_guard\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0088_saas_subscription_transition_approval_guard\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0089_saas_invoice_issue_approval_guard\tapplied_now' "$WORK_DIR/migrate.out"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_schema_migrations")" = "98"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approval_policies WHERE action_type = 'service_account.create' AND enabled = 1 AND amount_threshold_cents = 0 AND required_approvals = 2")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approval_policies WHERE action_type = 'service_account.key.revoke' AND enabled = 1 AND amount_threshold_cents = 0 AND required_approvals = 2")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approval_policies WHERE action_type = 'service_account.update' AND enabled = 1 AND amount_threshold_cents = 0 AND required_approvals = 2")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approval_policies WHERE action_type = 'service_account.key.rotate' AND enabled = 1 AND amount_threshold_cents = 0 AND required_approvals = 2")" = "1"
test "$(mysql_scalar "SHOW TABLES LIKE 'mochat_go_saas_service_accounts'")" = "mochat_go_saas_service_accounts"
test "$(mysql_scalar "SHOW TABLES LIKE 'mochat_go_saas_service_account_keys'")" = "mochat_go_saas_service_account_keys"
test "$(mysql_scalar "SHOW TABLES LIKE 'mochat_go_saas_service_account_usage_daily'")" = "mochat_go_saas_service_account_usage_daily"
test "$(mysql_scalar "SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'mochat_go_saas_service_accounts' AND COLUMN_NAME IN ('rate_limit_per_minute','daily_request_limit','minute_window_started_at','minute_request_count','minute_rejected_count','daily_window_date','daily_request_count','daily_rejected_count')")" = "8"
test "$(mysql_scalar "SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'mochat_go_saas_service_accounts' AND COLUMN_NAME IN ('usage_alert_enabled','usage_warning_percent','rejection_warning_count','usage_alert_cooldown_minutes','usage_alert_last_evaluated_at','usage_alert_last_notified_at','rejection_alert_last_notified_at')")" = "7"
test "$(mysql_scalar "SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'mochat_go_saas_service_account_keys' AND COLUMN_NAME = 'hash_key_id' AND REPLACE(COLUMN_DEFAULT, CHAR(39), '') = 'legacy-jwt'")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'mochat_go_saas_service_account_keys' AND INDEX_NAME = 'idx_mochat_go_saas_service_account_key_hash_key'")" = "3"
test "$(mysql_scalar "SELECT COUNT(*) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'mochat_go_saas_service_accounts' AND INDEX_NAME = 'idx_mochat_go_saas_service_account_rate_window'")" = "3"
test "$(mysql_scalar "SELECT service_account_usage_retention_days FROM mochat_go_saas_compliance_policies WHERE id = 1")" = "90"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_role_permissions WHERE permission_code = 'platform.integrations.read'")" = "3"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_role_permissions WHERE permission_code = 'platform.integrations.manage'")" = "1"

cat >"$WORK_DIR/tenants.csv" <<'CSV'
tenant_id,tenant_name,phone,password,user_name,role_name,package_code,package_name,max_corps,max_users,max_contacts,max_rooms,max_agents,channel_codes,shop_codes,radars,lotteries,room_infinite_pulls,room_fissions,room_clock_ins,room_qualities,room_calendars,room_reminds,contact_sops,room_sops,sensitive_words,storage_mb,contact_message_batches,room_message_batches,room_tag_pulls,work_room_auto_pulls,work_fissions,official_accounts,async_executions,package_expires_at,config_copy_mode
1,服务账号平台,13800000001,secret001,平台超级管理员,SaaS平台超级管理员,platform,平台版,10,100,100000,2000,20,500,300,300,300,300,300,300,300,300,300,300,300,300,4096,300,300,300,300,300,50,100000,2037-01-01,missing
961,OpenAPI租户,13800000961,secret961,租户管理员,SaaS租户超级管理员,starter,基础版,1,5,500,20,2,10,10,10,10,10,10,10,10,10,10,10,10,10,256,10,10,10,10,10,1,500,2037-01-01,missing
CSV
"$BOOTSTRAP_BIN" -dsn "$DSN" -secret "$SECRET" -batch-file "$WORK_DIR/tenants.csv" >"$WORK_DIR/bootstrap.out"

mysql_root "$DATABASE" <<'SQL'
INSERT INTO mc_user (phone, password, name, gender, department, position, status, tenant_id, created_at, updated_at, isSuperAdmin)
SELECT '13800000002', password, '平台运营', 0, '运营部', '运营', 1, 1, NOW(), NOW(), 0 FROM mc_user WHERE phone = '13800000001' LIMIT 1;
INSERT INTO mc_user (phone, password, name, gender, department, position, status, tenant_id, created_at, updated_at, isSuperAdmin)
SELECT '13800000003', password, '平台审计', 0, '审计部', '审计', 1, 1, NOW(), NOW(), 0 FROM mc_user WHERE phone = '13800000001' LIMIT 1;
INSERT INTO mc_user (phone, password, name, gender, department, position, status, tenant_id, created_at, updated_at, isSuperAdmin)
SELECT '13800000004', password, '平台独立审批人', 0, '内控部', '审批', 1, 1, NOW(), NOW(), 0 FROM mc_user WHERE phone = '13800000001' LIMIT 1;
INSERT INTO mochat_go_saas_alerts
  (alert_key, tenant_id, alert_type, severity, status, metric, period_key, current_value, limit_value, occurrence_count, source, message, first_seen_at, last_seen_at)
VALUES
  ('service-account-tenant-alert', 961, 'quota_exceeded', 'warning', 'open', 'contacts', 'lifetime', 501, 500, 1, 'smoke', 'tenant alert', NOW(), NOW()),
  ('service-account-platform-alert', 1, 'quota_exceeded', 'warning', 'open', 'contacts', 'lifetime', 101, 100, 1, 'smoke', 'platform alert', NOW(), NOW());
SQL

OPERATIONS_ID="$(mysql_scalar "SELECT id FROM mc_user WHERE phone = '13800000002' ORDER BY id DESC LIMIT 1")"
AUDITOR_ID="$(mysql_scalar "SELECT id FROM mc_user WHERE phone = '13800000003' ORDER BY id DESC LIMIT 1")"
APPROVER_ID="$(mysql_scalar "SELECT id FROM mc_user WHERE phone = '13800000004' ORDER BY id DESC LIMIT 1")"
OPERATIONS_ROLE_ID="$(mysql_scalar "SELECT id FROM mochat_go_saas_admin_roles WHERE code = 'platform_operations'")"
AUDITOR_ROLE_ID="$(mysql_scalar "SELECT id FROM mochat_go_saas_admin_roles WHERE code = 'platform_auditor'")"
APPROVER_ROLE_ID="$(mysql_scalar "SELECT id FROM mochat_go_saas_admin_roles WHERE code = 'platform_approver'")"
mysql_root "$DATABASE" <<SQL
INSERT INTO mochat_go_saas_admin_user_access (user_id, version, updated_by, created_at, updated_at) VALUES ($OPERATIONS_ID, 1, 1, NOW(), NOW()), ($AUDITOR_ID, 1, 1, NOW(), NOW()), ($APPROVER_ID, 1, 1, NOW(), NOW());
INSERT INTO mochat_go_saas_admin_user_roles (user_id, role_id, assigned_by, created_at) VALUES ($OPERATIONS_ID, $OPERATIONS_ROLE_ID, 1, NOW()), ($AUDITOR_ID, $AUDITOR_ROLE_ID, 1, NOW()), ($APPROVER_ID, $APPROVER_ROLE_ID, 1, NOW());
SQL

: >"$GO_LOG"
start_go 1 1
wait_mysql_at_least "SELECT COUNT(*) FROM mochat_go_background_task_executions WHERE task_name = 'cron-saas-service-account-usage-alert' AND kind = 'periodic_tick' AND status = 'succeeded'" 1
wait_mysql_at_least "SELECT COUNT(*) FROM mochat_go_background_task_executions WHERE task_name = 'cron-saas-service-account-usage-cleanup' AND kind = 'periodic_tick' AND status = 'succeeded'" 1

PLATFORM_TOKEN="$(login_token 13800000001 secret001 "$WORK_DIR/platform-auth.json")"
OPERATIONS_TOKEN="$(login_token 13800000002 secret001 "$WORK_DIR/operations-auth.json")"
AUDITOR_TOKEN="$(login_token 13800000003 secret001 "$WORK_DIR/auditor-auth.json")"
APPROVER_TOKEN="$(login_token 13800000004 secret001 "$WORK_DIR/approver-auth.json")"
TENANT_TOKEN="$(login_token 13800000961 secret961 "$WORK_DIR/tenant-auth.json")"

admin_get "$PLATFORM_TOKEN" "/dashboard/saasAdmin/serviceAccounts" "$WORK_DIR/empty.json"
python3 - "$WORK_DIR/empty.json" "$PEPPER_ID" <<'PY'
import json, pathlib, sys
data = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))["data"]
assert data["items"] == [] and len(data["scopes"]) == 3, data
protection = data["keyProtection"]
assert protection["activeKeyId"] == sys.argv[2] and protection["dedicatedConfigured"] is True, protection
assert protection["healthy"] is True and protection["storedKeyCount"] == 0 and protection["legacyUsableKeyCount"] == 0, protection
client_ip = data["clientIPResolution"]
assert client_ip["trustProxyHeaders"] is True and client_ip["trustedProxyCidrCount"] == 1, client_ip
PY
admin_get "$TENANT_TOKEN" "/dashboard/saasAdmin/serviceAccounts" "$WORK_DIR/tenant-forbidden.json" 403
admin_get "$AUDITOR_TOKEN" "/dashboard/saasAdmin/serviceAccounts" "$WORK_DIR/auditor-empty.json"
admin_get "$PLATFORM_TOKEN" "/dashboard/saasAdmin/serviceAccountUsage?days=7" "$WORK_DIR/usage-empty.json"
python3 - "$WORK_DIR/usage-empty.json" <<'PY'
import json, pathlib, sys
data = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))["data"]
assert data["retentionDays"] == 90 and data["requestCount"] == 0 and data["rejectedCount"] == 0, data
assert len(data["daily"]) == 7 and data["routes"] == [] and data["accounts"] == [], data
PY
admin_get "$TENANT_TOKEN" "/dashboard/saasAdmin/serviceAccountUsage?days=7" "$WORK_DIR/tenant-usage-forbidden.json" 403
admin_get "$AUDITOR_TOKEN" "/dashboard/saasAdmin/serviceAccountUsage?days=7" "$WORK_DIR/auditor-usage-empty.json"
admin_post "$AUDITOR_TOKEN" "/dashboard/saasAdmin/serviceAccount" '{}' "$WORK_DIR/auditor-create-forbidden.json" 403

CREATE_BODY='{"tenantId":961,"code":"data_sync","name":"数据同步","description":"客户数据仓库只读集成","status":"active","scopes":["tenant.profile.read","tenant.usage.read","tenant.alerts.read"],"allowedCidrs":["127.0.0.1"],"rateLimitPerMinute":100,"dailyRequestLimit":1000,"usageAlertEnabled":true,"usageWarningPercent":50,"rejectionWarningCount":1,"usageAlertCooldownMinutes":5,"expiresAt":"2037-01-01 00:00:00","keyName":"生产主密钥","keyExpiresAt":"2036-01-01 00:00:00"}'
admin_post "$OPERATIONS_TOKEN" "/dashboard/saasAdmin/serviceAccount" "$CREATE_BODY" "$WORK_DIR/create-direct.json" 428
jq -e '.data.actionType == "service_account.create" and .data.requiredApprovals == 2 and .data.amountThresholdCents == 0' "$WORK_DIR/create-direct.json" >/dev/null
approve_service_account_create "$CREATE_BODY" "create"
ACCOUNT_ID="$(json_value "$WORK_DIR/create-execute.json" data.result.account.id)"
ACCOUNT_VERSION="$(json_value "$WORK_DIR/create-execute.json" data.result.account.version)"
KEY_ID="$(json_value "$WORK_DIR/create-execute.json" data.result.key.id)"
KEY_VERSION="$(json_value "$WORK_DIR/create-execute.json" data.result.key.version)"
KEY_PREFIX="$(json_value "$WORK_DIR/create-execute.json" data.result.key.prefix)"
API_KEY="$(json_value "$WORK_DIR/create-execute.json" data.result.plainTextKey)"
python3 - "$WORK_DIR/create-execute.json" "$PEPPER_ID" <<'PY'
import json, pathlib, re, sys
payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
assert payload["code"] == 200, payload
data = payload["data"]["result"]
assert re.fullmatch(r"mch_live_[a-f0-9]{12}_[A-Za-z0-9_-]{43}", data["plainTextKey"]), data
assert data["account"]["tenantId"] == 961 and data["account"]["version"] == 1, data
assert data["account"]["allowedCidrs"] == ["127.0.0.1/32"], data
assert data["account"]["rateLimitPerMinute"] == 100 and data["account"]["dailyRequestLimit"] == 1000, data
assert data["account"]["usageAlertEnabled"] is True and data["account"]["usageWarningPercent"] == 50, data
assert data["account"]["rejectionWarningCount"] == 1 and data["account"]["usageAlertCooldownMinutes"] == 5, data
assert len(data["account"]["keys"]) == 1 and data["account"]["keys"][0]["status"] == "active", data
assert data["account"]["keys"][0]["hashKeyId"] == sys.argv[2] and data["account"]["keys"][0]["legacyPepper"] is False, data
assert data["key"]["id"] == data["account"]["keys"][0]["id"] and data["initialKey"] is True, data
assert "keyHash" not in pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"), data
PY
test "$ACCOUNT_VERSION" = "1"
test "$KEY_VERSION" = "1"
test "$(mysql_scalar "SELECT CHAR_LENGTH(key_hash) FROM mochat_go_saas_service_account_keys WHERE id = $KEY_ID")" = "64"
test "$(mysql_scalar "SELECT hash_key_id FROM mochat_go_saas_service_account_keys WHERE id = $KEY_ID")" = "$PEPPER_ID"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_service_account_keys WHERE key_prefix = '$KEY_PREFIX' AND key_hash <> '$API_KEY'")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_operation_logs WHERE after_json LIKE '%$API_KEY%' OR before_json LIKE '%$API_KEY%'")" = "0"

DEDICATED_API_KEY="$API_KEY"
key_get "$DEDICATED_API_KEY" "/api/saas/v1/whoami" "$WORK_DIR/dedicated-whoami.json"
LEGACY_API_KEY="$(python3 - "$KEY_PREFIX" <<'PY'
import sys
print("mch_live_" + sys.argv[1] + "_" + "L" * 43)
PY
)"
LEGACY_HASH="$(python3 - "$SECRET" "$LEGACY_API_KEY" <<'PY'
import hashlib, hmac, sys
print(hmac.new(sys.argv[1].encode(), sys.argv[2].encode(), hashlib.sha256).hexdigest())
PY
)"
mysql_scalar "UPDATE mochat_go_saas_service_account_keys SET key_hash = '$LEGACY_HASH', hash_key_id = 'legacy-jwt', last_four = 'LLLL' WHERE id = $KEY_ID" >/dev/null
API_KEY="$LEGACY_API_KEY"
key_get "$API_KEY" "/api/saas/v1/whoami" "$WORK_DIR/whoami.json"
python3 - "$WORK_DIR/whoami.json" <<'PY'
import json, pathlib, sys
data = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))["data"]
assert data["tenantId"] == 961 and data["serviceAccountCode"] == "data_sync", data
assert sorted(data["scopes"]) == ["tenant.alerts.read", "tenant.profile.read", "tenant.usage.read"], data
PY
key_get "$DEDICATED_API_KEY" "/api/saas/v1/whoami" "$WORK_DIR/replaced-dedicated-key.json" 401
admin_get "$PLATFORM_TOKEN" "/dashboard/saasAdmin/serviceAccounts?tenantId=961" "$WORK_DIR/legacy-protection.json"
jq -e --arg key_id "$PEPPER_ID" '.data.keyProtection.activeKeyId == $key_id and .data.keyProtection.dedicatedConfigured == true and .data.keyProtection.legacyUsableKeyCount == 1 and .data.keyProtection.missingKeyIds == [] and .data.keyProtection.healthy == true' "$WORK_DIR/legacy-protection.json" >/dev/null

stop_go
start_go 0 0
key_get "$API_KEY" "/api/saas/v1/whoami" "$WORK_DIR/legacy-disabled.json" 401
admin_get "$PLATFORM_TOKEN" "/dashboard/saasAdmin/serviceAccounts?tenantId=961" "$WORK_DIR/legacy-disabled-protection.json"
jq -e '.data.keyProtection.legacyJwtEnabled == false and .data.keyProtection.missingKeyIds == ["legacy-jwt"] and .data.keyProtection.healthy == false' "$WORK_DIR/legacy-disabled-protection.json" >/dev/null
admin_get "$PLATFORM_TOKEN" "/dashboard/saasAdmin/systemHealth" "$WORK_DIR/legacy-disabled-health.json"
jq -e '[.data.checks[] | select(.code == "service_account_key_protection")][0] | .status == "critical" and (.detail | contains("legacy-jwt"))' "$WORK_DIR/legacy-disabled-health.json" >/dev/null
stop_go
start_go 1 0
key_get "$API_KEY" "/api/saas/v1/whoami" "$WORK_DIR/legacy-restored.json"
key_get "$API_KEY" "/api/saas/v1/usage" "$WORK_DIR/usage.json"
python3 - "$WORK_DIR/usage.json" <<'PY'
import json, pathlib, sys
data = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))["data"]
assert data["tenant"]["tenantId"] == 961 and len(data["metrics"]) > 0, data
PY
key_get "$API_KEY" "/api/saas/v1/alerts?status=open" "$WORK_DIR/alerts.json"
python3 - "$WORK_DIR/alerts.json" <<'PY'
import json, pathlib, sys
data = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))["data"]
assert data["total"] == 1 and len(data["items"]) == 1 and data["items"][0]["tenantId"] == 961, data
PY
key_get "${API_KEY%?}x" "/api/saas/v1/whoami" "$WORK_DIR/wrong-key.json" 401
key_get "" "/api/saas/v1/whoami" "$WORK_DIR/missing-key.json" 401

WHOAMI_USAGE_BEFORE_RATE="$(mysql_scalar "SELECT COALESCE(SUM(request_count), 0) FROM mochat_go_saas_service_account_usage_daily WHERE service_account_id = $ACCOUNT_ID AND usage_date = CURDATE() AND route_key = 'GET /api/saas/v1/whoami'")"
WHOAMI_REJECTED_BEFORE_RATE="$(mysql_scalar "SELECT COALESCE(SUM(rejected_count), 0) FROM mochat_go_saas_service_account_usage_daily WHERE service_account_id = $ACCOUNT_ID AND usage_date = CURDATE() AND route_key = 'GET /api/saas/v1/whoami'")"
mysql_root "$DATABASE" <<SQL
UPDATE mochat_go_saas_service_accounts
SET rate_limit_per_minute = 2, daily_request_limit = 4,
    minute_window_started_at = NULL, minute_request_count = 0, minute_rejected_count = 0,
    daily_window_date = NULL, daily_request_count = 0, daily_rejected_count = 0
WHERE id = $ACCOUNT_ID;
SQL
key_get "$API_KEY" "/api/saas/v1/whoami" "$WORK_DIR/rate-first.json"
grep -qi '^X-RateLimit-Limit: 2' "$WORK_DIR/rate-first.json.headers"
grep -qi '^X-RateLimit-Remaining: 1' "$WORK_DIR/rate-first.json.headers"
grep -qi '^X-RateLimit-Daily-Limit: 4' "$WORK_DIR/rate-first.json.headers"
key_get "$API_KEY" "/api/saas/v1/whoami" "$WORK_DIR/rate-second.json"
grep -qi '^X-RateLimit-Remaining: 0' "$WORK_DIR/rate-second.json.headers"
key_get "$API_KEY" "/api/saas/v1/whoami" "$WORK_DIR/rate-rejected.json" 429
grep -qi '^Retry-After: [1-9][0-9]*' "$WORK_DIR/rate-rejected.json.headers"
grep -qi '^X-RateLimit-Remaining: 0' "$WORK_DIR/rate-rejected.json.headers"
python3 - "$WORK_DIR/rate-rejected.json" <<'PY'
import json, pathlib, sys
payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
assert payload["code"] == 429 and payload["data"]["rateLimit"]["limitedBy"] == "minute", payload
assert payload["data"]["rateLimit"]["minuteRequestCount"] == 2, payload
PY
test "$(mysql_scalar "SELECT CONCAT(minute_request_count, ':', minute_rejected_count, ':', daily_request_count, ':', daily_rejected_count) FROM mochat_go_saas_service_accounts WHERE id = $ACCOUNT_ID")" = "2:1:2:1"
WHOAMI_USAGE_AFTER_RATE="$((WHOAMI_USAGE_BEFORE_RATE + 2))"
WHOAMI_REJECTED_AFTER_RATE="$((WHOAMI_REJECTED_BEFORE_RATE + 1))"
test "$(mysql_scalar "SELECT CONCAT(request_count, ':', rejected_count) FROM mochat_go_saas_service_account_usage_daily WHERE service_account_id = $ACCOUNT_ID AND usage_date = CURDATE() AND route_key = 'GET /api/saas/v1/whoami'")" = "$WHOAMI_USAGE_AFTER_RATE:$WHOAMI_REJECTED_AFTER_RATE"
ALERT_EVALUATE_BODY="{\"tenantId\":961,\"serviceAccountId\":$ACCOUNT_ID,\"limit\":10}"
admin_post "$TENANT_TOKEN" "/dashboard/saasAdmin/serviceAccountUsageAlertEvaluate" "$ALERT_EVALUATE_BODY" "$WORK_DIR/tenant-alert-evaluate-forbidden.json" 403
admin_post "$AUDITOR_TOKEN" "/dashboard/saasAdmin/serviceAccountUsageAlertEvaluate" "$ALERT_EVALUATE_BODY" "$WORK_DIR/auditor-alert-evaluate-forbidden.json" 403
admin_post "$OPERATIONS_TOKEN" "/dashboard/saasAdmin/serviceAccountUsageAlertEvaluate" "$ALERT_EVALUATE_BODY" "$WORK_DIR/alert-evaluate.json"
jq -e '.data.scannedAccounts == 1 and .data.eligibleAccounts == 1 and .data.usageWarningAccounts == 1 and .data.rejectionWarningAccounts == 1 and .data.notificationsQueued == 2 and .data.operationId > 0' "$WORK_DIR/alert-evaluate.json" >/dev/null
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_alerts WHERE tenant_id = 961 AND metric = 'service_account_usage' AND alert_type IN ('service_account_usage_warning','service_account_rejection_warning') AND period_key = CONCAT('account:', $ACCOUNT_ID) AND status = 'open'")" = "2"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_alert_notifications WHERE tenant_id = 961 AND alert_key LIKE '%:service_account_usage:%' AND status = 'pending'")" = "2"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_alert_notifications WHERE CAST(alert_json AS CHAR) LIKE '%mch_live_%'")" = "0"
admin_get "$OPERATIONS_TOKEN" "/dashboard/saasAdmin/alerts?tenantId=961&status=open&metric=service_account_usage&alertType=service_account_usage_warning&limit=10" "$WORK_DIR/usage-alert-list.json"
jq -e '.data.summary.openCount == 1 and (.data.alerts | length) == 1 and .data.alerts[0].context.serviceAccountId == '"$ACCOUNT_ID" "$WORK_DIR/usage-alert-list.json" >/dev/null
admin_post "$OPERATIONS_TOKEN" "/dashboard/saasAdmin/serviceAccountUsageAlertEvaluate" "$ALERT_EVALUATE_BODY" "$WORK_DIR/alert-evaluate-cooldown.json"
jq -e '.data.usageWarningAccounts == 1 and .data.rejectionWarningAccounts == 1 and .data.notificationsQueued == 0' "$WORK_DIR/alert-evaluate-cooldown.json" >/dev/null
mysql_scalar "UPDATE mochat_go_saas_service_accounts SET usage_alert_last_notified_at = DATE_SUB(NOW(), INTERVAL 6 MINUTE), rejection_alert_last_notified_at = DATE_SUB(NOW(), INTERVAL 6 MINUTE) WHERE id = $ACCOUNT_ID" >/dev/null
"$MAINTENANCE_BIN" -dsn "$DSN" -action evaluate-service-account-usage-alerts -tenant-id 961 -service-account-id "$ACCOUNT_ID" -service-account-usage-alert-limit 10 >"$WORK_DIR/usage-alert-maintenance.out"
grep -q $'action\tevaluate-service-account-usage-alerts' "$WORK_DIR/usage-alert-maintenance.out"
grep -q $'usage_warning_accounts\t1' "$WORK_DIR/usage-alert-maintenance.out"
grep -q $'rejection_warning_accounts\t1' "$WORK_DIR/usage-alert-maintenance.out"
grep -q $'notifications_queued\t2' "$WORK_DIR/usage-alert-maintenance.out"
grep -q $'notifications_closed\t0' "$WORK_DIR/usage-alert-maintenance.out"
mysql_root "$DATABASE" <<SQL
INSERT INTO mochat_go_saas_service_account_usage_daily
  (service_account_id, usage_date, route_key, request_count, rejected_count, last_used_at, last_used_ip, created_at, updated_at)
VALUES
  ($ACCOUNT_ID, DATE_SUB(CURDATE(), INTERVAL 6 DAY), 'GET /api/saas/v1/whoami', 7, 2, DATE_SUB(NOW(), INTERVAL 6 DAY), '127.0.0.1', NOW(), NOW());
SQL
admin_get "$PLATFORM_TOKEN" "/dashboard/saasAdmin/serviceAccountUsage?days=7&tenantId=961&serviceAccountId=$ACCOUNT_ID&limit=10" "$WORK_DIR/usage-history.json"
python3 - "$WORK_DIR/usage-history.json" "$((WHOAMI_USAGE_AFTER_RATE + 7))" "$((WHOAMI_REJECTED_AFTER_RATE + 2))" <<'PY'
import json, pathlib, sys
text = pathlib.Path(sys.argv[1]).read_text(encoding="utf-8")
data = json.loads(text)["data"]
assert data["retentionDays"] == 90 and len(data["daily"]) == 7, data
assert data["requestCount"] >= 10 and data["rejectedCount"] >= 3, data
assert data["activeAccountCount"] == 1 and data["limitedAccountCount"] == 1, data
assert data["openAlertCount"] == 2, data
assert len(data["accounts"]) == 1 and data["accounts"][0]["serviceAccountCode"] == "data_sync", data
assert any(item["routeKey"] == "GET /api/saas/v1/whoami" and item["requestCount"] == int(sys.argv[2]) and item["rejectedCount"] == int(sys.argv[3]) for item in data["routes"]), data
assert data["daily"][0]["requestCount"] == 7 and data["daily"][0]["rejectedCount"] == 2, data
assert "keyHash" not in text and "plainTextKey" not in text, text
PY
admin_get "$PLATFORM_TOKEN" "/dashboard/saasAdmin/serviceAccountUsage?days=91" "$WORK_DIR/usage-days-invalid.json" 400

mysql_scalar "UPDATE mochat_go_saas_service_accounts SET daily_request_count = 0, daily_rejected_count = 0 WHERE id = $ACCOUNT_ID" >/dev/null
admin_post "$OPERATIONS_TOKEN" "/dashboard/saasAdmin/serviceAccountUsageAlertEvaluate" "$ALERT_EVALUATE_BODY" "$WORK_DIR/alert-evaluate-resolve.json"
jq -e '.data.resolvedAlerts == 2 and .data.usageWarningAccounts == 0 and .data.rejectionWarningAccounts == 0 and .data.notificationsQueued == 0 and .data.notificationsClosed == 2' "$WORK_DIR/alert-evaluate-resolve.json" >/dev/null
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_alerts WHERE tenant_id = 961 AND metric = 'service_account_usage' AND status = 'resolved'")" = "2"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_alert_notifications WHERE tenant_id = 961 AND alert_key LIKE '%:service_account_usage:%' AND status = 'closed' AND next_retry_at IS NULL AND last_error = 'alert condition recovered before delivery'")" = "2"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_service_accounts WHERE id = $ACCOUNT_ID AND usage_alert_last_evaluated_at IS NOT NULL AND usage_alert_last_notified_at IS NULL AND rejection_alert_last_notified_at IS NULL")" = "1"
mysql_scalar "UPDATE mochat_go_saas_service_accounts SET daily_request_count = 2, daily_rejected_count = 1 WHERE id = $ACCOUNT_ID" >/dev/null

mysql_root "$DATABASE" <<SQL
INSERT INTO mochat_go_saas_service_account_usage_daily
  (service_account_id, usage_date, route_key, request_count, rejected_count, last_used_at, last_used_ip, created_at, updated_at)
VALUES
  ($ACCOUNT_ID, DATE_SUB(CURDATE(), INTERVAL 120 DAY), 'GET /api/saas/v1/usage', 4, 0, DATE_SUB(NOW(), INTERVAL 120 DAY), '127.0.0.1', NOW(), NOW());
SQL
"$MAINTENANCE_BIN" -dsn "$DSN" -action cleanup-service-account-usage -service-account-usage-cleanup-limit 10 >"$WORK_DIR/usage-cleanup.out"
grep -q $'action\tcleanup-service-account-usage' "$WORK_DIR/usage-cleanup.out"
grep -q $'retention_days\t90' "$WORK_DIR/usage-cleanup.out"
grep -q $'deleted_rows\t1' "$WORK_DIR/usage-cleanup.out"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_service_account_usage_daily WHERE service_account_id = $ACCOUNT_ID AND usage_date = DATE_SUB(CURDATE(), INTERVAL 120 DAY)")" = "0"

mysql_root "$DATABASE" <<SQL
INSERT INTO mochat_go_saas_service_account_usage_daily
  (service_account_id, usage_date, route_key, request_count, rejected_count, last_used_at, last_used_ip, created_at, updated_at)
VALUES
  ($ACCOUNT_ID, DATE_SUB(CURDATE(), INTERVAL 121 DAY), 'GET /api/saas/v1/alerts', 5, 0, DATE_SUB(NOW(), INTERVAL 121 DAY), '127.0.0.1', NOW(), NOW());
INSERT INTO mochat_go_saas_legal_holds
  (hold_no, tenant_id, status, active_slot, reason, starts_at, version, created_by, updated_by, created_at, updated_at)
VALUES
  ('service-account-usage-hold', 961, 'active', 961, '保全 OpenAPI 调用记录', DATE_SUB(NOW(), INTERVAL 1 DAY), 1, 1, 1, NOW(), NOW());
SQL
"$MAINTENANCE_BIN" -dsn "$DSN" -action cleanup-service-account-usage -service-account-usage-cleanup-limit 10 >"$WORK_DIR/usage-cleanup-held.out"
grep -q $'protected_rows\t1' "$WORK_DIR/usage-cleanup-held.out"
grep -q $'deleted_rows\t0' "$WORK_DIR/usage-cleanup-held.out"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_service_account_usage_daily WHERE service_account_id = $ACCOUNT_ID AND usage_date = DATE_SUB(CURDATE(), INTERVAL 121 DAY)")" = "1"
mysql_scalar "UPDATE mochat_go_saas_service_accounts SET rate_limit_per_minute = 100, daily_request_limit = 900, usage_alert_last_evaluated_at = NOW(), usage_alert_last_notified_at = NOW() WHERE id = $ACCOUNT_ID" >/dev/null

INVALID_RATE="{\"id\":$ACCOUNT_ID,\"name\":\"数据同步\",\"description\":\"非法限额\",\"status\":\"active\",\"scopes\":[\"tenant.profile.read\"],\"allowedCidrs\":[\"127.0.0.1/32\"],\"rateLimitPerMinute\":0,\"dailyRequestLimit\":1000,\"expiresAt\":\"2037-01-01 00:00:00\",\"expectedVersion\":1}"
admin_put "$OPERATIONS_TOKEN" "/dashboard/saasAdmin/serviceAccount" "$INVALID_RATE" "$WORK_DIR/invalid-rate.json" 400
INVALID_ALERT_POLICY="{\"id\":$ACCOUNT_ID,\"name\":\"数据同步\",\"description\":\"非法预警\",\"status\":\"active\",\"scopes\":[\"tenant.profile.read\"],\"allowedCidrs\":[\"127.0.0.1/32\"],\"rateLimitPerMinute\":100,\"dailyRequestLimit\":1000,\"usageWarningPercent\":0,\"usageAlertCooldownMinutes\":4,\"expiresAt\":\"2037-01-01 00:00:00\",\"expectedVersion\":1}"
admin_put "$OPERATIONS_TOKEN" "/dashboard/saasAdmin/serviceAccount" "$INVALID_ALERT_POLICY" "$WORK_DIR/invalid-alert-policy.json" 400

UPDATE_PROFILE_ONLY="{\"id\":$ACCOUNT_ID,\"name\":\"数据同步\",\"description\":\"仅身份读取\",\"status\":\"active\",\"scopes\":[\"tenant.profile.read\"],\"allowedCidrs\":[\"127.0.0.1/32\"],\"rateLimitPerMinute\":100,\"dailyRequestLimit\":1000,\"expiresAt\":\"2037-01-01 00:00:00\",\"expectedVersion\":1}"
admin_put "$OPERATIONS_TOKEN" "/dashboard/saasAdmin/serviceAccount" "$UPDATE_PROFILE_ONLY" "$WORK_DIR/update-profile-only-direct.json" 428
jq -e '.code == 428 and .data.actionType == "service_account.update" and .data.requiredApprovals == 2' "$WORK_DIR/update-profile-only-direct.json" >/dev/null
approve_service_account_update "$UPDATE_PROFILE_ONLY" "update-profile-only"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_service_accounts WHERE id = $ACCOUNT_ID AND daily_request_limit = 1000 AND usage_alert_last_evaluated_at IS NULL AND usage_alert_last_notified_at IS NULL")" = "1"
key_get "$API_KEY" "/api/saas/v1/whoami" "$WORK_DIR/profile-only-whoami.json"
key_get "$API_KEY" "/api/saas/v1/usage" "$WORK_DIR/profile-only-usage.json" 403

UPDATE_IP_DENY="{\"id\":$ACCOUNT_ID,\"name\":\"数据同步\",\"description\":\"IP 限制\",\"status\":\"active\",\"scopes\":[\"tenant.profile.read\",\"tenant.usage.read\",\"tenant.alerts.read\"],\"allowedCidrs\":[\"10.0.0.0/8\"],\"rateLimitPerMinute\":100,\"dailyRequestLimit\":1000,\"expiresAt\":\"2037-01-01 00:00:00\",\"expectedVersion\":2}"
admin_put "$OPERATIONS_TOKEN" "/dashboard/saasAdmin/serviceAccount" "$UPDATE_IP_DENY" "$WORK_DIR/update-ip-deny-direct.json" 428
approve_service_account_update "$UPDATE_IP_DENY" "update-ip-deny"
key_get "$API_KEY" "/api/saas/v1/whoami" "$WORK_DIR/ip-denied.json" 403
key_get "$API_KEY" "/api/saas/v1/whoami" "$WORK_DIR/ip-forwarded-allowed.json" 200 "10.23.45.67"
test "$(mysql_scalar "SELECT last_used_ip FROM mochat_go_saas_service_accounts WHERE id = $ACCOUNT_ID")" = "10.23.45.67"

UPDATE_RESTORE="{\"id\":$ACCOUNT_ID,\"name\":\"数据同步\",\"description\":\"恢复访问\",\"status\":\"active\",\"scopes\":[\"tenant.profile.read\",\"tenant.usage.read\",\"tenant.alerts.read\"],\"allowedCidrs\":[\"127.0.0.1/32\"],\"rateLimitPerMinute\":100,\"dailyRequestLimit\":1000,\"expiresAt\":\"2037-01-01 00:00:00\",\"expectedVersion\":3}"
admin_put "$OPERATIONS_TOKEN" "/dashboard/saasAdmin/serviceAccount" "$UPDATE_RESTORE" "$WORK_DIR/update-restore-direct.json" 428
approve_service_account_update "$UPDATE_RESTORE" "update-restore"
STALE_UPDATE_REQUEST="{\"actionType\":\"service_account.update\",\"payload\":$UPDATE_PROFILE_ONLY,\"reason\":\"旧版本服务账号变更\"}"
admin_post "$OPERATIONS_TOKEN" "/dashboard/saasAdmin/approvalRequest" "$STALE_UPDATE_REQUEST" "$WORK_DIR/stale-account-version.json" 409
jq -e '.code == 409 and (.msg | contains("版本已变化"))' "$WORK_DIR/stale-account-version.json" >/dev/null

ROTATE_BODY="{\"serviceAccountId\":$ACCOUNT_ID,\"expectedVersion\":4,\"name\":\"2026-Q3\",\"expiresAt\":\"2036-01-01 00:00:00\",\"graceMinutes\":60}"
admin_post "$OPERATIONS_TOKEN" "/dashboard/saasAdmin/serviceAccountKeyRotate" "$ROTATE_BODY" "$WORK_DIR/rotate-direct.json" 428
jq -e '.code == 428 and .data.actionType == "service_account.key.rotate" and .data.requiredApprovals == 2' "$WORK_DIR/rotate-direct.json" >/dev/null
approve_service_account_key_rotate "$ROTATE_BODY" "rotate"
ROTATED_KEY="$(json_value "$WORK_DIR/rotate-execute.json" data.result.plainTextKey)"
ROTATED_KEY_ID="$(json_value "$WORK_DIR/rotate-execute.json" data.result.key.id)"
test "$(json_value "$WORK_DIR/rotate-execute.json" data.result.key.hashKeyId)" = "$PEPPER_ID"
test "$(json_value "$WORK_DIR/rotate-execute.json" data.result.account.version)" = "5"
test "$(json_value "$WORK_DIR/rotate-execute.json" data.result.retiringKeys)" = "1"
test "$(mysql_scalar "SELECT status FROM mochat_go_saas_service_account_keys WHERE id = $KEY_ID")" = "retiring"
key_get "$API_KEY" "/api/saas/v1/whoami" "$WORK_DIR/grace-old-key.json"
key_get "$ROTATED_KEY" "/api/saas/v1/whoami" "$WORK_DIR/rotated-key.json"

ROTATE_NOW_BODY="{\"serviceAccountId\":$ACCOUNT_ID,\"expectedVersion\":5,\"name\":\"立即切换\",\"expiresAt\":\"2036-01-01 00:00:00\",\"graceMinutes\":0}"
admin_post "$OPERATIONS_TOKEN" "/dashboard/saasAdmin/serviceAccountKeyRotate" "$ROTATE_NOW_BODY" "$WORK_DIR/rotate-now-direct.json" 428
approve_service_account_key_rotate "$ROTATE_NOW_BODY" "rotate-now"
LATEST_KEY="$(json_value "$WORK_DIR/rotate-now-execute.json" data.result.plainTextKey)"
LATEST_KEY_ID="$(json_value "$WORK_DIR/rotate-now-execute.json" data.result.key.id)"
LATEST_KEY_VERSION="$(json_value "$WORK_DIR/rotate-now-execute.json" data.result.key.version)"
test "$(json_value "$WORK_DIR/rotate-now-execute.json" data.result.key.hashKeyId)" = "$PEPPER_ID"
key_get "$ROTATED_KEY" "/api/saas/v1/whoami" "$WORK_DIR/retired-key.json" 401
key_get "$LATEST_KEY" "/api/saas/v1/whoami" "$WORK_DIR/latest-key.json"

admin_post "$AUDITOR_TOKEN" "/dashboard/saasAdmin/serviceAccountKeyRotate" "$ROTATE_NOW_BODY" "$WORK_DIR/auditor-rotate-forbidden.json" 403
REVOKE_LATEST="{\"serviceAccountId\":$ACCOUNT_ID,\"keyId\":$LATEST_KEY_ID,\"expectedVersion\":$LATEST_KEY_VERSION}"
admin_post "$OPERATIONS_TOKEN" "/dashboard/saasAdmin/serviceAccountKeyRevoke" "$REVOKE_LATEST" "$WORK_DIR/revoke-latest-direct.json" 428
jq -e '.code == 428 and .data.actionType == "service_account.key.revoke" and .data.requiredApprovals == 2' "$WORK_DIR/revoke-latest-direct.json" >/dev/null
approve_service_account_key_revoke "$LATEST_KEY_ID" "$LATEST_KEY_VERSION" "revoke-latest"
key_get "$LATEST_KEY" "/api/saas/v1/whoami" "$WORK_DIR/revoked-latest.json" 401

DISABLE_BODY="{\"id\":$ACCOUNT_ID,\"name\":\"数据同步\",\"description\":\"停用验证\",\"status\":\"disabled\",\"scopes\":[\"tenant.profile.read\",\"tenant.usage.read\",\"tenant.alerts.read\"],\"allowedCidrs\":[\"127.0.0.1/32\"],\"rateLimitPerMinute\":100,\"dailyRequestLimit\":1000,\"expiresAt\":\"2037-01-01 00:00:00\",\"expectedVersion\":7}"
admin_put "$OPERATIONS_TOKEN" "/dashboard/saasAdmin/serviceAccount" "$DISABLE_BODY" "$WORK_DIR/disable-direct.json" 428
approve_service_account_update "$DISABLE_BODY" "disable"
key_get "$API_KEY" "/api/saas/v1/whoami" "$WORK_DIR/disabled-account.json" 401

ENABLE_BODY="{\"id\":$ACCOUNT_ID,\"name\":\"数据同步\",\"description\":\"恢复验证\",\"status\":\"active\",\"scopes\":[\"tenant.profile.read\",\"tenant.usage.read\",\"tenant.alerts.read\"],\"allowedCidrs\":[\"127.0.0.1/32\"],\"rateLimitPerMinute\":100,\"dailyRequestLimit\":1000,\"expiresAt\":\"2037-01-01 00:00:00\",\"expectedVersion\":8}"
admin_put "$OPERATIONS_TOKEN" "/dashboard/saasAdmin/serviceAccount" "$ENABLE_BODY" "$WORK_DIR/enable-direct.json" 428
approve_service_account_update "$ENABLE_BODY" "enable"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_service_accounts WHERE id = $ACCOUNT_ID AND usage_alert_last_evaluated_at IS NULL")" = "1"
key_get "$API_KEY" "/api/saas/v1/whoami" "$WORK_DIR/re-enabled-old-key.json"
admin_post "$OPERATIONS_TOKEN" "/dashboard/saasAdmin/serviceAccountUsageAlertEvaluate" "$ALERT_EVALUATE_BODY" "$WORK_DIR/alert-evaluate-reenabled.json"
jq -e '.data.scannedAccounts == 1 and .data.usageWarningAccounts == 0 and .data.rejectionWarningAccounts == 1 and .data.notificationsQueued == 1' "$WORK_DIR/alert-evaluate-reenabled.json" >/dev/null
REVOKE_INITIAL="{\"serviceAccountId\":$ACCOUNT_ID,\"keyId\":$KEY_ID,\"expectedVersion\":2}"
admin_post "$OPERATIONS_TOKEN" "/dashboard/saasAdmin/serviceAccountKeyRevoke" "$REVOKE_INITIAL" "$WORK_DIR/revoke-initial-direct.json" 428
approve_service_account_key_revoke "$KEY_ID" "2" "revoke-initial"
key_get "$API_KEY" "/api/saas/v1/whoami" "$WORK_DIR/revoked-initial.json" 401

admin_get "$PLATFORM_TOKEN" "/dashboard/saasAdmin/serviceAccounts?tenantId=961&keyword=data_sync" "$WORK_DIR/final-list.json"
python3 - "$WORK_DIR/final-list.json" <<'PY'
import json, pathlib, sys
text = pathlib.Path(sys.argv[1]).read_text(encoding="utf-8")
data = json.loads(text)["data"]
assert len(data["items"]) == 1 and data["items"][0]["tenantId"] == 961, data
assert data["summary"]["accountCount"] == 1 and data["summary"]["keyCount"] == 3, data
account = data["items"][0]
assert account["rateLimitPerMinute"] == 100 and account["dailyRequestLimit"] == 1000, account
assert account["usageAlertEnabled"] is True and account["usageWarningPercent"] == 50, account
assert account["rejectionWarningCount"] == 1 and account["usageAlertCooldownMinutes"] == 5, account
assert account["usageAlertLastEvaluatedAt"], account
assert account["dailyRequestCount"] > 0 and account["dailyRejectedCount"] == 1, account
assert data["summary"]["todayRequestCount"] == account["dailyRequestCount"], data
assert data["summary"]["todayRejectedCount"] == 1 and data["summary"]["limitedAccountCount"] == 1, data
protection = data["keyProtection"]
assert protection["dedicatedConfigured"] is True and protection["healthy"] is True and protection["missingKeyIds"] == [], protection
assert protection["legacyStoredKeyCount"] == 1 and protection["legacyUsableKeyCount"] == 0, protection
assert any(route["routeKey"] == "GET /api/saas/v1/whoami" and route["rejectedCount"] == 1 for route in account["todayRoutes"]), account
assert "keyHash" not in text and "plainTextKey" not in text and "mch_live_" in text, text
PY
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_service_account_keys WHERE service_account_id = $ACCOUNT_ID")" = "3"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_service_account_keys WHERE service_account_id = $ACCOUNT_ID AND hash_key_id = '$PEPPER_ID'")" = "2"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_service_account_keys WHERE service_account_id = $ACCOUNT_ID AND hash_key_id = 'legacy-jwt'")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_service_account_keys WHERE service_account_id = $ACCOUNT_ID AND status = 'revoked'")" = "2"
test "$(mysql_scalar "SELECT use_count > 0 FROM mochat_go_saas_service_accounts WHERE id = $ACCOUNT_ID")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_operation_logs WHERE action = 'saas.admin.service_account.create'")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_operation_logs WHERE action = 'saas.admin.service_account.update'")" = "5"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_operation_logs WHERE action = 'saas.admin.service_account.rotate'")" = "2"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_operation_logs WHERE action = 'saas.admin.service_account.revoke'")" = "2"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approvals WHERE action_type = 'service_account.create' AND status = 'executed' AND approval_count = 2 AND effect_applied_at IS NOT NULL AND effect_operation_id > 0")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approvals WHERE action_type = 'service_account.update' AND status = 'executed' AND approval_count = 2 AND effect_applied_at IS NOT NULL AND effect_operation_id > 0")" = "5"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approvals WHERE action_type = 'service_account.key.rotate' AND status = 'executed' AND approval_count = 2 AND effect_applied_at IS NOT NULL AND effect_operation_id > 0")" = "2"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approvals WHERE action_type = 'service_account.key.revoke' AND status = 'executed' AND approval_count = 2 AND effect_applied_at IS NOT NULL AND effect_operation_id > 0")" = "2"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_operation_logs WHERE action = 'saas.admin.service_account.usage_alert.evaluate'")" = "4"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_operation_logs WHERE before_json LIKE '%mch_live_%' OR after_json LIKE '%mch_live_%'")" = "0"

curl -sS -f "http://$GO_ADDR/compat/routes" >"$WORK_DIR/routes.json"
grep -q 'GET /dashboard/saasAdmin/serviceAccounts' "$WORK_DIR/routes.json"
grep -q 'GET /dashboard/saasAdmin/serviceAccountUsage' "$WORK_DIR/routes.json"
grep -q 'POST /dashboard/saasAdmin/serviceAccountUsageAlertEvaluate' "$WORK_DIR/routes.json"
grep -q 'PUT /dashboard/saasAdmin/serviceAccountKeyRevoke' "$WORK_DIR/routes.json"
grep -q 'GET /api/saas/v1/whoami' "$WORK_DIR/routes.json"
grep -q 'GET /api/saas/v1/usage' "$WORK_DIR/routes.json"
grep -q 'GET /api/saas/v1/alerts' "$WORK_DIR/routes.json"
grep -q 'SaaS service account pepper manager enabled: active_key_id=' "$GO_LOG"

echo "SaaS service account smoke passed"
