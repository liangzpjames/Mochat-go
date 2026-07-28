#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

if [ "${1:-}" = "-h" ] || [ "${1:-}" = "--help" ]; then
  cat <<'EOF'
Usage: ./scripts/audit_goal_completion.sh

按用户目标生成完成度审计报告：
  - 能运行的独立 Go 项目
  - 不依赖原 mochat/PHP 项目
  - 全部 manifest 功能迁移、PHP 清单对齐和本地短验收
  - SaaS、worker、cron、frontend 本地证据
  - 生产外部证据缺口

环境变量：
  MOCHAT_GOAL_COMPLETION_OUT
      写入 Markdown 报告的路径；未设置时输出到 stdout。
  MOCHAT_GOAL_COMPLETION_JSON_OUT
      写入机器可读 JSON 报告的路径；未设置时不输出 JSON。
  MOCHAT_GOAL_COMPLETION_EVIDENCE_DIR
      本地短证据包目录，默认 docs/phases/phase-pre0-standalone/evidence/latest。
  MOCHAT_GOAL_COMPLETION_SOURCE_ROOT
      PHP 源码清单对齐使用的原 MoChat 源码目录，默认 ../mochat。
  MOCHAT_GOAL_COMPLETION_STRICT
      设为 1 时，目标未完成返回非 0。
  MOCHAT_GOAL_COMPLETION_REQUIRE_24H
      兼容开关。设为 1 时，把满 24 小时长稳证据作为额外完成条件；默认不要求。

该脚本本身不会启动 24 小时持续运行。
EOF
  exit 0
fi

STRICT="${MOCHAT_GOAL_COMPLETION_STRICT:-0}"
REQUIRE_24H="${MOCHAT_GOAL_COMPLETION_REQUIRE_24H:-0}"
OUT="${MOCHAT_GOAL_COMPLETION_OUT:-}"
JSON_OUT="${MOCHAT_GOAL_COMPLETION_JSON_OUT:-}"
EVIDENCE_DIR="${MOCHAT_GOAL_COMPLETION_EVIDENCE_DIR:-docs/phases/phase-pre0-standalone/evidence/latest}"

case "$STRICT" in
  0|1) ;;
  *)
    echo "unknown MOCHAT_GOAL_COMPLETION_STRICT=$STRICT" >&2
    exit 2
    ;;
esac

case "$REQUIRE_24H" in
  0|1) ;;
  *)
    echo "unknown MOCHAT_GOAL_COMPLETION_REQUIRE_24H=$REQUIRE_24H" >&2
    exit 2
    ;;
esac

python3 - "$STRICT" "$REQUIRE_24H" "$OUT" "$EVIDENCE_DIR" "$JSON_OUT" <<'PY'
import datetime as dt
import glob
import json
import os
import pathlib
import socket
import subprocess
import sys
import tempfile

strict = sys.argv[1] == "1"
require_24h = sys.argv[2] == "1"
out_path = pathlib.Path(sys.argv[3]) if len(sys.argv) > 3 and sys.argv[3] else None
evidence_dir_arg = pathlib.Path(sys.argv[4]) if len(sys.argv) > 4 and sys.argv[4] else pathlib.Path("docs/phases/phase-pre0-standalone/evidence/latest")
json_out_path = pathlib.Path(sys.argv[5]) if len(sys.argv) > 5 and sys.argv[5] else None
repo = pathlib.Path.cwd()
evidence_dir = evidence_dir_arg if evidence_dir_arg.is_absolute() else repo / evidence_dir_arg
evidence_dir_display = evidence_dir_arg.as_posix()
php_source_root = os.environ.get("MOCHAT_GOAL_COMPLETION_SOURCE_ROOT", "../mochat")


def summarize_output(output_text: str) -> str:
    lines = [line.strip() for line in output_text.splitlines() if line.strip()]
    if not lines:
        return ""
    frontend_dist_line = next((line for line in lines if line.startswith("frontend dist API coverage audit ")), "")
    if frontend_dist_line:
        return frontend_dist_line
    independent_package_line = next((line for line in lines if line == "independent package smoke passed"), "")
    if independent_package_line:
        return independent_package_line
    source_line = next((line for line in lines if line.startswith("queue annotation coverage source=")), "")
    passed_line = next((line for line in lines if line == "queue annotation coverage passed"), "")
    if source_line and passed_line:
        return f"{source_line} / {passed_line}"
    for line in lines:
        if " passed" in line or "通过" in line:
            return line
    return " / ".join(lines[-2:])


def run_check(name, cmd, extra_env=None):
    env = os.environ.copy()
    env.setdefault("MOCHAT_QUEUE_AUDIT_SOURCE", "embedded")
    if extra_env:
        env.update(extra_env)
    completed = subprocess.run(
        cmd,
        cwd=repo,
        env=env,
        text=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        check=False,
    )
    return {
        "name": name,
        "ok": completed.returncode == 0,
        "summary": summarize_output(completed.stdout),
        "output": completed.stdout,
    }


def free_port() -> int:
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as sock:
        sock.bind(("127.0.0.1", 0))
        return int(sock.getsockname()[1])


manifest = json.loads((repo / "internal/server/compat_manifest_embedded.json").read_text(encoding="utf-8"))
routes = manifest.get("routes", [])
tables = manifest.get("tables", [])
crontabs = manifest.get("crontabs", [])
events = manifest.get("event_handlers", [])
queues = manifest.get("async_queues", [])

with tempfile.TemporaryDirectory(prefix="mochat-go-goal-audit.") as tmp:
    tmp_dir = pathlib.Path(tmp)
    route_json = tmp_dir / "route-coverage.json"
    prod_report = tmp_dir / "production-evidence.md"
    route_project = f"mochat-go-goal-audit-{os.getpid()}"
    route_go_port = free_port()
    route_mysql_port = free_port()
    route_redis_port = free_port()

    checks = [
        run_check("standalone independence", ["./scripts/audit_standalone_independence.sh"]),
        run_check("independent package smoke", ["./scripts/smoke_independent_package.sh"]),
        run_check("PHP inventory parity", ["./scripts/standalone_inventory_parity.sh"], {"MOCHAT_SOURCE_ROOT": php_source_root}),
        run_check("acceptance suite coverage", ["./scripts/audit_acceptance_suite_coverage.sh"]),
        run_check("manifest route smoke coverage", ["./scripts/audit_manifest_route_smoke_coverage.sh"]),
        run_check("functional module matrix", ["./scripts/audit_functional_module_matrix.sh"]),
        run_check(
            "frontend dist API coverage",
            ["./scripts/audit_frontend_dist_api_coverage.sh"],
            {
                "MOCHAT_FRONTEND_DIST_API_STRICT": "1",
            },
        ),
        run_check("queue annotation coverage", ["./scripts/audit_queue_annotation_coverage.sh"]),
        run_check("wework callback event coverage", ["./scripts/audit_wework_callback_event_coverage.sh"]),
        run_check("worker SaaS usage assertions", ["./scripts/audit_worker_saas_usage_assertions.sh"]),
        run_check("SaaS metric coverage", ["./scripts/audit_saas_metric_coverage.sh"]),
        run_check("SaaS storage reclaim coverage", ["./scripts/audit_saas_storage_reclaim_coverage.sh"]),
        run_check(
            "runtime route coverage",
            ["./scripts/standalone_route_coverage.sh"],
            {
                "MOCHAT_ROUTE_COVERAGE_PROJECT": route_project,
                "MOCHAT_ROUTE_COVERAGE_MAX_MISSING": "0",
                "MOCHAT_ROUTE_COVERAGE_OUT": str(route_json),
                "MOCHAT_GO_ADDR": f"127.0.0.1:{route_go_port}",
                "MOCHAT_MYSQL_PORT": str(route_mysql_port),
                "MOCHAT_REDIS_PORT": str(route_redis_port),
                "MOCHAT_SOURCE_ROOT": str(tmp_dir / "should-not-read-php-source"),
                "MOCHAT_COMPAT_MANIFEST": str(tmp_dir / "should-not-read-manifest.json"),
            },
        ),
        run_check(
            "production evidence check",
            ["./scripts/production_evidence_check.sh"],
            {
                "MOCHAT_PRODUCTION_EVIDENCE_OUT": str(prod_report),
                "MOCHAT_PRODUCTION_EVIDENCE_STRICT": "1",
            },
        ),
    ]

    route_summary = None
    if route_json.exists():
        route_summary = json.loads(route_json.read_text(encoding="utf-8"))
    production_report_text = prod_report.read_text(encoding="utf-8") if prod_report.exists() else ""

latest_results_path = evidence_dir / "results.jsonl"
latest_results = {}
if latest_results_path.exists():
    for line in latest_results_path.read_text(encoding="utf-8").splitlines():
        if line.strip():
            item = json.loads(line)
            latest_results[item.get("slug", "")] = item

required_latest_slugs = [
    "test",
    "independent-package",
    "inventory-parity",
    "acceptance-core",
    "acceptance-saas",
    "acceptance-workers",
    "acceptance-cron",
    "acceptance-frontend",
    "acceptance-mysql57",
    "route-coverage",
    "frontend-dist-api-coverage",
    "stage-report",
    "production-evidence",
]
latest_missing = [slug for slug in required_latest_slugs if slug not in latest_results]
latest_failed = [slug for slug in required_latest_slugs if latest_results.get(slug, {}).get("returncode") not in (0, None)]

latest_fingerprint_path = evidence_dir / "source-fingerprint.json"
latest_fingerprint = None
current_fingerprint = None
fingerprint_issue = ""
if latest_fingerprint_path.exists():
    try:
        latest_fingerprint = json.loads(latest_fingerprint_path.read_text(encoding="utf-8"))
    except Exception as exc:
        fingerprint_issue = f"latest source-fingerprint.json 无法解析：{exc}"
else:
    fingerprint_issue = f"{evidence_dir_display} 证据包缺少 source-fingerprint.json。"

with tempfile.TemporaryDirectory(prefix="mochat-go-source-fingerprint.") as tmp:
    current_fingerprint_path = pathlib.Path(tmp) / "source-fingerprint.json"
    fingerprint_run = subprocess.run(
        ["python3", "./scripts/source_fingerprint.py", "--out", str(current_fingerprint_path)],
        cwd=repo,
        text=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        check=False,
    )
    if fingerprint_run.returncode == 0 and current_fingerprint_path.exists():
        current_fingerprint = json.loads(current_fingerprint_path.read_text(encoding="utf-8"))
    else:
        fingerprint_issue = "当前源码指纹生成失败：" + summarize_output(fingerprint_run.stdout)

latest_fingerprint_ok = (
    latest_fingerprint is not None
    and current_fingerprint is not None
    and latest_fingerprint.get("fingerprint") == current_fingerprint.get("fingerprint")
)
if latest_fingerprint is not None and current_fingerprint is not None and not latest_fingerprint_ok:
    fingerprint_issue = (
        f"{evidence_dir_display} 证据包源码指纹过期：evidence={latest_fingerprint.get('fingerprint')} "
        f"current={current_fingerprint.get('fingerprint')}。"
    )

latest_all_required_ok = not latest_missing and not latest_failed and latest_fingerprint_ok

def check_ok(name):
    return next((item["ok"] for item in checks if item["name"] == name), False)


def latest_gap() -> str:
    parts = []
    if latest_missing:
        parts.append(f"{evidence_dir_display} 证据包缺失 {latest_missing}")
    if latest_failed:
        parts.append(f"{evidence_dir_display} 证据包失败 {latest_failed}")
    if fingerprint_issue:
        parts.append(fingerprint_issue)
    return "；".join(parts)


production_missing_lines = [
    line for line in production_report_text.splitlines()
    if line.startswith("- 缺失 `") or line.startswith("- 无效 `")
]
production_evidence_ok = not production_missing_lines and bool(production_report_text)
route_ok = bool(route_summary) and route_summary.get("missing_route_total") == 0 and route_summary.get("migrated_manifest_route_total") == route_summary.get("route_total") == len(routes)
frontend_api_check = next((item for item in checks if item["name"] == "frontend dist API coverage"), None)
frontend_api_ok = bool(frontend_api_check and frontend_api_check["ok"])

soak_candidates = []
for pattern in (
    "/tmp/mochat-go-soak-24h-*/soak.ndjson",
    "/tmp/mochat-go-soak.*/soak.ndjson",
):
    soak_candidates.extend(pathlib.Path(path) for path in glob.glob(pattern))
soak_candidates = [path for path in soak_candidates if path.exists()]
soak_summary = None
if soak_candidates:
    soak_path = max(soak_candidates, key=lambda path: path.stat().st_mtime)
    rows = []
    for line in soak_path.read_text(encoding="utf-8").splitlines():
        if line.strip():
            rows.append(json.loads(line))
    if rows:
        first = dt.datetime.fromisoformat(rows[0]["ts"])
        last = dt.datetime.fromisoformat(rows[-1]["ts"])
        duration_hours = max(0.0, (last - first).total_seconds() / 3600)
        soak_summary = {
            "path": str(soak_path),
            "iterations": len(rows),
            "duration_hours": duration_hours,
            "route_total_set": sorted({row.get("route_total") for row in rows}),
            "migrated_count_set": sorted({row.get("migrated_route_count") for row in rows}),
        }

soak_process_result = subprocess.run(
    ["./scripts/list_standalone_soak_processes.sh"],
    cwd=repo,
    text=True,
    stdout=subprocess.PIPE,
    stderr=subprocess.DEVNULL,
    check=False,
)
long_running = [line.strip() for line in soak_process_result.stdout.splitlines() if line.strip()]

try:
    docker_output = subprocess.check_output(
        ["docker", "ps", "--filter", "name=mochat-go", "--format", "{{.Names}} {{.Status}} {{.Ports}}"],
        cwd=repo,
        text=True,
        stderr=subprocess.DEVNULL,
    )
    docker_lines = [line for line in docker_output.splitlines() if line.strip()]
except Exception:
    docker_lines = []

requirements = [
    {
        "name": "能运行的独立 Go 项目",
        "ok": check_ok("standalone independence") and check_ok("independent package smoke") and route_ok,
        "evidence": "standalone independence + independent package smoke + runtime route coverage",
        "gap": "" if route_ok else "运行时路由覆盖未证明 224/224。",
    },
    {
        "name": "不依赖原 mochat/PHP 项目",
        "ok": check_ok("independent package smoke"),
        "evidence": "临时目录没有原 mochat/ 兄弟目录，MOCHAT_SOURCE_ROOT 和 MOCHAT_COMPAT_MANIFEST 指向不存在路径。",
        "gap": "" if check_ok("independent package smoke") else "独立交付包动态 smoke 未通过。",
    },
    {
        "name": "全部 manifest 功能已纳入 Go 本地验收",
        "ok": route_ok and check_ok("PHP inventory parity") and check_ok("manifest route smoke coverage") and check_ok("functional module matrix") and latest_all_required_ok,
        "evidence": f"PHP 源码清单对齐、manifest routes 224/224、smoke 直接覆盖、功能模块矩阵、{evidence_dir_display} 全套短验收。",
        "gap": "" if (check_ok("PHP inventory parity") and latest_all_required_ok) else "；".join(item for item in [
            "" if check_ok("PHP inventory parity") else "PHP 源码清单对齐未通过。",
            latest_gap(),
        ] if item),
    },
    {
        "name": "旧前端 dist API 全量承接",
        "ok": frontend_api_ok,
        "evidence": "扫描 dashboard/sidebar/operation 内置 dist 的静态 API 声明，并与 Go runtime dispatch/routes 对比。",
        "gap": "" if frontend_api_ok else (frontend_api_check["summary"] if frontend_api_check else "前端 dist API 覆盖审计未执行。"),
    },
    {
        "name": "SaaS、worker、cron、frontend 本地闭环",
        "ok": latest_all_required_ok and frontend_api_ok and check_ok("worker SaaS usage assertions") and check_ok("SaaS metric coverage") and check_ok("SaaS storage reclaim coverage"),
        "evidence": f"{evidence_dir_display} acceptance-saas/workers/cron/frontend + 前端 dist API 覆盖 + SaaS 指标和存储回收审计。",
        "gap": "；".join(item for item in [
            "" if latest_all_required_ok else latest_gap(),
            "" if frontend_api_ok else (frontend_api_check["summary"] if frontend_api_check else "前端 dist API 覆盖审计未执行。"),
        ] if item),
    },
    {
        "name": "生产外部证据",
        "ok": production_evidence_ok,
        "evidence": "MOCHAT_EVIDENCE_* 指向的真实证据文件。",
        "gap": "；".join(line.removeprefix("- ") for line in production_missing_lines),
    },
    {
        "name": "满 24 小时长稳证据" if require_24h else "短稳/外部监控稳定性证据（本轮不要求 24 小时）",
        "ok": (not require_24h) or (soak_summary is not None and soak_summary["duration_hours"] >= 24),
        "evidence": "standalone_soak_24h.sh 的 soak.ndjson 或目标部署环境等价长稳记录。" if require_24h else "本轮不启动 24 小时持续运行；稳定性默认接受短稳回归、健康检查和外部监控记录。",
        "gap": "" if not require_24h else "未找到满 24 小时 soak.ndjson。",
    },
]

goal_complete = all(item["ok"] for item in requirements)
gap_labels = []
local_requirement_names = {
    "能运行的独立 Go 项目",
    "不依赖原 mochat/PHP 项目",
    "全部 manifest 功能已纳入 Go 本地验收",
    "旧前端 dist API 全量承接",
    "SaaS、worker、cron、frontend 本地闭环",
}
if any((not item["ok"]) for item in requirements if item["name"] in local_requirement_names):
    gap_labels.append("本地验收")
if not frontend_api_ok:
    gap_labels.append("前端 API")
if not production_evidence_ok:
    gap_labels.append("生产证据")
if require_24h and not (soak_summary is not None and soak_summary["duration_hours"] >= 24):
    gap_labels.append("长稳证据")
if not gap_labels and not goal_complete:
    gap_labels.append("本地验收")
goal_status = "目标已完成" if goal_complete else f"目标未完成，继续保留{'、'.join(gap_labels)}缺口"
generated_at = dt.datetime.now().astimezone().strftime("%Y-%m-%d %H:%M:%S %Z")

lines = [
    "# MoChat Go 目标完成度审计",
    "",
    f"- 生成时间：`{generated_at}`",
    f"- 当前结论：**{goal_status}**",
    f"- 严格模式：`{strict}`",
    f"- 是否要求本轮满 24 小时长稳证据：`{require_24h}`",
    f"- 本地短证据包：`{evidence_dir_display}`",
    "",
    "## 目标拆解",
    "",
]

for item in requirements:
    marker = "通过" if item["ok"] else "未完成"
    lines.append(f"- {marker} `{item['name']}`：{item['evidence']}")
    if item["gap"]:
        lines.append(f"  缺口：{item['gap']}")

lines.extend([
    "",
    "## 当前门禁结果",
    "",
])
for item in checks:
    marker = "通过" if item["ok"] else "失败"
    summary = f"：{item['summary']}" if item["summary"] else ""
    lines.append(f"- {marker} `{item['name']}`{summary}")

lines.extend([
    "",
    "## manifest 与路由",
    "",
    f"- 路由：`{len(routes)}`",
    f"- 表：`{len(tables)}`",
    f"- crontab：`{len(crontabs)}`",
    f"- 事件处理器：`{len(events)}`",
    f"- 异步队列注解：`{len(queues)}`",
])
if route_summary:
    lines.extend([
        f"- 运行时 route_total：`{route_summary.get('route_total')}`",
        f"- 运行时 migrated_manifest_route_total：`{route_summary.get('migrated_manifest_route_total')}`",
        f"- 运行时 missing_route_total：`{route_summary.get('missing_route_total')}`",
        f"- Go extra_route_total：`{route_summary.get('extra_route_total')}`",
    ])

lines.extend([
    "",
    f"## 本地短证据包：{evidence_dir_display}",
    "",
])
if latest_results:
    for slug in required_latest_slugs:
        item = latest_results.get(slug)
        if not item:
            lines.append(f"- 缺失 `{slug}`")
        else:
            marker = "通过" if item.get("returncode") == 0 else "失败"
            lines.append(f"- {marker} `{slug}`：`{item.get('log')}`")
else:
    lines.append(f"- 未找到 `{evidence_dir_display}/results.jsonl`。")
if latest_fingerprint and current_fingerprint:
    lines.append(f"- 证据包源码指纹：`{latest_fingerprint.get('fingerprint')}`")
    lines.append(f"- 当前源码指纹：`{current_fingerprint.get('fingerprint')}`")
    lines.append(f"- 源码指纹匹配：`{latest_fingerprint_ok}`")
elif fingerprint_issue:
    lines.append(f"- 源码指纹：{fingerprint_issue}")

lines.extend([
    "",
    "## 持续运行与残留",
    "",
])
if soak_summary:
    lines.append(f"- 最近 soak：`{soak_summary['path']}`，`{soak_summary['iterations']}` 轮，约 `{soak_summary['duration_hours']:.2f}` 小时。")
else:
    lines.append("- 未找到可解析的 soak.ndjson。")
if long_running:
    lines.extend(f"- 发现 `standalone_soak_24h.sh` 进程：`{line}`" for line in long_running)
else:
    lines.append("- 当前未发现 `standalone_soak_24h.sh` 进程。")
if docker_lines:
    lines.extend(f"- 发现 mochat-go 容器：`{line}`" for line in docker_lines)
else:
    lines.append("- 当前未发现正在运行的 `mochat-go` Docker 容器。")

lines.extend([
    "",
    "## 生产证据缺口",
    "",
])
if production_missing_lines:
    lines.extend(production_missing_lines)
else:
    lines.append("- 未发现缺失或无效生产证据。")

report = "\n".join(lines) + "\n"
if out_path:
    out_path.parent.mkdir(parents=True, exist_ok=True)
    out_path.write_text(report, encoding="utf-8")
else:
    sys.stdout.write(report)

if json_out_path:
    json_out_path.parent.mkdir(parents=True, exist_ok=True)
    payload = {
        "schema": 1,
        "generated_at": generated_at,
        "status": goal_status,
        "goal_complete": goal_complete,
        "strict": strict,
        "require_24h": require_24h,
        "evidence_dir": evidence_dir_display,
        "php_source_root": php_source_root,
        "requirements": [
            {
                "name": item["name"],
                "ok": bool(item["ok"]),
                "evidence": item["evidence"],
                "gap": item["gap"],
            }
            for item in requirements
        ],
        "checks": [
            {
                "name": item["name"],
                "ok": bool(item["ok"]),
                "summary": item["summary"],
            }
            for item in checks
        ],
        "manifest": {
            "routes": len(routes),
            "tables": len(tables),
            "crontabs": len(crontabs),
            "event_handlers": len(events),
            "async_queues": len(queues),
        },
        "route_summary": route_summary or {},
        "frontend_api_ok": frontend_api_ok,
        "production_evidence_ok": production_evidence_ok,
        "production_evidence_missing": production_missing_lines,
        "latest_evidence": {
            "required_slugs": required_latest_slugs,
            "missing": latest_missing,
            "failed": latest_failed,
            "all_required_ok": latest_all_required_ok,
            "results": {
                slug: {
                    "returncode": latest_results.get(slug, {}).get("returncode"),
                    "log": latest_results.get(slug, {}).get("log"),
                }
                for slug in required_latest_slugs
                if slug in latest_results
            },
        },
        "source_fingerprint": {
            "latest": latest_fingerprint.get("fingerprint") if latest_fingerprint else "",
            "current": current_fingerprint.get("fingerprint") if current_fingerprint else "",
            "latest_file_count": latest_fingerprint.get("file_count") if latest_fingerprint else 0,
            "current_file_count": current_fingerprint.get("file_count") if current_fingerprint else 0,
            "matches": latest_fingerprint_ok,
            "issue": fingerprint_issue,
        },
        "soak": soak_summary or {},
        "running_soak_processes": long_running,
        "running_mochat_go_containers": docker_lines,
        "gap_labels": gap_labels,
    }
    json_out_path.write_text(json.dumps(payload, ensure_ascii=False, indent=2, sort_keys=True) + "\n", encoding="utf-8")

if strict and not goal_complete:
    sys.exit(1)
PY
