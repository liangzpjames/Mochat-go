#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

if [ "${1:-}" = "-h" ] || [ "${1:-}" = "--help" ]; then
  cat <<'EOF'
Usage: ./scripts/production_evidence_doctor.sh

生产证据采集诊断入口：检查 6 类真实生产证据的标准文件、采集环境变量、严格证据门禁状态，并生成中文 readiness 报告。

默认只诊断，不访问生产、不启动 24 小时持续运行。

环境变量：
  MOCHAT_PRODUCTION_EVIDENCE_DIR
      生产证据目录，默认 docs/phases/phase-pre0-standalone/evidence/production。
  MOCHAT_PRODUCTION_EVIDENCE_DOCTOR_OUT
      输出报告，默认 docs/phases/phase-pre0-standalone/evidence/production/readiness.md。
  MOCHAT_PRODUCTION_EVIDENCE_DOCTOR_JSON_OUT
      输出机器可读 JSON，默认 docs/phases/phase-pre0-standalone/evidence/production/readiness.json。
  MOCHAT_PRODUCTION_EVIDENCE_DOCTOR_ENV_OUT
      输出待填写环境变量清单，默认 docs/phases/phase-pre0-standalone/evidence/production/readiness.env.todo。
  MOCHAT_PRODUCTION_EVIDENCE_DOCTOR_RUN
      设为 1 时，对采集环境变量已齐备但标准证据缺失/需刷新的项目执行采集或导入脚本，默认 0。
  MOCHAT_PRODUCTION_EVIDENCE_DOCTOR_STRICT
      设为 1 时，只要严格生产证据门禁未通过或采集失败即返回非 0，默认 0。
  MOCHAT_PRODUCTION_EVIDENCE_DOCTOR_REFRESH
      设为 1 时，即使标准证据文件已存在，也会在 RUN=1 时刷新可采集项目，默认 0。

MySQL 5.7 amd64 证据导入需要设置：
  MOCHAT_MYSQL57_AMD64_EVIDENCE_SOURCE

其他真实账号、SaaS、生产前端和稳定性证据所需变量见：
  docs/phases/phase-pre0-standalone/evidence/production/README.md

该脚本不会启动 24 小时持续运行。
EOF
  exit 0
fi

python3 - <<'PY'
import datetime as dt
import json
import os
import pathlib
import re
import subprocess
import sys

repo = pathlib.Path.cwd()


def bool_env(name: str, default: str = "0") -> bool:
    value = os.environ.get(name, default)
    if value not in {"0", "1"}:
        raise SystemExit(f"unknown {name}={value}; valid values: 0, 1")
    return value == "1"


def to_path(raw: str) -> pathlib.Path:
    path = pathlib.Path(raw)
    return path if path.is_absolute() else repo / path


def display_path(path: pathlib.Path) -> str:
    try:
        return path.resolve().relative_to(repo).as_posix()
    except ValueError:
        return path.resolve().as_posix()


def env_present(name: str) -> bool:
    return bool(os.environ.get(name, "").strip())


def env_group_ready(groups: list[list[str]]) -> tuple[bool, list[str]]:
    missing_by_group = [[name for name in group if not env_present(name)] for group in groups]
    if any(not missing for missing in missing_by_group):
        return True, []
    preferred = missing_by_group[0] if missing_by_group else []
    return False, preferred


def run_capture(item: dict, log_dir: pathlib.Path, evidence_dir: pathlib.Path) -> dict:
    env = os.environ.copy()
    env.update(item.get("default_env", {}))
    log_path = log_dir / f"{item['key']}.log"
    log_path.parent.mkdir(parents=True, exist_ok=True)
    started = dt.datetime.now().astimezone().strftime("%Y-%m-%d %H:%M:%S %Z")
    try:
        completed = subprocess.run(
            item["command"],
            cwd=repo,
            env=env,
            text=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            check=False,
        )
        output = completed.stdout
        returncode = completed.returncode
    except Exception as exc:
        output = f"无法执行: {exc}\n"
        returncode = 1
    finished = dt.datetime.now().astimezone().strftime("%Y-%m-%d %H:%M:%S %Z")
    log_path.write_text(output, encoding="utf-8")
    return {
        "returncode": returncode,
        "log": display_path(log_path),
        "started_at": started,
        "finished_at": finished,
    }


def parse_gate_validations(json_path: pathlib.Path, report_path: pathlib.Path) -> dict:
    if json_path.exists():
        try:
            payload = json.loads(json_path.read_text(encoding="utf-8"))
        except Exception:
            payload = {}
        validations = {}
        for item in payload.get("evidence", []):
            title = str(item.get("title") or "")
            if not title:
                continue
            valid = bool(item.get("valid"))
            if valid:
                status = "已验证"
                issue = ""
            elif item.get("path"):
                status = "无效"
                issue = str(item.get("issue") or "严格生产证据门禁未通过。")
            else:
                status = "缺失"
                issue = str(item.get("issue") or "证据文件缺失。")
            validations[title] = {
                "valid": valid,
                "status": status,
                "issue": issue,
            }
        if validations:
            return validations

    if not report_path.exists():
        return {}
    validations = {}
    pattern = re.compile(r"^- (已验证|无效|缺失) `([^`]+)`[：:](.*)$")
    for line in report_path.read_text(encoding="utf-8", errors="ignore").splitlines():
        match = pattern.match(line)
        if not match:
            continue
        status, title, detail = match.groups()
        validations[title] = {
            "valid": status == "已验证",
            "status": status,
            "issue": detail.strip(),
        }
    return validations


evidence_dir = to_path(os.environ.get("MOCHAT_PRODUCTION_EVIDENCE_DIR", "docs/phases/phase-pre0-standalone/evidence/production"))
out_path = to_path(os.environ.get("MOCHAT_PRODUCTION_EVIDENCE_DOCTOR_OUT", str(evidence_dir / "readiness.md")))
json_out_path = to_path(os.environ.get("MOCHAT_PRODUCTION_EVIDENCE_DOCTOR_JSON_OUT", str(evidence_dir / "readiness.json")))
env_out_path = to_path(os.environ.get("MOCHAT_PRODUCTION_EVIDENCE_DOCTOR_ENV_OUT", str(evidence_dir / "readiness.env.todo")))
run_enabled = bool_env("MOCHAT_PRODUCTION_EVIDENCE_DOCTOR_RUN", "0")
strict = bool_env("MOCHAT_PRODUCTION_EVIDENCE_DOCTOR_STRICT", "0")
refresh = bool_env("MOCHAT_PRODUCTION_EVIDENCE_DOCTOR_REFRESH", "0")
log_dir = evidence_dir / "doctor-logs"

items = [
    {
        "key": "mysql57-amd64",
        "title": "MySQL 5.7 amd64 真实容器门禁",
        "evidence_env": "MOCHAT_EVIDENCE_MYSQL57_AMD64",
        "file": evidence_dir / "mysql57-amd64.log",
        "ready_groups": [["MOCHAT_MYSQL57_AMD64_EVIDENCE_SOURCE"]],
        "command": ["./scripts/import_mysql57_amd64_evidence.sh"],
        "default_env": {
            "MOCHAT_MYSQL57_AMD64_EVIDENCE_TARGET": display_path(evidence_dir / "mysql57-amd64.log"),
        },
        "next": "设置 MOCHAT_MYSQL57_AMD64_EVIDENCE_SOURCE 指向 amd64 CI artifact/log 后运行本脚本或 import_mysql57_amd64_evidence.sh。",
    },
    {
        "key": "real-wecom",
        "title": "真实企业微信账号联调",
        "evidence_env": "MOCHAT_EVIDENCE_REAL_WECOM",
        "file": evidence_dir / "real-wecom.md",
        "ready_groups": [[
            "MOCHAT_REAL_WECOM_BASE_URL",
            "MOCHAT_REAL_WECOM_TOKEN",
            "MOCHAT_REAL_WECOM_CALLBACK_EVIDENCE",
            "MOCHAT_REAL_WECOM_AGENT_MESSAGE_EVIDENCE",
            "MOCHAT_REAL_WECOM_LOG_REF",
        ]],
        "command": ["./scripts/capture_real_wecom_evidence.sh"],
        "default_env": {
            "MOCHAT_REAL_WECOM_OUT": display_path(evidence_dir / "real-wecom.md"),
        },
        "next": "准备真实企微 dashboard token、回调解密记录、应用消息记录和日志/工单引用。",
    },
    {
        "key": "real-wechat-open",
        "title": "真实微信开放平台联调",
        "evidence_env": "MOCHAT_EVIDENCE_REAL_WECHAT_OPEN",
        "file": evidence_dir / "real-wechat-open.md",
        "ready_groups": [[
            "MOCHAT_REAL_WECHAT_OPEN_BASE_URL",
            "MOCHAT_REAL_WECHAT_OPEN_TOKEN",
            "MOCHAT_REAL_WECHAT_OPEN_TICKET_EVIDENCE",
            "MOCHAT_REAL_WECHAT_OPEN_AUTH_REDIRECT_EVIDENCE",
            "MOCHAT_REAL_WECHAT_OPEN_CANCEL_EVIDENCE",
            "MOCHAT_REAL_WECHAT_OPEN_MESSAGE_CALLBACK_EVIDENCE",
            "MOCHAT_REAL_WECHAT_OPEN_LOG_REF",
        ]],
        "command": ["./scripts/capture_real_wechat_open_evidence.sh"],
        "default_env": {
            "MOCHAT_REAL_WECHAT_OPEN_OUT": display_path(evidence_dir / "real-wechat-open.md"),
        },
        "next": "准备真实开放平台 ticket、预授权/授权回跳、取消授权、消息回调和日志/工单引用。",
    },
    {
        "key": "real-saas-tenants",
        "title": "真实 SaaS 多租户数据回归",
        "evidence_env": "MOCHAT_EVIDENCE_REAL_SAAS_TENANTS",
        "file": evidence_dir / "real-saas-tenants.md",
        "ready_groups": [[
            "MOCHAT_REAL_SAAS_BASE_URL",
            "MOCHAT_REAL_SAAS_TENANT_A_TOKEN",
            "MOCHAT_REAL_SAAS_TENANT_B_TOKEN",
            "MOCHAT_REAL_SAAS_TENANT_A_MARKER",
            "MOCHAT_REAL_SAAS_TENANT_B_MARKER",
            "MOCHAT_REAL_SAAS_TENANT_A_FORBIDDEN_PATHS",
            "MOCHAT_REAL_SAAS_TENANT_B_FORBIDDEN_PATHS",
            "MOCHAT_REAL_SAAS_QUOTA_EVIDENCE",
            "MOCHAT_REAL_SAAS_UPLOAD_LEDGER_EVIDENCE",
            "MOCHAT_REAL_SAAS_ASYNC_EVIDENCE",
            "MOCHAT_REAL_SAAS_ALERT_EVIDENCE",
        ]],
        "command": ["./scripts/capture_real_saas_tenants_evidence.sh"],
        "default_env": {
            "MOCHAT_REAL_SAAS_OUT": display_path(evidence_dir / "real-saas-tenants.md"),
        },
        "next": "准备两个真实租户 token、租户标识、互访禁止路径、额度/账本/异步/告警证据。",
    },
    {
        "key": "prod-frontend",
        "title": "生产前端浏览器回归",
        "evidence_env": "MOCHAT_EVIDENCE_PROD_FRONTEND",
        "file": evidence_dir / "prod-frontend.md",
        "ready_groups": [["MOCHAT_PROD_FRONTEND_BASE_URL"]],
        "command": ["./scripts/capture_prod_frontend_evidence.sh"],
        "default_env": {
            "MOCHAT_PROD_FRONTEND_OUT": display_path(evidence_dir / "prod-frontend.md"),
            "MOCHAT_PROD_FRONTEND_SCREENSHOT_DIR": display_path(evidence_dir / "frontend-screenshots"),
        },
        "next": "设置生产 base URL；需要登录时先准备 MOCHAT_PROD_FRONTEND_AUTH_STATE。",
    },
    {
        "key": "stability",
        "title": "稳定性记录",
        "evidence_env": "MOCHAT_EVIDENCE_STABILITY",
        "file": evidence_dir / "stability.md",
        "ready_groups": [
            ["MOCHAT_STABILITY_MONITOR_EVIDENCE", "MOCHAT_STABILITY_HEALTH_EVIDENCE", "MOCHAT_STABILITY_RESOURCE_EVIDENCE", "MOCHAT_STABILITY_TIME_RANGE", "MOCHAT_STABILITY_LOG_REF"],
            ["MOCHAT_STABILITY_SOAK_LOG", "MOCHAT_STABILITY_LOG_REF"],
        ],
        "command": ["./scripts/capture_stability_evidence.sh"],
        "default_env": {
            "MOCHAT_STABILITY_OUT": display_path(evidence_dir / "stability.md"),
        },
        "next": "提供短稳/外部监控、健康检查和资源记录；legacy soak.ndjson 仍可导入。本脚本不会启动新的 24 小时运行。",
    },
]

evidence_dir.mkdir(parents=True, exist_ok=True)

results = []
capture_failures = []
for item in items:
    path = pathlib.Path(item["file"])
    exists = path.exists() and path.is_file()
    size = path.stat().st_size if exists else 0
    ready, missing_env = env_group_ready(item["ready_groups"])
    should_run = run_enabled and ready and (refresh or not exists)
    capture = None
    if should_run:
        capture = run_capture(item, log_dir, evidence_dir)
        if capture["returncode"] != 0:
            capture_failures.append(item["title"])
        exists = path.exists() and path.is_file()
        size = path.stat().st_size if exists else 0
    results.append(
        {
            "item": item,
            "exists": exists,
            "size": size,
            "ready": ready,
            "missing_env": missing_env,
            "capture": capture,
        }
    )

gate_report = evidence_dir / "readiness-production-evidence.md"
gate_json = evidence_dir / "readiness-production-evidence.json"
gate_log = evidence_dir / "readiness-production-evidence.log"
gate_env = os.environ.copy()
gate_env.update(
    {
        "MOCHAT_PRODUCTION_EVIDENCE_SKIP_QUICK": "1",
        "MOCHAT_PRODUCTION_EVIDENCE_STRICT": "1",
        "MOCHAT_PRODUCTION_EVIDENCE_OUT": display_path(gate_report),
        "MOCHAT_PRODUCTION_EVIDENCE_JSON_OUT": display_path(gate_json),
    }
)
for result in results:
    item = result["item"]
    gate_env[item["evidence_env"]] = display_path(item["file"])

completed = subprocess.run(
    ["./scripts/production_evidence_check.sh"],
    cwd=repo,
    env=gate_env,
    text=True,
    stdout=subprocess.PIPE,
    stderr=subprocess.STDOUT,
    check=False,
)
gate_log.write_text(completed.stdout, encoding="utf-8")
gate_ok = completed.returncode == 0
gate_validations = parse_gate_validations(gate_json, gate_report)
for result in results:
    title = result["item"]["title"]
    validation = gate_validations.get(title, {
        "valid": False,
        "status": "未知",
        "issue": "严格生产证据门禁未返回该证据项状态。",
    })
    result["evidence_valid"] = bool(validation["valid"])
    result["evidence_validation_status"] = validation["status"]
    result["evidence_issue"] = validation["issue"]

generated_at = dt.datetime.now().astimezone().strftime("%Y-%m-%d %H:%M:%S %Z")
complete_files = all(result["exists"] for result in results)
valid_files = all(result["evidence_valid"] for result in results)
ready_to_capture = all(result["ready"] or result["evidence_valid"] for result in results)


def append_env_assignment(lines: list[str], emitted: set[str], name: str) -> None:
    if name in emitted:
        lines.append(f"# {name}=  # 已在上方列出")
        return
    lines.append(f"{name}=")
    emitted.add(name)


env_lines = [
    "# MoChat Go 生产证据采集待填写环境变量",
    "",
    f"# 生成时间：{generated_at}",
    f"# 生产证据目录：{display_path(evidence_dir)}",
    "# 只包含变量名和空值；不要直接把 token/secret 写入本模板。",
    "# 建议先复制为本地文件再填写，本地文件已加入 .gitignore：",
    f"#   cp {display_path(env_out_path)} {display_path(evidence_dir / 'readiness.env.local')}",
    "# 填写本地文件后先预检，再在受控终端加载并执行采集：",
    f"#   MOCHAT_PRODUCTION_EVIDENCE_ENV_FILE={display_path(evidence_dir / 'readiness.env.local')} ./scripts/production_evidence_env_preflight.sh",
    "#   set -a",
    f"#   . {display_path(evidence_dir / 'readiness.env.local')}",
    "#   set +a",
    "#   MOCHAT_PRODUCTION_EVIDENCE_DOCTOR_RUN=1 ./scripts/production_evidence_doctor.sh",
    "#   ./scripts/collect_production_evidence_pack.sh",
    "",
]

emitted_env: set[str] = set()
for result in results:
    item = result["item"]
    env_lines.extend(
        [
            f"# {item['title']}",
            f"# 标准文件：{display_path(item['file'])}",
            f"# 采集命令：{' '.join(item['command'])}",
            f"# 下一步：{item['next']}",
        ]
    )
    if result["exists"]:
        env_lines.append("# 当前标准文件已存在；仅在需要刷新证据时填写下面变量。")
    else:
        env_lines.append("# 当前标准文件缺失；请按下面变量补齐采集或导入输入。")
    groups = item["ready_groups"]
    if len(groups) == 1:
        for env_name in groups[0]:
            append_env_assignment(env_lines, emitted_env, env_name)
    else:
        for index, group in enumerate(groups, start=1):
            env_lines.append(f"# 可选方案 {index}：填完本组即可采集")
            for env_name in group:
                append_env_assignment(env_lines, emitted_env, env_name)
    env_lines.append("")

env_out_path.parent.mkdir(parents=True, exist_ok=True)
env_out_path.write_text("\n".join(env_lines) + "\n", encoding="utf-8")

lines = [
    "# MoChat Go 生产证据采集诊断",
    "",
    f"- 生成时间：`{generated_at}`",
    f"- 生产证据目录：`{display_path(evidence_dir)}`",
    f"- 待填写环境变量清单：`{display_path(env_out_path)}`",
    f"- 执行采集/导入：`{run_enabled}`",
    f"- 刷新已存在证据：`{refresh}`",
    f"- 严格退出：`{strict}`",
    "- 24 小时持续运行：`未启动`",
    f"- 标准证据文件齐备：`{complete_files}`",
    f"- 标准证据文件有效：`{valid_files}`",
    f"- 采集环境变量齐备或有效文件已存在：`{ready_to_capture}`",
    f"- 严格生产证据文件门禁：`{'通过' if gate_ok else '未通过'}`",
    "",
    "## 证据项",
    "",
    "| 证据项 | 标准文件 | 文件状态 | 采集准备 | 本轮动作 | 下一步 |",
    "| --- | --- | --- | --- | --- | --- |",
]

for result in results:
    item = result["item"]
    if result["evidence_valid"]:
        file_status = f"有效（{result['size']} bytes）"
    elif result["exists"]:
        file_status = f"存在但无效（{result['size']} bytes）：{result['evidence_issue']}"
    else:
        file_status = "缺失"
    if result["ready"]:
        ready_status = "可采集"
    else:
        ready_status = "缺少 " + ", ".join(f"`{name}`" for name in result["missing_env"])
    capture = result["capture"]
    if capture is None:
        action = "未执行"
    else:
        action = ("通过" if capture["returncode"] == 0 else "失败") + f" `{capture['log']}`"
    next_step = "运行严格门禁复核" if result["exists"] and result["ready"] else item["next"]
    lines.append(
        f"| {item['title']} | `{display_path(item['file'])}` | {file_status} | {ready_status} | {action} | {next_step} |"
    )

lines.extend(
    [
        "",
        "## 严格证据门禁",
        "",
        f"- 报告：`{display_path(gate_report)}`",
        f"- JSON：`{display_path(gate_json)}`",
        f"- 日志：`{display_path(gate_log)}`",
        f"- 返回码：`{completed.returncode}`",
        "",
        "## 采集命令",
        "",
        "默认仅诊断：",
        "",
        "```bash",
        "./scripts/production_evidence_doctor.sh",
        "```",
        "",
        "环境变量齐备后执行采集/导入并复核：",
        "",
        f"先复制 `{display_path(env_out_path)}` 为 `docs/phases/phase-pre0-standalone/evidence/production/readiness.env.local` 并填写，再执行预检：",
        "",
        "```bash",
        "MOCHAT_PRODUCTION_EVIDENCE_ENV_FILE=docs/phases/phase-pre0-standalone/evidence/production/readiness.env.local ./scripts/production_evidence_env_preflight.sh",
        "```",
        "",
        "预检通过后加载变量并采集：",
        "",
        "```bash",
        "set -a",
        ". docs/phases/phase-pre0-standalone/evidence/production/readiness.env.local",
        "set +a",
        "MOCHAT_PRODUCTION_EVIDENCE_DOCTOR_RUN=1 ./scripts/production_evidence_doctor.sh",
        "```",
        "",
        "全部证据齐备后进入生产候选：",
        "",
        "```bash",
        "./scripts/collect_production_evidence_pack.sh",
        "```",
    ]
)

out_path.parent.mkdir(parents=True, exist_ok=True)
out_path.write_text("\n".join(lines) + "\n", encoding="utf-8")

json_items = []
for result in results:
    item = result["item"]
    capture = result["capture"]
    json_items.append(
        {
            "key": item["key"],
            "title": item["title"],
            "evidence_env": item["evidence_env"],
            "standard_file": display_path(item["file"]),
            "exists": result["exists"],
            "size": result["size"],
            "evidence_valid": result["evidence_valid"],
            "evidence_validation_status": result["evidence_validation_status"],
            "evidence_issue": result["evidence_issue"],
            "capture_ready": result["ready"],
            "missing_env": result["missing_env"],
            "required_env_groups": item["ready_groups"],
            "command": item["command"],
            "next": item["next"],
            "capture": capture,
        }
    )

json_payload = {
    "schema": 1,
    "generated_at": generated_at,
    "production_evidence_dir": display_path(evidence_dir),
    "run_enabled": run_enabled,
    "refresh_existing": refresh,
    "strict": strict,
    "started_24h_run": False,
    "standard_files_complete": complete_files,
    "standard_files_valid": valid_files,
    "capture_env_ready_or_file_exists": ready_to_capture,
    "gate_ok": gate_ok,
    "gate_returncode": completed.returncode,
    "gate_report": display_path(gate_report),
    "gate_json": display_path(gate_json),
    "gate_log": display_path(gate_log),
    "env_template": display_path(env_out_path),
    "missing_count": sum(1 for item in json_items if not item["exists"]),
    "not_ready_count": sum(1 for item in json_items if not item["capture_ready"] and not item["exists"]),
    "items": json_items,
}
json_out_path.parent.mkdir(parents=True, exist_ok=True)
json_out_path.write_text(json.dumps(json_payload, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")

print(display_path(out_path))
print(display_path(env_out_path))
print(display_path(json_out_path))

if strict and (not gate_ok or capture_failures):
    sys.exit(1)
PY
