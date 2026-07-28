#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

if [ "${1:-}" = "-h" ] || [ "${1:-}" = "--help" ]; then
  cat <<'EOF'
Usage: ./scripts/import_mysql57_amd64_evidence.sh <mysql57-amd64.log|artifact.zip>

校验并导入 MySQL 5.7 amd64 真实容器门禁证据。

输入可以是：
  - `.github/workflows/mysql57-amd64.yml` 产出的 mysql57-amd64.log
  - GitHub artifact 下载得到的 zip，内部需包含 mysql57-amd64.log

环境变量：
  MOCHAT_MYSQL57_AMD64_EVIDENCE_SOURCE
      未传入参数时读取的来源文件。
  MOCHAT_MYSQL57_AMD64_EVIDENCE_TARGET
      导入目标，默认 docs/phases/phase-pre0-standalone/evidence/production/mysql57-amd64.log。

该脚本只校验和复制证据文件，不启动 24 小时持续运行。
EOF
  exit 0
fi

SOURCE="${1:-${MOCHAT_MYSQL57_AMD64_EVIDENCE_SOURCE:-}}"
TARGET="${MOCHAT_MYSQL57_AMD64_EVIDENCE_TARGET:-docs/phases/phase-pre0-standalone/evidence/production/mysql57-amd64.log}"

if [ -z "$SOURCE" ]; then
  echo "missing source; pass a log/artifact path or set MOCHAT_MYSQL57_AMD64_EVIDENCE_SOURCE" >&2
  exit 2
fi

if [ ! -f "$SOURCE" ]; then
  echo "source evidence file does not exist: $SOURCE" >&2
  exit 2
fi

WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-mysql57-evidence.XXXXXX")"
cleanup() {
  rm -rf "$WORK_DIR"
}
trap cleanup EXIT INT TERM

EXTRACTED="$WORK_DIR/mysql57-amd64.log"

case "$SOURCE" in
  *.zip)
    if ! command -v unzip >/dev/null 2>&1; then
      echo "unzip is required to import artifact zip files" >&2
      exit 2
    fi
    if ! unzip -p "$SOURCE" 'mysql57-amd64.log' >"$EXTRACTED" 2>/dev/null; then
      echo "artifact zip does not contain mysql57-amd64.log: $SOURCE" >&2
      exit 2
    fi
    ;;
  *)
    cp "$SOURCE" "$EXTRACTED"
    ;;
esac

if [ ! -s "$EXTRACTED" ]; then
  echo "mysql57 amd64 evidence is empty: $SOURCE" >&2
  exit 2
fi

if grep -qi 'skipped on arm64' "$EXTRACTED" || grep -Eqi 'mysql:5\.7 is amd64-only.*skipped' "$EXTRACTED"; then
  echo "mysql57 amd64 evidence is an arm64 skip log, not production evidence: $SOURCE" >&2
  exit 1
fi

if ! grep -Eqi 'mysql57 amd64 CI gate passed|mysql 5\.7 schema migration smoke passed' "$EXTRACTED"; then
  echo "mysql57 amd64 evidence is missing required pass marker" >&2
  echo "expected: mysql57 amd64 CI gate passed OR mysql 5.7 schema migration smoke passed" >&2
  exit 1
fi

CURRENT_SOURCE_FINGERPRINT="$(python3 ./scripts/source_fingerprint.py | python3 -c 'import json, sys; print(json.load(sys.stdin)["fingerprint"])')"
if ! grep -Eiq "(源码指纹：|source fingerprint:)[[:space:]]*\`?$CURRENT_SOURCE_FINGERPRINT\`?" "$EXTRACTED"; then
  echo "mysql57 amd64 evidence is missing current source fingerprint" >&2
  echo "expected: 源码指纹：$CURRENT_SOURCE_FINGERPRINT" >&2
  exit 1
fi

mkdir -p "$(dirname "$TARGET")"
cp "$EXTRACTED" "$TARGET"

printf 'imported mysql57 amd64 evidence: %s\n' "$TARGET"
printf 'next: MOCHAT_PRODUCTION_EVIDENCE_RUN_CANDIDATE=0 ./scripts/collect_production_evidence_pack.sh\n'
