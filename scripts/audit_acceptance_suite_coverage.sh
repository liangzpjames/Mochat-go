#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

AUDIT_PYTHON="${MOCHAT_PYTHON_BIN:-python3}"
"$AUDIT_PYTHON" - "$(pwd)" <<'PY'
from pathlib import Path
import json
import re
import sys

repo = Path(sys.argv[1])
acceptance_path = repo / "scripts/standalone_acceptance.sh"
acceptance = acceptance_path.read_text(encoding="utf-8")

existing = {
    path.name
    for path in (repo / "scripts").glob("smoke_*.sh")
    if path.is_file()
}
inventory = json.loads((repo / "scripts/acceptance_smoke_inventory.json").read_text(encoding="utf-8"))
deferred = set(inventory.get("deferredHistorical", []))
active = set(re.findall(r'^\s*run_step\s+"[^"]+"\s+\./scripts/(smoke_[A-Za-z0-9_]+\.sh)', acceptance, re.MULTILINE))

failures = []
missing_from_inventory = sorted(existing - active - deferred)
stale_references = sorted((active | deferred) - existing)
overlapping_references = sorted(active & deferred)

for name in missing_from_inventory:
    failures.append(f"scripts/{name}: smoke script is absent from active acceptance and deferred inventory")
for name in stale_references:
    failures.append(f"acceptance smoke inventory references missing script scripts/{name}")
for name in overlapping_references:
    failures.append(f"scripts/{name}: cannot be both active acceptance and deferred historical")

if failures:
    print("acceptance suite coverage audit failed:", file=sys.stderr)
    for failure in failures:
        print(f"- {failure}", file=sys.stderr)
    sys.exit(1)

print(f"acceptance suite coverage audit passed: smoke_scripts={len(existing)} active={len(active)} deferred_historical={len(deferred)}")
if deferred:
    print("deferred historical smoke scripts (restored; excluded from Task12 cutover execution):")
    for name in sorted(deferred):
        print(f"- scripts/{name}")
PY
