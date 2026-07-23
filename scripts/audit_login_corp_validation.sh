#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

python3 - <<'PY'
from pathlib import Path
import re
import sys

failures: list[str] = []


def fail(path: Path, message: str) -> None:
    failures.append(f"{path}: {message}")


dashboard_dir = Path("internal/dashboard")
for path in sorted(dashboard_dir.glob("*.go")):
    if path.name.endswith("_test.go"):
        continue
    content = path.read_text(encoding="utf-8")
    if path.name != "session_bridge.go" and "session.ResolveLoginCorpInfo(" in content:
        fail(path, "dashboard handlers must use ResolveValidatedLoginCorpInfoFromStore instead of trusting raw login corp cache")
    if path.name != "session_bridge.go" and re.search(r"\bResolveLoginCorpInfoFromStore\(", content):
        fail(path, "dashboard handlers must not use the legacy unvalidated login corp bridge")

bridge = (dashboard_dir / "session_bridge.go").read_text(encoding="utf-8")
required_fragments = [
    "func ResolveValidatedLoginCorpInfoFromStore",
    "CorpIDsByTenant(ctx context.Context, tenantID int)",
    "CorpIDsByUser(ctx context.Context, userID int)",
    "fallbackLoginCorpInfo",
]
for fragment in required_fragments:
    if fragment not in bridge:
        fail(dashboard_dir / "session_bridge.go", f"validated login corp bridge is missing {fragment!r}")

if failures:
    print("login corp validation audit failed:", file=sys.stderr)
    for item in failures:
        print(f"- {item}", file=sys.stderr)
    sys.exit(1)

print("login corp validation audit passed")
PY
