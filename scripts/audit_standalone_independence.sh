#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

python3 - <<'PY'
from pathlib import Path
import re
import sys

failures = []


def read(path: str) -> str:
    return Path(path).read_text(encoding="utf-8")


def fail(path: str, message: str) -> None:
    failures.append(f"{path}: {message}")


standalone_compose_path = "deploy/standalone/docker-compose.yml"
standalone_compose = read(standalone_compose_path)
if not re.search(r"(?m)^\s*app\s*:", standalone_compose):
    fail(standalone_compose_path, "standalone compose must define a Go app service")
if "profiles:" not in standalone_compose or "app" not in standalone_compose:
    fail(standalone_compose_path, "standalone app service must be guarded by an app profile")
if "MOCHAT_GO_STANDALONE" not in standalone_compose:
    fail(standalone_compose_path, "standalone app service must set MOCHAT_GO_STANDALONE")
standalone_forbidden = [
    (r"(?m)^\s*php\s*:", "standalone compose must not define a PHP service"),
    (r"\.\./mochat", "standalone compose must not mount the original MoChat checkout"),
    (r"mochat/api-server", "standalone compose must not mount the original PHP api-server"),
    (r"MOCHAT_PHP", "standalone compose must not configure PHP fallback"),
    (r"MOCHAT_SOURCE_ROOT", "standalone compose must not configure a PHP source root"),
    (r"MOCHAT_COMPAT_MANIFEST", "standalone compose must not configure an external manifest"),
]
for pattern, message in standalone_forbidden:
    if re.search(pattern, standalone_compose):
        fail(standalone_compose_path, message)

dockerfile_path = "Dockerfile"
dockerfile = read(dockerfile_path)
dockerfile_required = [
    ("FROM golang:", "Dockerfile must build Go binaries from an official Go image"),
    ("COPY web ./web", "Dockerfile must include embedded frontend dist assets"),
    ("COPY deploy/standalone ./deploy/standalone", "Dockerfile must include standalone migrations"),
    ('ENTRYPOINT ["mochat-go"]', "Dockerfile must start the Go service by default"),
]
for needle, message in dockerfile_required:
    if needle not in dockerfile:
        fail(dockerfile_path, message)
for pattern, message in [
    (r"\.\./mochat", "Dockerfile must not copy the original MoChat checkout"),
    (r"mochat/api-server", "Dockerfile must not copy the original PHP api-server"),
    (r"MOCHAT_PHP", "Dockerfile must not configure PHP fallback"),
    (r"MOCHAT_SOURCE_ROOT", "Dockerfile must not configure a PHP source root"),
    (r"MOCHAT_COMPAT_MANIFEST", "Dockerfile must not configure an external manifest"),
]:
    if re.search(pattern, dockerfile):
        fail(dockerfile_path, message)

core_standalone_scripts = [
    "scripts/smoke_standalone.sh",
    "scripts/standalone_stack_check.sh",
    "scripts/standalone_route_coverage.sh",
    "scripts/smoke_dashboard_frontend_login.sh",
]
standalone_smoke_scripts = core_standalone_scripts
script_forbidden = [
    (r"\bPHP_ADDR\b|\bPHP_LOG\b|\bPHP_PID\b", "standalone smoke must not start or manage a PHP process"),
    (r"fake[- ]PHP|fake php", "standalone smoke must not start fake PHP fallback"),
    (r"MOCHAT_SOURCE_ROOT=../mochat", "standalone smoke must not point at the original MoChat source root"),
    (r"MOCHAT_COMPAT_MANIFEST=../", "standalone smoke must not point at an external manifest"),
    (r"MOCHAT_PHP_UPSTREAM=http", "standalone smoke must not configure a real PHP fallback upstream"),
    (r"MOCHAT_GO_MIGRATE_[A-Z0-9_]+=", "standalone smoke must not rely on per-route migration flags"),
]
for path in standalone_smoke_scripts:
    content = read(path)
    for pattern, message in script_forbidden:
        if re.search(pattern, content, flags=re.IGNORECASE):
            fail(path, message)

for path in core_standalone_scripts:
    content = read(path)
    if "MOCHAT_GO_STANDALONE=1" not in content:
        fail(path, "standalone smoke must set MOCHAT_GO_STANDALONE=1")

config_go = read("internal/config/config.go")
if not re.search(r"if standalone \{\s+phpUpstream = \"\"\s+sourceRoot = \"\"\s+manifestPath = \"\"", config_go):
    fail("internal/config/config.go", "standalone config must clear PHP upstream, source root, and external manifest")

readme = read("README.md")
first_heading = readme.find("## 运行方式")
compat_heading = readme.find("迁移期需要和 PHP 原接口对照时")
if first_heading == -1 or compat_heading == -1:
    fail("README.md", "README must explain standalone run mode before compatibility mode")
elif compat_heading < first_heading:
    fail("README.md", "compatibility mode must not appear before the standalone run section")
if "./scripts/standalone_stack_check.sh" not in readme[:1500]:
    fail("README.md", "README must put standalone stack check in the quick start section")

if failures:
    print("standalone independence audit failed:", file=sys.stderr)
    for item in failures:
        print(f"- {item}", file=sys.stderr)
    sys.exit(1)

print("standalone independence audit passed")
PY
