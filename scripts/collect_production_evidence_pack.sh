#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

if [ "${1:-}" = "-h" ] || [ "${1:-}" = "--help" ]; then
  cat <<'EOF'
Usage: ./scripts/collect_production_evidence_pack.sh

收拢真实生产证据文件，生成可复核证据包，并执行严格生产证据检查。
复制证据前会扫描未脱敏敏感值、非生产 URL 和源码指纹；疑似包含原始 token/secret/password、example/localhost/127.0.0.1 等地址，或缺少当前源码指纹的源证据会被拒绝复制。

环境变量：
  MOCHAT_PRODUCTION_EVIDENCE_SOURCE_DIR
      生产证据来源目录，默认 docs/phases/phase-pre0-standalone/evidence/production。
      未显式设置 MOCHAT_EVIDENCE_* 时，会读取该目录下的标准文件名。
  MOCHAT_PRODUCTION_EVIDENCE_PACK_DIR
      输出证据包目录，默认 docs/phases/phase-pre0-standalone/evidence/production/current。
  MOCHAT_PRODUCTION_EVIDENCE_RUN_CANDIDATE
      设为 1 时继续运行 skip-local 生产候选门禁，默认 1。
  MOCHAT_PRODUCTION_EVIDENCE_COPY
      设为 1 时把来源文件复制到证据包目录，默认 1。

标准文件名：
  mysql57-amd64.log
  real-wecom.md
  real-wechat-open.md
  real-saas-tenants.md
  prod-frontend.md
  stability.md

也可以直接设置 MOCHAT_EVIDENCE_MYSQL57_AMD64 等环境变量指向任意来源文件。
该脚本不会启动 24 小时持续运行。
EOF
  exit 0
fi

python3 - <<'PY'
import datetime as dt
import hashlib
import json
import os
import pathlib
import re
import shutil
import subprocess
import sys

repo = pathlib.Path.cwd()

items = [
    ("MOCHAT_EVIDENCE_MYSQL57_AMD64", "mysql57-amd64.log", "MySQL 5.7 amd64 真实容器门禁"),
    ("MOCHAT_EVIDENCE_REAL_WECOM", "real-wecom.md", "真实企业微信账号联调"),
    ("MOCHAT_EVIDENCE_REAL_WECHAT_OPEN", "real-wechat-open.md", "真实微信开放平台联调"),
    ("MOCHAT_EVIDENCE_REAL_SAAS_TENANTS", "real-saas-tenants.md", "真实 SaaS 多租户数据回归"),
    ("MOCHAT_EVIDENCE_PROD_FRONTEND", "prod-frontend.md", "生产前端浏览器回归"),
    ("MOCHAT_EVIDENCE_STABILITY", "stability.md", "稳定性记录"),
]


def bool_env(name: str, default: str) -> bool:
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


def sha256_file(path: pathlib.Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def load_current_source_fingerprint() -> str:
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
    return fingerprint if re.fullmatch(r"[0-9a-f]{64}", fingerprint) else ""


current_source_fingerprint = load_current_source_fingerprint()


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


def is_redacted(value: str) -> bool:
    lowered = value.lower()
    return any(marker in lowered for marker in redacted_markers)


def sensitive_findings(path: pathlib.Path) -> list[tuple[int, str]]:
    try:
        text = path.read_text(encoding="utf-8", errors="ignore")
    except Exception as exc:
        return [(0, f"无法读取敏感值扫描: {exc}")]
    findings: list[tuple[int, str]] = []
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


def is_non_production_host(host) -> bool:
    normalized = (host or "").strip().strip("[]").lower().rstrip(".")
    return normalized in non_production_hosts or any(normalized.endswith(suffix) for suffix in non_production_suffixes)


def non_production_url_findings(path: pathlib.Path) -> list[tuple[int, str]]:
    try:
        from urllib.parse import urlparse

        text = path.read_text(encoding="utf-8", errors="ignore")
    except Exception as exc:
        return [(0, f"无法读取 URL 扫描: {exc}")]
    findings: list[tuple[int, str]] = []
    for line_no, line in enumerate(text.splitlines(), start=1):
        for match in url_pattern.finditer(line):
            raw = match.group(0).rstrip(".,;:")
            parsed = urlparse(raw)
            if is_non_production_host(parsed.hostname):
                findings.append((line_no, parsed.hostname or raw))
                break
        if len(findings) >= 5:
            break
    return findings


source_fingerprint_pattern = re.compile(
    r"(?:源码指纹|source\s+fingerprint)\s*[:：]\s*`?([0-9a-f]{64})`?",
    re.IGNORECASE,
)


def source_fingerprint_issue(path: pathlib.Path) -> str:
    if not current_source_fingerprint:
        return "无法生成当前源码指纹"
    try:
        text = path.read_text(encoding="utf-8", errors="ignore")
    except Exception as exc:
        return f"无法读取源码指纹: {exc}"
    fingerprints = {item.lower() for item in source_fingerprint_pattern.findall(text)}
    if not fingerprints:
        return f"缺少当前源码指纹 {current_source_fingerprint}"
    if fingerprints != {current_source_fingerprint}:
        found = ", ".join(sorted(fingerprints)) or "无"
        return f"证据源码指纹 {found} 与当前 {current_source_fingerprint} 不匹配"
    return ""


def read_conclusion(path: pathlib.Path) -> str:
    if not path.exists():
        return ""
    for line in path.read_text(encoding="utf-8", errors="ignore").splitlines():
        if line.startswith("- 当前结论："):
            return line.split("：", 1)[1].strip()
    return ""


def run_capture(name: str, slug: str, cmd: list[str], env: dict[str, str], out_dir: pathlib.Path) -> dict:
    log_path = out_dir / f"{slug}.log"
    started = dt.datetime.now().astimezone().strftime("%Y-%m-%d %H:%M:%S %Z")
    completed = subprocess.run(
        cmd,
        cwd=repo,
        env=env,
        text=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        check=False,
    )
    finished = dt.datetime.now().astimezone().strftime("%Y-%m-%d %H:%M:%S %Z")
    log_path.write_text(completed.stdout, encoding="utf-8")
    return {
        "name": name,
        "slug": slug,
        "returncode": completed.returncode,
        "started_at": started,
        "finished_at": finished,
        "log": display_path(log_path),
    }


source_dir = to_path(os.environ.get("MOCHAT_PRODUCTION_EVIDENCE_SOURCE_DIR", "docs/phases/phase-pre0-standalone/evidence/production"))
pack_dir = to_path(os.environ.get("MOCHAT_PRODUCTION_EVIDENCE_PACK_DIR", "docs/phases/phase-pre0-standalone/evidence/production/current"))
run_candidate = bool_env("MOCHAT_PRODUCTION_EVIDENCE_RUN_CANDIDATE", "1")
copy_files = bool_env("MOCHAT_PRODUCTION_EVIDENCE_COPY", "1")

pack_dir.mkdir(parents=True, exist_ok=True)

known_outputs = [
    "index.md",
    "manifest.json",
    "env.production-evidence",
    "production-evidence.md",
    "production-evidence.json",
    "production-evidence.log",
    "production-candidate.log",
    "results.jsonl",
]
if pack_dir.resolve() != source_dir.resolve():
    for _, file_name, _ in items:
        target = pack_dir / file_name
        if target.exists() and target.is_file():
            target.unlink()
    for name in known_outputs:
        target = pack_dir / name
        if target.exists() and target.is_file():
            target.unlink()
    candidate_dir = pack_dir / "candidate"
    if candidate_dir.exists() and candidate_dir.is_dir():
        shutil.rmtree(candidate_dir)

pack_env: dict[str, str] = {}
manifest_items = []

for env_name, file_name, title in items:
    raw_source = os.environ.get(env_name, "").strip()
    source = to_path(raw_source) if raw_source else source_dir / file_name
    destination = pack_dir / file_name if copy_files else source
    copied = False
    copy_blocked = False
    copy_block_reason = ""
    copy_issue = ""

    if source.exists() and source.is_file() and copy_files:
        findings = sensitive_findings(source)
        if findings:
            copy_blocked = True
            copy_block_reason = "sensitive"
            copy_issue = "；".join(f"第 {line_no} 行 {label}" for line_no, label in findings)
        else:
            url_findings = non_production_url_findings(source)
            if url_findings:
                copy_blocked = True
                copy_block_reason = "non_production_url"
                copy_issue = "；".join(f"第 {line_no} 行 {host}" for line_no, host in url_findings)
            else:
                fingerprint_issue = source_fingerprint_issue(source)
                if fingerprint_issue:
                    copy_blocked = True
                    copy_block_reason = "source_fingerprint"
                    copy_issue = fingerprint_issue
                else:
                    destination.parent.mkdir(parents=True, exist_ok=True)
                    if source.resolve() != destination.resolve():
                        shutil.copy2(source, destination)
                        copied = True

    path_for_env = destination if copy_blocked else (destination if copy_files else source)
    pack_env[env_name] = display_path(path_for_env)

    exists = path_for_env.exists() and path_for_env.is_file()
    entry = {
        "env": env_name,
        "title": title,
        "source": display_path(source),
        "path": display_path(path_for_env),
        "copied": copied,
        "copy_blocked": copy_blocked,
        "copy_block_reason": copy_block_reason,
        "copy_issue": copy_issue,
        "exists": exists,
        "size": path_for_env.stat().st_size if exists else 0,
        "sha256": sha256_file(path_for_env) if exists else "",
    }
    manifest_items.append(entry)

env_file = pack_dir / "env.production-evidence"
env_file.write_text(
    "".join(f"{key}={value}\n" for key, value in pack_env.items()),
    encoding="utf-8",
)

manifest = {
    "schema": 1,
    "generated_at": dt.datetime.now().astimezone().strftime("%Y-%m-%d %H:%M:%S %Z"),
    "source_fingerprint": current_source_fingerprint,
    "source_dir": display_path(source_dir),
    "pack_dir": display_path(pack_dir),
    "copy_files": copy_files,
    "run_candidate": run_candidate,
    "items": manifest_items,
}
(pack_dir / "manifest.json").write_text(json.dumps(manifest, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")

base_env = os.environ.copy()
base_env.update(pack_env)

results = []
evidence_report = pack_dir / "production-evidence.md"
evidence_json = pack_dir / "production-evidence.json"
evidence_env = base_env.copy()
evidence_env.update(
    {
        "MOCHAT_PRODUCTION_EVIDENCE_STRICT": "1",
        "MOCHAT_PRODUCTION_EVIDENCE_SKIP_QUICK": "1",
        "MOCHAT_PRODUCTION_EVIDENCE_OUT": display_path(evidence_report),
        "MOCHAT_PRODUCTION_EVIDENCE_JSON_OUT": display_path(evidence_json),
    }
)
results.append(
    run_capture(
        "严格生产证据文件检查",
        "production-evidence",
        ["./scripts/production_evidence_check.sh"],
        evidence_env,
        pack_dir,
    )
)

candidate_report = pack_dir / "candidate" / "index.md"
if run_candidate:
    candidate_env = base_env.copy()
    candidate_env.update(
        {
            "MOCHAT_PRODUCTION_CANDIDATE_SKIP_LOCAL": "1",
            "MOCHAT_PRODUCTION_CANDIDATE_DIR": display_path(pack_dir / "candidate"),
        }
    )
    results.append(
        run_capture(
            "skip-local 生产候选门禁",
            "production-candidate",
            ["./scripts/production_candidate_gate.sh"],
            candidate_env,
            pack_dir,
        )
    )

results_path = pack_dir / "results.jsonl"
with results_path.open("w", encoding="utf-8") as handle:
    for item in results:
        handle.write(json.dumps(item, ensure_ascii=False) + "\n")

all_ok = all(item["returncode"] == 0 for item in results)
evidence_conclusion = read_conclusion(evidence_report)
candidate_conclusion = read_conclusion(candidate_report)
generated_at = dt.datetime.now().astimezone().strftime("%Y-%m-%d %H:%M:%S %Z")

lines = [
    "# MoChat Go 生产证据包",
    "",
    f"- 生成时间：`{generated_at}`",
    f"- 当前结论：**{'通过，可进入生产候选复核' if all_ok else '未通过，不能标记最终完成'}**",
    f"- 来源目录：`{display_path(source_dir)}`",
    f"- 输出目录：`{display_path(pack_dir)}`",
    f"- 当前源码指纹：`{current_source_fingerprint or '生成失败'}`",
    f"- 复制证据文件：`{copy_files}`",
    f"- 运行生产候选门禁：`{run_candidate}`",
    "- 24 小时持续运行：`未启动`",
    "",
    "## 证据文件",
    "",
    "| 证据项 | 环境变量 | 文件 | 状态 | 大小 | sha256 |",
    "| --- | --- | --- | --- | ---: | --- |",
]
for item in manifest_items:
    if item.get("copy_blocked"):
        if item.get("copy_block_reason") == "non_production_url":
            status = "非生产地址拒绝，未复制"
        elif item.get("copy_block_reason") == "source_fingerprint":
            status = "源码指纹拒绝，未复制"
        else:
            status = "敏感值拒绝，未复制"
    else:
        status = "存在" if item["exists"] else "缺失"
    digest = item["sha256"][:16] if item["sha256"] else ""
    lines.append(
        f"| {item['title']} | `{item['env']}` | `{item['path']}` | {status} | {item['size']} | `{digest}` |"
    )

blocked_items = [item for item in manifest_items if item.get("copy_blocked")]
if blocked_items:
    lines.extend(["", "## 拒绝复制的证据", ""])
    for item in blocked_items:
        if item.get("copy_block_reason") == "non_production_url":
            lines.append(
                f"- `{item['title']}`：源文件包含示例域名或本机地址（{item['copy_issue']}），未复制到生产证据包。"
            )
            continue
        if item.get("copy_block_reason") == "source_fingerprint":
            lines.append(
                f"- `{item['title']}`：源文件缺少当前源码指纹或源码指纹过期（{item['copy_issue']}），未复制到生产证据包。"
            )
            continue
        lines.append(
            f"- `{item['title']}`：源文件疑似包含未脱敏敏感值（{item['copy_issue']}），未复制到生产证据包。"
        )

lines.extend(["", "## 命令结果", ""])
for item in results:
    marker = "通过" if item["returncode"] == 0 else "失败"
    lines.append(f"- {marker} `{item['name']}`：`{item['log']}`")

lines.extend(["", "## 关键结论", ""])
lines.append(f"- 严格生产证据文件检查：{evidence_conclusion or '未生成当前结论。'}")
lines.append("- 机器可读生产证据检查：`production-evidence.json`")
if run_candidate:
    lines.append(f"- skip-local 生产候选门禁：{candidate_conclusion or '未生成当前结论。'}")
else:
    lines.append("- skip-local 生产候选门禁：未运行。")

lines.extend(
    [
        "",
        "## 复跑方式",
        "",
        "```bash",
        "set -a",
        f". {display_path(env_file)}",
        "set +a",
        "MOCHAT_PRODUCTION_CANDIDATE_SKIP_LOCAL=1 ./scripts/production_candidate_gate.sh",
        "```",
    ]
)
(pack_dir / "index.md").write_text("\n".join(lines) + "\n", encoding="utf-8")

print(display_path(pack_dir / "index.md"))
sys.exit(0 if all_ok else 1)
PY
