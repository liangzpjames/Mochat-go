#!/usr/bin/env bash
set -Eeuo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-saas-tenant-domain-approval-check}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13423}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26473}"
GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18197}"
DATABASE="mochat_saas_tenant_domain_approval"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-saas-tenant-domain-approval.XXXXXX")"
MIGRATE_BIN="$WORK_DIR/mochat-migrate"
BOOTSTRAP_BIN="$WORK_DIR/mochat-bootstrap"
GO_BIN="$WORK_DIR/mochat-go"
GO_LOG="$WORK_DIR/go.log"
GO_PID=""
JWT_SECRET="${MOCHAT_SIMPLE_JWT_SECRET:-saas-tenant-domain-approval-jwt-secret}"

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
trap 'echo "SaaS tenant domain approval smoke failed at line $LINENO" >&2; tail -180 "$GO_LOG" >&2 || true' ERR
trap cleanup EXIT INT TERM

assert_port_free() {
  local port="$1"
  if lsof -nP -iTCP:"$port" -sTCP:LISTEN >/dev/null 2>&1; then
    echo "port $port is already in use" >&2
    return 1
  fi
}

wait_service_healthy() {
  local service="$1" deadline=$((SECONDS + ${MOCHAT_SERVICE_HEALTH_TIMEOUT_SECONDS:-360}))
  while [ "$SECONDS" -lt "$deadline" ]; do
    local status
    status="$(compose ps --format json "$service" 2>/dev/null | python3 -c 'import json,sys; data=sys.stdin.read().strip(); print(json.loads(data).get("Health", "")) if data else print("")' 2>/dev/null || true)"
    [ "$status" = "healthy" ] && return 0
    sleep 2
  done
  compose ps >&2 || true
  compose logs --tail=120 "$service" >&2 || true
  return 1
}

wait_url() {
  local url="$1" expected="$2" deadline=$((SECONDS + 60))
  while [ "$SECONDS" -lt "$deadline" ]; do
    [ "$(curl -s -o /dev/null -w '%{http_code}' "$url" || true)" = "$expected" ] && return 0
    sleep 1
  done
  tail -120 "$GO_LOG" >&2 || true
  return 1
}

mysql_root() {
  compose exec -T mysql mariadb -uroot -pmochat_root "$@"
}

mysql_scalar() {
  compose exec -T mysql mariadb -umochat -pmochat_pass -N -B "$DATABASE" -e "$1" </dev/null | tr -d '\r'
}

api_get() {
  local token="$1" path="$2" output="$3" expected="${4:-200}" status
  status="$(curl -sS -o "$output" -w '%{http_code}' -H "Authorization: Bearer $token" "http://$GO_ADDR$path")"
  test "$status" = "$expected" || { echo "GET $path returned $status, expected $expected" >&2; cat "$output" >&2; return 1; }
}

api_post() {
  local token="$1" path="$2" body="$3" output="$4" expected="${5:-200}" status
  local headers=(-H "Content-Type: application/json")
  if [ -n "$token" ]; then
    headers+=(-H "Authorization: Bearer $token")
  fi
  status="$(curl -sS -o "$output" -w '%{http_code}' "${headers[@]}" -d "$body" "http://$GO_ADDR$path")"
  test "$status" = "$expected" || { echo "POST $path returned $status, expected $expected" >&2; cat "$output" >&2; return 1; }
}

json_value() {
  local file="$1" expression="$2"
  python3 - "$file" "$expression" <<'PY'
import json, pathlib, sys
value = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
for part in sys.argv[2].split("."):
    value = value[int(part)] if isinstance(value, list) else value[part]
if isinstance(value, bool):
    print(str(value).lower())
elif isinstance(value, (dict, list)):
    print(json.dumps(value, ensure_ascii=False, separators=(",", ":")))
else:
    print(value)
PY
}

login_token() {
  local phone="$1" password="$2" output="$3"
  api_post "" "/dashboard/user/auth" "{\"phone\":\"$phone\",\"password\":\"$password\"}" "$output"
  json_value "$output" data.token
}

request_domain_approval() {
  local action="$1" domain_id="$2" version="$3" key="$4" output="$5" payload body
  payload="$(jq -cn --arg action "$action" --argjson id "$domain_id" --argjson version "$version" '{action:$action,id:$id,expectedVersion:$version}')"
  body="$(jq -cn --argjson payload "$payload" --arg key "$key" '{actionType:"tenant.domain.command",payload:$payload,reason:"复核租户域名路由变更窗口",idempotencyKey:$key}')"
  api_post "$REQUESTER_TOKEN" "/dashboard/saasAdmin/approvalRequest" "$body" "$output"
}

request_domain_create_approval() {
  local tenant_id="$1" hostname="$2" key="$3" output="$4" payload body
  payload="$(jq -cn --argjson tenantId "$tenant_id" --arg hostname "$hostname" '{action:"create",tenantId:$tenantId,hostname:$hostname}')"
  body="$(jq -cn --argjson payload "$payload" --arg key "$key" '{actionType:"tenant.domain.create",payload:$payload,reason:"复核客户域名归属与租户绑定关系",idempotencyKey:$key}')"
  api_post "$REQUESTER_TOKEN" "/dashboard/saasAdmin/approvalRequest" "$body" "$output"
}

approve_twice() {
  local approval_id="$1" approval_version="$2" prefix="$3"
  api_post "$APPROVER1_TOKEN" "/dashboard/saasAdmin/approvalDecision" \
    "{\"approvalId\":$approval_id,\"expectedVersion\":$approval_version,\"decision\":\"approve\",\"reason\":\"第一复核人确认域名归属与切换窗口\"}" \
    "$WORK_DIR/$prefix-vote-one.json"
  approval_version="$(json_value "$WORK_DIR/$prefix-vote-one.json" data.approval.version)"
  jq -e '.data.approval.status == "pending" and .data.approval.approvalCount == 1' "$WORK_DIR/$prefix-vote-one.json" >/dev/null
  api_post "$APPROVER2_TOKEN" "/dashboard/saasAdmin/approvalDecision" \
    "{\"approvalId\":$approval_id,\"expectedVersion\":$approval_version,\"decision\":\"approve\",\"reason\":\"第二复核人确认租户完整路由快照\"}" \
    "$WORK_DIR/$prefix-vote-two.json"
  jq -e '.data.approval.status == "approved" and .data.approval.approvalCount == 2' "$WORK_DIR/$prefix-vote-two.json" >/dev/null
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
DSN="mochat:mochat_pass@tcp(127.0.0.1:$MYSQL_PORT)/$DATABASE?parseTime=true&loc=Local"
"$MIGRATE_BIN" -dsn "$DSN" -project-root "$PWD" -action apply >"$WORK_DIR/migrate.out"
grep -q $'0094_saas_tenant_domain_command_guard\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0096_saas_tenant_enable_approval_guard\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0098_scrm_lead_foundation\tapplied_now' "$WORK_DIR/migrate.out"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_schema_migrations")" = "98"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approval_policies")" = "31"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approval_policies WHERE action_type = 'tenant.domain.command' AND enabled = 1 AND amount_threshold_cents = 0 AND required_approvals = 2 AND sla_minutes = 120 AND reminder_minutes = 30 AND expiry_hours = 12")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approval_policies WHERE action_type = 'tenant.domain.create' AND enabled = 1 AND amount_threshold_cents = 0 AND required_approvals = 2 AND sla_minutes = 120 AND reminder_minutes = 30 AND expiry_hours = 12")" = "1"

printf '%s\n' \
  'tenant_id,tenant_name,phone,password,user_name,role_name,package_code,package_name,max_corps,max_users,max_contacts,max_rooms,max_agents,channel_codes,shop_codes,radars,lotteries,room_infinite_pulls,room_fissions,room_clock_ins,room_qualities,room_calendars,room_reminds,contact_sops,room_sops,sensitive_words,storage_mb,contact_message_batches,room_message_batches,room_tag_pulls,work_room_auto_pulls,work_fissions,official_accounts,async_executions,package_expires_at,config_copy_mode' \
  '1,域名审批平台,13800000083,secret083,平台超级管理员,SaaS平台超级管理员,platform,平台版,10,100,100000,2000,20,500,300,300,300,300,300,300,300,300,300,300,300,300,4096,300,300,300,300,300,50,100000,2037-01-01,missing' \
  '994,域名审批业务租户,13800000994,secret994,租户管理员,SaaS租户超级管理员,starter,基础版,1,5,500,20,2,10,10,10,10,10,10,10,10,10,10,10,10,10,256,10,10,10,10,10,1,500,2037-01-01,missing' \
  '995,域名新增审批租户,13800000995,secret995,租户管理员,SaaS租户超级管理员,starter,基础版,1,5,500,20,2,10,10,10,10,10,10,10,10,10,10,10,10,10,256,10,10,10,10,10,1,500,2037-01-01,missing' \
  >"$WORK_DIR/tenants.csv"
"$BOOTSTRAP_BIN" -dsn "$DSN" -secret "$JWT_SECRET" -batch-file "$WORK_DIR/tenants.csv" >"$WORK_DIR/bootstrap.out"

mysql_root "$DATABASE" <<'SQL'
INSERT INTO mc_user (phone, password, name, gender, department, position, status, tenant_id, created_at, updated_at, isSuperAdmin)
SELECT '13800000084', password, '域名变更申请人', 0, '运营部', '域名运营', 1, 1, NOW(), NOW(), 0 FROM mc_user WHERE phone = '13800000083' LIMIT 1;
INSERT INTO mc_user (phone, password, name, gender, department, position, status, tenant_id, created_at, updated_at, isSuperAdmin)
SELECT '13800000085', password, '域名复核人一', 0, '内控部', '审批', 1, 1, NOW(), NOW(), 0 FROM mc_user WHERE phone = '13800000083' LIMIT 1;
INSERT INTO mc_user (phone, password, name, gender, department, position, status, tenant_id, created_at, updated_at, isSuperAdmin)
SELECT '13800000086', password, '域名复核人二', 0, '内控部', '审批', 1, 1, NOW(), NOW(), 0 FROM mc_user WHERE phone = '13800000083' LIMIT 1;

INSERT INTO mochat_go_saas_tenant_domains
  (tenant_id, hostname, hostname_active, status, is_primary, primary_slot, verification_method, verification_token,
   verification_error, last_verification_at, verified_at, version, created_by, updated_by, created_at, updated_at)
VALUES
  (994, 'login.approval.customer.test', 'login.approval.customer.test', 'active', 1, 994, 'dns_txt', 'seed-domain-token-a', '', NOW(), NOW(), 1, 1, 1, NOW(), NOW()),
  (994, 'app.approval.customer.test', 'app.approval.customer.test', 'active', 0, NULL, 'dns_txt', 'seed-domain-token-b', '', NOW(), NOW(), 1, 1, 1, NOW(), NOW()),
  (994, 'disabled.approval.customer.test', 'disabled.approval.customer.test', 'disabled', 0, NULL, 'dns_txt', 'seed-domain-token-c', '', NOW(), NOW(), 1, 1, 1, NOW(), NOW());
SQL

REQUESTER_ID="$(mysql_scalar "SELECT id FROM mc_user WHERE phone = '13800000084'")"
APPROVER1_ID="$(mysql_scalar "SELECT id FROM mc_user WHERE phone = '13800000085'")"
APPROVER2_ID="$(mysql_scalar "SELECT id FROM mc_user WHERE phone = '13800000086'")"
OPERATIONS_ROLE_ID="$(mysql_scalar "SELECT id FROM mochat_go_saas_admin_roles WHERE code = 'platform_operations'")"
APPROVER_ROLE_ID="$(mysql_scalar "SELECT id FROM mochat_go_saas_admin_roles WHERE code = 'platform_approver'")"
DOMAIN_A_ID="$(mysql_scalar "SELECT id FROM mochat_go_saas_tenant_domains WHERE hostname = 'login.approval.customer.test'")"
DOMAIN_B_ID="$(mysql_scalar "SELECT id FROM mochat_go_saas_tenant_domains WHERE hostname = 'app.approval.customer.test'")"
DOMAIN_C_ID="$(mysql_scalar "SELECT id FROM mochat_go_saas_tenant_domains WHERE hostname = 'disabled.approval.customer.test'")"
mysql_root "$DATABASE" <<SQL
INSERT INTO mochat_go_saas_admin_user_access (user_id, version, updated_by, created_at, updated_at)
VALUES ($REQUESTER_ID, 1, 0, NOW(), NOW()), ($APPROVER1_ID, 1, 0, NOW(), NOW()), ($APPROVER2_ID, 1, 0, NOW(), NOW());
INSERT INTO mochat_go_saas_admin_user_roles (user_id, role_id, assigned_by, created_at)
VALUES ($REQUESTER_ID, $OPERATIONS_ROLE_ID, 0, NOW()), ($APPROVER1_ID, $APPROVER_ROLE_ID, 0, NOW()), ($APPROVER2_ID, $APPROVER_ROLE_ID, 0, NOW());
SQL

env -u GOROOT \
  MOCHAT_GO_STANDALONE=1 \
  MOCHAT_GO_ADDR="$GO_ADDR" \
  MOCHAT_MYSQL_DSN="$DSN" \
  MOCHAT_REDIS_ADDR="127.0.0.1:$REDIS_PORT" \
  MOCHAT_SIMPLE_JWT_SECRET="$JWT_SECRET" \
  MOCHAT_GO_MIGRATE_AUTH=1 \
  MOCHAT_GO_MIGRATE_LOGIN_SHOW=1 \
  MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1 \
  MOCHAT_GO_SAAS_ADMIN_APPROVAL_REQUIRED=1 \
  MOCHAT_GO_SAAS_PLATFORM_ADMIN_TENANT_ID=1 \
  "$GO_BIN" >"$GO_LOG" 2>&1 &
GO_PID="$!"
wait_url "http://$GO_ADDR/readyz" 200

PLATFORM_TOKEN="$(login_token 13800000083 secret083 "$WORK_DIR/platform-auth.json")"
REQUESTER_TOKEN="$(login_token 13800000084 secret083 "$WORK_DIR/requester-auth.json")"
APPROVER1_TOKEN="$(login_token 13800000085 secret083 "$WORK_DIR/approver-one-auth.json")"
APPROVER2_TOKEN="$(login_token 13800000086 secret083 "$WORK_DIR/approver-two-auth.json")"

api_get "$REQUESTER_TOKEN" "/dashboard/saasAdmin/approvalPolicies" "$WORK_DIR/policies.json"
jq -e '.data.required == true and (.data.policies | length) == 31 and ([.data.policies[] | select(.riskLevel == "critical")] | length) == 31 and ([.data.policies[] | select(.actionType == "tenant.domain.create")][0] | .enabled == true and .requiredApprovals == 2 and .requiredPermission == "platform.domains.manage" and .targetType == "saas_tenant_domain" and .governanceLocked == true and .minimumApprovals == 2 and .amountThresholdLocked == true) and ([.data.policies[] | select(.actionType == "tenant.domain.command")][0] | .enabled == true and .requiredApprovals == 2 and .requiredPermission == "platform.domains.manage" and .targetType == "saas_tenant_domain" and .governanceLocked == true and .minimumApprovals == 2 and .amountThresholdLocked == true)' "$WORK_DIR/policies.json" >/dev/null

POLICY_VERSION="$(jq -r '.data.policies[] | select(.actionType == "tenant.domain.command") | .version' "$WORK_DIR/policies.json")"
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/approvalRequest" \
  "{\"actionType\":\"approval.policy.update\",\"payload\":{\"actionType\":\"tenant.domain.command\",\"enabled\":false,\"amountThresholdCents\":1,\"requiredApprovals\":1,\"slaMinutes\":120,\"reminderMinutes\":30,\"expiryHours\":12,\"expectedVersion\":$POLICY_VERSION},\"reason\":\"尝试弱化域名路由门禁\",\"idempotencyKey\":\"tenant-domain-policy-weaken-0094\"}" \
  "$WORK_DIR/policy-weaken-blocked.json" 400
jq -e '.code == 400' "$WORK_DIR/policy-weaken-blocked.json" >/dev/null

CREATE_POLICY_VERSION="$(jq -r '.data.policies[] | select(.actionType == "tenant.domain.create") | .version' "$WORK_DIR/policies.json")"
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/approvalRequest" \
  "{\"actionType\":\"approval.policy.update\",\"payload\":{\"actionType\":\"tenant.domain.create\",\"enabled\":false,\"amountThresholdCents\":1,\"requiredApprovals\":1,\"slaMinutes\":120,\"reminderMinutes\":30,\"expiryHours\":12,\"expectedVersion\":$CREATE_POLICY_VERSION},\"reason\":\"尝试弱化域名新增门禁\",\"idempotencyKey\":\"tenant-domain-create-policy-weaken-0095\"}" \
  "$WORK_DIR/create-policy-weaken-blocked.json" 400
jq -e '.code == 400' "$WORK_DIR/create-policy-weaken-blocked.json" >/dev/null

api_post "$REQUESTER_TOKEN" "/dashboard/saasAdmin/tenantDomain" \
  '{"action":"create","tenantId":995,"hostname":"new.approval.customer.test"}' \
  "$WORK_DIR/direct-create-blocked.json" 428
jq -e '.code == 428 and .data.actionType == "tenant.domain.create" and .data.requiredApprovals == 2' "$WORK_DIR/direct-create-blocked.json" >/dev/null
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_tenant_domains WHERE tenant_id = 995")" = "0"

request_domain_create_approval 995 'New.Approval.Customer.Test.' tenant-domain-create-0095 "$WORK_DIR/create-request.json"
CREATE_APPROVAL_ID="$(json_value "$WORK_DIR/create-request.json" data.approval.id)"
CREATE_APPROVAL_VERSION="$(json_value "$WORK_DIR/create-request.json" data.approval.version)"
jq -e '.data.approval.actionType == "tenant.domain.create" and .data.approval.riskLevel == "critical" and .data.approval.requiredApprovals == 2 and .data.approval.targetId == "995:new.approval.customer.test" and .data.approval.targetName == "域名新增审批租户 / new.approval.customer.test" and .data.approval.request.action == "create" and .data.approval.request.tenantId == 995 and .data.approval.request.hostname == "new.approval.customer.test" and (.data.approval.request | has("token") | not) and (.data.approval.request | tostring | contains("verificationToken") | not)' "$WORK_DIR/create-request.json" >/dev/null
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approvals WHERE id = $CREATE_APPROVAL_ID AND request_json NOT LIKE '%verificationToken%' AND request_json NOT LIKE '%mochat-domain-verification%'")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_tenant_domains WHERE tenant_id = 995")" = "0"
request_domain_create_approval 995 'new.approval.customer.test' tenant-domain-create-0095 "$WORK_DIR/create-idempotent.json"
jq -e --argjson approvalId "$CREATE_APPROVAL_ID" '.data.idempotent == true and .data.approval.id == $approvalId' "$WORK_DIR/create-idempotent.json" >/dev/null

approve_twice "$CREATE_APPROVAL_ID" "$CREATE_APPROVAL_VERSION" create
CREATE_APPROVAL_VERSION="$(json_value "$WORK_DIR/create-vote-two.json" data.approval.version)"
api_post "$APPROVER1_TOKEN" "/dashboard/saasAdmin/approvalExecute" \
  "{\"approvalId\":$CREATE_APPROVAL_ID,\"expectedVersion\":$CREATE_APPROVAL_VERSION}" "$WORK_DIR/create-execute.json"
CREATE_DOMAIN_ID="$(json_value "$WORK_DIR/create-execute.json" data.result.domain.id)"
CREATE_OPERATION_ID="$(json_value "$WORK_DIR/create-execute.json" data.result.operationId)"
CREATE_TOKEN="$(json_value "$WORK_DIR/create-execute.json" data.result.domain.verificationToken)"
jq -e '.data.approval.status == "executed" and .data.approval.effectOperationId == .data.result.operationId and .data.result.domain.tenantId == 995 and .data.result.domain.hostname == "new.approval.customer.test" and .data.result.domain.status == "pending" and .data.result.domain.isPrimary == false and .data.result.domain.version == 1 and (.data.result.domain.verificationToken | length) == 43 and (.data.result.domain.verificationRecordName | startswith("_mochat.")) and (.data.result.domain.verificationRecordValue | startswith("mochat-domain-verification=")) and .data.result.operationId > 0' "$WORK_DIR/create-execute.json" >/dev/null
test "${#CREATE_TOKEN}" = "43"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_tenant_domains WHERE id = $CREATE_DOMAIN_ID AND tenant_id = 995 AND hostname = 'new.approval.customer.test' AND hostname_active = 'new.approval.customer.test' AND status = 'pending' AND is_primary = 0 AND CHAR_LENGTH(verification_token) = 43 AND version = 1")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_tenant_domain_deliveries WHERE domain_id = $CREATE_DOMAIN_ID AND tenant_id = 995 AND delivery_status = 'unconfigured' AND routing_status = 'pending' AND certificate_status = 'pending' AND version = 1")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_operation_logs WHERE id = $CREATE_OPERATION_ID AND action = 'saas.admin.tenant_domain.create' AND target_type = 'saas_tenant_domain' AND target_id = '$CREATE_DOMAIN_ID'")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approvals WHERE id = $CREATE_APPROVAL_ID AND status = 'executed' AND approval_count = 2 AND effect_operation_id = $CREATE_OPERATION_ID AND effect_applied_at IS NOT NULL AND JSON_UNQUOTE(JSON_EXTRACT(result_json, '$.domain.verificationTokenDelivered')) = 'true' AND JSON_EXTRACT(result_json, '$.domain.verificationToken') IS NULL AND JSON_EXTRACT(result_json, '$.domain.verificationRecordValue') IS NULL AND LOCATE('$CREATE_TOKEN', result_json) = 0")" = "1"

for direct in "set_primary:$DOMAIN_B_ID:1" "disable:$DOMAIN_A_ID:1" "enable:$DOMAIN_C_ID:1" "rotate_token:$DOMAIN_B_ID:1" "delete:$DOMAIN_C_ID:1"; do
  action="${direct%%:*}"
  rest="${direct#*:}"
  domain_id="${rest%%:*}"
  version="${rest##*:}"
  api_post "$REQUESTER_TOKEN" "/dashboard/saasAdmin/tenantDomain" \
    "{\"action\":\"$action\",\"id\":$domain_id,\"expectedVersion\":$version}" \
    "$WORK_DIR/direct-$action-blocked.json" 428
  jq -e '.code == 428 and .data.actionType == "tenant.domain.command" and .data.requiredApprovals == 2' "$WORK_DIR/direct-$action-blocked.json" >/dev/null
done
test "$(mysql_scalar "SELECT CONCAT(SUM(is_primary), ':', SUM(version)) FROM mochat_go_saas_tenant_domains WHERE tenant_id = 994 AND deleted_at IS NULL")" = "1:3"

request_domain_approval set_primary "$DOMAIN_B_ID" 1 tenant-domain-primary-0094 "$WORK_DIR/primary-request.json"
PRIMARY_APPROVAL_ID="$(json_value "$WORK_DIR/primary-request.json" data.approval.id)"
PRIMARY_APPROVAL_VERSION="$(json_value "$WORK_DIR/primary-request.json" data.approval.version)"
jq -e --argjson domainId "$DOMAIN_B_ID" '.data.approval.actionType == "tenant.domain.command" and .data.approval.riskLevel == "critical" and .data.approval.requiredApprovals == 2 and .data.approval.targetId == ($domainId|tostring) and .data.approval.request.schemaVersion == 1 and .data.approval.request.command.id == $domainId and .data.approval.request.command.action == "set_primary" and .data.approval.request.command.expectedVersion == 1 and (.data.approval.request.command | has("token") | not) and .data.approval.request.domain.id == $domainId and (.data.approval.request.routingDomains | length) == 3 and (.data.approval.request.routingSha256 | length) == 64 and all(.data.approval.request.routingDomains[]; .tenantId == 994 and .version == 1)' "$WORK_DIR/primary-request.json" >/dev/null
request_domain_approval set_primary "$DOMAIN_B_ID" 1 tenant-domain-primary-0094 "$WORK_DIR/primary-idempotent.json"
jq -e --argjson approvalId "$PRIMARY_APPROVAL_ID" '.data.idempotent == true and .data.approval.id == $approvalId' "$WORK_DIR/primary-idempotent.json" >/dev/null

approve_twice "$PRIMARY_APPROVAL_ID" "$PRIMARY_APPROVAL_VERSION" primary
PRIMARY_APPROVAL_VERSION="$(json_value "$WORK_DIR/primary-vote-two.json" data.approval.version)"
api_post "$APPROVER1_TOKEN" "/dashboard/saasAdmin/approvalExecute" \
  "{\"approvalId\":$PRIMARY_APPROVAL_ID,\"expectedVersion\":$PRIMARY_APPROVAL_VERSION}" "$WORK_DIR/primary-execute.json"
PRIMARY_OPERATION_ID="$(json_value "$WORK_DIR/primary-execute.json" data.result.operationId)"
jq -e --argjson domainId "$DOMAIN_B_ID" '.data.approval.status == "executed" and .data.approval.effectOperationId == .data.result.operationId and .data.result.domain.id == $domainId and .data.result.domain.isPrimary == true and .data.result.domain.version == 2 and .data.result.operationId > 0' "$WORK_DIR/primary-execute.json" >/dev/null
test "$(mysql_scalar "SELECT CONCAT(is_primary, ':', version) FROM mochat_go_saas_tenant_domains WHERE id = $DOMAIN_A_ID")" = "0:2"
test "$(mysql_scalar "SELECT CONCAT(is_primary, ':', version) FROM mochat_go_saas_tenant_domains WHERE id = $DOMAIN_B_ID")" = "1:2"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_tenant_domain_delivery_jobs WHERE domain_id = $DOMAIN_B_ID AND action = 'refresh' AND status = 'pending' AND operation_id = $PRIMARY_OPERATION_ID")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_operation_logs WHERE id = $PRIMARY_OPERATION_ID AND action = 'saas.admin.tenant_domain.set_primary' AND target_type = 'saas_tenant_domain' AND target_id = '$DOMAIN_B_ID'")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approvals WHERE id = $PRIMARY_APPROVAL_ID AND status = 'executed' AND approval_count = 2 AND effect_operation_id = $PRIMARY_OPERATION_ID AND effect_applied_at IS NOT NULL")" = "1"

request_domain_approval disable "$DOMAIN_B_ID" 2 tenant-domain-drift-0094 "$WORK_DIR/drift-request.json"
DRIFT_APPROVAL_ID="$(json_value "$WORK_DIR/drift-request.json" data.approval.id)"
DRIFT_APPROVAL_VERSION="$(json_value "$WORK_DIR/drift-request.json" data.approval.version)"
approve_twice "$DRIFT_APPROVAL_ID" "$DRIFT_APPROVAL_VERSION" drift
DRIFT_APPROVAL_VERSION="$(json_value "$WORK_DIR/drift-vote-two.json" data.approval.version)"
mysql_scalar "UPDATE mochat_go_saas_tenant_domains SET verification_error = '审批外路由漂移', version = version + 1 WHERE id = $DOMAIN_A_ID" >/dev/null
test "$(mysql_scalar "SELECT version FROM mochat_go_saas_tenant_domains WHERE id = $DOMAIN_A_ID")" = "3"
api_post "$APPROVER1_TOKEN" "/dashboard/saasAdmin/approvalExecute" \
  "{\"approvalId\":$DRIFT_APPROVAL_ID,\"expectedVersion\":$DRIFT_APPROVAL_VERSION}" "$WORK_DIR/drift-execute.json" 409
jq -e '.code == 409 and (.msg | contains("路由快照已变化"))' "$WORK_DIR/drift-execute.json" >/dev/null
test "$(mysql_scalar "SELECT CONCAT(status, ':', is_primary, ':', version) FROM mochat_go_saas_tenant_domains WHERE id = $DOMAIN_B_ID")" = "active:1:2"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approvals WHERE id = $DRIFT_APPROVAL_ID AND status = 'approved' AND effect_applied_at IS NULL AND effect_operation_id = 0 AND last_error LIKE '%路由快照已变化%'")" = "1"

ORIGINAL_TOKEN="$(mysql_scalar "SELECT verification_token FROM mochat_go_saas_tenant_domains WHERE id = $DOMAIN_B_ID")"
request_domain_approval rotate_token "$DOMAIN_B_ID" 2 tenant-domain-rotate-0094 "$WORK_DIR/rotate-request.json"
ROTATE_APPROVAL_ID="$(json_value "$WORK_DIR/rotate-request.json" data.approval.id)"
ROTATE_APPROVAL_VERSION="$(json_value "$WORK_DIR/rotate-request.json" data.approval.version)"
jq -e '.data.approval.request.command.action == "rotate_token" and (.data.approval.request.command | has("token") | not) and (.data.approval.request | tostring | contains("seed-domain-token") | not)' "$WORK_DIR/rotate-request.json" >/dev/null
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approvals WHERE id = $ROTATE_APPROVAL_ID AND request_json NOT LIKE '%seed-domain-token%' AND request_json NOT LIKE '%verificationToken%'")" = "1"
approve_twice "$ROTATE_APPROVAL_ID" "$ROTATE_APPROVAL_VERSION" rotate
ROTATE_APPROVAL_VERSION="$(json_value "$WORK_DIR/rotate-vote-two.json" data.approval.version)"
api_post "$APPROVER2_TOKEN" "/dashboard/saasAdmin/approvalExecute" \
  "{\"approvalId\":$ROTATE_APPROVAL_ID,\"expectedVersion\":$ROTATE_APPROVAL_VERSION}" "$WORK_DIR/rotate-execute.json"
ROTATE_OPERATION_ID="$(json_value "$WORK_DIR/rotate-execute.json" data.result.operationId)"
ROTATED_TOKEN="$(mysql_scalar "SELECT verification_token FROM mochat_go_saas_tenant_domains WHERE id = $DOMAIN_B_ID")"
test "$ROTATED_TOKEN" != "$ORIGINAL_TOKEN"
test "${#ROTATED_TOKEN}" = "43"
test "$(mysql_scalar "SELECT CONCAT(status, ':', is_primary, ':', version) FROM mochat_go_saas_tenant_domains WHERE id = $DOMAIN_B_ID")" = "pending:0:3"
test "$(mysql_scalar "SELECT CONCAT(is_primary, ':', version) FROM mochat_go_saas_tenant_domains WHERE id = $DOMAIN_A_ID")" = "1:4"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_tenant_domain_delivery_jobs WHERE domain_id = $DOMAIN_B_ID AND action = 'disable' AND status = 'pending' AND operation_id = $ROTATE_OPERATION_ID")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approvals WHERE id = $ROTATE_APPROVAL_ID AND status = 'executed' AND approval_count = 2 AND effect_operation_id = $ROTATE_OPERATION_ID AND effect_applied_at IS NOT NULL")" = "1"

api_get "$PLATFORM_TOKEN" "/dashboard/saasAdmin/page" "$WORK_DIR/page.html"
grep -q "approvalActionRequired('tenant.domain.create', 0)" "$WORK_DIR/page.html"
grep -q "approvalActionRequired('tenant.domain.command', 0)" "$WORK_DIR/page.html"
grep -q '提交添加审批' "$WORK_DIR/page.html"
grep -q '提交主域名审批' "$WORK_DIR/page.html"
grep -q '提交启用审批' "$WORK_DIR/page.html"
grep -q '提交停用审批' "$WORK_DIR/page.html"
grep -q '提交删除审批' "$WORK_DIR/page.html"

echo "SaaS tenant domain create and command approval smoke passed"
