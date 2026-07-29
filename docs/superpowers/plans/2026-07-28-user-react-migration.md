# User React Migration Plan

**Goal:** Migrate `/user/index` with filtering, account creation/editing, batch enablement, phone-to-department lookup, roles, and password reset.

**Architecture:** Add a typed eight-contract API and a React Query page. Keep account writes permission-gated by the audited add/search actions, validate mainland mobile numbers and matching passwords client-side, and invalidate the account list after every mutation.

### Tasks

1. Test and implement list, create, update, detail, status, department lookup, role select, and password reset APIs.
2. Test and implement filtering, row selection/batch enablement, account editor, department lookup, roles, and password reset.
3. Switch the route to React, refresh metadata, and run the complete quality gates.
