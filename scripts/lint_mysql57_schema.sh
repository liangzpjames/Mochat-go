#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

python3 - <<'PY'
import pathlib
import re
import sys

paths = [pathlib.Path("deploy/standalone/schema/mochat.sql")]
migration_dir = pathlib.Path("deploy/standalone/migrations")
if migration_dir.exists():
    paths.extend(sorted(migration_dir.glob("*.sql")))

checks = [
    ("mysql8_collation", re.compile(r"utf8mb4_0900", re.I), "MySQL 5.7 does not support utf8mb4_0900 collations"),
    ("check_constraint", re.compile(r"\bCHECK\s*\(", re.I), "MySQL 5.7 parses but ignores CHECK constraints"),
    ("invisible_index", re.compile(r"\bINVISIBLE\b", re.I), "MySQL 5.7 does not support invisible indexes"),
    ("functional_index", re.compile(r"CREATE\s+INDEX\s+[^;]*\(\s*\(", re.I | re.S), "MySQL 5.7 does not support MySQL 8 functional index syntax"),
    ("cte", re.compile(r"\bWITH\s+RECURSIVE\b", re.I), "MySQL 5.7 does not support recursive CTEs"),
    ("window_function", re.compile(r"\b(ROW_NUMBER|RANK|DENSE_RANK|LAG|LEAD)\s*\(", re.I), "MySQL 5.7 does not support window functions"),
    ("json_non_null_default", re.compile(r"\bjson\s+NOT\s+NULL\s+DEFAULT\b", re.I), "MySQL 5.7 does not allow non-NULL defaults on JSON columns"),
    ("json_literal_default", re.compile(r"\bjson\s+DEFAULT\s+(?!NULL\b)", re.I), "MySQL 5.7 only allows DEFAULT NULL on JSON columns"),
]

failures = []
for path in paths:
    text = path.read_text(encoding="utf-8")
    for name, pattern, message in checks:
        for match in pattern.finditer(text):
            line = text.count("\n", 0, match.start()) + 1
            failures.append(f"{path}:{line}: {name}: {message}")

if failures:
    print("\n".join(failures), file=sys.stderr)
    sys.exit(1)

print(f"mysql 5.7 schema lint passed ({len(paths)} files)")
PY
