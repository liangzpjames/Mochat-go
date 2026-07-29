# Menu React Migration Plan

**Goal:** Migrate `/menu/index` with hierarchical listing, search, add/edit, enable/disable, and removal.

**Architecture:** Add a typed eight-contract API and a React Query page. Preserve the five-level parent selection model and conditional fields. Replace the legacy icon picker with an audited used-icon-aware selector without coupling the page to Vue components.

### Tasks

1. Test and implement list, tree select, detail, used icons, create, update, status, and delete APIs.
2. Test and implement permission-gated search, hierarchical parent selection, conditional validation, editing, status, and removal.
3. Switch the route to React, refresh metadata, and run the complete quality gates.
