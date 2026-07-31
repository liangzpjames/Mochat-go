# Task 5 Report: P0 Real Dashboard Overview

## Status

Implemented and verified.

Implementation commits:

- `fca32bd` (`feat: add real dashboard overview`)
- `a7798e7` (`fix: harden dashboard overview states`)

## Delivered

- Reused the existing authenticated `/dashboard/corpData/index` capability rather
  than adding a synonymous endpoint.
- Extended the existing response with stable `cards`, `trend`, and `updatedAt`
  fields while retaining the legacy summary fields.
- Added explicit `corpId`, `from`, and `to` request contracts:
  - `corpId` must match the authenticated, validated selected corp or the handler
    returns `403` before querying the store.
  - `from` and `to` are serialized as `YYYY-MM-DD` local-calendar dates.
  - Ranges are inclusive and limited to 31 days, matching the existing trend
    window.
- Changed the MySQL trend query from a generated date list to a corp-scoped
  inclusive `BETWEEN` query.
- Added the real dashboard API adapter:

  ```ts
  DashboardOverviewApi.load(input: {
    corpId: string;
    from: string;
    to: string;
  }): Promise<DashboardOverview>
  ```

- Added the Yuanhu-style `/index` overview page with:
  - real response-backed statistic cards and trend bars;
  - local date-range filters;
  - refresh through the injected API;
  - loading, empty, forbidden, and retryable error states;
  - the backend update timestamp.
- Registered `/index` through the P0 page registry and the authenticated API
  client in `main.tsx`.
- Validated the response shape at the API boundary. A legacy PHP fallback
  response is converted to a controlled error state instead of reaching
  `cards.length` and crashing.
- Suppressed cached cards and trend values whenever a refresh enters an error or
  forbidden state, so revoked corp access cannot leave stale statistics visible.
- Mirrored the backend's inclusive 31-day range limit in the page filter.
- Added no production mock or fixture data.

`internal/server/server.go` did not require a production change because it
already dispatches `GET /dashboard/corpData/index` through
`WithCorpDataIndexHandler`. Its contract test was extended to prove the overview
query string reaches the migrated handler unchanged.

## Tenant, Corp, and Permission Boundaries

- Authentication and tenant/corp resolution remain in the existing
  `ResolveValidatedLoginCorpInfoFromStore` flow.
- The requested `corpId` is checked against the one validated selected corp.
- A mismatched corp returns `403`; neither summary nor trend storage is called.
- The summary queries retain their existing `corp_id = ?` constraints.
- The trend query includes both `corp_id = ?` and the requested inclusive date
  bounds.
- Frontend corp scope comes only from `DashboardAccessContext`; the component
  cannot supply an arbitrary corp and does not construct an endpoint URL.

## Changed Files

- `.superpowers/sdd/task-5-report.md`
- `internal/dashboard/corp_data.go`
- `internal/dashboard/corp_data_test.go`
- `internal/server/server_test.go`
- `internal/store/corp_data_test.go`
- `internal/store/mysql.go`
- `web/apps/dashboard/src/benchmark/page-registry.tsx`
- `web/apps/dashboard/src/features/dashboard-overview/dashboard-overview-api.ts`
- `web/apps/dashboard/src/features/dashboard-overview/dashboard-overview-api.test.ts`
- `web/apps/dashboard/src/features/dashboard-overview/dashboard-overview-page.tsx`
- `web/apps/dashboard/src/features/dashboard-overview/dashboard-overview-page.test.tsx`
- `web/apps/dashboard/src/main.tsx`
- `web/apps/dashboard/src/styles/index.css`

## TDD Evidence

1. Existing capability check:

   ```text
   go test ./internal/server ./internal/store ./internal/dashboard \
     -run 'Overview|CorpData|Statistics' -count=1
   ```

   Passed in all three packages, proving the existing corp-data handlers and
   server dispatch were present before implementation.

2. Backend RED:

   ```text
   go test ./internal/dashboard ./internal/store -run 'CorpData' -count=1
   ```

   Failed to compile because the requested range-aware
   `CorpDataLineChat(ctx, corpID, from, to)` contract and
   `corpDataTrendQuery` did not exist.

3. Backend GREEN:

   The same focused command passed after the minimum handler and store changes.

4. Empty-contract RED:

   ```text
   go test ./internal/dashboard \
     -run 'TestCorpDataIndexReturnsEmptyArrays' -count=1
   ```

   Failed because an empty corp produced four zero-valued cards.

5. Empty-contract GREEN:

   The focused dashboard/store corp-data suite passed after returning empty
   `cards` and `trend` arrays for a fully empty summary.

6. Frontend RED:

   ```text
   pnpm exec vitest run src/features/dashboard-overview
   ```

   Failed because `dashboard-overview-api.ts` and
   `dashboard-overview-page.tsx` did not exist.

7. Frontend GREEN:

   ```text
   Test Files  2 passed (2)
   Tests       6 passed (6)
   ```

8. Build feedback:

   The first typecheck correctly rejected an overly generic test client seam.
   The API client dependency was narrowed to the concrete overview response;
   the production dashboard then typechecked and built successfully.

9. Lint feedback:

   The first lint run found one stale test suppression and an existing
   promise-returning `refreshAccess` callback in touched `main.tsx`. Both were
   corrected; the fresh lint run exited successfully with no findings.

10. Review RED:

    A read-only independent review found that React Query could retain successful
    data after a failed refresh, and that a legacy PHP response lacked the new
    arrays. New tests proved both defects:

    - success followed by a `403` still rendered the old `137` card;
    - the legacy payload resolved instead of being rejected.

11. Review GREEN:

    The page now renders data only when `query.isError` is false, the API adapter
    validates the complete overview shape, and the fallback-route integration
    test reaches a controlled alert. The page also rejects ranges longer than
    31 inclusive days before making a request.

## Final Verification

- `go test ./internal/... -count=1`
  - Exit `0`.
  - All internal packages passed; packages without tests were reported normally.
- `go test ./internal/server -run 'CorpData' -count=1`
  - Exit `0`.
  - Migrated route dispatch preserves `corpId`, `from`, and `to`.
- `pnpm --filter @mochat/dashboard test -- src/features/dashboard-overview`
  - Exit `0`.
  - `41` test files passed; `269` tests passed.
  - The repository script currently forwards the extra `--`, so this command
    runs the full dashboard suite, including the ten focused overview tests.
- `pnpm exec vitest run src/features/dashboard-overview`
  - Exit `0`.
  - `2` test files passed; `10` tests passed.
- `pnpm --filter @mochat/dashboard build`
  - Exit `0`.
  - TypeScript passed and Vite built `1660` modules.
- `pnpm --filter @mochat/dashboard lint`
  - Exit `0`, no findings.
- `git diff --check`
  - Exit `0`; only Git's Windows line-ending notices were printed.

## Self-Review

- No duplicate overview endpoint was introduced.
- No mock or fixture is reachable from the P0 production route.
- Every new backend query is corp scoped and the requested corp is rejected
  before store access when it differs from authenticated context.
- Frontend tests prove request serialization, response-derived card/trend
  values, refresh, empty, forbidden, loading, retry, success-to-403 revocation,
  legacy response handling, and the 31-day filter boundary.
- Existing legacy summary fields and `/corpData/lineChat` compatibility are
  retained.
- No unrelated task report was changed.

## Remaining Concerns

- The deployed Go dashboard should keep `MOCHAT_GO_MIGRATE_CORP_DATA_INDEX`
  enabled (normally covered by `MOCHAT_GO_ENABLE_ALL_MIGRATED_ROUTES`). When it
  is disabled, the page now shows a controlled "overview API not enabled" error
  rather than crashing or fabricating data.
- Date grouping follows the process-local project timezone and MySQL `DATE`
  semantics. Deployment should keep the Go process and MySQL session timezone
  aligned, as required by the existing corp-day aggregation jobs.
- The store package has no SQL row-mocking harness. Its unit contract therefore
  verifies corp/date arguments, inclusive `BETWEEN`, ascending order, and the
  31-row cap; row scanning and empty result behavior continue to be covered by
  the existing MySQL implementation path rather than a synthetic query-result
  fixture.
