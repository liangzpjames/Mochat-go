# Work Contact Tag React Migration Plan

**Goal:** Migrate `/workContactTag/index`, the final Dashboard Batch 2 page.

**Architecture:** Add a typed eleven-contract API and a React Query page for tag groups and paged tags. Preserve permission-gated group CRUD, tag CRUD, batch delete/move, and enterprise-WeChat synchronization. Keep the fixed ungrouped group immutable and validate unique tag names of at most 15 characters.

### Tasks

1. Test and implement tag list/detail/create/update/delete/move/sync and group list/detail/create/update/delete APIs.
2. Test and implement group selection, tag selection, CRUD dialogs, batch operations, validation, and synchronization.
3. Switch the route to React, refresh metadata, run all quality gates, and close the Phase 2 candidate queue.
