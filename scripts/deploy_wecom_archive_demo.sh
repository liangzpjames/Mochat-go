#!/usr/bin/env bash
set -euo pipefail

if [[ ${EUID:-$(id -u)} -ne 0 ]]; then
  echo "run as root" >&2
  exit 1
fi
if [[ $# -ne 1 ]]; then
  echo "usage: $0 <release-directory>" >&2
  exit 2
fi

release=$(readlink -f "$1")
target=/opt/wecom-archive-demo
container=wecom-archive-demo

[[ -d "$release" ]] || { echo "release directory not found" >&2; exit 1; }
[[ ! -e "$target" ]] || { echo "$target already exists; refusing to overwrite" >&2; exit 1; }
if docker container inspect "$container" >/dev/null 2>&1; then
  echo "container $container already exists" >&2
  exit 1
fi
if ss -lnt | awk '{print $4}' | grep -Eq '(^|:)19090$|(^|:)19091$'; then
  echo "port 19090 or 19091 is already in use" >&2
  exit 1
fi

cd "$release"
sha256sum -c checksums.sha256
before=$(docker ps --no-trunc --format '{{.ID}}|{{.Names}}|{{.Status}}' | sort)
docker load -i image.tar

install -d -m 0700 "$target" "$target/secrets"
install -d -m 0700 -o 65532 -g 65532 "$target/data"
install -m 0600 -o 65532 -g 65532 secrets/config.json "$target/secrets/config.json"
install -m 0600 secrets/private_key.pem "$target/secrets/private_key.pem"
install -m 0600 secrets/admin-token.txt "$target/secrets/admin-token.txt"
install -m 0644 secrets/public_key.pem "$target/public_key.pem"
install -m 0644 secrets/wecom-fill.txt "$target/wecom-fill.txt"
install -m 0644 image-name.txt "$target/image-name.txt"
install -m 0755 run_wecom_archive_demo.sh "$target/run_wecom_archive_demo.sh"
install -m 0755 configure_wecom_archive_demo.sh "$target/configure_wecom_archive_demo.sh"
printf 'WECOM_ARCHIVE_CORP_ID=\nWECOM_ARCHIVE_SECRET=\n' > "$target/secrets/archive.env"
chmod 0600 "$target/secrets/archive.env"

"$target/run_wecom_archive_demo.sh"
for _ in $(seq 1 30); do
  if curl -fsS http://127.0.0.1:19090/healthz >/dev/null; then
    break
  fi
  sleep 1
done
curl -fsS http://127.0.0.1:19090/healthz
after=$(docker ps --no-trunc --format '{{.ID}}|{{.Names}}|{{.Status}}' | grep -v '|wecom-archive-demo|' | sort)
if [[ "$before" != "$after" ]]; then
  echo "existing container snapshot changed; inspect immediately" >&2
  exit 1
fi
printf '%s\n' "$before" > "$target/existing-containers.before.txt"
printf '%s\n' "$after" > "$target/existing-containers.after.txt"
echo "deployed $container without modifying existing containers"
