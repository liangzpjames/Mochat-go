# Phase 1 Corrective Completion Implementation Plan

> **For agentic workers:** Execute inline with test-first changes and a verification checkpoint after each task.

**Goal:** Close the SaaS Admin publishing, Dashboard navigation, and login presentation gaps without migrating additional business pages.

**Architecture:** Preserve the existing workspace, Go static wrappers, migration manifest, authentication flow, and permission semantics. Add release-chain identity checks, retain the authorized menu tree in access state, render it in the Dashboard shell, and apply the existing Ant Design dependency to the login presentation.

**Tech Stack:** Docker, pnpm workspace, React 19, React Router, Ant Design 6, Vitest, Testing Library, Playwright, Go static frontend wrappers.

## Global Constraints

- Do not change API paths, authentication fields, permission decisions, tenant boundaries, or manifest targets.
- Do not migrate any additional business route in Phase 1.
- Do not copy legacy images with unresolved provenance.
- Preserve `.workbuddy/` and the untracked `web/saas-admin/`.
- Keep the existing two frontend audit failures recorded rather than modifying unrelated Go evidence.

### Task 1: Lock the frontend release-chain contract

**Files:**
- Create: `scripts/check_frontend_release_chain.test.mjs`
- Modify: `Dockerfile`
- Modify: `scripts/sync_frontend_dist.sh`
- Modify: `scripts/smoke_standalone_compose_app.sh`

Steps:

1. Add a Node test asserting both workspace apps are built, both `dist` directories are copied from `frontend-build`, the sync script builds both apps, and compose smoke rejects identical Dashboard/SaaS HTML.
2. Run the test and observe failure on the missing SaaS build/copy commands.
3. Add the minimal Dockerfile and sync-script commands.
4. Strengthen compose smoke identity assertions.
5. Run the test and both production builds.
6. Commit as `fix: publish saas admin with dashboard`.

### Task 2: Preserve and render authorized navigation

**Files:**
- Modify: `web/apps/dashboard/src/app/access-loader.ts`
- Modify: `web/apps/dashboard/src/app/access-loader.test.ts`
- Modify: `web/apps/dashboard/src/app/access-context.tsx`
- Modify: `web/apps/dashboard/src/layout/dashboard-layout.tsx`
- Modify: `web/apps/dashboard/src/layout/dashboard-layout.test.tsx`
- Modify: `web/apps/dashboard/src/styles/index.css`

Steps:

1. Add a failing access-loader assertion that the returned context contains the loaded menu.
2. Add failing layout tests for authorized internal links and `/saas-admin/`.
3. Return `menu` in `AccessContext` without changing route/action authorization.
4. Add an optional access hook for layout tests that run without an access loader.
5. Render grouped authorized links with React Router `NavLink`; keep an explicit SaaS Admin anchor.
6. Add responsive sidebar and active-link styles.
7. Run Dashboard unit tests, lint and typecheck.
8. Commit as `feat: render authorized dashboard navigation`.

### Task 3: Restore the login page presentation

**Files:**
- Modify: `web/apps/dashboard/src/features/auth/login-page.test.tsx`
- Modify: `web/apps/dashboard/src/features/auth/login-page.tsx`
- Modify: `web/apps/dashboard/src/styles/index.css`

Steps:

1. Add a failing test for a named login region, branded heading and styled Ant inputs/button.
2. Preserve the React Hook Form and Zod submit flow while rendering Ant Design presentation components.
3. Add desktop and mobile login styles without external image assets.
4. Run login tests and the complete Dashboard suite.
5. Commit as `feat: complete dashboard login presentation`.

### Task 4: Build, deploy, and close Phase 1

**Files:**
- Modify: `docs/phases/phase-1-frontend-foundation/README.md`
- Modify: `docs/phases/phase-1-frontend-foundation/verification/FRONTEND_PHASE1_VERIFICATION.zh-CN.md`
- Modify: `docs/PROJECT_PROGRESS.zh-CN.md`

Steps:

1. Run frontend tests, lint, typecheck and both builds.
2. Rebuild the standalone Compose app with the corrected Dockerfile.
3. Verify health and distinct Dashboard/SaaS Admin HTML and assets.
4. Browser-test login, authorized navigation and `/saas-admin/`.
5. Record commands, results and known unrelated failures.
6. Mark Phase 1 complete and point the total progress document at Phase 2 design.
7. Commit as `docs: close frontend foundation phase`.

