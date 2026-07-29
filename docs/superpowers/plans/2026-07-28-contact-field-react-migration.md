# Contact Field React Migration Plan

**Goal:** Migrate `/contactField/index` to React with field listing, filtering, create/edit, status, delete, and batch editing.

**Architecture:** Use one typed API adapter for the six audited contracts and a React Query page with modal editors. Preserve system-field restrictions. Treat single-record deletion as part of the existing `edit` permission because the legacy permission model has no delete action; batch deletion remains under `batch`.

### Task 1: Typed API

- Test exact list query and all five write request contracts.
- Implement list, create, update, status update, delete, and batch update.

### Task 2: Page behavior

- Test the `advanced` tab gate and `all` status filter.
- Test `add`, `edit`, `close`, and `batch` action gates.
- Test system field restrictions, create/edit validation, delete confirmation, and cache refresh.
- Implement table, editors, and batch mode.

### Task 3: React cutover

- Register the page and change its manifest target to `react`.
- Update metadata and regenerate candidate reports.
- Run all tests, typecheck, lint, build, audit tests, filtered audit, and diff checks.
