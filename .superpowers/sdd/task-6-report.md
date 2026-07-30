# Phase 2.2 Task 6 Report

## Outcome

- Added `Config.EnablePhase22SCRMPilot`, parsed only from
  `MOCHAT_GO_ENABLE_PHASE2_2_SCRM_PILOT`; it remains disabled by default.
- Added the SCRM bootstrap boundary with explicit database and principal
  dependencies when enabled. Disabled registration accepts zero-value
  dependencies and installs no routes.
- Added UTC clock and `crypto/rand`-backed UUID v4 ID adapters.
- Adapted the existing authenticated user ID resolver to an SCRM
  `PrincipalResolver`. Tenant ID is loaded from the authenticated user's
  database record; `X-Mochat-Go-Tenant-ID` is ignored.
- Wired one shared module router into the compatibility server through the
  existing generic `WithModuleRouter` option. No SCRM-specific server option
  was added.
- Extracted the existing user-resolver builder into a small composition-root
  file so `cmd/mochat-go/main.go` is smaller than its architecture baseline.

## TDD Evidence

### RED

- `go test ./internal/config -run Phase22`
  - Failed to compile because `Config.EnablePhase22SCRMPilot` was undefined.
- `go test ./internal/app/bootstrap -run SCRM`
  - Failed to compile because `RegisterSCRM`, `SCRMDependencies`, and the
    principal/infrastructure adapters were undefined.
- `go test ./cmd/mochat-go -run SCRMModuleRouter`
  - Failed to compile because `newSCRMModuleRouter` was undefined.

### GREEN

- `go test ./internal/config -run Phase22` — pass.
- `go test ./internal/app/bootstrap -run SCRM` — pass.
- `go test ./cmd/mochat-go -run SCRMModuleRouter` — pass.
- `go test ./internal/modules/scrm/...` — pass.

Coverage includes disabled zero dependencies/no routes, enabled missing
dependencies, both enabled routes, duplicate registration, forged tenant
header isolation, UUID v4 shape, and composition-root lazy dependency
resolution.

## Quality Gates

- `go test ./...` — pass.
- `go vet ./...` — pass.
- `go run ./cmd/mochat-architecture -root .` — pass.
- `go test -race ./internal/app/bootstrap ./internal/modules/scrm/...` could
  not start in this Windows environment:
  - default toolchain: race requires cgo;
  - with `CGO_ENABLED=1`: `gcc` is not installed.

## Scope Notes

- `internal/modules/scrm` has no dependency on `internal/dashboard`,
  `internal/store`, `internal/server`, or `internal/config`.
- Existing unrelated untracked `.superpowers/sdd` files were not modified or
  staged.

## Review Fix RED/GREEN

### RED

- `go test ./internal/config -run Phase22`
  - The enabled pilot incorrectly accepted missing MySQL/JWT configuration,
    `MOCHAT_GO_DEV_AUTH_HEADER=true`, and
    `MOCHAT_GO_SKIP_JWT_BLACKLIST=true`.
- `go test ./internal/authjwt ./internal/identitysecurity ./internal/app/bootstrap ./internal/modules/scrm/transport/http`
  - `ErrBackendUnavailable`, `ErrPrincipalUnauthorized`, and
    `ErrPrincipalUnavailable` were undefined.
  - Persistent identity session errors had no invalid-credential
    classification.
  - Transport had no 503 mapping for principal backend failures.

### GREEN

- The pilot now participates in the existing `mysqlBacked` and
  `dashboardStateChanging` paths, requiring production MySQL, JWT, and Redis
  configuration and rejecting development-header or skipped-blacklist auth.
- Auth failures are classified without backend details:
  - missing, invalid, expired, or revoked credentials and missing user/tenant
    map to 401;
  - Redis blacklist, persistent-session, and MySQL user lookup failures map
    to 503.
- Review-fix verification:
  - `go test ./internal/config -run Phase22` - pass.
  - `go test ./internal/app/bootstrap -run SCRM` - pass.
  - `go test ./internal/modules/scrm/transport/http` - pass.
  - `go test ./cmd/mochat-go -run SCRMModuleRouter` - pass.
  - `go test ./internal/modules/scrm/...` - pass.
  - `go test ./...` - pass.
  - `go vet ./...` - pass.
  - `go run ./cmd/mochat-architecture -root .` - pass.
