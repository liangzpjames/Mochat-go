#!/usr/bin/env bash
set -euo pipefail

target=/opt/wecom-archive-demo
[[ -x "$target/run_wecom_archive_demo.sh" ]] || { echo "demo is not deployed" >&2; exit 1; }

read -r -p 'CorpID: ' corp_id
read -r -s -p '会话内容存档 Secret: ' archive_secret
printf '\n'
[[ "$corp_id" == ww* ]] || { echo "CorpID must start with ww" >&2; exit 1; }
[[ -n "$archive_secret" ]] || { echo "Secret cannot be empty" >&2; exit 1; }

temporary=$(mktemp "$target/secrets/archive.env.XXXXXX")
trap 'rm -f "$temporary"' EXIT
chmod 0600 "$temporary"
printf 'WECOM_ARCHIVE_CORP_ID=%s\nWECOM_ARCHIVE_SECRET=%s\n' "$corp_id" "$archive_secret" > "$temporary"
mv "$temporary" "$target/secrets/archive.env"
trap - EXIT

"$target/run_wecom_archive_demo.sh"
for _ in $(seq 1 30); do
  if curl -fsS http://127.0.0.1:19090/healthz >/dev/null; then
    break
  fi
  sleep 1
done
admin_token=$(tr -d '\r\n' < "$target/secrets/admin-token.txt")
curl -fsS -X POST -H "Authorization: Bearer $admin_token" http://127.0.0.1:19091/admin/pull
printf '\n'
