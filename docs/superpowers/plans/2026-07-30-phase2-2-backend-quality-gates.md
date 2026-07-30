# Phase 2.2 Backend Quality Gates Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Establish enforceable backend module boundaries and prove them with a disabled-by-default, tenant-scoped SCRM lead create/list vertical slice before Phase 3 begins.

**Architecture:** Add a cross-platform Go AST architecture auditor, a single reusable module router integration point, and a production-shaped `internal/modules/scrm` module split into domain, application, ports, MySQL adapter, and HTTP transport. Preserve every legacy route and keep the SCRM route surface disabled unless `MOCHAT_GO_ENABLE_PHASE2_2_SCRM_PILOT=true`.

**Tech Stack:** Go 1.26, standard-library `go/parser`/`go/ast`, `net/http`, `database/sql`, MySQL 5.7/MariaDB, existing migration runner, GitHub Actions, Docker Compose.

## Global Constraints

- Existing API URLs, JSON contracts, authentication, runtime roles, and fallback behavior must not change.
- `internal/modules/scrm` must not import `internal/dashboard`, `internal/store`, or `internal/server`.
- Domain and application tests must not start MySQL, Redis, or an HTTP server.
- Tenant IDs used by SCRM use cases must come from an injected authenticated principal resolver, never from request JSON or query parameters.
- Duplicate `(tenant_id, business_key)` creates must return the existing lead and must be safe under concurrent requests.
- SCRM pilot routes remain disabled unless `MOCHAT_GO_ENABLE_PHASE2_2_SCRM_PILOT=true`.
- Do not increase any limit in `scripts/architecture-size-baseline.txt`.
- Migration `0098` is reserved by this plan; re-check immediately before implementation and use the next continuous number if `0098` has been occupied.
- No Phase 3 UI, lead conversion, contacts, assignments, opportunities, follow-ups, reports, risk, archive, or AI work is included.
- Every task uses failing-test-first development and ends with an independently reviewable commit.

---

## File Structure

### Architecture gate

- `internal/architecture/rules.go`: rule IDs, layer classification, protected paths, and exception schema.
- `internal/architecture/audit.go`: repository walking, import parsing, exception validation, protected-file and forbidden-path checks.
- `internal/architecture/audit_test.go`: repository audit plus negative fixture tests.
- `internal/architecture/testdata/`: isolated invalid package trees and policies.
- `architecture-policy.json`: production module list, protected-size limits, forbidden legacy additions, and exceptions.
- `cmd/mochat-architecture/main.go`: cross-platform CLI used by local scripts and CI.

### Module registration

- `internal/app/modules/router.go`: module-owned route registration and matching.
- `internal/app/modules/router_test.go`: method/path matching, duplicate registration, and fallback tests.
- `internal/server/server.go`: one optional modular-router field, one Option, and one dispatch branch.
- `internal/server/server_test.go`: modular dispatch and unchanged fallback behavior.

### SCRM thin slice

- `internal/modules/scrm/domain/lead.go`: `Lead`, `LeadName`, `LeadSource`, and validation.
- `internal/modules/scrm/domain/errors.go`: stable domain errors.
- `internal/modules/scrm/domain/lead_test.go`: invariant tests.
- `internal/modules/scrm/ports/lead_repository.go`: persistence input/output contract.
- `internal/modules/scrm/ports/clock.go`: deterministic time contract.
- `internal/modules/scrm/ports/id_generator.go`: deterministic ID contract.
- `internal/modules/scrm/application/service.go`: create/list use cases and protocol-independent errors.
- `internal/modules/scrm/application/service_test.go`: fake-repository use-case tests.
- `internal/modules/scrm/adapters/mysql/lead_repository.go`: MySQL implementation and duplicate-key recovery.
- `internal/modules/scrm/adapters/mysql/lead_repository_integration_test.go`: tenant, idempotency, pagination, and concurrency tests.
- `internal/modules/scrm/transport/http/principal.go`: authenticated tenant principal contract.
- `internal/modules/scrm/transport/http/lead_handler.go`: request/response mapping.
- `internal/modules/scrm/transport/http/routes.go`: module-local route registration.
- `internal/modules/scrm/transport/http/lead_handler_test.go`: HTTP contract and tenant-spoofing tests.
- `internal/modules/scrm/module.go`: explicit production assembly.
- `internal/modules/scrm/module_test.go`: registration and disabled-module isolation tests.

### Runtime and delivery

- `deploy/standalone/migrations/0098_scrm_lead_foundation.up.sql`: additive lead table.
- `deploy/standalone/migrations/0098_scrm_lead_foundation.down.sql`: exact rollback.
- `internal/config/config.go`: disabled-by-default pilot flag.
- `internal/config/config_test.go`: environment parsing and default test.
- `cmd/mochat-go/main.go`: one module registry construction point.
- `scripts/audit_architecture_boundaries.sh`: compatibility wrapper invoking the Go auditor.
- `scripts/test_audit_architecture_boundaries.sh`: wrapper/CLI smoke test.
- `scripts/dev_check.sh`: architecture, module, and race gates.
- `scripts/test.sh`: architecture, module, and race gates.
- `.github/workflows/mysql57-amd64.yml`: CI architecture/race/integration steps.
- `.github/PULL_REQUEST_TEMPLATE.md`: backend module declaration checklist.
- `docs/phases/phase-2.2-backend-quality-gates/acceptance.md`: reproducible Phase 2.2 evidence.
- `docs/superpowers/plans/2026-07-29-phase3-yuanhu-business-foundation.md`: replace legacy SCRM paths with module paths.

---

### Task 1: Cross-platform architecture auditor

**Files:**

- Create: `architecture-policy.json`
- Create: `internal/architecture/rules.go`
- Create: `internal/architecture/audit.go`
- Create: `internal/architecture/audit_test.go`
- Create: `internal/architecture/testdata/domain-imports-sql/internal/modules/bad/domain/bad.go`
- Create: `internal/architecture/testdata/application-imports-adapter/internal/modules/bad/application/bad.go`
- Create: `internal/architecture/testdata/module-imports-dashboard/internal/modules/bad/module.go`
- Create: `internal/architecture/testdata/cross-module-adapter/internal/modules/alpha/application/bad.go`
- Create: `internal/architecture/testdata/cross-module-adapter/internal/modules/beta/adapters/mysql/repository.go`
- Create: `internal/architecture/testdata/legacy-scrm/internal/store/scrm.go`
- Create: `cmd/mochat-architecture/main.go`

**Interfaces:**

- Produces:

```go
type Violation struct {
    RuleID string
    Path   string
    Detail string
}

type Policy struct {
    ProductionModules []string    `json:"productionModules"`
    ProtectedFiles    []SizeLimit `json:"protectedFiles"`
    ForbiddenNewFiles []string    `json:"forbiddenNewFiles"`
    Exceptions        []Exception `json:"exceptions"`
}

func LoadPolicy(path string) (Policy, error)
func Audit(root string, policy Policy, now time.Time) ([]Violation, error)
```

- Consumes: Go source tree and `architecture-policy.json`.

- [ ] **Step 1: Write failing rule tests**

Create table-driven tests in `internal/architecture/audit_test.go`:

```go
func TestAuditRejectsInvalidDependencies(t *testing.T) {
    cases := []struct {
        name   string
        root   string
        ruleID string
    }{
        {"domain imports SQL", "testdata/domain-imports-sql", "ARCH-DOMAIN-DEPENDENCY"},
        {"application imports adapter", "testdata/application-imports-adapter", "ARCH-APPLICATION-DEPENDENCY"},
        {"module imports dashboard", "testdata/module-imports-dashboard", "ARCH-LEGACY-DEPENDENCY"},
        {"module imports another adapter", "testdata/cross-module-adapter", "ARCH-CROSS-MODULE-PRIVATE"},
        {"legacy SCRM file", "testdata/legacy-scrm", "ARCH-FORBIDDEN-LEGACY-FILE"},
    }
    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            policy := testPolicy()
            violations, err := Audit(tc.root, policy, time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC))
            if err != nil {
                t.Fatal(err)
            }
            if !containsRule(violations, tc.ruleID) {
                t.Fatalf("violations = %#v, want %s", violations, tc.ruleID)
            }
        })
    }
}

func TestAuditRejectsExpiredException(t *testing.T) {
    policy := testPolicy()
    policy.Exceptions = []Exception{{
        RuleID: "ARCH-LEGACY-DEPENDENCY",
        Path: "internal/modules/bad/module.go",
        Import: "jiyi/mochat-go/internal/dashboard",
        Reason: "temporary migration bridge",
        Owner: "backend",
        CreatedOn: "2026-07-01",
        ExpiresOn: "2026-07-29",
        Cleanup: "replace with a port",
    }}
    violations, err := Audit("testdata/module-imports-dashboard", policy, time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC))
    if err != nil {
        t.Fatal(err)
    }
    if !containsRule(violations, "ARCH-EXCEPTION-EXPIRED") {
        t.Fatalf("violations = %#v", violations)
    }
}
```

- [ ] **Step 2: Run the architecture tests and verify failure**

Run:

```bash
go test ./internal/architecture
```

Expected: compilation fails because `Audit`, `Policy`, and `Exception` do not exist.

- [ ] **Step 3: Add exact policy and rule types**

Create `architecture-policy.json` with the current byte limits copied without increases:

```json
{
  "productionModules": ["scrm"],
  "protectedFiles": [
    {"path": "internal/dashboard/saas_admin_page.go", "maxBytes": 860906},
    {"path": "internal/dashboard/saas_admin.go", "maxBytes": 696324},
    {"path": "internal/store/mysql.go", "maxBytes": 840395},
    {"path": "cmd/mochat-go/main.go", "maxBytes": 206017}
  ],
  "forbiddenNewFiles": [
    "internal/dashboard/scrm*.go",
    "internal/store/scrm*.go"
  ],
  "exceptions": []
}
```

Implement rule constants and validated JSON types in `rules.go`. `Exception.Valid` must reject blank fields, malformed ISO dates, expiry before creation, wildcard paths, and expiry later than 90 days after creation.

- [ ] **Step 4: Implement AST/import auditing**

Implement `Audit` using `filepath.WalkDir`, `parser.ParseFile`, and `strconv.Unquote`. Normalize all policy and finding paths with `filepath.ToSlash`. Classification must recognize:

```go
type layer int
const (
    layerUnknown layer = iota
    layerDomain
    layerPorts
    layerApplication
    layerAdapters
    layerTransport
    layerModule
)
```

Apply the dependency table from the design. Treat `database/sql`, `net/http`, `jiyi/mochat-go/internal/{dashboard,store,server,config}` as forbidden where specified. Identify a module name from `internal/modules/<name>/...` and reject imports of another module's `adapters` or `transport`.

Apply an exception only when rule ID, normalized path, and exact import all match and the exception is valid and unexpired. Emit stable sorted violations by `RuleID`, `Path`, then `Detail`.

- [ ] **Step 5: Add the CLI**

Implement `cmd/mochat-architecture/main.go`:

```go
func main() {
    root := flag.String("root", ".", "repository root")
    policyPath := flag.String("policy", "architecture-policy.json", "policy file")
    flag.Parse()

    policy, err := architecture.LoadPolicy(filepath.Join(*root, *policyPath))
    if err != nil {
        log.Fatal(err)
    }
    violations, err := architecture.Audit(*root, policy, time.Now().UTC())
    if err != nil {
        log.Fatal(err)
    }
    for _, violation := range violations {
        fmt.Printf("%s %s: %s\n", violation.RuleID, violation.Path, violation.Detail)
    }
    if len(violations) > 0 {
        os.Exit(1)
    }
    fmt.Println("architecture boundaries passed")
}
```

- [ ] **Step 6: Verify fixtures and the real repository**

Run:

```bash
go test ./internal/architecture
go run ./cmd/mochat-architecture -root .
```

Expected: tests pass and the real repository prints `architecture boundaries passed`. If pre-existing imports violate newly introduced rules, register only exact pre-existing files as exceptions with complete metadata; do not add an exception for any SCRM file.

- [ ] **Step 7: Commit**

```bash
git add architecture-policy.json internal/architecture cmd/mochat-architecture
git commit -m "test: add cross-platform architecture gate"
```

---

### Task 2: Reusable module router and single server integration point

**Files:**

- Create: `internal/app/modules/router.go`
- Create: `internal/app/modules/router_test.go`
- Modify: `internal/server/server.go`
- Modify: `internal/server/server_test.go`

**Interfaces:**

- Produces:

```go
type RouteRegistrar interface {
    Handle(method, pattern string, handler http.Handler) error
}

type Router struct { /* private immutable-after-build route map */ }

func NewRouter() *Router
func (r *Router) Handle(method, pattern string, handler http.Handler) error
func (r *Router) Match(req *http.Request) (http.Handler, bool)
func (r *Router) ServeHTTP(http.ResponseWriter, *http.Request)
```

- `server.WithModuleRouter` consumes:

```go
type ModuleRouter interface {
    Match(*http.Request) (http.Handler, bool)
}
```

- [ ] **Step 1: Write failing router tests**

Cover exact method/path matching, method mismatch, duplicate registration, invalid method/path, and concurrent reads after registration:

```go
func TestRouterMatchesMethodAndPath(t *testing.T) {
    router := NewRouter()
    handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
        w.WriteHeader(http.StatusNoContent)
    })
    if err := router.Handle(http.MethodGet, "/api/example", handler); err != nil {
        t.Fatal(err)
    }
    request := httptest.NewRequest(http.MethodGet, "/api/example", nil)
    matched, ok := router.Match(request)
    if !ok {
        t.Fatal("route did not match")
    }
    response := httptest.NewRecorder()
    matched.ServeHTTP(response, request)
    if response.Code != http.StatusNoContent {
        t.Fatalf("status = %d", response.Code)
    }
}

func TestRouterRejectsDuplicateRoute(t *testing.T) {
    router := NewRouter()
    handler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
    if err := router.Handle("GET", "/api/example", handler); err != nil {
        t.Fatal(err)
    }
    if err := router.Handle("GET", "/api/example", handler); !errors.Is(err, ErrDuplicateRoute) {
        t.Fatalf("error = %v", err)
    }
}
```

- [ ] **Step 2: Verify the router test fails**

Run:

```bash
go test ./internal/app/modules
```

Expected: compilation fails because `Router` is undefined.

- [ ] **Step 3: Implement the minimal router**

Store routes by normalized uppercase method plus exact absolute path. Reject nil handlers, empty methods, paths not beginning with `/`, paths containing query strings, and duplicate keys. `Match` must not mutate state. `ServeHTTP` returns JSON `404` only when called directly; the server integration uses `Match` so unmatched legacy requests continue through the existing switch.

- [ ] **Step 4: Write failing server integration tests**

Add:

```go
func TestModuleRouterRunsBeforeLegacyFallback(t *testing.T) {
    router := modules.NewRouter()
    if err := router.Handle(http.MethodGet, "/api/phase2-2/ping", http.HandlerFunc(
        func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) },
    )); err != nil {
        t.Fatal(err)
    }
    server, err := New(config.Config{}, WithModuleRouter(router))
    if err != nil {
        t.Fatal(err)
    }
    response := httptest.NewRecorder()
    server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/phase2-2/ping", nil))
    if response.Code != http.StatusNoContent {
        t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
    }
}

func TestUnmatchedModuleRouteKeepsLegacyHealthRoute(t *testing.T) {
    server, err := New(config.Config{}, WithModuleRouter(modules.NewRouter()))
    if err != nil {
        t.Fatal(err)
    }
    response := httptest.NewRecorder()
    server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/healthz", nil))
    if response.Code != http.StatusOK {
        t.Fatalf("status = %d", response.Code)
    }
}
```

- [ ] **Step 5: Add the single server integration point**

Add one `moduleRouter ModuleRouter` field, one interface, one Option, and this branch after bundled-path normalization but before the legacy switch:

```go
if s.moduleRouter != nil {
    if handler, ok := s.moduleRouter.Match(r); ok {
        handler.ServeHTTP(w, r)
        return
    }
}
```

Do not add a field or Option for any SCRM handler.

- [ ] **Step 6: Verify router, server, race, and architecture gates**

Run:

```bash
go test ./internal/app/modules ./internal/server
go test -race ./internal/app/modules
go run ./cmd/mochat-architecture -root .
```

Expected: all commands exit 0. If the protected `server.go` baseline is desired later, add it as a new protected file at its current byte size; do not change the four existing limits.

- [ ] **Step 7: Commit**

```bash
git add internal/app/modules internal/server/server.go internal/server/server_test.go
git commit -m "refactor: add reusable module route registry"
```

---

### Task 3: SCRM lead domain and application use cases

**Files:**

- Create: `internal/modules/scrm/domain/errors.go`
- Create: `internal/modules/scrm/domain/lead.go`
- Create: `internal/modules/scrm/domain/lead_test.go`
- Create: `internal/modules/scrm/ports/clock.go`
- Create: `internal/modules/scrm/ports/id_generator.go`
- Create: `internal/modules/scrm/ports/lead_repository.go`
- Create: `internal/modules/scrm/application/service.go`
- Create: `internal/modules/scrm/application/service_test.go`

**Interfaces:**

- Produces:

```go
type Lead struct {
    ID          string
    TenantID    int64
    BusinessKey string
    Name        LeadName
    Source      LeadSource
    Status      LeadStatus
    Version     int64
    CreatedAt   time.Time
    UpdatedAt   time.Time
}

type LeadRepository interface {
    CreateOrGet(context.Context, domain.Lead) (lead domain.Lead, created bool, err error)
    List(context.Context, ListLeadsFilter) (LeadPage, error)
}

type ListLeadsFilter struct {
    TenantID int64
    Cursor   string
    Limit    int
}

type LeadPage struct {
    Items      []domain.Lead
    NextCursor string
}

type Clock interface {
    Now() time.Time
}

type IDGenerator interface {
    NewID() (string, error)
}

type Service struct { /* repository, clock, id generator */ }
func NewService(ports.LeadRepository, ports.Clock, ports.IDGenerator) (Service, error)
func (s Service) CreateLead(context.Context, CreateLeadCommand) (LeadView, error)
func (s Service) ListLeads(context.Context, ListLeadsQuery) (LeadPage, error)
```

- [ ] **Step 1: Write failing domain invariant tests**

Cover:

- zero/negative tenant ID rejected;
- blank business key rejected;
- business key longer than 128 bytes rejected;
- blank/over-200-byte name rejected;
- unsupported source rejected;
- timestamps converted to UTC;
- new lead status/version fixed to `new`/`1`.

Use fixed time and assert stable sentinel errors with `errors.Is`.

- [ ] **Step 2: Verify domain tests fail**

Run:

```bash
go test ./internal/modules/scrm/domain
```

Expected: compilation fails because `NewLead` and domain types are undefined.

- [ ] **Step 3: Implement the domain model**

Allowed sources are exactly:

```go
const (
    LeadSourceManual LeadSource = "manual"
    LeadSourceImport LeadSource = "import"
    LeadSourceWeCom  LeadSource = "wecom"
)
```

Use private values for `LeadName` and constructors/accessors rather than allowing invalid string assignment. `NewLead` receives all infrastructure values explicitly:

```go
func NewLead(id string, tenantID int64, businessKey, rawName string, source LeadSource, now time.Time) (Lead, error)
```

- [ ] **Step 4: Write failing application tests**

Implement fakes in `service_test.go` and cover:

```go
func TestCreateLeadUsesServerInputsAndReturnsExistingOnRetry(t *testing.T)
func TestCreateLeadRejectsMissingTenant(t *testing.T)
func TestListLeadsAlwaysScopesRepositoryByTenant(t *testing.T)
func TestListLeadsAppliesDefaultAndMaximumPageSize(t *testing.T)
func TestNewServiceRejectsNilDependencies(t *testing.T)
```

Use `PageSize=0 -> 20`, maximum `100`, and reject negative cursor/page size. The repository fake must record `TenantID` and verify no application path can issue an unscoped query.

- [ ] **Step 5: Verify application tests fail**

Run:

```bash
go test ./internal/modules/scrm/application
```

Expected: compilation fails because `Service` is undefined.

- [ ] **Step 6: Implement ports and use cases**

Define protocol-independent sentinel errors:

```go
var (
    ErrInvalidArgument = errors.New("invalid argument")
    ErrUnavailable     = errors.New("repository unavailable")
)
```

Wrap domain validation errors with `ErrInvalidArgument`. Do not import `net/http`, `database/sql`, MySQL packages, config, dashboard, store, or server.

- [ ] **Step 7: Verify module unit and architecture tests**

Run:

```bash
go test ./internal/modules/scrm/domain ./internal/modules/scrm/application
go run ./cmd/mochat-architecture -root .
```

Expected: all commands exit 0.

- [ ] **Step 8: Commit**

```bash
git add internal/modules/scrm/domain internal/modules/scrm/ports internal/modules/scrm/application
git commit -m "feat: add SCRM lead domain and use cases"
```

---

### Task 4: Migration and MySQL adapter

**Files:**

- Create: `deploy/standalone/migrations/0098_scrm_lead_foundation.up.sql`
- Create: `deploy/standalone/migrations/0098_scrm_lead_foundation.down.sql`
- Create: `internal/modules/scrm/adapters/mysql/lead_repository.go`
- Create: `internal/modules/scrm/adapters/mysql/lead_repository_integration_test.go`
- Modify: `scripts/smoke_schema_migrate.sh`

**Interfaces:**

- Consumes: `ports.LeadRepository`.
- Produces:

```go
func NewLeadRepository(db *sql.DB) (*LeadRepository, error)
func (r *LeadRepository) CreateOrGet(context.Context, domain.Lead) (domain.Lead, bool, error)
func (r *LeadRepository) List(context.Context, ports.ListLeadsFilter) (ports.LeadPage, error)
```

- [ ] **Step 1: Write the migration**

Use a MySQL 5.7-compatible table:

```sql
CREATE TABLE `mochat_go_scrm_leads` (
  `id` varchar(36) NOT NULL,
  `tenant_id` bigint unsigned NOT NULL,
  `business_key` varchar(128) NOT NULL,
  `name` varchar(200) NOT NULL,
  `source` varchar(32) NOT NULL,
  `status` varchar(32) NOT NULL,
  `version` bigint unsigned NOT NULL DEFAULT 1,
  `created_at` datetime(6) NOT NULL,
  `updated_at` datetime(6) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_scrm_leads_tenant_business_key` (`tenant_id`,`business_key`),
  KEY `idx_scrm_leads_tenant_created_id` (`tenant_id`,`created_at`,`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
```

Down migration:

```sql
DROP TABLE IF EXISTS `mochat_go_scrm_leads`;
```

- [ ] **Step 2: Extend migration lifecycle assertions**

Add exact assertions to `scripts/smoke_schema_migrate.sh` for table existence, all required columns, unique key, rollback absence, and replay presence.

- [ ] **Step 3: Write failing tagged MySQL integration tests**

Use the repository's existing MySQL integration environment convention and an explicit `//go:build integration` tag. Cover:

```go
func TestLeadRepositoryTenantIsolation(t *testing.T)
func TestLeadRepositoryCreateOrGetIsIdempotent(t *testing.T)
func TestLeadRepositoryAllowsSameBusinessKeyAcrossTenants(t *testing.T)
func TestLeadRepositoryListUsesStableCreatedAtAndIDOrder(t *testing.T)
func TestLeadRepositoryConcurrentCreateProducesOneRow(t *testing.T)
```

The concurrency test starts 10 goroutines with the same tenant/business key and asserts one `created=true`, nine `created=false`, identical returned IDs, and one database row.

- [ ] **Step 4: Verify adapter tests fail**

Run against the project integration database:

```bash
go test -tags=integration ./internal/modules/scrm/adapters/mysql
```

Expected: compilation fails because `NewLeadRepository` is undefined.

- [ ] **Step 5: Implement the adapter**

Use parameterized SQL. `CreateOrGet` attempts INSERT first. On MySQL error 1062, query by exact `(tenant_id, business_key)` and return `created=false`. Any query by ID or business key must include tenant ID. List query must use:

```sql
WHERE tenant_id = ?
  AND (created_at < ? OR (created_at = ? AND id < ?))
ORDER BY created_at DESC, id DESC
LIMIT ?
```

Return an opaque application cursor encoded from UTC timestamp plus ID; do not expose SQL offsets as a stable contract.

- [ ] **Step 6: Verify adapter, migration, and architecture**

Run:

```bash
go test -tags=integration ./internal/modules/scrm/adapters/mysql
./scripts/smoke_schema_migrate.sh
go run ./cmd/mochat-architecture -root .
```

Expected: all commands exit 0 and the migration script proves apply/checksum/rollback/replay.

- [ ] **Step 7: Commit**

```bash
git add deploy/standalone/migrations/0098_scrm_lead_foundation.* internal/modules/scrm/adapters/mysql scripts/smoke_schema_migrate.sh
git commit -m "feat: add tenant-scoped SCRM lead persistence"
```

---

### Task 5: HTTP transport and module assembly

**Files:**

- Create: `internal/modules/scrm/transport/http/principal.go`
- Create: `internal/modules/scrm/transport/http/lead_handler.go`
- Create: `internal/modules/scrm/transport/http/routes.go`
- Create: `internal/modules/scrm/transport/http/lead_handler_test.go`
- Create: `internal/modules/scrm/module.go`
- Create: `internal/modules/scrm/module_test.go`

**Interfaces:**

- Produces:

```go
type Principal struct {
    UserID   int64
    TenantID int64
}

type PrincipalResolver interface {
    Resolve(*http.Request) (Principal, error)
}

type Dependencies struct {
    DB                *sql.DB
    Clock             ports.Clock
    IDGenerator       ports.IDGenerator
    PrincipalResolver transporthttp.PrincipalResolver
}

func New(Dependencies) (*Module, error)
func (m *Module) RegisterRoutes(appmodules.RouteRegistrar) error
```

- [ ] **Step 1: Write failing HTTP tests**

Cover:

- authenticated create returns `201`;
- idempotent retry returns `200` and the same object;
- tenant ID in JSON is ignored/rejected and cannot override the principal;
- missing/invalid principal returns `401`;
- validation failure returns `422`;
- list returns only repository results for the principal tenant;
- malformed cursor/page size returns `400`;
- repository unavailable returns `503`;
- method mismatch is not registered.

Use a fake `PrincipalResolver`, fake service interface, and `httptest`; do not import dashboard.

- [ ] **Step 2: Verify HTTP tests fail**

Run:

```bash
go test ./internal/modules/scrm/transport/http
```

Expected: compilation fails because handler and resolver types are undefined.

- [ ] **Step 3: Implement transport mapping**

Request:

```json
{
  "businessKey": "manual:2026-0001",
  "name": "示例线索",
  "source": "manual"
}
```

Response:

```json
{
  "data": {
    "id": "generated-id",
    "businessKey": "manual:2026-0001",
    "name": "示例线索",
    "source": "manual",
    "status": "new",
    "version": 1,
    "createdAt": "2026-07-30T00:00:00Z",
    "updatedAt": "2026-07-30T00:00:00Z"
  }
}
```

Never include or accept a mutable tenant ID in the body. Limit request bodies with `http.MaxBytesReader`, reject unknown JSON fields, and reject trailing JSON values.

- [ ] **Step 4: Write failing module tests**

Test that `New` rejects each nil dependency, `RegisterRoutes` installs exactly POST/GET `/api/phase2-2/scrm/leads`, and registering a second internal handler only changes the module/transport tests—not `server.Server`.

- [ ] **Step 5: Implement explicit module assembly**

`New` constructs only:

```text
MySQL repository -> application service -> HTTP handler -> Module
```

No environment reads and no hidden singleton. `RegisterRoutes` returns duplicate/invalid route errors to the composition root.

- [ ] **Step 6: Verify transport, module, race, and architecture**

Run:

```bash
go test ./internal/modules/scrm/...
go test -race ./internal/modules/scrm/...
go run ./cmd/mochat-architecture -root .
```

Expected: all commands exit 0.

- [ ] **Step 7: Commit**

```bash
git add internal/modules/scrm/transport internal/modules/scrm/module.go internal/modules/scrm/module_test.go
git commit -m "feat: expose disabled SCRM lead pilot module"
```

---

### Task 6: Disabled-by-default runtime wiring

**Files:**

- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Create: `internal/app/bootstrap/scrm.go`
- Create: `internal/app/bootstrap/scrm_test.go`
- Modify: `cmd/mochat-go/main.go`

**Interfaces:**

- Produces:

```go
Config.EnablePhase22SCRMPilot bool

type SCRMDependencies struct {
    DB                *sql.DB
    PrincipalResolver transporthttp.PrincipalResolver
}

func RegisterSCRM(router *appmodules.Router, enabled bool, deps SCRMDependencies) error
```

- [ ] **Step 1: Write failing config tests**

Add:

```go
func TestPhase22SCRMPilotDisabledByDefault(t *testing.T)
func TestPhase22SCRMPilotCanBeEnabled(t *testing.T)
```

The second test sets `MOCHAT_GO_ENABLE_PHASE2_2_SCRM_PILOT=true`. Both tests must isolate the environment with `t.Setenv`.

- [ ] **Step 2: Verify config tests fail**

Run:

```bash
go test ./internal/config -run Phase22
```

Expected: compilation fails because `EnablePhase22SCRMPilot` is undefined.

- [ ] **Step 3: Add the exact config flag**

Add one field and parse it only with:

```go
EnablePhase22SCRMPilot: envBool("MOCHAT_GO_ENABLE_PHASE2_2_SCRM_PILOT"),
```

Do not derive it from standalone mode or `EnableAllMigratedRoutes`.

- [ ] **Step 4: Write failing bootstrap tests**

Cover:

- disabled registration accepts nil SCRM dependencies and installs no route;
- enabled registration rejects nil DB/resolver;
- enabled registration installs both routes;
- duplicate registration fails startup;
- a header claiming another tenant cannot change the injected principal tenant.

- [ ] **Step 5: Implement the bootstrap boundary**

Create concrete Clock and ID generator adapters in `internal/app/bootstrap/scrm.go`. Use `crypto/rand`-backed UUID-compatible IDs without adding a new dependency. Adapt the existing authenticated user/tenant facilities behind a type implementing only `PrincipalResolver`; the adapter may depend on legacy authentication packages, but `internal/modules/scrm` may not.

If the current authentication stack cannot resolve tenant ID without dashboard types, create a small shared principal contract under `internal/authprincipal` and adapt both sides. Do not move SCRM transport into dashboard and do not trust `X-Mochat-Go-Tenant-ID` outside explicitly marked development/test mode.

- [ ] **Step 6: Wire one module router into main**

Near server construction:

```go
moduleRouter := appmodules.NewRouter()
if err := bootstrap.RegisterSCRM(moduleRouter, cfg.EnablePhase22SCRMPilot, bootstrap.SCRMDependencies{
    DB:                getMySQLStore().DB(), // expose DB through mysqlconn/bootstrap, not through SCRM's store dependency
    PrincipalResolver: principalResolver,
}); err != nil {
    log.Fatalf("register SCRM pilot module: %v", err)
}
options = append(options, compatserver.WithModuleRouter(moduleRouter))
```

Do not add an SCRM handler Option. If `MySQLStore.DB()` would expand legacy coupling, retain the `*sql.DB` returned by `mysqlconn.Open` in the composition root and pass that directly to both constructions.

- [ ] **Step 7: Verify disabled and enabled behavior**

Run:

```bash
go test ./internal/config ./internal/app/bootstrap ./internal/server ./internal/modules/scrm/...
go test ./...
go vet ./...
go run ./cmd/mochat-architecture -root .
```

Expected: all commands exit 0; tests prove the default does not register pilot routes.

- [ ] **Step 8: Commit**

```bash
git add internal/config internal/app/bootstrap cmd/mochat-go/main.go
git commit -m "feat: wire opt-in SCRM pilot module"
```

---

### Task 7: Make the gates mandatory in local and CI workflows

**Files:**

- Modify: `scripts/audit_architecture_boundaries.sh`
- Modify: `scripts/test_audit_architecture_boundaries.sh`
- Modify: `scripts/dev_check.sh`
- Modify: `scripts/test.sh`
- Modify: `.github/workflows/mysql57-amd64.yml`
- Create: `.github/PULL_REQUEST_TEMPLATE.md`

**Interfaces:**

- Consumes: `go run ./cmd/mochat-architecture -root .`, module tests, tagged integration tests.
- Produces: identical architecture verdict on Windows and Linux plus mandatory PR declarations.

- [ ] **Step 1: Write failing wrapper tests**

Update `scripts/test_audit_architecture_boundaries.sh` to create a temporary invalid module tree, invoke:

```bash
go run ./cmd/mochat-architecture -root "$TEMP_ROOT" -policy architecture-policy.json
```

and assert non-zero exit plus the exact rule ID. Then invoke the real root and assert `architecture boundaries passed`.

- [ ] **Step 2: Verify the wrapper test fails before replacement**

Run:

```bash
sh scripts/test_audit_architecture_boundaries.sh
```

Expected: failure because the old shell auditor does not produce the new stable rule IDs.

- [ ] **Step 3: Replace the shell implementation with a compatibility wrapper**

`scripts/audit_architecture_boundaries.sh` becomes:

```sh
#!/usr/bin/env sh
set -eu
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
cd "$ROOT"
exec go run ./cmd/mochat-architecture -root .
```

Keep the filename so existing developer and CI entry points remain stable.

- [ ] **Step 4: Add local quality commands**

In `scripts/dev_check.sh quick` and `scripts/test.sh`, execute in this order:

```bash
go run ./cmd/mochat-architecture -root .
go test ./internal/app/modules/... ./internal/modules/...
go test -race ./internal/modules/...
go test ./...
go vet ./...
```

Do not run tagged MySQL integration tests in the no-database quick path.

- [ ] **Step 5: Add CI gates**

Add separate named steps:

```yaml
- name: Go architecture gate
  run: go run ./cmd/mochat-architecture -root .

- name: Go module race gate
  run: go test -race ./internal/modules/...

- name: SCRM MySQL integration gate
  run: go test -tags=integration ./internal/modules/scrm/adapters/mysql
```

Place the integration step after the workflow's MySQL service/readiness setup. Ensure migration `0098` is applied before adapter tests.

- [ ] **Step 6: Add the PR template**

Require checked declarations for:

```markdown
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
```

- [ ] **Step 7: Verify workflows locally**

Run:

```bash
sh scripts/test_audit_architecture_boundaries.sh
go run ./cmd/mochat-architecture -root .
go test -race ./internal/modules/...
go test ./...
go vet ./...
```

Expected: all commands exit 0.

- [ ] **Step 8: Commit**

```bash
git add scripts/audit_architecture_boundaries.sh scripts/test_audit_architecture_boundaries.sh scripts/dev_check.sh scripts/test.sh .github/workflows/mysql57-amd64.yml .github/PULL_REQUEST_TEMPLATE.md
git commit -m "ci: enforce backend module quality gates"
```

---

### Task 8: Revise Phase 3 plan and record Phase 2.2 acceptance

**Files:**

- Modify: `docs/superpowers/plans/2026-07-29-phase3-yuanhu-business-foundation.md`
- Modify: `docs/phases/phase-2.2-backend-quality-gates/README.md`
- Create: `docs/phases/phase-2.2-backend-quality-gates/acceptance.md`

**Interfaces:**

- Consumes: all Phase 2.2 verification outputs.
- Produces: a Phase 3 plan that extends `internal/modules/scrm` and reproducible acceptance evidence.

- [ ] **Step 1: Rewrite Phase 3 backend paths**

Replace:

```text
internal/store/scrm.go
internal/dashboard/scrm.go
internal/dashboard/scrm_metrics.go
```

with task-specific files under:

```text
internal/modules/scrm/domain/
internal/modules/scrm/application/
internal/modules/scrm/ports/
internal/modules/scrm/adapters/mysql/
internal/modules/scrm/transport/http/
```

Add the Phase 2.2 architecture command to every Phase 3 backend task. State explicitly that Phase 2.2 pilot routes are not the final Phase 3 API contract.

- [ ] **Step 2: Run final verification from a clean tree**

Run:

```bash
git status --short
go run ./cmd/mochat-architecture -root .
go test ./internal/architecture/... ./internal/app/modules/... ./internal/modules/...
go test -race ./internal/modules/...
go test ./...
go vet ./...
go build ./cmd/mochat-go ./cmd/mochat-inventory ./cmd/mochat-migrate ./cmd/mochat-bootstrap ./cmd/mochat-saas-maintenance ./cmd/mochat-architecture
```

Run with MySQL/Docker available:

```bash
go test -tags=integration ./internal/modules/scrm/adapters/mysql
./scripts/smoke_schema_migrate.sh
```

Expected: every command exits 0. The initial `git status --short` may list only the acceptance documentation being authored; after commit it must be empty.

- [ ] **Step 3: Perform an explicit negative gate check**

In a temporary copy outside the repository:

1. Add `internal/modules/scrm/domain/bad.go` importing `database/sql`.
2. Run the architecture CLI against the temporary root.
3. Assert exit code 1 and `ARCH-DOMAIN-DEPENDENCY`.
4. Delete the temporary copy.

Do not modify or clean the real worktree to perform this test.

- [ ] **Step 4: Write acceptance evidence**

Record:

- branch and commit SHA;
- OS, Go version, MySQL version;
- every command above and exit status;
- tenant-isolation/idempotency/concurrency scenarios;
- migration apply/checksum/rollback/replay result;
- negative architecture finding;
- proof that the pilot flag defaults false;
- proof that legacy `/healthz` and representative migrated routes still pass;
- remaining legacy debt;
- final `Phase 3 backend ready: yes/no` verdict.

No screenshots are required for this backend-only phase; attach CI logs or text artifacts where available.

- [ ] **Step 5: Mark the phase complete only if every gate passed**

Update the Phase 2.2 README status from `设计阶段` to `已完成` only when all acceptance checkboxes have objective evidence. Otherwise use `实施中` and list exact blockers.

- [ ] **Step 6: Commit**

```bash
git add docs/superpowers/plans/2026-07-29-phase3-yuanhu-business-foundation.md docs/phases/phase-2.2-backend-quality-gates
git commit -m "docs: complete phase2.2 backend readiness evidence"
```

- [ ] **Step 7: Confirm clean completion**

Run:

```bash
git status --short
git log --oneline --max-count=10
```

Expected: empty status and a reviewable sequence of eight Phase 2.2 commits.
