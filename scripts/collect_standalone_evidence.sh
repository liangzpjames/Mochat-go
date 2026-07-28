#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

if [ "${1:-}" = "-h" ] || [ "${1:-}" = "--help" ]; then
  cat <<'EOF'
Usage: ./scripts/collect_standalone_evidence.sh

生成本地短验收证据包，默认输出到 docs/phases/phase-pre0-standalone/evidence/latest/。

环境变量：
  MOCHAT_LOCAL_EVIDENCE_DIR
      输出目录，默认 docs/phases/phase-pre0-standalone/evidence/latest。
  MOCHAT_LOCAL_EVIDENCE_ACCEPTANCE_SUITES
      要运行的非 PHP standalone_acceptance 套件，默认 core。
      支持空格或逗号分隔：core saas workers cron frontend mysql57。
      all 会展开为 core saas workers cron frontend mysql57。
  MOCHAT_LOCAL_EVIDENCE_INVENTORY_PARITY
      是否在证据包中额外记录 PHP 源码清单对齐，默认 auto。
      auto 会在源码目录存在时执行；1 要求必须执行；0 跳过。
  MOCHAT_LOCAL_EVIDENCE_RESUME
      是否从已有证据目录恢复执行，默认 0。设为 1 后会保留已有文件，
      仅在源码指纹未变化时跳过 results.jsonl 中已成功的步骤，并重跑
      失败或缺失步骤；源码指纹变化或缺失时自动执行全新重建。全新
      验收会在第一步前写入指纹检查点，中断后可安全恢复。
  MOCHAT_LOCAL_EVIDENCE_SOURCE_ROOT
      PHP 源码清单对齐使用的原 MoChat 源码目录，默认读取 MOCHAT_SOURCE_ROOT，
      未设置时使用 ../mochat。
  MOCHAT_LOCAL_EVIDENCE_GO_ADDR / GO_PORT / SIDEBAR_PORT / OPERATION_PORT
  MOCHAT_LOCAL_EVIDENCE_MYSQL_PORT / REDIS_PORT
      所有短验收套件的隔离地址与端口，默认分别为
      127.0.0.1:18190/18190/18191/18192/13418/26491，避免与保留中的
      本地预览栈冲突。旧的 MOCHAT_LOCAL_EVIDENCE_CORE_* 变量仍兼容。

该脚本会污染 MOCHAT_SOURCE_ROOT 和 MOCHAT_COMPAT_MANIFEST，并设置
MOCHAT_QUEUE_AUDIT_SOURCE=embedded，证明独立交付包不依赖原 PHP 源码或外部 manifest。
EOF
  exit 0
fi

OUT_DIR="${MOCHAT_LOCAL_EVIDENCE_DIR:-docs/phases/phase-pre0-standalone/evidence/latest}"
RESULTS_JSONL="$OUT_DIR/results.jsonl"
POISON_SOURCE_ROOT="$OUT_DIR/should-not-read-php-source"
POISON_MANIFEST="$OUT_DIR/should-not-read-manifest.json"
ACCEPTANCE_SUITES_RAW="${MOCHAT_LOCAL_EVIDENCE_ACCEPTANCE_SUITES:-core}"
INVENTORY_PARITY_MODE="${MOCHAT_LOCAL_EVIDENCE_INVENTORY_PARITY:-auto}"
RESUME_MODE="${MOCHAT_LOCAL_EVIDENCE_RESUME:-0}"
INVENTORY_SOURCE_ROOT="${MOCHAT_LOCAL_EVIDENCE_SOURCE_ROOT:-${MOCHAT_SOURCE_ROOT:-../mochat}}"
EVIDENCE_GO_PORT="${MOCHAT_LOCAL_EVIDENCE_GO_PORT:-${MOCHAT_LOCAL_EVIDENCE_CORE_GO_PORT:-18190}}"
EVIDENCE_GO_ADDR="${MOCHAT_LOCAL_EVIDENCE_GO_ADDR:-127.0.0.1:$EVIDENCE_GO_PORT}"
EVIDENCE_SIDEBAR_PORT="${MOCHAT_LOCAL_EVIDENCE_SIDEBAR_PORT:-${MOCHAT_LOCAL_EVIDENCE_CORE_SIDEBAR_PORT:-18191}}"
EVIDENCE_OPERATION_PORT="${MOCHAT_LOCAL_EVIDENCE_OPERATION_PORT:-${MOCHAT_LOCAL_EVIDENCE_CORE_OPERATION_PORT:-18192}}"
EVIDENCE_MYSQL_PORT="${MOCHAT_LOCAL_EVIDENCE_MYSQL_PORT:-${MOCHAT_LOCAL_EVIDENCE_CORE_MYSQL_PORT:-13418}}"
EVIDENCE_REDIS_PORT="${MOCHAT_LOCAL_EVIDENCE_REDIS_PORT:-${MOCHAT_LOCAL_EVIDENCE_CORE_REDIS_PORT:-26491}}"
EVIDENCE_COMPOSE_PROJECT="${MOCHAT_LOCAL_EVIDENCE_COMPOSE_PROJECT:-${MOCHAT_LOCAL_EVIDENCE_CORE_COMPOSE_PROJECT:-mochat-go-local-evidence-compose-app}}"

case "$INVENTORY_PARITY_MODE" in
  auto|0|1)
    ;;
  *)
    echo "unknown MOCHAT_LOCAL_EVIDENCE_INVENTORY_PARITY=$INVENTORY_PARITY_MODE" >&2
    echo "valid values: auto, 0, 1" >&2
    exit 2
    ;;
esac

case "$RESUME_MODE" in
  0|1)
    ;;
  *)
    echo "unknown MOCHAT_LOCAL_EVIDENCE_RESUME=$RESUME_MODE" >&2
    echo "valid values: 0, 1" >&2
    exit 2
    ;;
esac

if [ "$RESUME_MODE" = "1" ]; then
  resume_fingerprint="$OUT_DIR/source-fingerprint.json"
  resume_current_fingerprint="$(mktemp "${TMPDIR:-/tmp}/mochat-go-evidence-resume-fingerprint.XXXXXX")"
  if [ ! -f "$resume_fingerprint" ] || ! python3 ./scripts/source_fingerprint.py \
    --out "$resume_current_fingerprint" \
    --check "$resume_fingerprint" >/dev/null 2>&1; then
    printf '==> 源码指纹变化或缺失，恢复模式自动改为全新重建\n'
    RESUME_MODE=0
  fi
  rm -f "$resume_current_fingerprint"
fi

mkdir -p "$OUT_DIR"
if [ "$RESUME_MODE" = "0" ]; then
  find "$OUT_DIR" -maxdepth 1 -type f \( \
    -name '*.log' \
    -o -name '*.jsonl' \
    -o -name '*.json' \
    -o -name '*.md' \
    -o -name '*.txt' \
  \) -delete
  : >"$RESULTS_JSONL"
elif [ ! -f "$RESULTS_JSONL" ]; then
  : >"$RESULTS_JSONL"
fi

# 先落盘本轮源码检查点，避免长验收中断后因缺少指纹而无法恢复。
python3 ./scripts/source_fingerprint.py --out "$OUT_DIR/source-fingerprint.json"

status=0

ACCEPTANCE_SUITES="$(
  python3 - "$ACCEPTANCE_SUITES_RAW" <<'PY'
import re
import sys

raw = sys.argv[1].strip()
requested = [item for item in re.split(r"[\s,]+", raw) if item]
if not requested:
    requested = ["core"]
expanded = []
for item in requested:
    if item == "all":
        expanded.extend(["core", "saas", "workers", "cron", "frontend", "mysql57"])
    else:
        expanded.append(item)
valid = {"core", "saas", "workers", "cron", "frontend", "mysql57"}
invalid = [item for item in expanded if item not in valid]
if invalid:
    raise SystemExit(f"invalid MOCHAT_LOCAL_EVIDENCE_ACCEPTANCE_SUITES value(s): {', '.join(invalid)}; valid values: core, saas, workers, cron, frontend, mysql57, all")
deduped = []
for item in expanded:
    if item not in deduped:
        deduped.append(item)
print(" ".join(deduped))
PY
)"

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
entries = []
if path.exists():
    for line in path.read_text(encoding="utf-8").splitlines():
        if not line.strip():
            continue
        current = json.loads(line)
        if current.get("slug") != entry["slug"]:
            entries.append(current)
entries.append(entry)
temporary = path.with_suffix(path.suffix + ".tmp")
with temporary.open("w", encoding="utf-8") as handle:
    for current in entries:
        handle.write(json.dumps(current, ensure_ascii=False) + "\n")
temporary.replace(path)
PY
}

result_already_passed() {
  local slug="$1"
  python3 - "$RESULTS_JSONL" "$slug" <<'PY'
import json
import pathlib
import sys

path = pathlib.Path(sys.argv[1])
slug = sys.argv[2]
if not path.exists():
    raise SystemExit(1)
for line in path.read_text(encoding="utf-8").splitlines():
    if not line.strip():
        continue
    entry = json.loads(line)
    if entry.get("slug") == slug and entry.get("returncode") == 0:
        raise SystemExit(0)
raise SystemExit(1)
PY
}

run_capture_with_mode() {
  local allow_resume="$1"
  shift
  local name="$1"
  local slug="$2"
  shift 2
  local log="$OUT_DIR/$slug.log"
  local started
  local finished
  local rc

  if [ "$allow_resume" = "1" ] && [ "$RESUME_MODE" = "1" ] && result_already_passed "$slug"; then
    printf '==> %s\n' "$name"
    printf '    already passed, keeping %s\n' "$log"
    return
  fi

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

run_capture() {
  run_capture_with_mode 1 "$@"
}

run_capture_refresh() {
  run_capture_with_mode 0 "$@"
}

run_capture "快速本地门禁" "test" env -u GOROOT MOCHAT_QUEUE_AUDIT_SOURCE=embedded MOCHAT_SOURCE_ROOT="$POISON_SOURCE_ROOT" MOCHAT_COMPAT_MANIFEST="$POISON_MANIFEST" ./scripts/test.sh
run_capture "独立交付包 smoke" "independent-package" env -u GOROOT ./scripts/smoke_independent_package.sh

if [ "$INVENTORY_PARITY_MODE" = "1" ] || { [ "$INVENTORY_PARITY_MODE" = "auto" ] && [ -d "$INVENTORY_SOURCE_ROOT/api-server" ]; }; then
  run_capture "迁移期 PHP 清单对齐" "inventory-parity" env -u GOROOT MOCHAT_SOURCE_ROOT="$INVENTORY_SOURCE_ROOT" ./scripts/standalone_inventory_parity.sh
else
  {
    printf 'standalone inventory parity skipped\n'
    printf 'MOCHAT_LOCAL_EVIDENCE_INVENTORY_PARITY=%s\n' "$INVENTORY_PARITY_MODE"
    printf 'MOCHAT_LOCAL_EVIDENCE_SOURCE_ROOT=%s\n' "$INVENTORY_SOURCE_ROOT"
    printf 'reason=source root not available or parity disabled\n'
  } >"$OUT_DIR/inventory-parity.txt"
  printf '==> 迁移期 PHP 清单对齐 skipped, see %s\n' "$OUT_DIR/inventory-parity.txt"
fi

for suite in $ACCEPTANCE_SUITES; do
  run_capture "独立验收套件：$suite" "acceptance-$suite" env -u GOROOT \
    MOCHAT_QUEUE_AUDIT_SOURCE=embedded \
    MOCHAT_SOURCE_ROOT="$POISON_SOURCE_ROOT" \
    MOCHAT_COMPAT_MANIFEST="$POISON_MANIFEST" \
    MOCHAT_ACCEPTANCE_SUITE="$suite" \
    MOCHAT_ACCEPTANCE_SKIP_INVENTORY_PARITY=1 \
    MOCHAT_COMPOSE_APP_PROJECT="$EVIDENCE_COMPOSE_PROJECT" \
    MOCHAT_GO_ADDR="$EVIDENCE_GO_ADDR" \
    MOCHAT_GO_PORT="$EVIDENCE_GO_PORT" \
    MOCHAT_SIDEBAR_PORT="$EVIDENCE_SIDEBAR_PORT" \
    MOCHAT_OPERATION_PORT="$EVIDENCE_OPERATION_PORT" \
    MOCHAT_MYSQL_PORT="$EVIDENCE_MYSQL_PORT" \
    MOCHAT_REDIS_PORT="$EVIDENCE_REDIS_PORT" \
    ./scripts/standalone_acceptance.sh
done

run_capture "运行时路由覆盖" "route-coverage" env -u GOROOT MOCHAT_SOURCE_ROOT="$POISON_SOURCE_ROOT" MOCHAT_COMPAT_MANIFEST="$POISON_MANIFEST" MOCHAT_ROUTE_COVERAGE_OUT="$OUT_DIR/route-coverage.json" ./scripts/standalone_route_coverage.sh
run_capture "前端 dist API 覆盖审计" "frontend-dist-api-coverage" env -u GOROOT MOCHAT_FRONTEND_DIST_API_OUT="$OUT_DIR/frontend-dist-api-coverage.md" ./scripts/audit_frontend_dist_api_coverage.sh
run_capture_refresh "阶段报告" "stage-report" env -u GOROOT MOCHAT_QUEUE_AUDIT_SOURCE=embedded MOCHAT_SOURCE_ROOT="$POISON_SOURCE_ROOT" MOCHAT_COMPAT_MANIFEST="$POISON_MANIFEST" MOCHAT_STAGE_REPORT_OUT="$OUT_DIR/current-stage-report.md" ./scripts/standalone_stage_report.sh
run_capture_refresh "生产证据检查" "production-evidence" env -u GOROOT MOCHAT_QUEUE_AUDIT_SOURCE=embedded MOCHAT_SOURCE_ROOT="$POISON_SOURCE_ROOT" MOCHAT_COMPAT_MANIFEST="$POISON_MANIFEST" MOCHAT_PRODUCTION_EVIDENCE_OUT="$OUT_DIR/production-evidence.md" MOCHAT_PRODUCTION_EVIDENCE_JSON_OUT="$OUT_DIR/production-evidence.json" ./scripts/production_evidence_check.sh
run_capture_refresh "验收期间源码指纹稳定" "source-fingerprint-stability" env -u GOROOT ./scripts/source_fingerprint.py --out "$OUT_DIR/source-fingerprint-current.json" --check "$OUT_DIR/source-fingerprint.json"
if result_already_passed "source-fingerprint-stability"; then
  mv "$OUT_DIR/source-fingerprint-current.json" "$OUT_DIR/source-fingerprint.json"
else
  rm -f "$OUT_DIR/source-fingerprint-current.json"
fi
run_capture_refresh "目标完成度审计" "goal-completion" env -u GOROOT MOCHAT_QUEUE_AUDIT_SOURCE=embedded MOCHAT_SOURCE_ROOT="$POISON_SOURCE_ROOT" MOCHAT_COMPAT_MANIFEST="$POISON_MANIFEST" MOCHAT_GOAL_COMPLETION_SOURCE_ROOT="$INVENTORY_SOURCE_ROOT" MOCHAT_GOAL_COMPLETION_EVIDENCE_DIR="$OUT_DIR" MOCHAT_GOAL_COMPLETION_OUT="$OUT_DIR/goal-completion.md" MOCHAT_GOAL_COMPLETION_JSON_OUT="$OUT_DIR/goal-completion.json" ./scripts/audit_goal_completion.sh

{
  printf 'MOCHAT_QUEUE_AUDIT_SOURCE=embedded\n'
  printf 'MOCHAT_ACCEPTANCE_SKIP_INVENTORY_PARITY=1\n'
  printf 'MOCHAT_LOCAL_EVIDENCE_ACCEPTANCE_SUITES=%s\n' "$ACCEPTANCE_SUITES"
  printf 'MOCHAT_LOCAL_EVIDENCE_INVENTORY_PARITY=%s\n' "$INVENTORY_PARITY_MODE"
  printf 'MOCHAT_LOCAL_EVIDENCE_RESUME=%s\n' "$RESUME_MODE"
  printf 'MOCHAT_LOCAL_EVIDENCE_SOURCE_ROOT=%s\n' "$INVENTORY_SOURCE_ROOT"
  printf 'MOCHAT_SOURCE_ROOT=%s\n' "$POISON_SOURCE_ROOT"
  printf 'MOCHAT_COMPAT_MANIFEST=%s\n' "$POISON_MANIFEST"
} >"$OUT_DIR/independence-env.txt"
./scripts/list_standalone_soak_processes.sh >"$OUT_DIR/long-running-processes.txt" 2>/dev/null || true
docker ps --filter name=mochat-go --format '{{.Names}} {{.Status}} {{.Ports}}' >"$OUT_DIR/docker-ps.txt" 2>/dev/null || true

python3 - "$OUT_DIR" "$RESULTS_JSONL" <<'PY'
import datetime as dt
import json
import pathlib
import sys

out_dir = pathlib.Path(sys.argv[1])
results_path = pathlib.Path(sys.argv[2])
results = []
if results_path.exists():
    for line in results_path.read_text(encoding="utf-8").splitlines():
        if line.strip():
            results.append(json.loads(line))

route_summary = None
route_path = out_dir / "route-coverage.json"
if route_path.exists():
    route_summary = json.loads(route_path.read_text(encoding="utf-8"))
fingerprint_path = out_dir / "source-fingerprint.json"
fingerprint = json.loads(fingerprint_path.read_text(encoding="utf-8")) if fingerprint_path.exists() else None

long_running_path = out_dir / "long-running-processes.txt"
docker_path = out_dir / "docker-ps.txt"
independence_env_path = out_dir / "independence-env.txt"
inventory_note_path = out_dir / "inventory-parity.txt"
long_running = long_running_path.read_text(encoding="utf-8").strip() if long_running_path.exists() else ""
docker_ps = docker_path.read_text(encoding="utf-8").strip() if docker_path.exists() else ""
independence_env = independence_env_path.read_text(encoding="utf-8").strip().splitlines() if independence_env_path.exists() else []
inventory_note = inventory_note_path.read_text(encoding="utf-8").strip().splitlines() if inventory_note_path.exists() else []
all_ok = all(item.get("returncode") == 0 for item in results)
generated_at = dt.datetime.now().astimezone().strftime("%Y-%m-%d %H:%M:%S %Z")

def extract_current_conclusion(path: pathlib.Path) -> str:
    if not path.exists():
        return ""
    for line in path.read_text(encoding="utf-8").splitlines():
        if line.startswith("- 当前结论："):
            return line.split("：", 1)[1].strip()
    return ""

production_conclusion = extract_current_conclusion(out_dir / "production-evidence.md")
frontend_conclusion = extract_current_conclusion(out_dir / "frontend-dist-api-coverage.md")
goal_conclusion = extract_current_conclusion(out_dir / "goal-completion.md")
goal_unfinished = "目标未完成" in goal_conclusion
production_missing = "仍缺生产证据" in production_conclusion or "不能标记最终完成" in production_conclusion
frontend_missing = "发现缺口" in frontend_conclusion or "前端 API" in goal_conclusion
if not all_ok:
    current_conclusion = "本地短门禁存在失败项"
elif goal_unfinished or production_missing or frontend_missing:
    gaps = []
    if frontend_missing:
        gaps.append("前端 API")
    if production_missing or "生产证据" in goal_conclusion:
        gaps.append("生产证据")
    if not gaps:
        gaps.append("后续证据")
    current_conclusion = f"本地短门禁通过，目标未完成，仍需补{'、'.join(gaps)}"
elif "目标已完成" in goal_conclusion:
    current_conclusion = "目标已完成"
else:
    current_conclusion = "本地短门禁通过，仍需补生产证据"

lines = [
    "# MoChat Go 本地独立版证据包",
    "",
    f"- 生成时间：`{generated_at}`",
    f"- 当前结论：**{current_conclusion}**",
    f"- 输出目录：`{out_dir}`",
    "",
    "## 命令结果",
    "",
]

for item in results:
    if item["slug"] in {"frontend-dist-api-coverage", "production-evidence", "goal-completion"} and item["returncode"] == 0:
        marker = "报告生成"
    else:
        marker = "通过" if item["returncode"] == 0 else "失败"
    lines.append(f"- {marker} `{item['name']}`：`{item['log']}`")

lines.extend(["", "## 关键结论", ""])
if frontend_conclusion:
    lines.append(f"- 前端 dist API 覆盖审计：{frontend_conclusion}")
else:
    lines.append("- 前端 dist API 覆盖审计：未生成当前结论。")
if production_conclusion:
    lines.append(f"- 生产证据检查：{production_conclusion}")
else:
    lines.append("- 生产证据检查：未生成当前结论。")
if goal_conclusion:
    lines.append(f"- 目标完成度审计：{goal_conclusion}")
else:
    lines.append("- 目标完成度审计：未生成当前结论。")
lines.append("- `生产证据检查` 和 `目标完成度审计` 返回 0 只代表报告生成成功；是否完成以报告内结论和严格生产候选门禁为准。")

if inventory_note:
    lines.extend(["", "## 迁移期清单对齐", ""])
    lines.extend(f"- `{line}`" for line in inventory_note)

lines.extend(["", "## 运行时路由覆盖", ""])
if route_summary:
    lines.extend(
        [
            f"- manifest 路由：`{route_summary.get('route_total')}`",
            f"- 已迁移 manifest 路由：`{route_summary.get('migrated_manifest_route_total')}`",
            f"- 未迁移 manifest 路由：`{route_summary.get('missing_route_total')}`",
            f"- Go 额外运行时路由：`{route_summary.get('extra_route_total')}`",
        ]
    )
else:
    lines.append("- 未生成 `route-coverage.json`。")

lines.extend(["", "## 源码与验收指纹", ""])
if fingerprint:
    lines.extend(
        [
            f"- 指纹：`{fingerprint.get('fingerprint')}`",
            f"- 文件数：`{fingerprint.get('file_count')}`",
            "- 指纹文件：`source-fingerprint.json`",
        ]
    )
else:
    lines.append("- 未生成 `source-fingerprint.json`。")

lines.extend(
    [
        "",
        "## 独立交付污染环境",
        "",
    ]
)
if independence_env:
    lines.extend(f"- `{line}`" for line in independence_env)
else:
    lines.append("- 未记录污染环境变量。")
lines.extend(
    [
        "",
        "## 报告文件",
        "",
        "- 阶段报告：`current-stage-report.md`",
        "- 前端 dist API 覆盖审计：`frontend-dist-api-coverage.md`",
        "- 生产证据检查：`production-evidence.md` 和 `production-evidence.json`",
        "- 目标完成度审计：`goal-completion.md` 和 `goal-completion.json`",
        "- 原始命令结果：`results.jsonl` 和 `*.log`",
        "",
        "## 运行残留",
        "",
    ]
)

if long_running:
    lines.append(f"- 发现持续运行脚本进程：`{long_running}`")
else:
    lines.append("- 未发现 `standalone_soak_24h.sh` 进程。")
if docker_ps:
    lines.append(f"- 发现 mochat-go 容器：`{docker_ps}`")
else:
    lines.append("- 未发现正在运行的 `mochat-go` Docker 容器。")

lines.extend(["", "## 仍需补证", ""])
if frontend_missing:
    lines.append("- 旧前端 dist API 全量承接：dashboard/sidebar/operation 内置 dist 中声明的 API 都需要由 Go runtime 响应。")
lines.extend(
    [
        "- 真实企业微信账号联调。",
        "- 真实微信开放平台联调。",
        "- MySQL 5.7 amd64 真实容器门禁。",
        "- 真实 SaaS 多租户业务数据回归。",
        "- 生产前端浏览器回归。",
        "- 稳定性证据默认走短稳回归、健康检查和外部监控记录；如上线前另行要求满 24 小时长稳，再单独启动持续运行脚本。",
    ]
)

(out_dir / "index.md").write_text("\n".join(lines) + "\n", encoding="utf-8")
print(out_dir / "index.md")
PY

if [ "$status" -ne 0 ]; then
  exit "$status"
fi
