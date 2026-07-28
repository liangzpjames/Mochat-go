#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

if [ "${1:-}" = "-h" ] || [ "${1:-}" = "--help" ]; then
  cat <<'EOF'
Usage: ./scripts/production_evidence_env_preflight.sh

生产证据采集环境预检：读取 readiness.json 和待加载的 env 文件，检查 6 类生产证据采集变量是否齐备、@文件引用是否存在、URL 是否是 http(s)，并拒绝把 MySQL 5.7 arm64 skip 日志当成 amd64 通过证据；外部稳定性摘要必须包含路由、健康检查、资源使用和明确时间范围。默认拒绝示例域名和本机地址作为真实生产采集地址。

默认不访问生产、不启动 24 小时持续运行、不打印 token/secret/Bearer 值。

环境变量：
  MOCHAT_PRODUCTION_EVIDENCE_READINESS_JSON
      doctor 生成的 JSON，默认 docs/phases/phase-pre0-standalone/evidence/production/readiness.json。
  MOCHAT_PRODUCTION_EVIDENCE_ENV_FILE
      待预检 env 文件，默认 docs/phases/phase-pre0-standalone/evidence/production/readiness.env.local。
      建议由 readiness.env.todo 复制后填写，不要直接把密钥写入 readiness.env.todo。
  MOCHAT_PRODUCTION_EVIDENCE_PREFLIGHT_OUT
      输出 Markdown 报告，默认 docs/phases/phase-pre0-standalone/evidence/production/preflight.md。
  MOCHAT_PRODUCTION_EVIDENCE_PREFLIGHT_JSON_OUT
      输出机器可读 JSON，默认 docs/phases/phase-pre0-standalone/evidence/production/preflight.json。
  MOCHAT_PRODUCTION_EVIDENCE_PREFLIGHT_STRICT
      设为 1 时，只要任一证据没有有效标准文件且采集环境仍未就绪即返回非 0，默认 0。
  MOCHAT_PRODUCTION_EVIDENCE_PREFLIGHT_ALLOW_NON_PROD
      设为 1 时允许 example.com、localhost、127.0.0.1 等非生产地址，仅用于本地 fixture / smoke，默认 0。

该脚本只做本地预检，不访问生产、不启动 24 小时持续运行。
EOF
  exit 0
fi

python3 - <<'PY'
import datetime as dt
import json
import os
import pathlib
import re
import sys
import zipfile
from urllib.parse import urlparse

repo = pathlib.Path.cwd()
MYSQL57_PASS_RE = re.compile(r"mysql57 amd64 CI gate passed|mysql 5\.7 schema migration smoke passed", re.IGNORECASE)
MYSQL57_ARM64_SKIP_RE = re.compile(r"skipped on arm64|mysql:5\.7 is amd64-only.*skipped", re.IGNORECASE)
DATE_RE = re.compile(r"\d{4}[-/]\d{1,2}[-/]\d{1,2}")
STABILITY_HEALTH_RE = re.compile(r"/readyz|health|健康|存活|探测|200", re.IGNORECASE)
STABILITY_RESOURCE_RE = re.compile(r"\brss\b|memory|cpu|连接|内存|资源|load|qps", re.IGNORECASE)
NON_PROD_HOSTS = {"localhost", "127.0.0.1", "0.0.0.0", "::1", "example.com", "example.org", "example.net"}
NON_PROD_SUFFIXES = (".localhost", ".example.com", ".example.org", ".example.net", ".test", ".invalid")


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


def parse_env_file(path: pathlib.Path) -> tuple[dict[str, str], bool, list[str]]:
    values: dict[str, str] = {}
    warnings: list[str] = []
    if not path.exists():
        return values, False, [f"env 文件不存在：`{display_path(path)}`"]
    for lineno, raw_line in enumerate(path.read_text(encoding="utf-8").splitlines(), start=1):
        line = raw_line.strip()
        if not line or line.startswith("#"):
            continue
        if line.startswith("export "):
            line = line[len("export "):].strip()
        if "=" not in line:
            warnings.append(f"第 {lineno} 行不是 KEY=VALUE 格式，已忽略")
            continue
        key, value = line.split("=", 1)
        key = key.strip()
        value = value.strip()
        if not re.fullmatch(r"[A-Za-z_][A-Za-z0-9_]*", key):
            warnings.append(f"第 {lineno} 行变量名格式异常，已忽略")
            continue
        if len(value) >= 2 and value[0] == value[-1] and value[0] in {"'", '"'}:
            value = value[1:-1]
        values[key] = value
    return values, True, warnings


def lookup_env(values: dict[str, str], name: str) -> str:
    live = os.environ.get(name, "")
    if live.strip():
        return live.strip()
    return values.get(name, "").strip()


def validate_url(name: str, value: str) -> list[str]:
    parsed = urlparse(value)
    if parsed.scheme not in {"http", "https"} or not parsed.netloc:
        return [f"`{name}` 不是有效 http(s) URL"]
    host = (parsed.hostname or "").strip().lower()
    allow_non_prod = bool_env("MOCHAT_PRODUCTION_EVIDENCE_PREFLIGHT_ALLOW_NON_PROD", "0")
    if not allow_non_prod and (host in NON_PROD_HOSTS or host.endswith(NON_PROD_SUFFIXES)):
        return [f"`{name}` 指向示例域名或本机地址 `{host}`，真实生产采集前请替换为目标部署环境地址"]
    return []


def validate_path_value(name: str, value: str) -> list[str]:
    raw = value[1:] if value.startswith("@") else value
    path = to_path(raw)
    if not path.exists():
        return [f"`{name}` 引用的文件不存在"]
    if path.is_file() and path.stat().st_size == 0:
        return [f"`{name}` 引用的文件为空"]
    return []


def read_text_value(name: str, value: str) -> tuple[str, list[str]]:
    raw = value[1:] if value.startswith("@") else value
    if not raw:
        return "", [f"`{name}` 未填写"]
    path = to_path(raw)
    if value.startswith("@") or path.exists():
        if not path.exists():
            return "", [f"`{name}` 引用的文件不存在"]
        if not path.is_file():
            return "", [f"`{name}` 引用的不是文件"]
        if path.stat().st_size == 0:
            return "", [f"`{name}` 引用的文件为空"]
        return path.read_text(encoding="utf-8", errors="replace"), []
    return value, []


def read_mysql57_evidence_text(name: str, value: str) -> tuple[str, list[str]]:
    raw = value[1:] if value.startswith("@") else value
    if not raw:
        return "", [f"`{name}` 未填写证据文件"]
    path = to_path(raw)
    if not path.exists():
        return "", [f"`{name}` 引用的文件不存在"]
    if not path.is_file():
        return "", [f"`{name}` 引用的不是文件"]
    if path.stat().st_size == 0:
        return "", [f"`{name}` 引用的文件为空"]

    if path.suffix.lower() == ".zip":
        try:
            with zipfile.ZipFile(path) as archive:
                try:
                    data = archive.read("mysql57-amd64.log")
                except KeyError:
                    return "", [f"`{name}` artifact zip 缺少 mysql57-amd64.log"]
        except zipfile.BadZipFile:
            return "", [f"`{name}` 不是有效 zip artifact"]
        return data.decode("utf-8", errors="replace"), []

    return path.read_text(encoding="utf-8", errors="replace"), []


def validate_mysql57_source(name: str, value: str) -> list[str]:
    text, errors = read_mysql57_evidence_text(name, value)
    if errors:
        return errors
    if not text.strip():
        return [f"`{name}` MySQL 5.7 amd64 证据为空"]
    if MYSQL57_ARM64_SKIP_RE.search(text):
        return [f"`{name}` MySQL 5.7 证据是 arm64 skip 日志，不是真实 amd64 通过日志"]
    if not MYSQL57_PASS_RE.search(text):
        return [f"`{name}` 缺少 MySQL 5.7 amd64 通过标记"]
    return []


def validate_stability_monitor(name: str, value: str) -> list[str]:
    text, errors = read_text_value(name, value)
    if errors:
        return errors
    if not text.strip():
        return [f"`{name}` 外部监控摘要为空"]
    expected_route_total = os.environ.get("MOCHAT_STABILITY_EXPECTED_ROUTE_TOTAL", "224").strip() or "224"
    lowered = text.lower()
    route_terms = ("route_total", "missing_route_total", "路由覆盖", "路由")
    if expected_route_total not in text or not any(term in lowered for term in route_terms):
        return [f"`{name}` 外部监控摘要必须包含路由覆盖和 `{expected_route_total}`"]
    return []


def validate_stability_health(name: str, value: str) -> list[str]:
    text, errors = read_text_value(name, value)
    if errors:
        return errors
    if not STABILITY_HEALTH_RE.search(text):
        return [f"`{name}` 健康检查证据必须包含 /readyz、health、健康、探测或 200 等标记"]
    return []


def validate_stability_resource(name: str, value: str) -> list[str]:
    text, errors = read_text_value(name, value)
    if errors:
        return errors
    if not STABILITY_RESOURCE_RE.search(text):
        return [f"`{name}` 资源证据必须包含 RSS、memory、CPU、连接、内存或资源等标记"]
    return []


def validate_stability_time_range(name: str, value: str) -> list[str]:
    if len(DATE_RE.findall(value)) < 2:
        return [f"`{name}` 必须包含明确起止日期，例如 `2026-07-08 10:00:00 CST 至 2026-07-08 10:30:00 CST`"]
    return []


def validate_value(name: str, value: str) -> list[str]:
    if not value:
        return []
    errors: list[str] = []
    if name.endswith("_BASE_URL"):
        errors.extend(validate_url(name, value))
    if name == "MOCHAT_MYSQL57_AMD64_EVIDENCE_SOURCE":
        errors.extend(validate_mysql57_source(name, value))
        return errors
    if name == "MOCHAT_STABILITY_MONITOR_EVIDENCE":
        errors.extend(validate_stability_monitor(name, value))
        return errors
    if name == "MOCHAT_STABILITY_HEALTH_EVIDENCE":
        errors.extend(validate_stability_health(name, value))
        return errors
    if name == "MOCHAT_STABILITY_RESOURCE_EVIDENCE":
        errors.extend(validate_stability_resource(name, value))
        return errors
    if name == "MOCHAT_STABILITY_TIME_RANGE":
        errors.extend(validate_stability_time_range(name, value))
        return errors
    file_like_names = {
        "MOCHAT_MYSQL57_AMD64_EVIDENCE_SOURCE",
        "MOCHAT_STABILITY_SOAK_LOG",
        "MOCHAT_PROD_FRONTEND_AUTH_STATE",
    }
    if value.startswith("@") or name in file_like_names:
        errors.extend(validate_path_value(name, value))
    if name.endswith("_FORBIDDEN_PATHS"):
        paths = [part for part in re.split(r"[\s,]+", value) if part]
        if not paths:
            errors.append(f"`{name}` 未包含任何路径")
    return errors


readiness_json = to_path(os.environ.get("MOCHAT_PRODUCTION_EVIDENCE_READINESS_JSON", "docs/phases/phase-pre0-standalone/evidence/production/readiness.json"))
env_file = to_path(os.environ.get("MOCHAT_PRODUCTION_EVIDENCE_ENV_FILE", "docs/phases/phase-pre0-standalone/evidence/production/readiness.env.local"))
out_path = to_path(os.environ.get("MOCHAT_PRODUCTION_EVIDENCE_PREFLIGHT_OUT", "docs/phases/phase-pre0-standalone/evidence/production/preflight.md"))
json_out_path = to_path(os.environ.get("MOCHAT_PRODUCTION_EVIDENCE_PREFLIGHT_JSON_OUT", "docs/phases/phase-pre0-standalone/evidence/production/preflight.json"))
strict = bool_env("MOCHAT_PRODUCTION_EVIDENCE_PREFLIGHT_STRICT", "0")
allow_non_prod = bool_env("MOCHAT_PRODUCTION_EVIDENCE_PREFLIGHT_ALLOW_NON_PROD", "0")

if not readiness_json.exists():
    raise SystemExit(f"readiness json not found: {display_path(readiness_json)}; run ./scripts/production_evidence_doctor.sh first")

readiness = json.loads(readiness_json.read_text(encoding="utf-8"))
env_values, env_file_exists, parse_warnings = parse_env_file(env_file)
generated_at = dt.datetime.now().astimezone().strftime("%Y-%m-%d %H:%M:%S %Z")

items = []
for source in readiness.get("items", []):
    group_results = []
    for index, group in enumerate(source.get("required_env_groups", []), start=1):
        missing = []
        invalid = []
        for name in group:
            value = lookup_env(env_values, name)
            if not value:
                missing.append(name)
                continue
            invalid.extend(validate_value(name, value))
        group_results.append(
            {
                "index": index,
                "missing_env": missing,
                "invalid": invalid,
                "ready": not missing and not invalid,
            }
        )
    ready_group = next((group for group in group_results if group["ready"]), None)
    invalid_group = next((group for group in group_results if group["invalid"]), None)
    best_group = group_results[0] if group_results else None
    standard_file_exists = bool(source.get("exists"))
    standard_file_valid = bool(source.get("evidence_valid"))
    standard_file_issue = source.get("evidence_issue", "")
    status = "ready" if ready_group else "not_ready"
    selected = ready_group or invalid_group or best_group or {"missing_env": [], "invalid": [], "index": None}
    selected_invalid = list(selected["invalid"])
    if standard_file_exists and not standard_file_valid:
        selected_invalid.append(f"标准文件无效：{standard_file_issue or '严格生产证据门禁未通过'}")
    if standard_file_exists and standard_file_valid and not invalid_group:
        status = "standard_file_exists"
    items.append(
        {
            "key": source.get("key"),
            "title": source.get("title"),
            "status": status,
            "standard_file": source.get("standard_file"),
            "standard_file_exists": standard_file_exists,
            "standard_file_valid": standard_file_valid,
            "selected_group_index": selected["index"],
            "missing_env": selected["missing_env"],
            "invalid": selected_invalid,
            "command": source.get("command", []),
            "next": source.get("next", ""),
        }
    )

ready_count = sum(1 for item in items if item["status"] in {"ready", "standard_file_exists"})
standard_file_ready_count = sum(1 for item in items if item["status"] == "standard_file_exists")
capture_env_ready_count = sum(1 for item in items if item["status"] == "ready")
not_ready_count = len(items) - ready_count
all_ready = not_ready_count == 0
if all_ready and standard_file_ready_count == len(items):
    conclusion = "标准生产证据文件均有效，可直接进入证据包收拢"
elif all_ready:
    conclusion = "采集环境变量齐备，可执行 doctor RUN 采集"
else:
    conclusion = "采集环境变量未齐备，不能执行生产采集"
display_warnings = [] if all_ready and not env_file_exists else parse_warnings

lines = [
    "# MoChat Go 生产证据采集环境预检",
    "",
    f"- 生成时间：`{generated_at}`",
    f"- readiness JSON：`{display_path(readiness_json)}`",
    f"- env 文件：`{display_path(env_file)}`",
    f"- env 文件存在：`{env_file_exists}`",
    f"- 严格退出：`{strict}`",
    f"- 允许非生产地址：`{allow_non_prod}`",
    "- 访问生产：`未访问`",
    "- 24 小时持续运行：`未启动`",
    f"- 采集环境就绪项：`{ready_count}/{len(items)}`",
    f"- 有效标准证据项：`{standard_file_ready_count}/{len(items)}`",
    f"- 采集变量就绪项：`{capture_env_ready_count}/{len(items)}`",
    f"- 当前结论：**{conclusion}**",
    "",
]

if display_warnings:
    lines.extend(["## env 文件提示", ""])
    for warning in display_warnings:
        lines.append(f"- {warning}")
    lines.append("")

lines.extend(
    [
        "## 证据项",
        "",
        "| 证据项 | 状态 | 缺失变量 | 格式/文件问题 | 下一步 |",
        "| --- | --- | --- | --- | --- |",
    ]
)
for item in items:
    missing = ", ".join(f"`{name}`" for name in item["missing_env"]) if item["missing_env"] else "-"
    invalid = "; ".join(item["invalid"]) if item["invalid"] else "-"
    if item["status"] == "ready":
        status = "可采集"
    elif item["status"] == "standard_file_exists":
        status = "标准文件已存在"
    elif item["standard_file_exists"] and not item["standard_file_valid"]:
        status = "标准文件无效"
    else:
        status = "未就绪"
    lines.append(f"| {item['title']} | {status} | {missing} | {invalid} | {item['next']} |")

lines.extend(
    [
        "",
        "## 后续命令",
        "",
        "环境变量预检通过后，在受控终端加载本地 env 文件并执行采集：",
        "如果 6 类标准生产证据文件已存在且有效，可直接跳过 env 采集，执行 `./scripts/collect_production_evidence_pack.sh`。",
        "",
        "```bash",
        "set -a",
        f". {display_path(env_file)}",
        "set +a",
        "MOCHAT_PRODUCTION_EVIDENCE_DOCTOR_RUN=1 ./scripts/production_evidence_doctor.sh",
        "./scripts/collect_production_evidence_pack.sh",
        "```",
    ]
)

out_path.parent.mkdir(parents=True, exist_ok=True)
out_path.write_text("\n".join(lines) + "\n", encoding="utf-8")

payload = {
    "schema": 1,
    "generated_at": generated_at,
    "readiness_json": display_path(readiness_json),
    "env_file": display_path(env_file),
    "env_file_exists": env_file_exists,
    "strict": strict,
    "allow_non_prod_urls": allow_non_prod,
    "accessed_production": False,
    "started_24h_run": False,
    "ready_count": ready_count,
    "standard_file_ready_count": standard_file_ready_count,
    "capture_env_ready_count": capture_env_ready_count,
    "not_ready_count": not_ready_count,
    "all_ready": all_ready,
    "conclusion": conclusion,
    "warnings": display_warnings,
    "items": items,
}
json_out_path.parent.mkdir(parents=True, exist_ok=True)
json_out_path.write_text(json.dumps(payload, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")

print(display_path(out_path))
print(display_path(json_out_path))

if strict and not all_ready:
    sys.exit(1)
PY
