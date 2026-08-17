#!/usr/bin/env bash
set -euo pipefail

target=/opt/wecom-archive-demo
container=wecom-archive-demo
image=$(tr -d '\r\n' < "$target/image-name.txt")

if [[ -z "$image" ]]; then
  echo "image name is empty" >&2
  exit 1
fi

if docker container inspect "$container" >/dev/null 2>&1; then
  docker rm -f "$container" >/dev/null
fi

docker run -d \
  --name "$container" \
  --restart unless-stopped \
  --pull=never \
  --read-only \
  --tmpfs /tmp:rw,noexec,nosuid,size=16m \
  --cap-drop ALL \
  --security-opt no-new-privileges \
  --log-opt max-size=10m \
  --log-opt max-file=3 \
  -p 0.0.0.0:19090:8080 \
  -p 127.0.0.1:19091:9091 \
  --env-file "$target/secrets/archive.env" \
  --mount type=bind,src="$target/secrets/config.json",dst=/config/config.json,readonly \
  --mount type=bind,src="$target/data",dst=/data \
  "$image" serve --config /config/config.json
