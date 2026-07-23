#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-production-evidence-gate.XXXXXX")"
EVIDENCE_DIR="$WORK_DIR/evidence"
SERVER_PIDS=()
SECRET_FIXTURE_PORT=""
SOURCE_FINGERPRINT="$(python3 ./scripts/source_fingerprint.py | python3 -c 'import json, sys; print(json.load(sys.stdin)["fingerprint"])')"
STALE_SOURCE_FINGERPRINT="0000000000000000000000000000000000000000000000000000000000000000"

cleanup() {
  for pid in "${SERVER_PIDS[@]:-}"; do
    kill "$pid" >/dev/null 2>&1 || true
  done
  rm -rf "$WORK_DIR"
}
trap cleanup EXIT INT TERM

if [ "${1:-}" = "-h" ] || [ "${1:-}" = "--help" ]; then
  cat <<'EOF'
Usage: ./scripts/smoke_production_evidence_gate.sh

验证生产证据门禁规则：
  - init_production_evidence_pack.sh 生成的模板必须被严格门禁拒绝
  - MySQL 5.7 arm64 skip 日志必须被拒绝
  - 含未脱敏 Authorization/Bearer/access_token 等敏感值的证据必须被拒绝
  - 生产证据包收拢时，含未脱敏敏感值的源证据不能被复制进 current 包
  - 生产证据包收拢时，含示例域名或本机地址的源证据不能被复制进 current 包
  - 生产证据采集脚本生成的响应摘要必须自动脱敏 JSON / query token
  - 含必填核验项和明确通过结论的证据文件必须通过
  - 生产证据采集脚本必须暴露帮助信息且不启动 24 小时持续运行

该脚本不会启动 24 小时持续运行。
EOF
  exit 0
fi

./scripts/capture_real_wecom_evidence.sh --help | grep -Fq "企业微信账号联调"
./scripts/capture_real_wechat_open_evidence.sh --help | grep -Fq "微信开放平台联调"
./scripts/capture_real_saas_tenants_evidence.sh --help | grep -Fq "真实 SaaS 多租户"
./scripts/capture_prod_frontend_evidence.sh --help | grep -Fq "生产前端"
./scripts/capture_stability_evidence.sh --help | grep -Fq "稳定性证据"
./scripts/production_evidence_doctor.sh --help | grep -Fq "生产证据采集诊断"
./scripts/production_evidence_env_preflight.sh --help | grep -Fq "生产证据采集环境预检"

expect_capture_url_rejection() {
  local name="$1"
  local env_name="$2"
  local script="$3"
  local log="$WORK_DIR/$name-url-rejection.log"
  set +e
  env -u GOROOT "$env_name=https://example.com" "$script" >"$log" 2>&1
  local rc=$?
  set -e
  if [ "$rc" -eq 0 ]; then
    echo "$script unexpectedly accepted placeholder production URL" >&2
    cat "$log" >&2
    exit 1
  fi
  grep -Fq "placeholder or local host" "$log"
}

expect_capture_url_rejection "real-wecom" MOCHAT_REAL_WECOM_BASE_URL ./scripts/capture_real_wecom_evidence.sh
expect_capture_url_rejection "real-wechat-open" MOCHAT_REAL_WECHAT_OPEN_BASE_URL ./scripts/capture_real_wechat_open_evidence.sh
expect_capture_url_rejection "real-saas" MOCHAT_REAL_SAAS_BASE_URL ./scripts/capture_real_saas_tenants_evidence.sh
expect_capture_url_rejection "prod-frontend" MOCHAT_PROD_FRONTEND_BASE_URL ./scripts/capture_prod_frontend_evidence.sh

run_gate() {
  local out="$1"
  local json_out="${out%.md}.json"
  shift
  env -u GOROOT \
    MOCHAT_PRODUCTION_EVIDENCE_SKIP_QUICK=1 \
    MOCHAT_PRODUCTION_EVIDENCE_STRICT=1 \
    MOCHAT_PRODUCTION_EVIDENCE_OUT="$out" \
    MOCHAT_PRODUCTION_EVIDENCE_JSON_OUT="$json_out" \
    MOCHAT_EVIDENCE_MYSQL57_AMD64="$EVIDENCE_DIR/mysql57-amd64.log" \
    MOCHAT_EVIDENCE_REAL_WECOM="$EVIDENCE_DIR/real-wecom.md" \
    MOCHAT_EVIDENCE_REAL_WECHAT_OPEN="$EVIDENCE_DIR/real-wechat-open.md" \
    MOCHAT_EVIDENCE_REAL_SAAS_TENANTS="$EVIDENCE_DIR/real-saas-tenants.md" \
    MOCHAT_EVIDENCE_PROD_FRONTEND="$EVIDENCE_DIR/prod-frontend.md" \
    MOCHAT_EVIDENCE_STABILITY="$EVIDENCE_DIR/stability.md" \
    ./scripts/production_evidence_check.sh "$@"
}

expect_fail() {
  local name="$1"
  local report="$2"
  shift 2
  set +e
  run_gate "$report" "$@" >/dev/null 2>&1
  local rc=$?
  set -e
  if [ "$rc" -eq 0 ]; then
    echo "$name unexpectedly passed" >&2
    [ -f "$report" ] && sed -n '1,120p' "$report" >&2
    exit 1
  fi
}

expect_pass() {
  local name="$1"
  local report="$2"
  shift 2
  if ! run_gate "$report" "$@" >/dev/null 2>&1; then
    echo "$name unexpectedly failed" >&2
    [ -f "$report" ] && sed -n '1,160p' "$report" >&2
    exit 1
  fi
}

run_pack() {
  local pack_dir="$1"
  shift
  env -u GOROOT \
    MOCHAT_PRODUCTION_EVIDENCE_SOURCE_DIR="$EVIDENCE_DIR" \
    MOCHAT_PRODUCTION_EVIDENCE_PACK_DIR="$pack_dir" \
    MOCHAT_PRODUCTION_EVIDENCE_RUN_CANDIDATE=0 \
    ./scripts/collect_production_evidence_pack.sh "$@"
}

run_doctor() {
  local report="$1"
  shift
  env -u GOROOT \
    MOCHAT_PRODUCTION_EVIDENCE_DIR="$EVIDENCE_DIR" \
    MOCHAT_PRODUCTION_EVIDENCE_DOCTOR_OUT="$report" \
    ./scripts/production_evidence_doctor.sh "$@"
}

run_preflight() {
  local readiness_json="$1"
  local env_file="$2"
  local report="$3"
  local json_out="$4"
  shift 4
  env -u GOROOT \
    MOCHAT_PRODUCTION_EVIDENCE_READINESS_JSON="$readiness_json" \
    MOCHAT_PRODUCTION_EVIDENCE_ENV_FILE="$env_file" \
    MOCHAT_PRODUCTION_EVIDENCE_PREFLIGHT_OUT="$report" \
    MOCHAT_PRODUCTION_EVIDENCE_PREFLIGHT_JSON_OUT="$json_out" \
    ./scripts/production_evidence_env_preflight.sh "$@"
}

start_secret_fixture() {
  local port_file="$WORK_DIR/secret-fixture.port"
  cat >"$WORK_DIR/secret-fixture.py" <<'PY'
import json
import http.server
import pathlib
import socketserver
import sys


class Handler(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        body = {
            "errcode": 0,
            "errmsg": "ok",
            "access_token": "raw-access-token-abcdefghijklmnopqrstuvwxyz",
            "component_access_token": "raw-component-access-token-abcdefghijklmnopqrstuvwxyz",
            "authorizer_refresh_token": "raw-authorizer-refresh-token-abcdefghijklmnopqrstuvwxyz",
            "corpsecret": "raw-corpsecret-abcdefghijklmnopqrstuvwxyz",
            "callback": "/callback?token=raw-query-token-abcdefghijklmnopqrstuvwxyz",
        }
        payload = json.dumps(body, ensure_ascii=False).encode("utf-8")
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(payload)))
        self.end_headers()
        self.wfile.write(payload)

    def log_message(self, *_args):
        return


class Server(socketserver.TCPServer):
    allow_reuse_address = True


with Server(("127.0.0.1", 0), Handler) as httpd:
    pathlib.Path(sys.argv[1]).write_text(str(httpd.server_address[1]), encoding="utf-8")
    httpd.serve_forever()
PY
  python3 "$WORK_DIR/secret-fixture.py" "$port_file" >"$WORK_DIR/secret-fixture.log" 2>&1 &
  SERVER_PIDS+=("$!")
  for _ in $(seq 1 50); do
    [ -s "$port_file" ] && break
    sleep 0.1
  done
  if [ ! -s "$port_file" ]; then
    echo "secret fixture failed to start" >&2
    [ -f "$WORK_DIR/secret-fixture.log" ] && cat "$WORK_DIR/secret-fixture.log" >&2
    exit 1
  fi
  SECRET_FIXTURE_PORT="$(cat "$port_file")"
}

env -u GOROOT \
  MOCHAT_PRODUCTION_EVIDENCE_DIR="$EVIDENCE_DIR" \
  MOCHAT_PRODUCTION_EVIDENCE_FORCE=1 \
  ./scripts/init_production_evidence_pack.sh >/dev/null

template_report="$WORK_DIR/template-report.md"
expect_fail "template evidence rejection" "$template_report"
grep -Fq "模板标记 MOCHAT_EVIDENCE_TEMPLATE_DO_NOT_USE" "$template_report"
python3 - "$template_report" "${template_report%.md}.json" <<'PY'
import json
import pathlib
import sys

markdown_path = pathlib.Path(sys.argv[1])
json_path = pathlib.Path(sys.argv[2])
payload = json.loads(json_path.read_text(encoding="utf-8"))
assert markdown_path.exists()
assert payload["schema"] == 1
assert payload["strict"] is True
assert payload["skip_quick"] is True
assert payload["quick_checks"] == []
assert payload["quick_ok"] is True
assert payload["evidence_ok"] is False
assert payload["ok"] is False
assert len(payload["evidence"]) == 6
assert len(payload["invalid_evidence"]) == 6
assert all(item["exists"] is True for item in payload["invalid_evidence"])
assert any("模板标记" in item["issue"] for item in payload["invalid_evidence"])
assert payload["current_source_fingerprint"]
serialized = json.dumps(payload, ensure_ascii=False)
assert "Bearer " not in serialized
assert "dashboard-jwt" not in serialized
PY

template_pack_dir="$WORK_DIR/template-pack"
set +e
run_pack "$template_pack_dir" >/dev/null 2>&1
template_pack_rc=$?
set -e
if [ "$template_pack_rc" -eq 0 ]; then
  echo "template evidence pack unexpectedly passed" >&2
  [ -f "$template_pack_dir/index.md" ] && sed -n '1,160p' "$template_pack_dir/index.md" >&2
  exit 1
fi
grep -Fq "未通过，不能标记最终完成" "$template_pack_dir/index.md"

doctor_report="$WORK_DIR/readiness.md"
doctor_output="$(run_doctor "$doctor_report")"
doctor_env_todo="$EVIDENCE_DIR/readiness.env.todo"
grep -Fq "生产证据采集诊断" "$doctor_report"
grep -Fq '24 小时持续运行：`未启动`' "$doctor_report"
grep -Fq "待填写环境变量清单" "$doctor_report"
grep -Fq "MOCHAT_MYSQL57_AMD64_EVIDENCE_SOURCE" "$doctor_report"
grep -Fq "MOCHAT_REAL_WECOM_BASE_URL" "$doctor_report"
grep -Fq '严格生产证据文件门禁：`未通过`' "$doctor_report"
[ -s "$doctor_env_todo" ]
grep -Fq "MOCHAT_REAL_WECOM_BASE_URL=" "$doctor_env_todo"
grep -Fq "MOCHAT_STABILITY_MONITOR_EVIDENCE=" "$doctor_env_todo"
grep -Fq "MOCHAT_STABILITY_SOAK_LOG=" "$doctor_env_todo"
grep -Fq "readiness.env.local" "$doctor_env_todo"
grep -Fq "production_evidence_env_preflight.sh" "$doctor_env_todo"
grep -Fq "MOCHAT_PRODUCTION_EVIDENCE_DOCTOR_RUN=1 ./scripts/production_evidence_doctor.sh" "$doctor_env_todo"
grep -Fq "./scripts/collect_production_evidence_pack.sh" "$doctor_env_todo"
if rg -q 'dashboard-jwt|Bearer ' "$doctor_env_todo"; then
  echo "doctor env todo leaked secret-like example material" >&2
  sed -n '1,180p' "$doctor_env_todo" >&2
  exit 1
fi
doctor_json="$(printf '%s\n' "$doctor_output" | tail -n 1)"
python3 - "$doctor_json" <<'PY'
import json
import pathlib
import sys

path = pathlib.Path(sys.argv[1])
payload = json.loads(path.read_text(encoding="utf-8"))
assert payload["schema"] == 1
assert payload["started_24h_run"] is False
assert payload["gate_ok"] is False
gate_json = pathlib.Path(payload["gate_json"])
if not gate_json.is_absolute():
    gate_json = pathlib.Path.cwd() / gate_json
assert gate_json.exists()
gate_payload = json.loads(gate_json.read_text(encoding="utf-8"))
assert gate_payload["schema"] == 1
assert gate_payload["evidence_ok"] is False
assert len(gate_payload["invalid_evidence"]) == 6
assert payload["env_template"].endswith("readiness.env.todo")
assert len(payload["items"]) == 6
assert payload["standard_files_valid"] is False
assert all(item["evidence_valid"] is False for item in payload["items"])
assert all("模板标记" in item["evidence_issue"] for item in payload["items"])
assert any("MOCHAT_REAL_WECOM_BASE_URL" in item["missing_env"] for item in payload["items"])
serialized = json.dumps(payload, ensure_ascii=False)
assert "dashboard-jwt" not in serialized
assert "Bearer " not in serialized
PY

preflight_missing_report="$WORK_DIR/preflight-missing.md"
preflight_missing_json="$WORK_DIR/preflight-missing.json"
set +e
MOCHAT_PRODUCTION_EVIDENCE_PREFLIGHT_STRICT=1 \
  run_preflight "$doctor_json" "$EVIDENCE_DIR/readiness.env.local" "$preflight_missing_report" "$preflight_missing_json" >/dev/null 2>&1
preflight_missing_rc=$?
set -e
if [ "$preflight_missing_rc" -eq 0 ]; then
  echo "production evidence env preflight unexpectedly passed with missing env file" >&2
  sed -n '1,160p' "$preflight_missing_report" >&2
  exit 1
fi
grep -Fq "采集环境变量未齐备" "$preflight_missing_report"
grep -Fq '24 小时持续运行：`未启动`' "$preflight_missing_report"
grep -Fq "标准文件无效" "$preflight_missing_report"

preflight_fixture_dir="$WORK_DIR/preflight-fixtures"
mkdir -p "$preflight_fixture_dir"
printf '源码指纹：%s\nmysql57 amd64 CI gate passed\n' "$SOURCE_FINGERPRINT" >"$preflight_fixture_dir/mysql57.log"
printf 'callback decrypted and stored\n' >"$preflight_fixture_dir/wecom-callback.txt"
printf 'agent message sent\n' >"$preflight_fixture_dir/wecom-agent.txt"
printf 'ticket callback stored\n' >"$preflight_fixture_dir/wechat-ticket.txt"
printf 'auth redirect completed\n' >"$preflight_fixture_dir/wechat-auth.txt"
printf 'cancel auth completed\n' >"$preflight_fixture_dir/wechat-cancel.txt"
printf 'message callback completed\n' >"$preflight_fixture_dir/wechat-message.txt"
printf 'quota blocked\n' >"$preflight_fixture_dir/saas-quota.txt"
printf 'upload ledger checked\n' >"$preflight_fixture_dir/saas-upload.txt"
printf 'async executions checked\n' >"$preflight_fixture_dir/saas-async.txt"
printf 'alert routed\n' >"$preflight_fixture_dir/saas-alert.txt"
printf 'route_total=224 missing_route_total=0 error_rate=0 alert_count=0\n' >"$preflight_fixture_dir/stability-monitor.txt"
printf '/readyz health check returned 200 in standalone-go mode\n' >"$preflight_fixture_dir/stability-health.txt"
printf 'RSS stable, CPU stable, database connections stable\n' >"$preflight_fixture_dir/stability-resource.txt"
printf 'monitor ok\n' >"$preflight_fixture_dir/stability-monitor-weak.txt"
printf 'mysql:5.7 is amd64-only and skipped on arm64\n' >"$preflight_fixture_dir/mysql57-arm64.log"
printf 'mysql 5.7 smoke ran without the expected marker\n' >"$preflight_fixture_dir/mysql57-no-marker.log"
python3 - "$preflight_fixture_dir/mysql57-amd64-evidence.zip" "$preflight_fixture_dir/mysql57.log" <<'PY'
import pathlib
import sys
import zipfile

zip_path = pathlib.Path(sys.argv[1])
log_path = pathlib.Path(sys.argv[2])
with zipfile.ZipFile(zip_path, "w") as archive:
    archive.write(log_path, "mysql57-amd64.log")
PY

preflight_env="$WORK_DIR/readiness.env.local"
cat >"$preflight_env" <<EOF
MOCHAT_MYSQL57_AMD64_EVIDENCE_SOURCE=$preflight_fixture_dir/mysql57-amd64-evidence.zip
MOCHAT_REAL_WECOM_BASE_URL=https://wecom-prod.mochat.cn
MOCHAT_REAL_WECOM_TOKEN=dashboard-jwt-abcdefghijklmnopqrstuvwxyz
MOCHAT_REAL_WECOM_CALLBACK_EVIDENCE=@$preflight_fixture_dir/wecom-callback.txt
MOCHAT_REAL_WECOM_AGENT_MESSAGE_EVIDENCE=@$preflight_fixture_dir/wecom-agent.txt
MOCHAT_REAL_WECOM_LOG_REF=preflight-log-ref
MOCHAT_REAL_WECHAT_OPEN_BASE_URL=https://wechat-open-prod.mochat.cn
MOCHAT_REAL_WECHAT_OPEN_TOKEN=wechat-open-token-abcdefghijklmnopqrstuvwxyz
MOCHAT_REAL_WECHAT_OPEN_TICKET_EVIDENCE=@$preflight_fixture_dir/wechat-ticket.txt
MOCHAT_REAL_WECHAT_OPEN_AUTH_REDIRECT_EVIDENCE=@$preflight_fixture_dir/wechat-auth.txt
MOCHAT_REAL_WECHAT_OPEN_CANCEL_EVIDENCE=@$preflight_fixture_dir/wechat-cancel.txt
MOCHAT_REAL_WECHAT_OPEN_MESSAGE_CALLBACK_EVIDENCE=@$preflight_fixture_dir/wechat-message.txt
MOCHAT_REAL_WECHAT_OPEN_LOG_REF=preflight-open-log-ref
MOCHAT_REAL_SAAS_BASE_URL=https://saas-prod.mochat.cn
MOCHAT_REAL_SAAS_TENANT_A_TOKEN=tenant-a-token-abcdefghijklmnopqrstuvwxyz
MOCHAT_REAL_SAAS_TENANT_B_TOKEN=tenant-b-token-abcdefghijklmnopqrstuvwxyz
MOCHAT_REAL_SAAS_TENANT_A_MARKER=tenant-a
MOCHAT_REAL_SAAS_TENANT_B_MARKER=tenant-b
MOCHAT_REAL_SAAS_TENANT_A_FORBIDDEN_PATHS=/dashboard/tenant-b/resource
MOCHAT_REAL_SAAS_TENANT_B_FORBIDDEN_PATHS=/dashboard/tenant-a/resource
MOCHAT_REAL_SAAS_QUOTA_EVIDENCE=@$preflight_fixture_dir/saas-quota.txt
MOCHAT_REAL_SAAS_UPLOAD_LEDGER_EVIDENCE=@$preflight_fixture_dir/saas-upload.txt
MOCHAT_REAL_SAAS_ASYNC_EVIDENCE=@$preflight_fixture_dir/saas-async.txt
MOCHAT_REAL_SAAS_ALERT_EVIDENCE=@$preflight_fixture_dir/saas-alert.txt
MOCHAT_PROD_FRONTEND_BASE_URL=https://frontend-prod.mochat.cn
MOCHAT_STABILITY_MONITOR_EVIDENCE=@$preflight_fixture_dir/stability-monitor.txt
MOCHAT_STABILITY_HEALTH_EVIDENCE=@$preflight_fixture_dir/stability-health.txt
MOCHAT_STABILITY_RESOURCE_EVIDENCE=@$preflight_fixture_dir/stability-resource.txt
MOCHAT_STABILITY_TIME_RANGE=2026-07-08 10:00:00 CST 至 2026-07-08 10:30:00 CST
MOCHAT_STABILITY_LOG_REF=preflight-stability-log-ref
EOF

preflight_arm64_env="$WORK_DIR/readiness-arm64.env.local"
sed "s|^MOCHAT_MYSQL57_AMD64_EVIDENCE_SOURCE=.*|MOCHAT_MYSQL57_AMD64_EVIDENCE_SOURCE=$preflight_fixture_dir/mysql57-arm64.log|" "$preflight_env" >"$preflight_arm64_env"
preflight_arm64_report="$WORK_DIR/preflight-arm64.md"
preflight_arm64_json="$WORK_DIR/preflight-arm64.json"
set +e
MOCHAT_PRODUCTION_EVIDENCE_PREFLIGHT_STRICT=1 \
  run_preflight "$doctor_json" "$preflight_arm64_env" "$preflight_arm64_report" "$preflight_arm64_json" >/dev/null 2>&1
preflight_arm64_rc=$?
set -e
if [ "$preflight_arm64_rc" -eq 0 ]; then
  echo "production evidence env preflight unexpectedly passed with arm64 mysql log" >&2
  sed -n '1,180p' "$preflight_arm64_report" >&2
  exit 1
fi
grep -Fq "arm64 skip" "$preflight_arm64_report"
grep -Fq "不是真实 amd64 通过日志" "$preflight_arm64_report"

preflight_no_marker_env="$WORK_DIR/readiness-no-marker.env.local"
sed "s|^MOCHAT_MYSQL57_AMD64_EVIDENCE_SOURCE=.*|MOCHAT_MYSQL57_AMD64_EVIDENCE_SOURCE=$preflight_fixture_dir/mysql57-no-marker.log|" "$preflight_env" >"$preflight_no_marker_env"
preflight_no_marker_report="$WORK_DIR/preflight-no-marker.md"
preflight_no_marker_json="$WORK_DIR/preflight-no-marker.json"
set +e
MOCHAT_PRODUCTION_EVIDENCE_PREFLIGHT_STRICT=1 \
  run_preflight "$doctor_json" "$preflight_no_marker_env" "$preflight_no_marker_report" "$preflight_no_marker_json" >/dev/null 2>&1
preflight_no_marker_rc=$?
set -e
if [ "$preflight_no_marker_rc" -eq 0 ]; then
  echo "production evidence env preflight unexpectedly passed without mysql pass marker" >&2
  sed -n '1,180p' "$preflight_no_marker_report" >&2
  exit 1
fi
grep -Fq "通过标记" "$preflight_no_marker_report"

preflight_placeholder_url_env="$WORK_DIR/readiness-placeholder-url.env.local"
sed "s|^MOCHAT_REAL_WECOM_BASE_URL=.*|MOCHAT_REAL_WECOM_BASE_URL=https://example.com|" "$preflight_env" >"$preflight_placeholder_url_env"
preflight_placeholder_url_report="$WORK_DIR/preflight-placeholder-url.md"
preflight_placeholder_url_json="$WORK_DIR/preflight-placeholder-url.json"
set +e
MOCHAT_PRODUCTION_EVIDENCE_PREFLIGHT_STRICT=1 \
  run_preflight "$doctor_json" "$preflight_placeholder_url_env" "$preflight_placeholder_url_report" "$preflight_placeholder_url_json" >/dev/null 2>&1
preflight_placeholder_url_rc=$?
set -e
if [ "$preflight_placeholder_url_rc" -eq 0 ]; then
  echo "production evidence env preflight unexpectedly passed with placeholder base URL" >&2
  sed -n '1,180p' "$preflight_placeholder_url_report" >&2
  exit 1
fi
grep -Fq "示例域名或本机地址" "$preflight_placeholder_url_report"

preflight_weak_stability_env="$WORK_DIR/readiness-weak-stability.env.local"
sed "s|^MOCHAT_STABILITY_MONITOR_EVIDENCE=.*|MOCHAT_STABILITY_MONITOR_EVIDENCE=@$preflight_fixture_dir/stability-monitor-weak.txt|" "$preflight_env" >"$preflight_weak_stability_env"
preflight_weak_stability_report="$WORK_DIR/preflight-weak-stability.md"
preflight_weak_stability_json="$WORK_DIR/preflight-weak-stability.json"
set +e
MOCHAT_PRODUCTION_EVIDENCE_PREFLIGHT_STRICT=1 \
  run_preflight "$doctor_json" "$preflight_weak_stability_env" "$preflight_weak_stability_report" "$preflight_weak_stability_json" >/dev/null 2>&1
preflight_weak_stability_rc=$?
set -e
if [ "$preflight_weak_stability_rc" -eq 0 ]; then
  echo "production evidence env preflight unexpectedly passed with weak stability monitor evidence" >&2
  sed -n '1,180p' "$preflight_weak_stability_report" >&2
  exit 1
fi
grep -Fq "路由覆盖" "$preflight_weak_stability_report"

preflight_ready_report="$WORK_DIR/preflight-ready.md"
preflight_ready_json="$WORK_DIR/preflight-ready.json"
MOCHAT_PRODUCTION_EVIDENCE_PREFLIGHT_STRICT=1 \
  run_preflight "$doctor_json" "$preflight_env" "$preflight_ready_report" "$preflight_ready_json" >/dev/null
grep -Fq "采集环境变量齐备" "$preflight_ready_report"
python3 - "$preflight_ready_json" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
assert payload["schema"] == 1
assert payload["all_ready"] is True
assert payload["started_24h_run"] is False
assert payload["accessed_production"] is False
assert payload["ready_count"] == 6
serialized = json.dumps(payload, ensure_ascii=False)
assert "dashboard-jwt" not in serialized
assert "tenant-a-token" not in serialized
assert "Bearer " not in serialized
PY

weak_stability_capture="$WORK_DIR/weak-stability-capture.md"
set +e
MOCHAT_STABILITY_MONITOR_EVIDENCE=@$preflight_fixture_dir/stability-monitor-weak.txt \
  MOCHAT_STABILITY_HEALTH_EVIDENCE=@$preflight_fixture_dir/stability-health.txt \
  MOCHAT_STABILITY_RESOURCE_EVIDENCE=@$preflight_fixture_dir/stability-resource.txt \
  MOCHAT_STABILITY_TIME_RANGE="2026-07-08 10:00:00 CST 至 2026-07-08 10:30:00 CST" \
  MOCHAT_STABILITY_LOG_REF=weak-stability-log-ref \
  MOCHAT_STABILITY_OUT="$weak_stability_capture" \
  ./scripts/capture_stability_evidence.sh >"$WORK_DIR/weak-stability-capture.out" 2>&1
weak_stability_capture_rc=$?
set -e
if [ "$weak_stability_capture_rc" -eq 0 ]; then
  echo "capture_stability_evidence.sh unexpectedly passed with weak monitor evidence" >&2
  sed -n '1,180p' "$weak_stability_capture" >&2
  exit 1
fi
grep -Fq "路由覆盖" "$weak_stability_capture"

strong_stability_capture="$WORK_DIR/strong-stability-capture.md"
MOCHAT_STABILITY_MONITOR_EVIDENCE=@$preflight_fixture_dir/stability-monitor.txt \
  MOCHAT_STABILITY_HEALTH_EVIDENCE=@$preflight_fixture_dir/stability-health.txt \
  MOCHAT_STABILITY_RESOURCE_EVIDENCE=@$preflight_fixture_dir/stability-resource.txt \
  MOCHAT_STABILITY_TIME_RANGE="2026-07-08 10:00:00 CST 至 2026-07-08 10:30:00 CST" \
  MOCHAT_STABILITY_LOG_REF=strong-stability-log-ref \
  MOCHAT_STABILITY_OUT="$strong_stability_capture" \
  ./scripts/capture_stability_evidence.sh >/dev/null
grep -Fq "结论：通过" "$strong_stability_capture"
grep -Fq "源码指纹：$SOURCE_FINGERPRINT" "$strong_stability_capture"

cat >"$EVIDENCE_DIR/real-wecom.md" <<'EOF'
# 真实企业微信账号联调证据

结论：通过

授权、回调、通讯录、客户、客户群、标签、素材均已完成真实账号验收。
企微应用消息链路验收通过。
EOF

cat >"$EVIDENCE_DIR/real-wechat-open.md" <<'EOF'
# 真实微信开放平台联调证据

结论：通过

ticket、预授权、授权回跳、资料回填、取消授权、消息回调均已完成真实平台验收。
EOF

cat >"$EVIDENCE_DIR/real-saas-tenants.md" <<'EOF'
# 真实 SaaS 多租户数据回归证据

结论：通过

两个租户的租户、菜单权限、企业归属、资源额度、上传账本、异步任务、告警隔离均已回归通过。
EOF

cat >"$EVIDENCE_DIR/prod-frontend.md" <<'EOF'
# 生产前端浏览器回归证据

结论：通过

dashboard、sidebar、operation 生产构建已完成浏览器回归，异常路径已验证。
EOF

cat >"$EVIDENCE_DIR/stability.md" <<'EOF'
# 短稳/外部监控稳定性记录

结论：通过

短稳回归监控结论通过，健康检查、路由覆盖、错误率和资源使用均已记录。
EOF

cat >"$EVIDENCE_DIR/mysql57-amd64.log" <<'EOF'
mysql:5.7 is amd64-only and skipped on arm64
EOF

arm64_report="$WORK_DIR/arm64-report.md"
expect_fail "arm64 mysql evidence rejection" "$arm64_report"
grep -Fq "arm64 skip" "$arm64_report"

set +e
MOCHAT_MYSQL57_AMD64_EVIDENCE_TARGET="$WORK_DIR/imported-arm64-skip.log" \
  ./scripts/import_mysql57_amd64_evidence.sh "$EVIDENCE_DIR/mysql57-amd64.log" >"$WORK_DIR/import-arm64-skip.out" 2>&1
import_arm64_rc=$?
set -e
if [ "$import_arm64_rc" -eq 0 ]; then
  echo "arm64 mysql evidence import unexpectedly passed" >&2
  cat "$WORK_DIR/import-arm64-skip.out" >&2
  exit 1
fi
grep -Fq "arm64 skip log" "$WORK_DIR/import-arm64-skip.out"

cat >"$EVIDENCE_DIR/mysql57-amd64.log" <<'EOF'
mysql57 amd64 CI gate passed
mysql 5.7 schema migration smoke passed
EOF

set +e
MOCHAT_MYSQL57_AMD64_EVIDENCE_TARGET="$WORK_DIR/imported-mysql57-no-source-fingerprint.log" \
  ./scripts/import_mysql57_amd64_evidence.sh "$EVIDENCE_DIR/mysql57-amd64.log" >"$WORK_DIR/import-mysql57-no-source-fingerprint.out" 2>&1
import_no_source_fingerprint_rc=$?
set -e
if [ "$import_no_source_fingerprint_rc" -eq 0 ]; then
  echo "mysql57 evidence import unexpectedly passed without source fingerprint" >&2
  cat "$WORK_DIR/import-mysql57-no-source-fingerprint.out" >&2
  exit 1
fi
grep -Fq "source fingerprint" "$WORK_DIR/import-mysql57-no-source-fingerprint.out"

cat >"$EVIDENCE_DIR/mysql57-amd64.log" <<EOF
源码指纹：$SOURCE_FINGERPRINT
mysql57 amd64 CI gate passed
mysql 5.7 schema migration smoke passed
EOF

MOCHAT_MYSQL57_AMD64_EVIDENCE_TARGET="$WORK_DIR/imported-mysql57-amd64.log" \
  ./scripts/import_mysql57_amd64_evidence.sh "$EVIDENCE_DIR/mysql57-amd64.log" >/dev/null
grep -Fq "mysql57 amd64 CI gate passed" "$WORK_DIR/imported-mysql57-amd64.log"

python3 - "$WORK_DIR/mysql57-amd64-evidence.zip" "$EVIDENCE_DIR/mysql57-amd64.log" <<'PY'
import pathlib
import sys
import zipfile

zip_path = pathlib.Path(sys.argv[1])
log_path = pathlib.Path(sys.argv[2])
with zipfile.ZipFile(zip_path, "w") as archive:
    archive.write(log_path, "mysql57-amd64.log")
PY
MOCHAT_MYSQL57_AMD64_EVIDENCE_TARGET="$WORK_DIR/imported-mysql57-amd64-from-zip.log" \
  ./scripts/import_mysql57_amd64_evidence.sh "$WORK_DIR/mysql57-amd64-evidence.zip" >/dev/null
grep -Fq "mysql 5.7 schema migration smoke passed" "$WORK_DIR/imported-mysql57-amd64-from-zip.log"

keyword_only_report="$WORK_DIR/keyword-only-report.md"
expect_fail "keyword-only evidence rejection" "$keyword_only_report"
grep -Fq "证据缺少结构化记录项" "$keyword_only_report"

cat >"$EVIDENCE_DIR/real-wecom.md" <<'EOF'
# 真实企业微信账号联调证据

- 执行时间：2026-07-07 10:00:00 CST
- 结论：通过

授权、回调、通讯录、客户、客户群、标签、素材均已完成真实账号验收。
企微应用消息链路验收通过。

## 证据记录

- 请求或操作路径：授权回跳、通讯录同步、客户同步、客户群同步、标签同步、素材上传。
- 关键响应摘要：所有接口返回 errcode=0，回调解密成功并落库。
- 失败重试或异常路径：重复回调幂等通过。
- 截图 / 日志 / 工单引用：real-wecom-run-001。
EOF

cat >"$EVIDENCE_DIR/real-wechat-open.md" <<'EOF'
# 真实微信开放平台联调证据

- 执行时间：2026-07-07 10:10:00 CST
- 结论：通过

ticket、预授权、授权回跳、资料回填、取消授权、消息回调均已完成真实平台验收。

## 证据记录

- 请求或操作路径：ticket 接收、预授权码、授权回跳、公众号资料回填、取消授权、消息回调。
- 关键响应摘要：component_access_token、authorizer_info 和消息回调均处理成功。
- 失败重试或异常路径：重复 ticket 幂等通过。
- 截图 / 日志 / 工单引用：real-wechat-open-run-001。
EOF

cat >"$EVIDENCE_DIR/real-saas-tenants.md" <<'EOF'
# 真实 SaaS 多租户数据回归证据

- 执行时间：2026-07-07 10:20:00 CST
- 租户 A：tenant-a
- 租户 B：tenant-b
- 结论：通过

两个租户的租户、菜单权限、企业归属、资源额度、上传账本、异步任务、告警隔离均已回归通过。

## 证据记录

- 跨租户访问隔离：租户 A 不能读取租户 B 企业、菜单和上传对象。
- 用量刷新和额度拦截：资源额度命中后写接口返回拦截错误。
- 上传账本与回收：上传、替换和删除后账本与 storage_mb 一致。
- 异步任务执行记录：双租户 async_executions 分别计数。
- 告警通知和处置：租户 A 超额只触发租户 A 告警。
EOF

cat >"$EVIDENCE_DIR/prod-frontend.md" <<'EOF'
# 生产前端浏览器回归证据

- 执行时间：2026-07-07 10:30:00 CST
- 构建版本：prod-build-001
- 结论：通过

dashboard、sidebar、operation 生产构建已完成浏览器回归，异常路径已验证。

## 证据记录

- 主要业务路径：dashboard 登录、企业选择、客户、客户群、营销插件；sidebar 客户详情；operation 任务宝活动。
- 异常路径：未登录、无权限、资源不存在均返回预期页面或错误。
- 控制台错误：无阻断性 console error。
- 网络请求错误：无本地资源 4xx/5xx，无业务 API 非预期 5xx。
- 截图 / 录像 / 报告引用：prod-frontend-run-001。
EOF

cat >"$EVIDENCE_DIR/stability.md" <<'EOF'
# 短稳/外部监控稳定性记录

- 时间范围：2026-07-07 00:00:00 CST 至 2026-07-08 00:00:00 CST
- 结论：通过

短稳回归监控结论通过，健康检查、路由覆盖、错误率和资源使用均已记录。

## 证据记录

- 短稳回归：30 分钟短稳探测无异常退出。
- 健康检查：/readyz 持续通过。
- 路由覆盖：route_total=224，missing_route_total=0。
- 错误率：业务 5xx 为 0。
- 资源使用：RSS 和连接数稳定。
- 日志 / 监控引用：prod-monitor-run-001。
EOF

cat >"$EVIDENCE_DIR/mysql57-amd64.log" <<EOF
源码指纹：$SOURCE_FINGERPRINT
mysql57 amd64 CI gate passed
mysql 5.7 schema migration smoke passed
EOF

missing_source_fingerprint_report="$WORK_DIR/missing-source-fingerprint-report.md"
expect_fail "missing source fingerprint evidence rejection" "$missing_source_fingerprint_report"
grep -Fq "源码指纹" "$missing_source_fingerprint_report"

missing_source_fingerprint_pack_dir="$WORK_DIR/missing-source-fingerprint-pack"
set +e
run_pack "$missing_source_fingerprint_pack_dir" >/dev/null 2>&1
missing_source_fingerprint_pack_rc=$?
set -e
if [ "$missing_source_fingerprint_pack_rc" -eq 0 ]; then
  echo "missing source fingerprint evidence pack unexpectedly passed" >&2
  [ -f "$missing_source_fingerprint_pack_dir/index.md" ] && sed -n '1,180p' "$missing_source_fingerprint_pack_dir/index.md" >&2
  exit 1
fi
grep -Fq "源码指纹拒绝，未复制" "$missing_source_fingerprint_pack_dir/index.md"
if [ -f "$missing_source_fingerprint_pack_dir/real-wecom.md" ]; then
  echo "missing source fingerprint evidence was copied into production evidence pack" >&2
  sed -n '1,80p' "$missing_source_fingerprint_pack_dir/real-wecom.md" >&2
  exit 1
fi

printf '\n- 源码指纹：%s\n' "$STALE_SOURCE_FINGERPRINT" >>"$EVIDENCE_DIR/real-wecom.md"
stale_source_fingerprint_report="$WORK_DIR/stale-source-fingerprint-report.md"
expect_fail "stale source fingerprint evidence rejection" "$stale_source_fingerprint_report"
grep -Fq "源码指纹与当前源码不匹配" "$stale_source_fingerprint_report"

stale_source_fingerprint_pack_dir="$WORK_DIR/stale-source-fingerprint-pack"
set +e
run_pack "$stale_source_fingerprint_pack_dir" >/dev/null 2>&1
stale_source_fingerprint_pack_rc=$?
set -e
if [ "$stale_source_fingerprint_pack_rc" -eq 0 ]; then
  echo "stale source fingerprint evidence pack unexpectedly passed" >&2
  [ -f "$stale_source_fingerprint_pack_dir/index.md" ] && sed -n '1,180p' "$stale_source_fingerprint_pack_dir/index.md" >&2
  exit 1
fi
grep -Fq "源码指纹拒绝，未复制" "$stale_source_fingerprint_pack_dir/index.md"
grep -Fq "与当前" "$stale_source_fingerprint_pack_dir/index.md"
if [ -f "$stale_source_fingerprint_pack_dir/real-wecom.md" ]; then
  echo "stale source fingerprint evidence was copied into production evidence pack" >&2
  sed -n '1,80p' "$stale_source_fingerprint_pack_dir/real-wecom.md" >&2
  exit 1
fi

cat >"$EVIDENCE_DIR/real-wecom.md" <<EOF
# 真实企业微信账号联调证据

- 执行时间：2026-07-07 10:00:00 CST
- 源码指纹：$SOURCE_FINGERPRINT
- 结论：通过

授权、回调、通讯录、客户、客户群、标签、素材均已完成真实账号验收。
企微应用消息链路验收通过。

## 证据记录

- 请求或操作路径：授权回跳、通讯录同步、客户同步、客户群同步、标签同步、素材上传。
- 关键响应摘要：所有接口返回 errcode=0，回调解密成功并落库。
- 失败重试或异常路径：重复回调幂等通过。
- 截图 / 日志 / 工单引用：real-wecom-run-001。
EOF

for evidence_file in \
  "$EVIDENCE_DIR/real-wechat-open.md" \
  "$EVIDENCE_DIR/real-saas-tenants.md" \
  "$EVIDENCE_DIR/prod-frontend.md" \
  "$EVIDENCE_DIR/stability.md"
do
  printf '\n- 源码指纹：%s\n' "$SOURCE_FINGERPRINT" >>"$evidence_file"
done

cat >>"$EVIDENCE_DIR/real-wecom.md" <<'EOF'
- 调试头：Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ0ZW5hbnQiOiJwcm9kIn0.signature123456789
EOF

sensitive_report="$WORK_DIR/sensitive-report.md"
expect_fail "sensitive evidence rejection" "$sensitive_report"
grep -Fq "未脱敏敏感值" "$sensitive_report"

sensitive_pack_dir="$WORK_DIR/sensitive-pack"
set +e
run_pack "$sensitive_pack_dir" >/dev/null 2>&1
sensitive_pack_rc=$?
set -e
if [ "$sensitive_pack_rc" -eq 0 ]; then
  echo "sensitive evidence pack unexpectedly passed" >&2
  [ -f "$sensitive_pack_dir/index.md" ] && sed -n '1,180p' "$sensitive_pack_dir/index.md" >&2
  exit 1
fi
grep -Fq "敏感值拒绝，未复制" "$sensitive_pack_dir/index.md"
if [ -f "$sensitive_pack_dir/real-wecom.md" ]; then
  echo "sensitive source evidence was copied into production evidence pack" >&2
  sed -n '1,80p' "$sensitive_pack_dir/real-wecom.md" >&2
  exit 1
fi
python3 - "$sensitive_pack_dir/env.production-evidence" "$EVIDENCE_DIR/real-wecom.md" "$sensitive_pack_dir/real-wecom.md" <<'PY'
import pathlib
import sys

env_file = pathlib.Path(sys.argv[1])
source = pathlib.Path(sys.argv[2]).resolve()
expected = pathlib.Path(sys.argv[3]).resolve()
values = {}
for line in env_file.read_text(encoding="utf-8").splitlines():
    if "=" in line:
        key, value = line.split("=", 1)
        values[key] = value

actual_raw = values.get("MOCHAT_EVIDENCE_REAL_WECOM")
if not actual_raw:
    raise SystemExit("MOCHAT_EVIDENCE_REAL_WECOM missing from env.production-evidence")

actual_path = pathlib.Path(actual_raw)
if not actual_path.is_absolute():
    actual_path = pathlib.Path.cwd() / actual_path
actual = actual_path.resolve()
if actual == source:
    raise SystemExit("sensitive source evidence path leaked into production evidence env file")
if actual != expected:
    raise SystemExit(f"unexpected blocked evidence env path: {actual} != {expected}")
PY

cat >"$EVIDENCE_DIR/real-wecom.md" <<'EOF'
# 真实企业微信账号联调证据

- 执行时间：2026-07-07 10:00:00 CST
- 生产站点：http://127.0.0.1:18080
- 结论：通过

授权、回调、通讯录、客户、客户群、标签、素材均已完成真实账号验收。
企微应用消息链路验收通过。

## 证据记录

- 请求或操作路径：授权回跳、通讯录同步、客户同步、客户群同步、标签同步、素材上传。
- 关键响应摘要：所有接口返回 errcode=0，回调解密成功并落库。
- 失败重试或异常路径：重复回调幂等通过。
- 截图 / 日志 / 工单引用：real-wecom-run-001。
EOF

non_prod_url_report="$WORK_DIR/non-production-url-report.md"
expect_fail "non-production URL evidence rejection" "$non_prod_url_report"
grep -Fq "示例域名或本机地址" "$non_prod_url_report"

non_prod_url_pack_dir="$WORK_DIR/non-production-url-pack"
set +e
run_pack "$non_prod_url_pack_dir" >/dev/null 2>&1
non_prod_url_pack_rc=$?
set -e
if [ "$non_prod_url_pack_rc" -eq 0 ]; then
  echo "non-production URL evidence pack unexpectedly passed" >&2
  [ -f "$non_prod_url_pack_dir/index.md" ] && sed -n '1,180p' "$non_prod_url_pack_dir/index.md" >&2
  exit 1
fi
grep -Fq "非生产地址拒绝，未复制" "$non_prod_url_pack_dir/index.md"
if [ -f "$non_prod_url_pack_dir/real-wecom.md" ]; then
  echo "non-production URL source evidence was copied into production evidence pack" >&2
  sed -n '1,80p' "$non_prod_url_pack_dir/real-wecom.md" >&2
  exit 1
fi
python3 - "$non_prod_url_pack_dir/env.production-evidence" "$EVIDENCE_DIR/real-wecom.md" "$non_prod_url_pack_dir/real-wecom.md" <<'PY'
import pathlib
import sys

env_file = pathlib.Path(sys.argv[1])
source = pathlib.Path(sys.argv[2]).resolve()
expected = pathlib.Path(sys.argv[3]).resolve()
values = {}
for line in env_file.read_text(encoding="utf-8").splitlines():
    if "=" in line:
        key, value = line.split("=", 1)
        values[key] = value

actual_raw = values.get("MOCHAT_EVIDENCE_REAL_WECOM")
if not actual_raw:
    raise SystemExit("MOCHAT_EVIDENCE_REAL_WECOM missing from env.production-evidence")

actual_path = pathlib.Path(actual_raw)
if not actual_path.is_absolute():
    actual_path = pathlib.Path.cwd() / actual_path
actual = actual_path.resolve()
if actual == source:
    raise SystemExit("non-production URL source evidence path leaked into production evidence env file")
if actual != expected:
    raise SystemExit(f"unexpected blocked evidence env path: {actual} != {expected}")
PY

start_secret_fixture
redacted_capture="$WORK_DIR/redacted-real-wecom.md"
MOCHAT_PRODUCTION_EVIDENCE_ALLOW_NON_PROD_URLS=1 \
  MOCHAT_REAL_WECOM_BASE_URL="http://127.0.0.1:$SECRET_FIXTURE_PORT" \
  MOCHAT_REAL_WECOM_TOKEN="fixture-dashboard-token-abcdefghijklmnopqrstuvwxyz" \
  MOCHAT_REAL_WECOM_CALLBACK_EVIDENCE='callback access_token=callback-token-abcdefghijklmnopqrstuvwxyz verify_ticket=verify-ticket-abcdefghijklmnopqrstuvwxyz' \
  MOCHAT_REAL_WECOM_AGENT_MESSAGE_EVIDENCE='agent component_access_token=agent-component-token-abcdefghijklmnopqrstuvwxyz' \
  MOCHAT_REAL_WECOM_LOG_REF="redaction-fixture-log" \
  MOCHAT_REAL_WECOM_OUT="$redacted_capture" \
  ./scripts/capture_real_wecom_evidence.sh >/dev/null
grep -Fq "<redacted>" "$redacted_capture"
grep -Fq "源码指纹：$SOURCE_FINGERPRINT" "$redacted_capture"
if rg -q 'raw-access-token|raw-component-access-token|raw-authorizer-refresh-token|raw-corpsecret|raw-query-token|callback-token|verify-ticket|agent-component-token' "$redacted_capture"; then
  echo "capture_real_wecom_evidence.sh wrote unredacted secret material" >&2
  sed -n '1,180p' "$redacted_capture" >&2
  exit 1
fi

cat >"$EVIDENCE_DIR/real-wecom.md" <<EOF
# 真实企业微信账号联调证据

- 执行时间：2026-07-07 10:00:00 CST
- 源码指纹：$SOURCE_FINGERPRINT
- 结论：通过

授权、回调、通讯录、客户、客户群、标签、素材均已完成真实账号验收。
企微应用消息链路验收通过。

## 证据记录

- 请求或操作路径：授权回跳、通讯录同步、客户同步、客户群同步、标签同步、素材上传。
- 关键响应摘要：所有接口返回 errcode=0，回调解密成功并落库。
- 失败重试或异常路径：重复回调幂等通过。
- 截图 / 日志 / 工单引用：real-wecom-run-001。
EOF

valid_report="$WORK_DIR/valid-report.md"
expect_pass "valid production evidence" "$valid_report"
grep -Fq "当前结论：**可进入生产候选复核**" "$valid_report"
grep -Fq "已验证" "$valid_report"
python3 - "${valid_report%.md}.json" "$SOURCE_FINGERPRINT" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
assert payload["schema"] == 1
assert payload["strict"] is True
assert payload["skip_quick"] is True
assert payload["quick_checks"] == []
assert payload["quick_ok"] is True
assert payload["evidence_ok"] is True
assert payload["ok"] is True
assert payload["current_source_fingerprint"] == sys.argv[2]
assert len(payload["evidence"]) == 6
assert all(item["valid"] is True for item in payload["evidence"])
assert payload["invalid_evidence"] == []
assert payload["missing_envs"] == []
serialized = json.dumps(payload, ensure_ascii=False)
assert "Bearer " not in serialized
assert "raw-access-token" not in serialized
PY

valid_doctor_report="$WORK_DIR/valid-readiness.md"
valid_doctor_output="$(run_doctor "$valid_doctor_report")"
valid_doctor_json="$(printf '%s\n' "$valid_doctor_output" | tail -n 1)"
valid_preflight_existing_report="$WORK_DIR/preflight-valid-standard-files.md"
valid_preflight_existing_json="$WORK_DIR/preflight-valid-standard-files.json"
missing_valid_env="$WORK_DIR/valid-standard-files.env.local"
MOCHAT_PRODUCTION_EVIDENCE_PREFLIGHT_STRICT=1 \
  run_preflight "$valid_doctor_json" "$missing_valid_env" "$valid_preflight_existing_report" "$valid_preflight_existing_json" >/dev/null
grep -Fq "标准生产证据文件均有效" "$valid_preflight_existing_report"
grep -Fq "标准文件已存在" "$valid_preflight_existing_report"
python3 - "$valid_preflight_existing_json" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
assert payload["schema"] == 1
assert payload["all_ready"] is True
assert payload["env_file_exists"] is False
assert payload["ready_count"] == 6
assert payload["standard_file_ready_count"] == 6
assert payload["capture_env_ready_count"] == 0
assert payload["not_ready_count"] == 0
assert payload["warnings"] == []
assert payload["accessed_production"] is False
assert payload["started_24h_run"] is False
serialized = json.dumps(payload, ensure_ascii=False)
assert "Bearer " not in serialized
assert "dashboard-jwt" not in serialized
PY

valid_pack_dir="$WORK_DIR/valid-pack"
run_pack "$valid_pack_dir" >/dev/null
grep -Fq "通过，可进入生产候选复核" "$valid_pack_dir/index.md"
grep -Fq "env.production-evidence" "$valid_pack_dir/index.md"
grep -Fq "production-evidence.json" "$valid_pack_dir/index.md"
python3 - "$valid_pack_dir/production-evidence.json" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
assert payload["schema"] == 1
assert payload["evidence_ok"] is True
assert payload["ok"] is True
assert len(payload["evidence"]) == 6
PY

echo "production evidence gate smoke passed"
