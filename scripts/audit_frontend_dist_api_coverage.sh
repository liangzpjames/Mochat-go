#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

if [ "${1:-}" = "-h" ] || [ "${1:-}" = "--help" ]; then
  cat <<'EOF'
Usage: ./scripts/audit_frontend_dist_api_coverage.sh

扫描 dashboard、sidebar、operation 既有 dist 和 SaaS 总后台源码中真实声明的 API 调用，
并与 Go runtime 的 dispatch/routes 对比，生成前端 dist API 覆盖审计。

环境变量：
  MOCHAT_FRONTEND_DIST_API_OUT
      写入 Markdown 报告的路径；未设置时输出到 stdout。
  MOCHAT_FRONTEND_DIST_API_STRICT
      设为 1 时，发现缺失 API 返回非 0；默认只报告并返回 0。

该脚本不启动服务、不访问外网、不启动 24 小时持续运行。
EOF
  exit 0
fi

STRICT="${MOCHAT_FRONTEND_DIST_API_STRICT:-0}"
OUT="${MOCHAT_FRONTEND_DIST_API_OUT:-}"

case "$STRICT" in
  0|1) ;;
  *)
    echo "unknown MOCHAT_FRONTEND_DIST_API_STRICT=$STRICT" >&2
    exit 2
    ;;
esac

python3 - "$STRICT" "$OUT" <<'PY'
import datetime as dt
import os
import pathlib
import re
import sys

strict = sys.argv[1] == "1"
out_path = pathlib.Path(sys.argv[2]) if len(sys.argv) > 2 and sys.argv[2] else None
repo = pathlib.Path.cwd()
server_path = repo / "internal/server/server.go"

method_names = {
    "Get": "GET",
    "Post": "POST",
    "Put": "PUT",
    "Delete": "DELETE",
    "Patch": "PATCH",
    "Head": "HEAD",
    "Options": "OPTIONS",
}


def display(path: pathlib.Path) -> str:
    try:
        return path.relative_to(repo).as_posix()
    except ValueError:
        return path.as_posix()


def normalize_api_path(surface_prefix: str, url: str):
    if not url.startswith("/") or url.startswith("//"):
        return None
    url = url.split("?", 1)[0].split("#", 1)[0]
    if not url or "%" in url or any(ch.isspace() for ch in url):
        return None
    if url.startswith(("/dashboard/", "/sidebar/", "/operation/")):
        return url
    return f"{surface_prefix}{url}"


def runtime_routes(server_text: str) -> set[tuple[str, str]]:
    routes: set[tuple[str, str]] = set()

    for method, path in re.findall(r'"(GET|POST|PUT|DELETE|PATCH|HEAD|OPTIONS)\s+([^"]+)"', server_text):
        routes.add((method.upper(), path))

    path_re = re.compile(r'r\.URL\.Path\s*==\s*"([^"]+)"')
    method_re = re.compile(r'r\.Method\s*==\s*http\.Method(Get|Post|Put|Delete|Patch|Head|Options)')
    for line in server_text.splitlines():
        paths = path_re.findall(line)
        methods = [method_names[name] for name in method_re.findall(line)]
        if not paths or not methods:
            continue
        for method in methods:
            for path in paths:
                routes.add((method, path))

    # A GET handler also satisfies HEAD for the standard Go http server, but the
    # legacy frontend dist does not currently declare HEAD API calls. Keep the
    # set explicit so missing API rows stay actionable.
    return routes


def scan_dist_api_calls(root: pathlib.Path, surface_prefix: str) -> dict[tuple[str, str], set[str]]:
    calls: dict[tuple[str, str], set[str]] = {}
    if not root.exists():
        return calls

    patterns = [
        re.compile(r'\{\s*url\s*:\s*(["\'])(/[^"\']+)\1\s*,\s*method\s*:\s*(["\'])([A-Za-z]+)\3'),
        re.compile(r'\{\s*method\s*:\s*(["\'])([A-Za-z]+)\1\s*,\s*url\s*:\s*(["\'])(/[^"\']+)\3'),
    ]
    for js_path in sorted(root.rglob("*.js")):
        if js_path.name.endswith(".gz"):
            continue
        try:
            text = js_path.read_text(encoding="utf-8", errors="ignore")
        except OSError:
            continue
        found: list[tuple[str, str]] = []
        for match in patterns[0].finditer(text):
            found.append((match.group(4), match.group(2)))
        for match in patterns[1].finditer(text):
            found.append((match.group(2), match.group(4)))
        for method, url in found:
            path = normalize_api_path(surface_prefix, url)
            if not path:
                continue
            key = (method.upper(), path)
            calls.setdefault(key, set()).add(display(js_path))
    return calls


def scan_saas_admin_source_calls(root: pathlib.Path) -> dict[tuple[str, str], set[str]]:
    calls: dict[tuple[str, str], set[str]] = {}
    if not root.exists():
        return calls

    request_pattern = re.compile(
        r"apiRequest(?:<[^\n]*>)?\s*\(\s*([\"'`])(/dashboard/saasAdmin/[^\"'`]+)\1"
    )
    direct_pattern = re.compile(
        r"directPath\s*:\s*([\"'])(/dashboard/saasAdmin/[^\"']+)\1"
    )
    method_pattern = re.compile(r"^\s*,\s*jsonRequest\(\s*['\"]([A-Z]+)['\"]")
    direct_method_pattern = re.compile(r"directMethod\s*:\s*['\"]([A-Z]+)['\"]")

    for source_path in sorted(root.rglob("*.ts")) + sorted(root.rglob("*.tsx")):
        try:
            source = source_path.read_text(encoding="utf-8", errors="ignore")
        except OSError:
            continue
        source_name = display(source_path)
        for match in request_pattern.finditer(source):
            path = match.group(2).split("?", 1)[0].split("${", 1)[0]
            tail = source[match.end():match.end() + 300]
            method_match = method_pattern.match(tail)
            method = method_match.group(1) if method_match else "GET"
            calls.setdefault((method, path), set()).add(source_name)
        for match in direct_pattern.finditer(source):
            path = match.group(2).split("?", 1)[0]
            tail = source[match.end():match.end() + 300]
            method_match = direct_method_pattern.search(tail)
            method = method_match.group(1) if method_match else "POST"
            calls.setdefault((method, path), set()).add(source_name)
    return calls


if not server_path.exists():
    raise SystemExit(f"server file not found: {display(server_path)}")

routes = runtime_routes(server_path.read_text(encoding="utf-8"))
surfaces = [
    ("dashboard", repo / "web/dashboard/dist", lambda root: scan_dist_api_calls(root, "/dashboard")),
    ("sidebar", repo / "web/sidebar/dist", lambda root: scan_dist_api_calls(root, "/sidebar")),
    ("operation", repo / "web/operation/dist", lambda root: scan_dist_api_calls(root, "/operation")),
    ("saas-admin", repo / "web/apps/saas-admin/src", scan_saas_admin_source_calls),
]

surface_results = []
all_missing = []
all_endpoint_count = 0
for name, root, scanner in surfaces:
    calls = scanner(root)
    missing = [
        {
            "method": method,
            "path": path,
            "sources": sorted(sources),
        }
        for (method, path), sources in sorted(calls.items(), key=lambda item: (item[0][1], item[0][0]))
        if (method, path) not in routes
    ]
    surface_results.append(
        {
            "name": name,
            "root": display(root),
            "exists": root.exists(),
            "endpoint_count": len(calls),
            "missing": missing,
        }
    )
    all_endpoint_count += len(calls)
    all_missing.extend({"surface": name, **item} for item in missing)

generated_at = dt.datetime.now().astimezone().strftime("%Y-%m-%d %H:%M:%S %Z")
status = "通过" if not all_missing else "发现缺口"
summary = (
    "frontend dist API coverage audit passed: "
    if not all_missing
    else "frontend dist API coverage audit found gaps: "
) + f"surfaces={len(surfaces)} endpoints={all_endpoint_count} missing={len(all_missing)}"
if out_path:
    summary += f" report={display(out_path if out_path.is_absolute() else repo / out_path)}"

lines = [
    "# 前端 dist API 覆盖审计",
    "",
    f"- 生成时间：`{generated_at}`",
    f"- 当前结论：**{status}**",
    f"- 严格模式：`{strict}`",
    f"- Go runtime 路由候选：`{len(routes)}`",
    f"- 前端 dist API 唯一端点：`{all_endpoint_count}`",
    f"- 缺失 API：`{len(all_missing)}`",
    "",
    "## 分端结果",
    "",
    "| 端 | dist | API 端点 | 缺失 |",
    "| --- | --- | ---: | ---: |",
]
for result in surface_results:
    lines.append(
        f"| {result['name']} | `{result['root']}` | {result['endpoint_count']} | {len(result['missing'])} |"
    )

lines.extend(["", "## 缺失 API", ""])
if all_missing:
    lines.extend(["| 端 | 方法 | 路径 | 来源 |", "| --- | --- | --- | --- |"])
    for item in all_missing:
        source_text = "<br>".join(f"`{source}`" for source in item["sources"][:5])
        extra = len(item["sources"]) - 5
        if extra > 0:
            source_text += f"<br>另 {extra} 个文件"
        lines.append(f"| {item['surface']} | `{item['method']}` | `{item['path']}` | {source_text} |")
else:
    lines.append("- 未发现旧前端 dist 声明但 Go runtime 未承接的 API。")

lines.extend(
    [
        "",
        "## 口径说明",
        "",
        "- Go runtime 侧同时读取 `internal/server/server.go` 中的 `GET /path` route 列表和 `r.URL.Path == ... && r.Method == http.Method...` dispatch 分支。",
        "- 前端侧扫描三套既有 dist 的静态 API 声明，并扫描 SaaS 总后台 TypeScript 中的 `apiRequest` 与 `directPath` 声明。",
        "- `dashboard`、`sidebar`、`operation` dist 的相对 API 会分别归一到 `/dashboard`、`/sidebar`、`/operation` 前缀；SaaS 总后台使用完整 `/dashboard/saasAdmin/*` 路径。",
    ]
)

report = "\n".join(lines) + "\n"
if out_path:
    target = out_path if out_path.is_absolute() else repo / out_path
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_text(report, encoding="utf-8")
    print(summary)
else:
    print(summary)
    print(report, end="")

if strict and all_missing:
    raise SystemExit(1)
PY
