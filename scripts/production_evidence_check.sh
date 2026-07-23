#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

STRICT="${MOCHAT_PRODUCTION_EVIDENCE_STRICT:-0}"
OUT="${MOCHAT_PRODUCTION_EVIDENCE_OUT:-}"
JSON_OUT="${MOCHAT_PRODUCTION_EVIDENCE_JSON_OUT:-}"

python3 - "$STRICT" "$OUT" "$JSON_OUT" <<'PY'
import datetime as dt
import json
import os
import pathlib
import re
import subprocess
import sys

strict = sys.argv[1] == "1"
out_path = pathlib.Path(sys.argv[2]) if len(sys.argv) > 2 and sys.argv[2] else None
json_path = pathlib.Path(sys.argv[3]) if len(sys.argv) > 3 and sys.argv[3] else None
repo = pathlib.Path.cwd()
skip_quick = os.environ.get("MOCHAT_PRODUCTION_EVIDENCE_SKIP_QUICK", "0") == "1"
try:
    min_evidence_bytes = int(os.environ.get("MOCHAT_PRODUCTION_EVIDENCE_MIN_BYTES", "32"))
except ValueError:
    min_evidence_bytes = 32


def load_current_source_fingerprint():
    try:
        completed = subprocess.run(
            [sys.executable, "./scripts/source_fingerprint.py"],
            cwd=repo,
            text=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            check=False,
        )
    except Exception:
        return ""
    if completed.returncode != 0:
        return ""
    try:
        payload = json.loads(completed.stdout)
    except json.JSONDecodeError:
        return ""
    fingerprint = str(payload.get("fingerprint") or "").strip().lower()
    if not re.fullmatch(r"[0-9a-f]{64}", fingerprint):
        return ""
    return fingerprint


current_source_fingerprint = load_current_source_fingerprint()

quick_checks = [
    ("standalone independence", ["./scripts/audit_standalone_independence.sh"]),
    ("independent package smoke", ["./scripts/smoke_independent_package.sh"]),
    ("acceptance suite coverage", ["./scripts/audit_acceptance_suite_coverage.sh"]),
    ("manifest route smoke coverage", ["./scripts/audit_manifest_route_smoke_coverage.sh"]),
    ("functional module matrix", ["./scripts/audit_functional_module_matrix.sh"]),
    ("queue annotation coverage", ["./scripts/audit_queue_annotation_coverage.sh"]),
    ("wework callback event coverage", ["./scripts/audit_wework_callback_event_coverage.sh"]),
    ("worker SaaS usage assertions", ["./scripts/audit_worker_saas_usage_assertions.sh"]),
    ("SaaS metric coverage", ["./scripts/audit_saas_metric_coverage.sh"]),
    ("SaaS storage reclaim coverage", ["./scripts/audit_saas_storage_reclaim_coverage.sh"]),
]

def summarize_output(output_text):
    lines = [line.strip() for line in output_text.splitlines() if line.strip()]
    if not lines:
        return ""
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

def run_check(name, cmd):
    child_env = os.environ.copy()
    child_env.setdefault("MOCHAT_QUEUE_AUDIT_SOURCE", "embedded")
    try:
        completed = subprocess.run(
            cmd,
            cwd=repo,
            env=child_env,
            text=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            check=False,
        )
    except Exception as exc:
        return {
            "name": name,
            "ok": False,
            "summary": f"无法执行: {exc}",
        }
    return {
        "name": name,
        "ok": completed.returncode == 0,
        "summary": summarize_output(completed.stdout),
    }

check_results = [] if skip_quick else [run_check(name, cmd) for name, cmd in quick_checks]

manifest = json.loads((repo / "internal/server/compat_manifest_embedded.json").read_text(encoding="utf-8"))
routes = manifest.get("routes", [])
tables = manifest.get("tables", [])
crontabs = manifest.get("crontabs", [])
events = manifest.get("event_handlers", [])
queues = manifest.get("async_queues", [])

evidence_items = [
    (
        "MySQL 5.7 amd64 真实容器门禁",
        "MOCHAT_EVIDENCE_MYSQL57_AMD64",
        "在 amd64/x86_64 CI 执行 `env -u GOROOT ./scripts/ci_mysql57_amd64.sh` 的日志或报告。",
    ),
    (
        "真实企业微信账号联调",
        "MOCHAT_EVIDENCE_REAL_WECOM",
        "真实企业微信账号下授权、回调解密、通讯录、客户、客户群、标签、素材和企微应用链路验收记录。",
    ),
    (
        "真实微信开放平台联调",
        "MOCHAT_EVIDENCE_REAL_WECHAT_OPEN",
        "真实微信开放平台第三方平台 ticket、预授权、授权回跳、公众号资料回填、取消授权和消息回调验收记录。",
    ),
    (
        "真实 SaaS 多租户数据回归",
        "MOCHAT_EVIDENCE_REAL_SAAS_TENANTS",
        "两个以上真实租户业务数据下菜单权限、企业归属、资源额度、上传账本、异步任务和告警隔离验收记录。",
    ),
    (
        "生产前端浏览器回归",
        "MOCHAT_EVIDENCE_PROD_FRONTEND",
        "dashboard/sidebar/operation 生产构建产物的真实浏览器业务路径和异常路径回归记录。",
    ),
    (
        "稳定性记录",
        "MOCHAT_EVIDENCE_STABILITY",
        "目标部署环境短稳回归、健康检查或外部监控记录；legacy 场景可指向 `soak.ndjson`。",
    ),
]

required_terms = {
    "MOCHAT_EVIDENCE_REAL_WECOM": ["授权", "回调", "通讯录", "客户", "客户群", "标签", "素材"],
    "MOCHAT_EVIDENCE_REAL_WECHAT_OPEN": ["ticket", "预授权", "授权回跳", "资料回填", "取消授权", "消息回调"],
    "MOCHAT_EVIDENCE_REAL_SAAS_TENANTS": ["租户", "菜单权限", "企业归属", "资源额度", "上传账本", "异步任务", "告警"],
    "MOCHAT_EVIDENCE_PROD_FRONTEND": ["dashboard", "sidebar", "operation", "生产", "异常"],
    "MOCHAT_EVIDENCE_STABILITY": ["短稳回归", "结论"],
}

required_record_terms = {
    "MOCHAT_EVIDENCE_REAL_WECOM": ["执行时间", "证据记录", "请求或操作路径", "关键响应摘要", "截图 / 日志 / 工单引用"],
    "MOCHAT_EVIDENCE_REAL_WECHAT_OPEN": ["执行时间", "证据记录", "请求或操作路径", "关键响应摘要", "截图 / 日志 / 工单引用"],
    "MOCHAT_EVIDENCE_REAL_SAAS_TENANTS": ["执行时间", "租户 A", "租户 B", "跨租户访问隔离", "用量刷新和额度拦截", "异步任务执行记录"],
    "MOCHAT_EVIDENCE_PROD_FRONTEND": ["执行时间", "构建版本", "主要业务路径", "异常路径", "控制台错误", "网络请求错误"],
    "MOCHAT_EVIDENCE_STABILITY": ["时间范围", "短稳回归", "健康检查", "路由覆盖", "资源使用", "日志 / 监控引用"],
}

source_fingerprint_required = {item[1] for item in evidence_items}
source_fingerprint_pattern = re.compile(
    r"(?:源码指纹|source\s+fingerprint)\s*[:：]\s*`?([0-9a-f]{64})`?",
    re.IGNORECASE,
)

secret_value_patterns = [
    (
        "Authorization/Bearer token",
        re.compile(r"\bAuthorization\s*[:=]\s*(?:Bearer\s+)?([A-Za-z0-9._~+/=-]{20,})", re.IGNORECASE),
    ),
    (
        "Bearer token",
        re.compile(r"\bBearer\s+([A-Za-z0-9._~+/=-]{20,})", re.IGNORECASE),
    ),
    (
        "credential assignment",
        re.compile(
            r"[\"']?\b(access_token|refresh_token|component_access_token|authorizer_access_token|authorizer_refresh_token|pre_auth_code|verify_ticket|"
            r"token|secret|corpsecret|component_appsecret|encodingaeskey|password|passwd|session_key|jwt)\b[\"']?"
            r"\s*[:=]\s*[\"']?([^\"'`,\s&}]{12,})",
            re.IGNORECASE,
        ),
    ),
    (
        "credential query parameter",
        re.compile(
            r"[?&](access_token|refresh_token|component_access_token|authorizer_access_token|authorizer_refresh_token|pre_auth_code|verify_ticket|"
            r"token|secret|corpsecret|component_appsecret|encodingaeskey|password|passwd|session_key|jwt)=([^&\s]{8,})",
            re.IGNORECASE,
        ),
    ),
]

redacted_markers = ("redacted", "masked", "hidden", "<", ">", "***", "xxx", "脱敏", "已脱敏")

def is_redacted(value):
    lowered = value.lower()
    return any(marker in lowered for marker in redacted_markers)

def sensitive_findings(text):
    findings = []
    for line_no, line in enumerate(text.splitlines(), start=1):
        for label, pattern in secret_value_patterns:
            for match in pattern.finditer(line):
                value = match.group(match.lastindex or 1)
                if value and not is_redacted(value):
                    findings.append((line_no, label))
                    break
            if findings and findings[-1][0] == line_no:
                break
        if len(findings) >= 5:
            break
    return findings

non_production_hosts = {
    "localhost",
    "127.0.0.1",
    "0.0.0.0",
    "::1",
    "example.com",
    "example.org",
    "example.net",
}
non_production_suffixes = (
    ".localhost",
    ".example.com",
    ".example.org",
    ".example.net",
    ".test",
    ".invalid",
)
url_pattern = re.compile(r"https?://[^\s<>'\"`，。；、)）\]]+", re.IGNORECASE)

def is_non_production_host(host):
    normalized = (host or "").strip().strip("[]").lower().rstrip(".")
    return normalized in non_production_hosts or any(normalized.endswith(suffix) for suffix in non_production_suffixes)

def non_production_url_findings(text):
    findings = []
    for line_no, line in enumerate(text.splitlines(), start=1):
        for match in url_pattern.finditer(line):
            raw = match.group(0).rstrip(".,;:")
            try:
                from urllib.parse import urlparse

                parsed = urlparse(raw)
            except Exception:
                continue
            if is_non_production_host(parsed.hostname):
                findings.append((line_no, parsed.hostname or "", raw))
                break
        if len(findings) >= 5:
            break
    return findings

def source_fingerprint_issue(text):
    if not current_source_fingerprint:
        return "无法生成当前源码指纹，不能校验生产证据是否对应当前构建。"
    fingerprints = {item.lower() for item in source_fingerprint_pattern.findall(text)}
    if not fingerprints:
        return "证据缺少源码指纹，请记录 `源码指纹：<当前 scripts/source_fingerprint.py fingerprint>`。"
    if fingerprints != {current_source_fingerprint}:
        found = ", ".join(sorted(fingerprints)) or "无"
        return f"证据源码指纹与当前源码不匹配：证据 {found}，当前 {current_source_fingerprint}。"
    return ""

evidence_results = []
for title, env_name, description in evidence_items:
    raw = os.environ.get(env_name, "").strip()
    result = {
        "title": title,
        "env": env_name,
        "path": raw,
        "exists": False,
        "valid": False,
        "size": 0,
        "issue": "",
        "description": description,
    }
    if not raw:
        result["issue"] = f"设置 `{env_name}` 指向证据文件。"
        evidence_results.append(result)
        continue

    path = pathlib.Path(raw)
    if not path.exists():
        result["issue"] = f"`{env_name}={raw}` 指向的文件不存在。"
        evidence_results.append(result)
        continue
    result["exists"] = True
    if not path.is_file():
        result["issue"] = f"`{env_name}={raw}` 不是普通文件。"
        evidence_results.append(result)
        continue

    result["size"] = path.stat().st_size
    if result["size"] < min_evidence_bytes:
        result["issue"] = f"证据文件过小：{result['size']} bytes，小于 {min_evidence_bytes} bytes。"
        evidence_results.append(result)
        continue

    text = path.read_text(encoding="utf-8", errors="ignore")
    lowered = text.lower()
    if "MOCHAT_EVIDENCE_TEMPLATE_DO_NOT_USE".lower() in lowered:
        result["issue"] = "证据文件仍包含模板标记 MOCHAT_EVIDENCE_TEMPLATE_DO_NOT_USE，必须替换为真实验收记录。"
        evidence_results.append(result)
        continue
    if "状态：待验收" in text or "状态: 待验收" in text:
        result["issue"] = "证据文件仍标记为待验收，必须替换为真实已完成验收记录。"
        evidence_results.append(result)
        continue
    findings = sensitive_findings(text)
    if findings:
        summary = "；".join(f"第 {line_no} 行 {label}" for line_no, label in findings)
        result["issue"] = f"证据文件疑似包含未脱敏敏感值：{summary}。请仅保留脱敏摘要、截图/日志/工单引用或 `<redacted>` 占位。"
        evidence_results.append(result)
        continue
    url_findings = non_production_url_findings(text)
    if url_findings:
        summary = "；".join(f"第 {line_no} 行 `{host}`" for line_no, host, _url in url_findings)
        result["issue"] = f"证据文件包含示例域名或本机地址，不是真实生产证据：{summary}。"
        evidence_results.append(result)
        continue
    if env_name == "MOCHAT_EVIDENCE_MYSQL57_AMD64":
        if "skipped on arm64" in lowered or ("mysql:5.7 is amd64-only" in lowered and "skipped" in lowered):
            result["issue"] = "MySQL 5.7 证据包含 arm64 skip，不是 amd64 真实容器门禁。"
            evidence_results.append(result)
            continue
        markers = [
            "mysql57 amd64 ci gate passed",
            "mysql 5.7 schema migration smoke passed",
        ]
        if not any(marker in lowered for marker in markers):
            result["issue"] = "MySQL 5.7 证据缺少 amd64 门禁通过 marker。"
            evidence_results.append(result)
            continue
    terms = required_terms.get(env_name, [])
    if terms:
        missing_terms = [term for term in terms if term.lower() not in lowered]
        if missing_terms:
            result["issue"] = f"证据缺少必要核验项：{', '.join(missing_terms)}。"
            evidence_results.append(result)
            continue
        success_markers = [
            "结论：通过",
            "结论: 通过",
            "验收通过",
            "联调通过",
            "回归通过",
            "门禁通过",
            "结论：稳定",
            "结论: 稳定",
        ]
        if not any(marker.lower() in lowered for marker in success_markers):
            result["issue"] = "证据缺少明确通过结论 marker，例如 `结论：通过`、`验收通过` 或 `回归通过`。"
            evidence_results.append(result)
            continue
    record_terms = required_record_terms.get(env_name, [])
    if record_terms:
        missing_record_terms = [term for term in record_terms if term.lower() not in lowered]
        if missing_record_terms:
            result["issue"] = f"证据缺少结构化记录项：{', '.join(missing_record_terms)}。"
            evidence_results.append(result)
            continue
    if env_name in source_fingerprint_required:
        issue = source_fingerprint_issue(text)
        if issue:
            result["issue"] = issue
            evidence_results.append(result)
            continue

    result["valid"] = True
    evidence_results.append(result)

quick_ok = all(item["ok"] for item in check_results)
evidence_ok = all(item["valid"] for item in evidence_results)
generated_at = dt.datetime.now().astimezone().strftime("%Y-%m-%d %H:%M:%S %Z")
status = "可进入生产候选复核" if quick_ok and evidence_ok else "仍缺生产证据，不能标记最终完成"

lines = [
    "# MoChat Go 生产证据检查",
    "",
    f"- 生成时间：`{generated_at}`",
    f"- 当前结论：**{status}**",
    f"- 严格模式：`{strict}`",
    f"- 跳过快速本地门禁：`{skip_quick}`",
    f"- 当前源码指纹：`{current_source_fingerprint or '生成失败'}`",
    "",
    "## 当前迁移清单",
    "",
    f"- 路由：`{len(routes)}`",
    f"- 表：`{len(tables)}`",
    f"- crontab：`{len(crontabs)}`",
    f"- 事件处理器：`{len(events)}`",
    f"- 异步队列注解：`{len(queues)}`",
    "",
    "## 快速本地门禁",
    "",
]

if skip_quick:
    lines.append("- 已按 `MOCHAT_PRODUCTION_EVIDENCE_SKIP_QUICK=1` 跳过，仅校验证据文件。")
else:
    for item in check_results:
        marker = "通过" if item["ok"] else "失败"
        suffix = f"：{item['summary']}" if item["summary"] else ""
        lines.append(f"- {marker} `{item['name']}`{suffix}")

lines.extend(
    [
        "",
        "## 生产证据文件",
        "",
    ]
)

for item in evidence_results:
    if item["valid"]:
        lines.append(f"- 已验证 `{item['title']}`：`{item['path']}`（{item['size']} bytes）")
    elif item["path"]:
        lines.append(f"- 无效 `{item['title']}`：{item['issue']}{item['description']}")
    else:
        lines.append(f"- 缺失 `{item['title']}`：{item['issue']}{item['description']}")

lines.extend(
    [
        "",
        "## 使用方式",
        "",
        "- 默认模式只报告缺口并返回 0，适合阶段交接。",
        "- 上线前设置 `MOCHAT_PRODUCTION_EVIDENCE_STRICT=1`，缺任一生产证据或本地门禁失败都会返回非 0。",
        "- 仅自检证据文件规则时可设置 `MOCHAT_PRODUCTION_EVIDENCE_SKIP_QUICK=1`；生产候选门禁不要跳过快速本地门禁。",
        "- 可设置 `MOCHAT_PRODUCTION_EVIDENCE_MIN_BYTES` 调整证据文件最小字节数，默认 `32`。",
        "- 可设置 `MOCHAT_PRODUCTION_EVIDENCE_OUT=docs/production-evidence.md` 写入 Markdown 报告。",
        "- 可设置 `MOCHAT_PRODUCTION_EVIDENCE_JSON_OUT=docs/production-evidence.json` 同步写入不含完整命令输出和密钥的机器可读 JSON。",
        "- 生产证据文件必须脱敏；严格检查会拒绝疑似原始 Authorization、Bearer、access_token、corpsecret、password 等敏感值。",
        "- 每个生产证据文件必须记录当前 `scripts/source_fingerprint.py` 生成的源码指纹；指纹不匹配会被判定为过期证据。",
    ]
)

report = "\n".join(lines) + "\n"

payload = {
    "schema": 1,
    "generated_at": generated_at,
    "status": status,
    "strict": strict,
    "skip_quick": skip_quick,
    "min_evidence_bytes": min_evidence_bytes,
    "current_source_fingerprint": current_source_fingerprint,
    "manifest": {
        "routes": len(routes),
        "tables": len(tables),
        "crontabs": len(crontabs),
        "event_handlers": len(events),
        "async_queues": len(queues),
    },
    "quick_ok": quick_ok,
    "evidence_ok": evidence_ok,
    "ok": quick_ok and evidence_ok,
    "quick_checks": check_results,
    "evidence": evidence_results,
    "missing_envs": [item["env"] for item in evidence_results if not item["path"]],
    "invalid_evidence": [
        {
            "title": item["title"],
            "env": item["env"],
            "path": item["path"],
            "exists": item["exists"],
            "size": item["size"],
            "issue": item["issue"],
        }
        for item in evidence_results
        if not item["valid"]
    ],
}

if out_path:
    out_path.parent.mkdir(parents=True, exist_ok=True)
    out_path.write_text(report, encoding="utf-8")
else:
    sys.stdout.write(report)

if json_path:
    json_path.parent.mkdir(parents=True, exist_ok=True)
    json_path.write_text(json.dumps(payload, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")

if strict and (not quick_ok or not evidence_ok):
    sys.exit(1)
PY
