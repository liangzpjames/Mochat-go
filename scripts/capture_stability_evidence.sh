#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

if [ "${1:-}" = "-h" ] || [ "${1:-}" = "--help" ]; then
  cat <<'EOF'
Usage: MOCHAT_STABILITY_MONITOR_EVIDENCE=@monitor.txt ./scripts/capture_stability_evidence.sh

从目标部署环境已有短稳回归、健康检查或外部监控记录生成稳定性证据，输出 docs/evidence/production/stability.md。

必填环境变量：
  MOCHAT_STABILITY_LOG_REF
      日志、监控、告警平台、工单或健康检查引用。

二选一：
  MOCHAT_STABILITY_MONITOR_EVIDENCE
      外部监控或短稳回归摘要，可用 @文件路径。推荐使用。
  MOCHAT_STABILITY_SOAK_LOG
      legacy 兼容入口：standalone_soak_24h.sh 或目标环境等价探测输出的 NDJSON 文件。

可选环境变量：
  MOCHAT_STABILITY_OUT
      输出 Markdown，默认 docs/evidence/production/stability.md。
  MOCHAT_STABILITY_MIN_DURATION_SECONDS
      最小稳定性记录秒数，默认 300。
  MOCHAT_STABILITY_MIN_ITERATIONS
      最小探测轮数，默认 2。
  MOCHAT_STABILITY_EXPECTED_ROUTE_TOTAL
      期望路由下限，默认 224。
  MOCHAT_STABILITY_EXPECTED_MODE
      期望运行模式，默认 standalone-go。
  MOCHAT_STABILITY_HEALTH_EVIDENCE
      健康检查补充证据摘要或 @文件路径。
  MOCHAT_STABILITY_RESOURCE_EVIDENCE
      资源使用补充证据摘要或 @文件路径。
  MOCHAT_STABILITY_TIME_RANGE
      使用外部监控摘要时必须填写明确起止日期。

该脚本只读取已有稳定性证据，不启动 24 小时持续运行。
EOF
  exit 0
fi

SOURCE_FINGERPRINT="${MOCHAT_EVIDENCE_SOURCE_FINGERPRINT:-$(python3 ./scripts/source_fingerprint.py | python3 -c 'import json, sys; print(json.load(sys.stdin)["fingerprint"])')}"
export MOCHAT_EVIDENCE_SOURCE_FINGERPRINT="$SOURCE_FINGERPRINT"

WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-stability-evidence.XXXXXX")"
cleanup() {
  rm -rf "$WORK_DIR"
}
trap cleanup EXIT INT TERM

SCRIPT_PATH="$WORK_DIR/capture-stability.py"

cat >"$SCRIPT_PATH" <<'PY'
import datetime as dt
import json
import os
import pathlib
import re
import statistics
import sys

DATE_RE = re.compile(r"\d{4}[-/]\d{1,2}[-/]\d{1,2}")
HEALTH_RE = re.compile(r"/readyz|health|健康|存活|探测|200", re.IGNORECASE)
RESOURCE_RE = re.compile(r"\brss\b|memory|cpu|连接|内存|资源|load|qps", re.IGNORECASE)
ROUTE_RE = re.compile(r"route_total|missing_route_total|路由覆盖|路由", re.IGNORECASE)


def env(name, fallback=""):
    value = os.environ.get(name)
    return fallback if value is None or value == "" else value


def load_value(raw):
    if not raw:
        return ""
    if raw.startswith("@"):
        return pathlib.Path(raw[1:]).read_text(encoding="utf-8")
    path = pathlib.Path(raw)
    if path.exists() and path.is_file():
        return path.read_text(encoding="utf-8")
    return raw


credential_pattern = re.compile(
    r"([\"']?\b(?:access_token|refresh_token|component_access_token|authorizer_access_token|authorizer_refresh_token|pre_auth_code|verify_ticket|"
    r"token|secret|corpsecret|component_appsecret|encodingaeskey|password|passwd|session_key|jwt)\b[\"']?\s*[:=]\s*[\"']?)"
    r"[^\"'`,\s&}]{8,}",
    re.IGNORECASE,
)
query_credential_pattern = re.compile(
    r"([?&](?:access_token|refresh_token|component_access_token|authorizer_access_token|authorizer_refresh_token|pre_auth_code|verify_ticket|"
    r"token|secret|corpsecret|component_appsecret|encodingaeskey|password|passwd|session_key|jwt)=)[^&\s]+",
    re.IGNORECASE,
)
bearer_pattern = re.compile(r"\b(Bearer\s+)[A-Za-z0-9._~+/=-]{20,}", re.IGNORECASE)
authorization_pattern = re.compile(
    r"\b(Authorization\s*[:=]\s*)(Bearer\s+)?[A-Za-z0-9._~+/=-]{12,}",
    re.IGNORECASE,
)


def redact_sensitive_text(value):
    text = str(value or "")
    text = authorization_pattern.sub(lambda match: f"{match.group(1)}{match.group(2) or ''}<redacted>", text)
    text = bearer_pattern.sub(r"\1<redacted>", text)
    text = credential_pattern.sub(r"\1<redacted>", text)
    text = query_credential_pattern.sub(r"\1<redacted>", text)
    return text


def markdown_escape(value):
    return " ".join(redact_sensitive_text(value).split())


def parse_positive_int(name, fallback):
    raw = env(name, str(fallback))
    try:
        value = int(raw)
    except ValueError:
        raise SystemExit(f"{name} must be an integer, got: {raw}")
    if value < 0:
        raise SystemExit(f"{name} must be >= 0, got: {raw}")
    return value


def parse_ts(value):
    if not value:
        return None
    text = str(value)
    if text.endswith("Z"):
        text = text[:-1] + "+00:00"
    try:
        return dt.datetime.fromisoformat(text)
    except ValueError:
        return None


def parse_soak_log(path):
    records = []
    for line_no, line in enumerate(path.read_text(encoding="utf-8").splitlines(), start=1):
        text = line.strip()
        if not text:
            continue
        try:
            record = json.loads(text)
        except json.JSONDecodeError as exc:
            raise SystemExit(f"{path}:{line_no} is not valid JSON: {exc}")
        records.append(record)
    return records


out_path = pathlib.Path(env("MOCHAT_STABILITY_OUT", "docs/evidence/production/stability.md"))
soak_log_raw = env("MOCHAT_STABILITY_SOAK_LOG")
monitor_evidence = load_value(env("MOCHAT_STABILITY_MONITOR_EVIDENCE"))
health_evidence = load_value(env("MOCHAT_STABILITY_HEALTH_EVIDENCE"))
resource_evidence = load_value(env("MOCHAT_STABILITY_RESOURCE_EVIDENCE"))
log_ref = env("MOCHAT_STABILITY_LOG_REF")
source_fingerprint = env("MOCHAT_EVIDENCE_SOURCE_FINGERPRINT")
min_duration = parse_positive_int("MOCHAT_STABILITY_MIN_DURATION_SECONDS", 300)
min_iterations = parse_positive_int("MOCHAT_STABILITY_MIN_ITERATIONS", 2)
expected_route_total = parse_positive_int("MOCHAT_STABILITY_EXPECTED_ROUTE_TOTAL", 224)
expected_mode = env("MOCHAT_STABILITY_EXPECTED_MODE", "standalone-go")

failures = []
records = []
soak_path = None
if soak_log_raw:
    soak_path = pathlib.Path(soak_log_raw)
    if not soak_path.exists():
        failures.append(f"MOCHAT_STABILITY_SOAK_LOG 指向的文件不存在：{soak_log_raw}")
    elif not soak_path.is_file():
        failures.append(f"MOCHAT_STABILITY_SOAK_LOG 不是普通文件：{soak_log_raw}")
    else:
        records = parse_soak_log(soak_path)

if not records and not str(monitor_evidence or "").strip():
    failures.append("缺少 MOCHAT_STABILITY_MONITOR_EVIDENCE 或 legacy MOCHAT_STABILITY_SOAK_LOG")
if not log_ref:
    failures.append("缺少 MOCHAT_STABILITY_LOG_REF")
if not source_fingerprint:
    failures.append("缺少 MOCHAT_EVIDENCE_SOURCE_FINGERPRINT")

first_ts = None
last_ts = None
duration_seconds = 0
route_totals = []
migrated_counts = []
rss_values = []
health_summary = ""
route_summary = ""
resource_summary = ""

if records:
    timestamps = [parse_ts(item.get("ts")) for item in records]
    timestamps = [item for item in timestamps if item is not None]
    if timestamps:
        first_ts = min(timestamps)
        last_ts = max(timestamps)
        duration_seconds = max(0, int((last_ts - first_ts).total_seconds()))
    route_totals = [int(item.get("route_total") or 0) for item in records]
    migrated_counts = [int(item.get("migrated_route_count") or 0) for item in records]
    rss_values = [int(item.get("rss_kb")) for item in records if item.get("rss_kb") is not None]
    modes = {str(item.get("mode") or "") for item in records}
    standalone_values = {bool(item.get("standalone")) for item in records}

    if len(records) < min_iterations:
        failures.append(f"探测轮数不足：{len(records)} < {min_iterations}")
    if duration_seconds < min_duration:
        failures.append(f"稳定性记录时间不足：{duration_seconds}s < {min_duration}s")
    if route_totals and min(route_totals) < expected_route_total:
        failures.append(f"路由覆盖不足：min(route_total)={min(route_totals)} < {expected_route_total}")
    if migrated_counts and route_totals and min(migrated_counts) < max(route_totals):
        failures.append("migrated_route_count 小于 route_total")
    if expected_mode and modes != {expected_mode}:
        failures.append(f"运行模式不一致：{sorted(modes)}，期望 {expected_mode}")
    if standalone_values != {True}:
        failures.append(f"standalone 标记不全为 true：{sorted(standalone_values)}")

    health_summary = (
        f"{len(records)} 轮健康检查均为 standalone={sorted(standalone_values)}、mode={sorted(modes)}。"
    )
    route_summary = (
        f"route_total 集合 {sorted(set(route_totals))}，migrated_route_count 集合 {sorted(set(migrated_counts))}。"
    )
    if rss_values:
        resource_summary = (
            f"RSS {min(rss_values)}-{max(rss_values)} KB，平均 {int(statistics.mean(rss_values))} KB。"
        )
else:
    health_summary = markdown_escape(health_evidence or "外部监控摘要已提供。")
    route_summary = markdown_escape(monitor_evidence)
    resource_summary = markdown_escape(resource_evidence or monitor_evidence)
    if not str(health_evidence or monitor_evidence).strip():
        failures.append("缺少健康检查证据")
    elif not HEALTH_RE.search(str(health_evidence or monitor_evidence)):
        failures.append("健康检查证据必须包含 /readyz、health、健康、探测或 200 等标记")
    if str(expected_route_total) not in str(monitor_evidence) or not ROUTE_RE.search(str(monitor_evidence)):
        failures.append("外部监控摘要必须包含路由覆盖和期望路由数量")
    if not str(resource_evidence or monitor_evidence).strip():
        failures.append("缺少资源使用证据")
    elif not RESOURCE_RE.search(str(resource_evidence or monitor_evidence)):
        failures.append("资源使用证据必须包含 RSS、memory、CPU、连接、内存或资源等标记")
    if len(DATE_RE.findall(env("MOCHAT_STABILITY_TIME_RANGE"))) < 2:
        failures.append("外部监控稳定性证据必须设置包含起止日期的 MOCHAT_STABILITY_TIME_RANGE")

passed = not failures
generated_at = dt.datetime.now(dt.timezone.utc).isoformat()
time_range = (
    f"{first_ts.isoformat()} 至 {last_ts.isoformat()}"
    if first_ts and last_ts
    else markdown_escape(env("MOCHAT_STABILITY_TIME_RANGE", "外部监控记录时间范围见引用"))
)
soak_ref = str(soak_path) if soak_path else "未使用 soak.ndjson；使用外部监控或短稳摘要"
stability_record = (
    f"持续探测 {duration_seconds}s，探测轮数 {len(records)}。"
    if records
    else markdown_escape(monitor_evidence)
)

lines = [
    "# 短稳/外部监控稳定性记录",
    "",
    f"- 执行时间：{generated_at}",
    f"- 时间范围：{time_range}",
    f"- 源码指纹：{source_fingerprint}",
    f"- 结论：{'通过' if passed else '失败'}",
    "",
    f"目标部署环境稳定性记录{'已复核' if passed else '仍存在未满足项'}。",
    "",
    "## 证据记录",
    "",
    f"- 短稳回归：{stability_record}",
    f"- 健康检查：{markdown_escape(health_summary)}",
    f"- 路由覆盖：{markdown_escape(route_summary)}",
    f"- 资源使用：{markdown_escape(resource_summary or '未提供 RSS，见外部监控引用。')}",
    f"- 日志 / 监控引用：{markdown_escape(log_ref)}；{markdown_escape(soak_ref)}",
    "",
]

if records:
    lines.extend(
        [
            "## legacy soak.ndjson 摘要",
            "",
            f"- 记录数：`{len(records)}`",
            f"- 最小 route_total：`{min(route_totals) if route_totals else 0}`",
            f"- 最大 route_total：`{max(route_totals) if route_totals else 0}`",
            f"- 最小 migrated_route_count：`{min(migrated_counts) if migrated_counts else 0}`",
            f"- 最大 migrated_route_count：`{max(migrated_counts) if migrated_counts else 0}`",
            f"- RSS 范围：`{min(rss_values) if rss_values else 0}-{max(rss_values) if rss_values else 0} KB`",
            "",
        ]
    )

if failures:
    lines.extend(["## 未通过项", ""])
    lines.extend(f"- {failure}" for failure in failures)
    lines.append("")

out_path.parent.mkdir(parents=True, exist_ok=True)
out_path.write_text("\n".join(lines) + "\n", encoding="utf-8")

if not passed:
    print(f"持续运行稳定性证据未通过，已写入 {out_path}", file=sys.stderr)
    for failure in failures:
        print(f"- {failure}", file=sys.stderr)
    sys.exit(1)

print(out_path)
PY

python3 "$SCRIPT_PATH"
