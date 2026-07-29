# Work Employee React Migration Plan

**Goal:** Migrate `/workEmployee/index` to React with its audited list, search-condition, and synchronization contracts.

**Architecture:** Add a typed feature API and a React Query page. Keep filter state separate from applied query state, gate search/sync actions with existing permissions, and invalidate both list and condition queries after synchronization.

### Task 1: API adapter

- Add failing contract tests for:
  - `GET /workEmployee/index` query parameters.
  - `GET /workEmployee/searchCondition`.
  - `PUT /workEmployee/synEmployee`.
- Implement the typed adapter and make tests pass.

### Task 2: Employee page

- Add failing tests for initial data, permission visibility, applied filters, pagination, synchronization refresh, empty state, and API errors.
- Implement the table, filter drawer, and synchronization mutation.
- Make focused tests pass.

### Task 3: React cutover

- Add the page to the application composition root.
- Change `/workEmployee/index` to `target: react`.
- Update metadata and regenerate candidate reports.
- Run tests, typecheck, lint, build, audit tests, filtered Windows audit, and diff checks.
