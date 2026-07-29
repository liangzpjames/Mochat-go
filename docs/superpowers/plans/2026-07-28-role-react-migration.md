# Role React Migration Plan

**Goal:** Migrate `/role/index` with search, CRUD, copy, status, member inspection, and permission navigation.

**Architecture:** Add a typed seven-contract API and a React Query page. Keep `/role/permissionShow` as its separate routed feature. Prevent disabling or deleting roles that still contain employees.

### Tasks

1. Test and implement list, detail, members, create/copy, update, status, and delete APIs.
2. Test and implement permission-gated search, editor, copy, member modal, status guard, delete, and permission navigation.
3. Switch the route to React, refresh metadata, and run the complete quality gates.
