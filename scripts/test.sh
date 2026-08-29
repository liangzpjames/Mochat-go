#!/usr/bin/env sh
set -eu

# The user's shell currently exports an old GOROOT. Homebrew Go works when GOROOT
# is not forced, so tests intentionally clear it.
unset GOROOT

go run ./cmd/mochat-architecture -root .
go test ./internal/app/modules/... ./internal/modules/...
go test -race ./internal/modules/...
go test ./...
go vet ./...

./scripts/audit_standalone_independence.sh
./scripts/audit_architecture_boundaries.sh
./scripts/test_audit_architecture_boundaries.sh
./scripts/test_backend_quality_gate_contract.sh
./scripts/test_dev_check.sh
./scripts/audit_acceptance_suite_coverage.sh
./scripts/audit_manifest_route_smoke_coverage.sh
./scripts/audit_functional_module_matrix.sh
./scripts/audit_login_corp_validation.sh
./scripts/audit_queue_annotation_coverage.sh
./scripts/audit_wework_callback_event_coverage.sh
./scripts/audit_worker_saas_usage_assertions.sh
./scripts/audit_saas_metric_coverage.sh
./scripts/audit_saas_storage_reclaim_coverage.sh
./scripts/smoke_production_evidence_gate.sh
./scripts/frontend_check.sh quick
go build ./cmd/mochat-go ./cmd/mochat-inventory ./cmd/mochat-migrate ./cmd/mochat-bootstrap ./cmd/mochat-saas-maintenance ./cmd/mochat-architecture ./cmd/mochat-callback-legacy-cutover
