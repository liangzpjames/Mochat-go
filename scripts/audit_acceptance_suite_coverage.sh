#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

python3 - "$(pwd)" <<'PY'
from pathlib import Path
import re
import sys

repo = Path(sys.argv[1])
acceptance_path = repo / "scripts/standalone_acceptance.sh"
acceptance = acceptance_path.read_text(encoding="utf-8")

existing = {
    path.name
    for path in (repo / "scripts").glob("smoke_*.sh")
    if path.is_file() and path.name not in {"smoke_standalone_compose_app.sh"}
}
referenced = set(re.findall(r"(?:\./scripts/)?(smoke_[A-Za-z0-9_]+\.sh)", acceptance))

failures = []
missing_from_acceptance = sorted(existing - referenced)
stale_references = sorted(referenced - existing)

for name in missing_from_acceptance:
    failures.append(f"scripts/{name}: smoke script is not referenced by scripts/standalone_acceptance.sh")
for name in stale_references:
    failures.append(f"scripts/standalone_acceptance.sh: references missing smoke script scripts/{name}")

if failures:
    print("acceptance suite coverage audit failed:", file=sys.stderr)
    for failure in failures:
        print(f"- {failure}", file=sys.stderr)
    sys.exit(1)

print(f"acceptance suite coverage audit passed: smoke_scripts={len(existing)} referenced={len(referenced)} deferred_legacy=smoke_standalone_compose_app.sh")
PY
