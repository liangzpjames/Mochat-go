#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

if [ "${1:-}" = "-h" ] || [ "${1:-}" = "--help" ]; then
  cat <<'EOF'
Usage: ./scripts/production_candidate_gate.sh

执行生产候选总门禁：
  1. 默认先生成全套非 PHP 本地短验收证据包。
  2. 再执行严格生产证据检查，要求外部生产证据文件有效。
  3. 校验候选证据包的源码与验收指纹仍匹配当前工作区。
  4. 输出汇总报告到 docs/phases/phase-pre0-standalone/evidence/production/candidate/。

环境变量：
  MOCHAT_PRODUCTION_CANDIDATE_DIR
      输出目录，默认 docs/phases/phase-pre0-standalone/evidence/production/candidate。
  MOCHAT_PRODUCTION_CANDIDATE_LOCAL_SUITES
      本地短验收套件，默认 core saas workers cron frontend mysql57。
  MOCHAT_PRODUCTION_CANDIDATE_SKIP_LOCAL
      设为 1 时跳过本地短验收，只复核外部生产证据、目标完成度和 latest 指纹。

生产证据环境变量：
  MOCHAT_EVIDENCE_MYSQL57_AMD64
  MOCHAT_EVIDENCE_REAL_WECOM
  MOCHAT_EVIDENCE_REAL_WECHAT_OPEN
  MOCHAT_EVIDENCE_REAL_SAAS_TENANTS
  MOCHAT_EVIDENCE_PROD_FRONTEND
  MOCHAT_EVIDENCE_STABILITY

该脚本不会启动 24 小时持续运行。
EOF
  exit 0
fi

OUT_DIR="${MOCHAT_PRODUCTION_CANDIDATE_DIR:-docs/phases/phase-pre0-standalone/evidence/production/candidate}"
LOCAL_SUITES="${MOCHAT_PRODUCTION_CANDIDATE_LOCAL_SUITES:-core saas workers cron frontend mysql57}"
SKIP_LOCAL="${MOCHAT_PRODUCTION_CANDIDATE_SKIP_LOCAL:-0}"
RESULTS_JSONL="$OUT_DIR/results.jsonl"
REPORT="$OUT_DIR/index.md"

case "$SKIP_LOCAL" in
  0|1)
    ;;
  *)
    echo "unknown MOCHAT_PRODUCTION_CANDIDATE_SKIP_LOCAL=$SKIP_LOCAL" >&2
    echo "valid values: 0, 1" >&2
    exit 2
    ;;
esac

mkdir -p "$OUT_DIR"
rm -f "$RESULTS_JSONL" "$REPORT" \
  "$OUT_DIR"/production-evidence.log "$OUT_DIR"/production-evidence.md "$OUT_DIR"/production-evidence.json \
  "$OUT_DIR"/goal-completion.log "$OUT_DIR"/goal-completion.md "$OUT_DIR"/goal-completion.json \
  "$OUT_DIR"/source-fingerprint.log "$OUT_DIR"/source-fingerprint.json
if [ "$SKIP_LOCAL" = "0" ]; then
  rm -rf "$OUT_DIR/local"
fi
: >"$RESULTS_JSONL"

status=0

record_result() {
  local name="$1"
  local slug="$2"
  local rc="$3"
  local started="$4"
  local finished="$5"
  local log="$6"
  python3 - "$RESULTS_JSONL" "$name" "$slug" "$rc" "$started" "$finished" "$log" <<'PY'
import json
import pathlib
import sys

path = pathlib.Path(sys.argv[1])
entry = {
    "name": sys.argv[2],
    "slug": sys.argv[3],
    "returncode": int(sys.argv[4]),
    "started_at": sys.argv[5],
    "finished_at": sys.argv[6],
    "log": sys.argv[7],
}
with path.open("a", encoding="utf-8") as handle:
    handle.write(json.dumps(entry, ensure_ascii=False) + "\n")
PY
}

run_capture() {
  local name="$1"
  local slug="$2"
  shift 2
  local log="$OUT_DIR/$slug.log"
  local started
  local finished
  local rc

  printf '==> %s\n' "$name"
  started="$(date '+%Y-%m-%d %H:%M:%S %Z')"
  set +e
  "$@" >"$log" 2>&1
  rc=$?
  set -e
  finished="$(date '+%Y-%m-%d %H:%M:%S %Z')"
  record_result "$name" "$slug" "$rc" "$started" "$finished" "$log"

  if [ "$rc" -ne 0 ]; then
    status=1
    printf '    failed, see %s\n' "$log" >&2
  else
    printf '    passed, see %s\n' "$log"
  fi
}

if [ "$SKIP_LOCAL" = "0" ]; then
  run_capture "全套非 PHP 本地短验收证据包" "local-evidence" \
    env -u GOROOT \
      MOCHAT_LOCAL_EVIDENCE_DIR="$OUT_DIR/local" \
      MOCHAT_LOCAL_EVIDENCE_ACCEPTANCE_SUITES="$LOCAL_SUITES" \
      ./scripts/collect_standalone_evidence.sh
else
  printf '==> 全套非 PHP 本地短验收证据包 skipped by MOCHAT_PRODUCTION_CANDIDATE_SKIP_LOCAL=1\n'
fi

FINGERPRINT_REFERENCE="$OUT_DIR/local/source-fingerprint.json"
FINGERPRINT_REFERENCE_LABEL="本地短验收证据包"
GOAL_EVIDENCE_DIR="$OUT_DIR/local"
if [ "$SKIP_LOCAL" = "1" ]; then
  FINGERPRINT_REFERENCE="docs/phases/phase-pre0-standalone/evidence/latest/source-fingerprint.json"
  FINGERPRINT_REFERENCE_LABEL="latest 本地短验收证据包"
  GOAL_EVIDENCE_DIR="docs/phases/phase-pre0-standalone/evidence/latest"
fi

run_capture "严格生产证据检查" "production-evidence" \
  env -u GOROOT \
    MOCHAT_PRODUCTION_EVIDENCE_STRICT=1 \
    MOCHAT_PRODUCTION_EVIDENCE_OUT="$OUT_DIR/production-evidence.md" \
    MOCHAT_PRODUCTION_EVIDENCE_JSON_OUT="$OUT_DIR/production-evidence.json" \
    ./scripts/production_evidence_check.sh

run_capture "严格目标完成度审计" "goal-completion" \
  env -u GOROOT \
    MOCHAT_GOAL_COMPLETION_STRICT=1 \
    MOCHAT_GOAL_COMPLETION_REQUIRE_24H="${MOCHAT_GOAL_COMPLETION_REQUIRE_24H:-0}" \
    MOCHAT_GOAL_COMPLETION_EVIDENCE_DIR="$GOAL_EVIDENCE_DIR" \
    MOCHAT_GOAL_COMPLETION_OUT="$OUT_DIR/goal-completion.md" \
    MOCHAT_GOAL_COMPLETION_JSON_OUT="$OUT_DIR/goal-completion.json" \
    ./scripts/audit_goal_completion.sh

run_capture "源码与验收指纹匹配" "source-fingerprint" \
  env -u GOROOT \
    ./scripts/source_fingerprint.py \
      --out "$OUT_DIR/source-fingerprint.json" \
      --check "$FINGERPRINT_REFERENCE"

./scripts/list_standalone_soak_processes.sh >"$OUT_DIR/long-running-processes.txt" 2>/dev/null || true
docker ps --filter name=mochat-go --format '{{.Names}} {{.Status}} {{.Ports}}' >"$OUT_DIR/docker-ps.txt" 2>/dev/null || true

python3 - "$OUT_DIR" "$RESULTS_JSONL" "$SKIP_LOCAL" "$LOCAL_SUITES" "$FINGERPRINT_REFERENCE" "$FINGERPRINT_REFERENCE_LABEL" <<'PY'
import datetime as dt
import json
import os
import pathlib
import sys

out_dir = pathlib.Path(sys.argv[1])
results_path = pathlib.Path(sys.argv[2])
skip_local = sys.argv[3] == "1"
local_suites = sys.argv[4]
fingerprint_reference = pathlib.Path(sys.argv[5])
fingerprint_reference_label = sys.argv[6]

results = []
if results_path.exists():
    for line in results_path.read_text(encoding="utf-8").splitlines():
        if line.strip():
            results.append(json.loads(line))

required_envs = [
    "MOCHAT_EVIDENCE_MYSQL57_AMD64",
    "MOCHAT_EVIDENCE_REAL_WECOM",
    "MOCHAT_EVIDENCE_REAL_WECHAT_OPEN",
    "MOCHAT_EVIDENCE_REAL_SAAS_TENANTS",
    "MOCHAT_EVIDENCE_PROD_FRONTEND",
    "MOCHAT_EVIDENCE_STABILITY",
]
all_ok = all(item.get("returncode") == 0 for item in results)
generated_at = dt.datetime.now().astimezone().strftime("%Y-%m-%d %H:%M:%S %Z")
long_running = (out_dir / "long-running-processes.txt").read_text(encoding="utf-8").strip() if (out_dir / "long-running-processes.txt").exists() else ""
docker_ps = (out_dir / "docker-ps.txt").read_text(encoding="utf-8").strip() if (out_dir / "docker-ps.txt").exists() else ""

current_fingerprint_path = out_dir / "source-fingerprint.json"
current_fingerprint = {}
reference_fingerprint = {}
if current_fingerprint_path.exists():
    try:
        current_fingerprint = json.loads(current_fingerprint_path.read_text(encoding="utf-8"))
    except Exception:
        current_fingerprint = {}
if fingerprint_reference.exists():
    try:
        reference_fingerprint = json.loads(fingerprint_reference.read_text(encoding="utf-8"))
    except Exception:
        reference_fingerprint = {}
fingerprint_match = (
    bool(current_fingerprint.get("fingerprint"))
    and current_fingerprint.get("fingerprint") == reference_fingerprint.get("fingerprint")
)

def extract_current_conclusion(path: pathlib.Path) -> str:
    if not path.exists():
        return ""
    for line in path.read_text(encoding="utf-8").splitlines():
        if line.startswith("- 当前结论："):
            return line.split("：", 1)[1].strip()
    return ""

production_conclusion = extract_current_conclusion(out_dir / "production-evidence.md")
goal_conclusion = extract_current_conclusion(out_dir / "goal-completion.md")

lines = [
    "# MoChat Go 生产候选总门禁",
    "",
    f"- 生成时间：`{generated_at}`",
    f"- 当前结论：**{'通过，可进入生产候选复核' if all_ok else '未通过，不能标记最终完成'}**",
    f"- 输出目录：`{out_dir}`",
    f"- 本地短验收：`{'跳过' if skip_local else '执行'}`",
    f"- 本地短验收套件：`{local_suites}`",
    "- 24 小时持续运行：`未启动`",
    "",
    "## 命令结果",
    "",
]

if not results:
    lines.append("- 未记录命令结果。")
for item in results:
    marker = "通过" if item["returncode"] == 0 else "失败"
    lines.append(f"- {marker} `{item['name']}`：`{item['log']}`")

lines.extend(["", "## 关键结论", ""])
if production_conclusion:
    lines.append(f"- 严格生产证据检查：{production_conclusion}")
else:
    lines.append("- 严格生产证据检查：未生成当前结论。")
if goal_conclusion:
    lines.append(f"- 严格目标完成度审计：{goal_conclusion}")
else:
    lines.append("- 严格目标完成度审计：未生成当前结论。")
lines.append("- 生产候选总门禁只有在本地短验收、严格生产证据检查、严格目标完成度审计和源码指纹匹配全部返回 0 时才算通过。")
lines.append("- 源码与验收指纹必须匹配当前工作区，防止用旧证据包证明新代码。")

lines.extend(["", "## 源码与验收指纹", ""])
if current_fingerprint:
    lines.append(f"- 当前指纹：`{current_fingerprint.get('fingerprint', '')}`")
    lines.append(f"- 当前纳入文件数：`{current_fingerprint.get('file_count', '')}`")
else:
    lines.append("- 当前指纹：未生成。")
lines.append(f"- 对比来源：`{fingerprint_reference_label}`")
lines.append(f"- 对比文件：`{fingerprint_reference}`")
if reference_fingerprint:
    lines.append(f"- 对比指纹：`{reference_fingerprint.get('fingerprint', '')}`")
    lines.append(f"- 对比纳入文件数：`{reference_fingerprint.get('file_count', '')}`")
else:
    lines.append("- 对比指纹：缺失或无法解析。")
lines.append(f"- 指纹匹配：`{fingerprint_match}`")

lines.extend(["", "## 生产证据环境变量", ""])
for env_name in required_envs:
    value = os.environ.get(env_name, "").strip()
    if value:
        exists = pathlib.Path(value).exists()
        marker = "存在" if exists else "不存在"
        lines.append(f"- `{env_name}`：`{value}`（{marker}）")
    else:
        lines.append(f"- `{env_name}`：未设置")

lines.extend(["", "## 报告文件", ""])
if not skip_local:
    lines.append("- 本地短验收证据包：`local/index.md`")
lines.append("- 严格生产证据检查：`production-evidence.md` 和 `production-evidence.json`")
lines.append("- 严格目标完成度审计：`goal-completion.md` 和 `goal-completion.json`")
lines.append("- 原始命令结果：`results.jsonl` 和 `*.log`")

lines.extend(["", "## 运行残留", ""])
if long_running:
    lines.append(f"- 发现持续运行脚本进程：`{long_running}`")
else:
    lines.append("- 未发现 `standalone_soak_24h.sh` 进程。")
if docker_ps:
    lines.append(f"- 发现 mochat-go 容器：`{docker_ps}`")
else:
    lines.append("- 未发现正在运行的 `mochat-go` Docker 容器。")

(out_dir / "index.md").write_text("\n".join(lines) + "\n", encoding="utf-8")
print(out_dir / "index.md")
PY

exit "$status"
