#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

OUT="${MOCHAT_STAGE_REPORT_OUT:-}"

python3 - "$OUT" <<'PY'
import datetime as dt
import glob
import json
import os
import pathlib
import subprocess
import sys
import tempfile

out_path = pathlib.Path(sys.argv[1]) if len(sys.argv) > 1 and sys.argv[1] else None
repo = pathlib.Path.cwd()
manifest_path = repo / "internal/server/compat_manifest_embedded.json"
acceptance_path = repo / "scripts/standalone_acceptance.sh"

manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
routes = manifest.get("routes", [])
tables = manifest.get("tables", [])
crontabs = manifest.get("crontabs", [])
events = manifest.get("event_handlers", [])
queues = manifest.get("async_queues", [])

smoke_scripts = sorted((repo / "scripts").glob("smoke_*.sh"))
acceptance = acceptance_path.read_text(encoding="utf-8")
referenced_smokes = [path for path in smoke_scripts if f"./scripts/{path.name}" in acceptance]
unreferenced_smokes = [path.name for path in smoke_scripts if path not in referenced_smokes]

quick_checks = [
    ("独立运行静态门禁", ["./scripts/audit_standalone_independence.sh"], {}),
    ("独立交付包动态 smoke", ["./scripts/smoke_independent_package.sh"], {}),
    ("acceptance smoke 接入覆盖", ["./scripts/audit_acceptance_suite_coverage.sh"], {}),
    ("manifest 路由 smoke 直接覆盖", ["./scripts/audit_manifest_route_smoke_coverage.sh"], {}),
    ("功能模块矩阵", ["./scripts/audit_functional_module_matrix.sh"], {}),
    ("前端 dist API 覆盖", ["./scripts/audit_frontend_dist_api_coverage.sh"], {"MOCHAT_FRONTEND_DIST_API_STRICT": "1"}),
    ("队列注解覆盖", ["./scripts/audit_queue_annotation_coverage.sh"], {}),
    ("企微回调事件覆盖", ["./scripts/audit_wework_callback_event_coverage.sh"], {}),
    ("worker SaaS 用量断言", ["./scripts/audit_worker_saas_usage_assertions.sh"], {}),
    ("SaaS 指标覆盖", ["./scripts/audit_saas_metric_coverage.sh"], {}),
    ("上传存储回收覆盖", ["./scripts/audit_saas_storage_reclaim_coverage.sh"], {}),
]

def summarize_output(output: str) -> str:
    lines = [line.strip() for line in output.splitlines() if line.strip()]
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

quick_results = []
for name, cmd, extra_env in quick_checks:
    try:
        child_env = os.environ.copy()
        child_env.setdefault("MOCHAT_QUEUE_AUDIT_SOURCE", "embedded")
        child_env.update(extra_env)
        completed = subprocess.run(
            cmd,
            cwd=repo,
            env=child_env,
            text=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            check=False,
        )
        quick_results.append(
            {
                "name": name,
                "ok": completed.returncode == 0,
                "summary": summarize_output(completed.stdout),
            }
        )
    except Exception as exc:
        quick_results.append({"name": name, "ok": False, "summary": f"无法执行: {exc}"})

source_fingerprint = None
with tempfile.TemporaryDirectory(prefix="mochat-go-stage-fingerprint.") as tmp:
    fingerprint_path = pathlib.Path(tmp) / "source-fingerprint.json"
    fingerprint_run = subprocess.run(
        ["python3", "./scripts/source_fingerprint.py", "--out", str(fingerprint_path)],
        cwd=repo,
        text=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        check=False,
    )
    if fingerprint_run.returncode == 0 and fingerprint_path.exists():
        source_fingerprint = json.loads(fingerprint_path.read_text(encoding="utf-8"))
    else:
        source_fingerprint = {
            "error": summarize_output(fingerprint_run.stdout) or "source fingerprint generation failed"
        }

soak_candidates = []
explicit_soak = os.environ.get("MOCHAT_STAGE_SOAK_LOG", "").strip()
if explicit_soak:
    soak_candidates.append(pathlib.Path(explicit_soak))
else:
    for pattern in (
        "/tmp/mochat-go-soak-24h-*/soak.ndjson",
        "/tmp/mochat-go-soak.*/soak.ndjson",
    ):
        soak_candidates.extend(pathlib.Path(path) for path in glob.glob(pattern))
soak_candidates = [path for path in soak_candidates if path.exists()]
soak_path = max(soak_candidates, key=lambda path: path.stat().st_mtime) if soak_candidates else None

soak_summary = None
if soak_path:
    rows = []
    for line in soak_path.read_text(encoding="utf-8").splitlines():
        line = line.strip()
        if line:
            rows.append(json.loads(line))
    if rows:
        rss = [row.get("rss_kb") for row in rows if row.get("rss_kb") is not None]
        soak_summary = {
            "path": str(soak_path),
            "iterations": len(rows),
            "first_ts": rows[0].get("ts", ""),
            "last_ts": rows[-1].get("ts", ""),
            "last_iteration": rows[-1].get("iteration", ""),
            "route_total_set": sorted({row.get("route_total") for row in rows}),
            "migrated_count_set": sorted({row.get("migrated_route_count") for row in rows}),
            "rss_min": min(rss) if rss else "",
            "rss_max": max(rss) if rss else "",
        }

docker_lines = []
try:
    output = subprocess.check_output(
        ["docker", "ps", "--format", "{{.Names}} {{.Status}} {{.Ports}}"],
        text=True,
        stderr=subprocess.DEVNULL,
    )
    docker_lines = [line for line in output.splitlines() if "mochat-go" in line]
except Exception:
    docker_lines = ["docker ps unavailable"]

generated_at = dt.datetime.now().astimezone().strftime("%Y-%m-%d %H:%M:%S %Z")
frontend_dist_ok = next(
    (result["ok"] for result in quick_results if result["name"] == "前端 dist API 覆盖"),
    False,
)
if frontend_dist_ok:
    status = "收口验收/生产化补证阶段，不能标记最终完成"
else:
    status = "收口验收/前端 API 补齐/生产化补证阶段，不能标记最终完成"

lines = [
    "# MoChat Go 独立版阶段报告",
    "",
    f"- 生成时间：`{generated_at}`",
    f"- 当前结论：**{status}**",
    "",
    "## 已具备的主体证据",
    "",
    f"- 内置 manifest：路由 `{len(routes)}` 条，表 `{len(tables)}` 张，crontab `{len(crontabs)}` 个，事件处理器 `{len(events)}` 个，异步队列注解 `{len(queues)}` 条。",
    f"- smoke 脚本：`{len(smoke_scripts)}` 个；`standalone_acceptance.sh` 已引用 `{len(referenced_smokes)}` 个。",
    f"- 未纳入 acceptance 的 smoke：`{len(unreferenced_smokes)}` 个。",
    "- 独立性门禁应由 `scripts/test.sh`、`scripts/audit_standalone_independence.sh`、`scripts/smoke_independent_package.sh`、`scripts/audit_manifest_route_smoke_coverage.sh` 和 `scripts/standalone_route_coverage.sh` 共同证明。",
    "",
    "## 当前快速门禁",
    "",
]

for result in quick_results:
    marker = "通过" if result["ok"] else "失败"
    suffix = f"：{result['summary']}" if result["summary"] else ""
    lines.append(f"- {marker} `{result['name']}`{suffix}")

lines.extend(
    [
        "",
        "> 说明：本节是报告生成时的当前快速审计结果；`standalone_route_coverage.sh` 会启动独立 MySQL/Redis 做运行时路由覆盖验证，仍需作为独立命令保留。",
        "",
        "## 源码与验收指纹",
        "",
    ]
)

if source_fingerprint and not source_fingerprint.get("error"):
    lines.extend(
        [
            f"- 当前指纹：`{source_fingerprint.get('fingerprint')}`",
            f"- 纳入文件数：`{source_fingerprint.get('file_count')}`",
            "- 指纹范围：Go 源码、验收脚本、部署配置、前端构建产物和关键构建文件；排除 `docs/evidence/`、`storage/`、`output/`、`tmp/` 和 `node_modules` 等生成物。",
        ]
    )
else:
    error = source_fingerprint.get("error") if source_fingerprint else "未生成源码指纹。"
    lines.append(f"- 源码指纹生成失败：{error}")

lines.extend(
    [
        "",
        "## 最近持续运行证据",
        "",
    ]
)

if soak_summary:
    lines.extend(
        [
            f"- soak 日志：`{soak_summary['path']}`",
            f"- 探测轮数：`{soak_summary['iterations']}`，最后轮次：`{soak_summary['last_iteration']}`",
            f"- 时间范围：`{soak_summary['first_ts']}` 到 `{soak_summary['last_ts']}`",
            f"- route_total 集合：`{soak_summary['route_total_set']}`",
            f"- migrated_route_count 集合：`{soak_summary['migrated_count_set']}`",
            f"- RSS 范围：`{soak_summary['rss_min']}-{soak_summary['rss_max']} KB`",
        ]
    )
else:
    lines.append("- 未找到可解析的 `soak.ndjson`；稳定性证据默认可使用短稳回归、健康检查和外部监控记录。")

lines.extend(
    [
        "",
        "## 当前运行残留",
        "",
    ]
)
if docker_lines:
    lines.extend(f"- `{line}`" for line in docker_lines)
else:
    lines.append("- 未发现正在运行的 `mochat-go` Docker 容器。")

lines.extend(["", "## 不能标记最终完成的原因", ""])
if not frontend_dist_ok:
    lines.append("- 旧前端 dist API 仍需全量承接：必须让 dashboard/sidebar/operation 内置 dist 中声明的 API 都能被 Go runtime 响应，不能只依赖 manifest 224/224 判断。")
lines.extend(
    [
        "- 真实企业微信/微信开放平台账号联调尚未形成最终证据：授权、回调解密、通讯录、客户、客户群、标签、素材、公众号和企微应用链路仍需真实账号验证。",
        "- MySQL 5.7 真实容器迁移 smoke 需要在 amd64 CI 或等价环境执行；本机 arm64 跳过不能作为生产兼容证据。",
        "- SaaS 生产化仍需真实租户业务数据验证菜单权限、企业归属、资源额度、上传账本、异步任务、告警和运维处置。",
        "- 生产前端仍需按真实业务操作路径扩大浏览器回归，覆盖更多页面流转和异常路径。",
        "",
        "## 建议下一步",
        "",
        "1. 在 amd64 CI 执行 `env -u GOROOT ./scripts/ci_mysql57_amd64.sh`。",
        "2. 准备真实企业微信和微信开放平台测试账号，补真实外部链路验收记录。",
        "3. 用两个以上真实租户数据执行 SaaS 权限、额度、上传、异步任务和告警回归。",
        "4. 汇总生产证据文件后执行 `MOCHAT_PRODUCTION_EVIDENCE_STRICT=1 ./scripts/production_evidence_check.sh`。",
        "5. 上线前再执行一次完整 `env -u GOROOT ./scripts/standalone_acceptance.sh`。",
    ]
)

report = "\n".join(lines) + "\n"
if out_path:
    out_path.parent.mkdir(parents=True, exist_ok=True)
    out_path.write_text(report, encoding="utf-8")
else:
    sys.stdout.write(report)
PY
