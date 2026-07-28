# Department React Migration Plan

**Goal:** Migrate `/department/index` to React while preserving the department tree, filters, member dialog, permissions, pagination, and synchronization behavior.

**Architecture:** Add a typed department read API and reuse the existing employee conditions/synchronization API. Use React Query keys scoped to the selected corp. Keep department and member pagination independent.

### Task 1: Department API

- Test and implement `GET /workDepartment/pageIndex`.
- Test and implement `GET /workDepartment/showEmployee`.

### Task 2: Department page

- Test initial tree and sync time.
- Test `search`, `check`, and `sync` permission gates.
- Test applied filters and reset behavior.
- Test paged member dialog.
- Test synchronization invalidates department and condition queries.
- Implement the page and make focused checks pass.

### Task 3: React cutover

- Register the page in the composition root.
- Change `/department/index` to `target: react`.
- Update metadata and regenerate reports.
- Run the complete test, typecheck, lint, build, audit, and diff gates.
