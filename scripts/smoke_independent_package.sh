#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-independent-package.XXXXXX")"
PACKAGE_DIR="$WORK_DIR/mochat-go"
KEEP_WORK_DIR="${MOCHAT_KEEP_INDEPENDENT_PACKAGE_WORK_DIR:-0}"

cleanup() {
  if [ "$KEEP_WORK_DIR" != "1" ]; then
    rm -rf "$WORK_DIR"
  else
    echo "kept independent package work dir: $WORK_DIR"
  fi
}
trap cleanup EXIT INT TERM

if [ "${1:-}" = "-h" ] || [ "${1:-}" = "--help" ]; then
  cat <<'EOF'
Usage: ./scripts/smoke_independent_package.sh

复制当前 mochat-go 到一个没有原 mochat/ 兄弟目录的临时目录，并在该临时包内验证：
  - standalone 独立性静态审计
  - embedded 队列注解覆盖审计
  - standalone smoke 运行时不读取 PHP source / 外部 manifest / PHP upstream

环境变量：
  MOCHAT_KEEP_INDEPENDENT_PACKAGE_WORK_DIR
      设为 1 时保留临时目录，便于人工检查。

该脚本不会启动 24 小时持续运行。
EOF
  exit 0
fi

python3 - "$ROOT_DIR" "$PACKAGE_DIR" <<'PY'
import shutil
import sys
from pathlib import Path

src = Path(sys.argv[1])
dst = Path(sys.argv[2])

ignored_dirs = {
    ".git",
    "node_modules",
    "output",
    "storage",
    "tmp",
}
ignored_paths = {
    ("docs", "evidence", "latest"),
    ("docs", "evidence", "production", "candidate"),
}

def ignore(dir_path, names):
    rel = Path(dir_path).relative_to(src)
    ignored = set()
    for name in names:
        child_rel = rel / name
        if name in ignored_dirs:
            ignored.add(name)
            continue
        if name == ".env" or (name.startswith(".env.") and name != ".env.example"):
            ignored.add(name)
            continue
        if tuple(child_rel.parts) in ignored_paths:
            ignored.add(name)
    return ignored

shutil.copytree(src, dst, ignore=ignore)
PY

if [ -e "$WORK_DIR/mochat" ]; then
  echo "unexpected original mochat checkout in independent package work dir: $WORK_DIR/mochat" >&2
  exit 1
fi

cd "$PACKAGE_DIR"

export MOCHAT_QUEUE_AUDIT_SOURCE=embedded
export MOCHAT_SOURCE_ROOT="$WORK_DIR/should-not-read-php-source"
export MOCHAT_COMPAT_MANIFEST="$WORK_DIR/should-not-read-manifest.json"

test ! -e "$MOCHAT_SOURCE_ROOT"
test ! -e "$MOCHAT_COMPAT_MANIFEST"
for legal_file in LICENSE NOTICE.md SOURCE_OFFER.md MODIFICATIONS.md THIRD_PARTY_NOTICES.md; do
  test -s "$legal_file" || { echo "independent package missing $legal_file" >&2; exit 1; }
done
test -s "web/apps/dashboard/dist/index.html" || { echo "independent package missing React dashboard build" >&2; exit 1; }
grep -q 'GNU GENERAL PUBLIC LICENSE' LICENSE
grep -q '0056_saas_domain_delivery' MODIFICATIONS.md
grep -q '0057_saas_release_readiness' MODIFICATIONS.md
grep -q '0061_saas_service_account_usage_alerts' MODIFICATIONS.md
grep -q '0063_saas_audit_anchor_signatures' MODIFICATIONS.md
grep -q '0065_saas_service_account_key_pepper_ring' MODIFICATIONS.md
grep -q '0066_saas_alert_credential_encryption' MODIFICATIONS.md
grep -q '0067_wecom_credential_encryption' MODIFICATIONS.md
grep -q '0068_wechat_open_credential_encryption' MODIFICATIONS.md
grep -q '0069_saas_release_candidate_approval' MODIFICATIONS.md
grep -q '0070_saas_approval_policy_change_guard' MODIFICATIONS.md
grep -q '0071_saas_backup_policy_change_guard' MODIFICATIONS.md
grep -q '0072_saas_backup_cleanup_saga' MODIFICATIONS.md
grep -q '0073_saas_compliance_export_deletion_saga' MODIFICATIONS.md
grep -q '0074_saas_compliance_legal_hold_release_guard' MODIFICATIONS.md
grep -q '0076_saas_identity_policy_change_guard' MODIFICATIONS.md
grep -q '0077_saas_tenant_disable_approval_guard' MODIFICATIONS.md
grep -q '0078_saas_critical_approval_policy_guard' MODIFICATIONS.md
grep -q '0079_saas_service_account_key_revoke_guard' MODIFICATIONS.md
grep -q '0080_saas_service_account_update_guard' MODIFICATIONS.md
grep -q '0081_saas_service_account_key_rotate_guard' MODIFICATIONS.md
grep -q '0082_saas_service_account_create_guard' MODIFICATIONS.md
grep -q '0083_saas_identity_mfa_reset_guard' MODIFICATIONS.md
grep -q '0085_saas_tenant_package_assignment_guard' MODIFICATIONS.md
grep -q '0088_saas_subscription_transition_approval_guard' MODIFICATIONS.md
grep -q '0089_saas_invoice_issue_approval_guard' MODIFICATIONS.md
grep -q '0090_saas_payment_order_create_approval_guard' MODIFICATIONS.md
grep -q '0091_saas_payment_settlement_close_guard' MODIFICATIONS.md
grep -q '0092_saas_payment_settlement_reopen_guard' MODIFICATIONS.md
grep -q '0093_saas_payment_settlement_resolve_guard' MODIFICATIONS.md
grep -q '0094_saas_tenant_domain_command_guard' MODIFICATIONS.md
grep -q '0095_saas_tenant_domain_create_guard' MODIFICATIONS.md
grep -q '0096_saas_tenant_enable_approval_guard' MODIFICATIONS.md
grep -q '0097_saas_release_evidence_action_tracking' MODIFICATIONS.md
grep -q '^\*\*/\.env\.\*$' .dockerignore
grep -q '^!\*\*/\.env\.example$' .dockerignore
test ! -e deploy/standalone/.env.local
grep -q 'Vue Router' THIRD_PARTY_NOTICES.md

for migration in deploy/standalone/migrations/*.up.sql; do
  migration_name="$(basename "$migration")"
  grep -Fq "./migrations/$migration_name:/docker-entrypoint-initdb.d/" deploy/standalone/docker-compose.yml
done

python3 scripts/source_fingerprint.py | python3 -c '
import json
import pathlib
import sys

payload = json.load(sys.stdin)
for item in payload["files"]:
    for part in pathlib.PurePosixPath(item["path"]).parts:
        if part == ".env" or (part.startswith(".env.") and part != ".env.example"):
            raise SystemExit(f"private environment file entered source fingerprint: {item['"'"'path'"'"']}")
'

./scripts/audit_standalone_independence.sh
./scripts/audit_queue_annotation_coverage.sh
./scripts/smoke_standalone.sh

python3 - "$PACKAGE_DIR" <<'PY'
import json
import pathlib
import subprocess
import sys

repo = pathlib.Path(sys.argv[1])
manifest = json.loads((repo / "internal/server/compat_manifest_embedded.json").read_text(encoding="utf-8"))
if len(manifest.get("routes", [])) != 224:
    raise SystemExit(f"expected 224 manifest routes, got {len(manifest.get('routes', []))}")
if len(manifest.get("tables", [])) != 70:
    raise SystemExit(f"expected 70 manifest tables, got {len(manifest.get('tables', []))}")

for forbidden in ["../mochat", "mochat/api-server"]:
    completed = subprocess.run(
        ["rg", "-n", "-F", forbidden, "deploy/standalone/docker-compose.yml", "Dockerfile", "scripts/smoke_standalone.sh"],
        cwd=repo,
        text=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        check=False,
    )
    if completed.returncode == 0:
        raise SystemExit(f"forbidden reference {forbidden!r} found:\n{completed.stdout}")

print("independent package manifest check passed: routes=224 tables=70")
PY

echo "independent package smoke passed"
