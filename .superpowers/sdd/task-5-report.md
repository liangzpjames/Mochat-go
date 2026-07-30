# Task 5 Report: HTTP transport and module assembly

## Status

Implemented and verified.

## Delivered

- Added the SCRM HTTP principal contract and lead create/list handlers.
- Registered only `POST` and `GET /api/phase2-2/scrm/leads`.
- Derived tenant scope exclusively from the authenticated principal; request-body
  `tenantId` is rejected as an unknown field and never reaches the service.
- Added bounded and strict JSON decoding:
  - `http.MaxBytesReader` with a 64 KiB limit;
  - unknown-field rejection;
  - trailing-value rejection;
  - `413` for oversized bodies and `400` for invalid JSON.
- Mapped principal failures to `401`, create validation to `422`, malformed list
  input to `400`, dependency unavailability to `503`, and unexpected failures
  to `500`.
- Added explicit module construction:

  ```text
  MySQL repository -> application service -> HTTP handler -> Module
  ```

- Kept runtime configuration and `main`/server wiring out of this task.
- Added no imports from dashboard, store, or server.

## Required cross-task contract correction

Task 5 requires a reliable distinction between first creation (`201`) and an
idempotent retry (`200`), plus response timestamps. The pre-existing application
contract exposed neither. The minimum protocol-independent correction was made:

- `CreateLead` now returns `CreateLeadResult{Lead, Created}`.
- `LeadView` now carries UTC `CreatedAt` and `UpdatedAt`.
- Application tests prove `Created=true` on insertion, `Created=false` on retry,
  and persisted timestamp propagation.

Review also identified that malformed opaque MySQL cursors would otherwise be
reported as repository unavailability (`503`). A port-level
`ports.ErrInvalidCursor` sentinel now travels from the MySQL adapter to
`application.ErrInvalidArgument`, producing HTTP `400`. Tests cover invalid
base64, invalid JSON, and missing cursor fields.

## TDD evidence

1. Application RED:
   `go test ./internal/modules/scrm/application`
   failed because `CreateLeadResult`, `Created`, and timestamps were undefined.
2. Application GREEN:
   the same package passed after the minimum contract implementation.
3. Transport RED:
   `go test ./internal/modules/scrm/transport/http`
   failed because principal, handler, route, and constructor types were undefined.
4. Transport GREEN:
   the package passed after implementing strict request/response mapping.
5. Module RED:
   `go test ./internal/modules/scrm`
   failed because `Dependencies`, `Module`, and `New` were undefined.
6. Module GREEN:
   the package passed after explicit assembly and route registration.
7. Cursor RED:
   focused adapter/application tests failed because `ports.ErrInvalidCursor`
   was undefined.
8. Cursor GREEN:
   adapter, application, and transport packages passed after sentinel mapping.

## Review

An independent read-only review first found the malformed-cursor mapping issue.
After the TDD correction, re-review reported no Critical or Important issues and
returned a ready verdict.

## Verification

- `go test ./internal/modules/scrm/... -count=1`
- `go vet ./internal/modules/scrm/...`
- `docker run --rm -v "${PWD}:/src" -w /src golang:1.26-bookworm go test -race ./internal/modules/scrm/...`
- `go run ./cmd/mochat-architecture -root .`
- `git diff --check`

All verification commands completed successfully. The Docker race gate was used
because the Windows host Go toolchain has CGO disabled.

## Attention points

- The module is intentionally not connected to runtime config or `server.Server`;
  that composition-root work belongs to Task 6.
- Route registration is intentionally explicit and returns duplicate/invalid
  registration errors to the caller.
