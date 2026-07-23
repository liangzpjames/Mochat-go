#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

python3 - "$(pwd)" <<'PY'
import json
from pathlib import Path
import sys

repo = Path(sys.argv[1])
manifest_path = repo / "internal/server/compat_manifest_embedded.json"
scripts_dir = repo / "scripts"


def read_smoke_texts():
    texts = []
    for path in sorted(scripts_dir.glob("smoke_*.sh")):
        if not path.is_file():
            continue
        lines = []
        for line in path.read_text(encoding="utf-8", errors="ignore").splitlines():
            if line.lstrip().startswith("#"):
                continue
            lines.append(line)
        texts.append((path.relative_to(repo).as_posix(), "\n".join(lines)))
    return texts


def variants_for(path):
    variants = {path}
    if "{appId}" in path:
        variants.add(path.replace("{appId}", "component-app"))
        variants.add(path.replace("{appId}", "authorizer-work-fission-smoke"))
    if "{params?}" in path:
        variants.add(path.replace("{params?}", ""))
        variants.add(path.replace("{params?}", "legacy-load"))
    if "WW_verify_" in path:
        variants.add("/WW_verify_ABCDEF1234567890.txt")
    return {variant for variant in variants if variant}


manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
routes = []
for item in manifest.get("routes", []):
    method = (item.get("method") or "").upper()
    path = item.get("path") or ""
    if method and path:
        routes.append((method, path))

texts = read_smoke_texts()
missing = []
covered = {}
for method, path in routes:
    variants = variants_for(path)
    hits = [script for script, text in texts if any(variant in text for variant in variants)]
    if not hits:
        missing.append((method, path, sorted(variants)))
    else:
        covered[(method, path)] = hits

if missing:
    print("manifest route smoke coverage audit failed:", file=sys.stderr)
    for method, path, variants in missing:
        print(f"- {method} {path}: not directly mentioned in scripts/smoke_*.sh; variants={variants}", file=sys.stderr)
    sys.exit(1)

print(f"manifest route smoke coverage audit passed: manifest_routes={len(routes)} directly_covered={len(covered)} smoke_scripts={len(texts)}")
PY
