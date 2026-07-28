# Password Update React Migration Plan

**Goal:** Migrate `/passwordUpdate/index` from the legacy Vue dashboard to the React dashboard without changing its URL, permission, API contract, or logout behavior.

**Architecture:** Add a small typed API adapter and an isolated React feature page. The page receives its API and success callback as dependencies, reads the existing dashboard action permission, submits the audited `PUT /user/passwordUpdate` contract, and delegates session clearing/navigation to the application composition root.

**Tech Stack:** React 19, TypeScript, Ant Design, React Hook Form, Vitest, Testing Library.

---

### Task 1: Define and test the password update API adapter

**Files:**
- Create: `web/apps/dashboard/src/features/password/password-api.ts`
- Create: `web/apps/dashboard/src/features/password/password-api.test.ts`

1. Write a failing test for the exact `PUT /user/passwordUpdate` request body.
2. Run the focused test and confirm it fails because the adapter is absent.
3. Implement the minimal typed adapter.
4. Run the focused test and confirm it passes.

### Task 2: Build and test the password update page

**Files:**
- Create: `web/apps/dashboard/src/features/password/password-page.tsx`
- Create: `web/apps/dashboard/src/features/password/password-page.test.tsx`

1. Write failing tests for password inputs, alphanumeric validation, confirmation matching, permission-gated save, API error feedback, and success callback.
2. Run the focused tests and confirm they fail.
3. Implement the minimal accessible form with Ant Design.
4. Run the focused tests and confirm they pass.

### Task 3: Switch the route to React and wire logout

**Files:**
- Modify: `web/apps/dashboard/src/migration-routes.json`
- Modify: `web/apps/dashboard/src/main.tsx`
- Modify: `docs/phases/phase-1-frontend-foundation/audit/pages.csv`
- Modify: `docs/phases/phase-2-frontend-migration/audit/page-metadata-overrides.csv`

1. Add an integration assertion that the migrated route renders the injected React page.
2. Change `/passwordUpdate/index` to `target: react`.
3. Wire the page to the API adapter; on success clear the auth session and navigate to `/login`.
4. Refresh the generated Phase 2 reports while preserving unrelated Phase 1 audit files and the legacy source manifest.
5. Run dashboard tests, typecheck, lint, build, inventory tests, audit checks with the known Windows manifest-hash exception, and `git diff --check`.

