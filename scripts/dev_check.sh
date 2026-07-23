#!/usr/bin/env sh
set -eu

ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
COMMAND="${1:-quick}"
GO_IMAGE="${MOCHAT_GO_DEV_IMAGE:-golang:1.26-alpine}"
IMAGE_TAG="${MOCHAT_GO_IMAGE_TAG:-mochat-go:phase0}"

usage() {
  cat <<'EOF'
Usage: ./scripts/dev_check.sh <command>

Commands:
  quick       Run architecture checks, tests, vet, and command builds in Docker
  build       Build the production Docker image
  standalone  Run core, saas, and frontend standalone acceptance suites
  help        Show this help
EOF
}

require_docker() {
  if ! command -v docker >/dev/null 2>&1; then
    echo "Docker CLI is not installed or not on PATH." >&2
    exit 1
  fi
  if ! docker info >/dev/null 2>&1; then
    echo "Docker daemon is not ready. Start Docker Desktop and retry." >&2
    exit 1
  fi
}

run_go() {
  docker run --rm \
    -v "$ROOT:/src" \
    -v mochat-go-mod-cache:/go/pkg/mod \
    -v mochat-go-build-cache:/root/.cache/go-build \
    -w /src \
    "$GO_IMAGE" \
    sh -c "$1"
}

case "$COMMAND" in
  help|-h|--help)
    usage
    ;;
  quick)
    require_docker
    run_go '
      set -eu
      sh scripts/audit_architecture_boundaries.sh
      sh scripts/test_audit_architecture_boundaries.sh
      sh scripts/test_dev_check.sh
      go test ./...
      go vet ./...
      go build ./cmd/mochat-go ./cmd/mochat-inventory ./cmd/mochat-migrate ./cmd/mochat-bootstrap ./cmd/mochat-saas-maintenance
    '
    ;;
  build)
    require_docker
    docker build --tag "$IMAGE_TAG" "$ROOT"
    ;;
  standalone)
    require_docker
    for suite in core saas frontend; do
      echo "running standalone acceptance suite: $suite"
      MOCHAT_ACCEPTANCE_SUITE="$suite" "$ROOT/scripts/standalone_acceptance.sh"
    done
    ;;
  *)
    echo "Unknown dev check command: $COMMAND" >&2
    usage >&2
    exit 2
    ;;
esac
