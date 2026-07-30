## Backend module declaration

- Business module:
- Use cases changed:
- Tenant boundary:
- Idempotency strategy:
- Concurrency control:
- Migration apply/down/replay:
- External dependency fakes:
- Architecture exceptions (write `none` when empty):

## Verification

- [ ] `go run ./cmd/mochat-architecture -root .`
- [ ] `go test ./...`
- [ ] `go vet ./...`
- [ ] `go test -race ./internal/modules/...`
- [ ] Relevant MySQL integration tests
