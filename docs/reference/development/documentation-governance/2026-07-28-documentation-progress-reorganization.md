# Documentation Progress Reorganization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reorganize all project documentation by development phase, create one authoritative project progress reference, remove redundant artifacts, preserve valid evidence, and push the verified result to `origin/main`.

**Architecture:** `docs/PROJECT_PROGRESS.zh-CN.md` becomes the project-wide progress source of truth. Each directory under `docs/phases/` owns one phase and exposes its state through `README.md`; reusable material moves to `docs/reference/`, while superseded but historically valuable material moves to `docs/archive/`. Repository scripts and tests move with machine-readable evidence paths so no static reference is left broken.

**Tech Stack:** Markdown, CSV, JSON, PowerShell, Node.js, shell scripts, Git.

## Global Constraints

- Preserve final effective evidence, audit matrices, and compliance-relevant history.
- Remove duplicate `* 2.*` artifacts, zero-byte temporary logs, obsolete candidate snapshots, and session placeholders that add no independent information.
- Update every code or script reference before removing an old path.
- Do not change product code, database behavior, deployment behavior, or existing phase completion conclusions.
- Do not modify, delete, or commit the user-owned `.workbuddy/` and `web/saas-admin/` untracked directories.
- Commit to the current `main` branch and push only verified changes to `origin/main`.

---

### Task 1: Establish the Documentation Integrity Baseline

**Files:**
- Create: `scripts/check_documentation_structure.mjs`
- Create: `scripts/check_documentation_structure.test.mjs`
- Modify: `package.json`

**Interfaces:**
- Consumes: repository root and the final directory contract from `docs/superpowers/specs/2026-07-28-documentation-progress-reorganization-design.md`.
- Produces: `node scripts/check_documentation_structure.mjs`, returning exit code `0` only when required entry files exist, no forbidden duplicate/zero-byte artifacts remain, and Markdown links resolve.

- [ ] **Step 1: Write failing tests for the target structure**

Create tests using `node:test` that build temporary `docs/` fixtures and assert:

```js
test('requires the project progress entry and phase readmes', () => {
  const result = checkDocumentation(tempRoot);
  assert.deepEqual(result.missingRequired, [
    'docs/PROJECT_PROGRESS.zh-CN.md',
    'docs/phases/phase-pre0-standalone/README.md',
    'docs/phases/phase-0-go-foundation/README.md',
    'docs/phases/phase-1-frontend-foundation/README.md',
    'docs/phases/phase-2-frontend-migration/README.md',
  ]);
});

test('rejects duplicate and zero-byte evidence artifacts', () => {
  const result = checkDocumentation(tempRoot);
  assert.deepEqual(result.forbiddenArtifacts.sort(), [
    'docs/evidence/production/current/env 2.production-evidence',
    'docs/evidence/production/current/production-evidence.log',
  ]);
});

test('reports broken relative markdown links', () => {
  const result = checkDocumentation(tempRoot);
  assert.deepEqual(result.brokenLinks, [{
    file: 'docs/README.md',
    target: 'phases/missing/README.md',
  }]);
});
```

- [ ] **Step 2: Run the tests and verify RED**

Run:

```powershell
node --test scripts/check_documentation_structure.test.mjs
```

Expected: FAIL because `check_documentation_structure.mjs` and `checkDocumentation` do not exist.

- [ ] **Step 3: Implement the minimal structure checker**

Implement `checkDocumentation(root)` and a CLI entry that:

- checks the five required progress/phase entry files;
- flags files with a basename containing ` 2.`;
- flags zero-byte files under `docs/`;
- parses relative Markdown links and verifies their targets;
- ignores `http:`, `https:`, anchors, and image data URLs;
- prints grouped failures and exits `1`, otherwise prints `documentation structure: ok`.

Add:

```json
"docs:check": "node scripts/check_documentation_structure.mjs"
```

to root `package.json`.

- [ ] **Step 4: Run the tests and verify GREEN**

Run:

```powershell
node --test scripts/check_documentation_structure.test.mjs
```

Expected: all tests pass.

- [ ] **Step 5: Run the checker against the current repository**

Run:

```powershell
pnpm docs:check
```

Expected: FAIL because the phase entry files do not exist and duplicate/zero-byte evidence files remain. Save the failure categories for comparison after Task 5.

- [ ] **Step 6: Commit**

```powershell
git add package.json scripts/check_documentation_structure.mjs scripts/check_documentation_structure.test.mjs
git commit -m "test: add documentation structure gate"
```

---

### Task 2: Create the Project and Phase Progress Sources of Truth

**Files:**
- Create: `docs/README.md`
- Create: `docs/PROJECT_PROGRESS.zh-CN.md`
- Create: `docs/phases/phase-pre0-standalone/README.md`
- Create: `docs/phases/phase-0-go-foundation/README.md`
- Create: `docs/phases/phase-1-frontend-foundation/README.md`
- Create: `docs/phases/phase-2-frontend-migration/README.md`

**Interfaces:**
- Consumes: the phase design, existing plans, verification reports, `web/apps/dashboard/src/migration-routes.json`, and Git history.
- Produces: stable human entry points used by the structure checker and all later documentation.

- [ ] **Step 1: Write `docs/README.md`**

The entry document must link to `PROJECT_PROGRESS.zh-CN.md`, every phase README, `reference/`, and `archive/`, and explain:

```text
总进度只在 PROJECT_PROGRESS.zh-CN.md 维护；
阶段事实只在对应 phase README 维护；
证据文件不作为进度入口；
新实施开始前必须更新当前阶段的“实施前状态”。
```

- [ ] **Step 2: Write the total progress document**

`PROJECT_PROGRESS.zh-CN.md` must record:

- current stage: Phase 1 corrective work before Phase 2;
- Phase Pre-0 and Phase 0 as completed with evidence links;
- Phase 1 foundation as technically completed but product usability corrective work still open;
- Phase 2 as not started and not baseline-percented;
- actual frontend state: one React business route (`/corp/index`), 60 legacy Dashboard routes, Sidebar and Operation not migrated;
- current P0 issue: SaaS Admin is absent from the Docker image and `/saas-admin/` falls through to Dashboard;
- exact next task: repair SaaS Admin Docker publishing, redeploy, then baseline Phase 2 page metadata.

- [ ] **Step 3: Write each phase README**

Every phase README must contain these exact sections:

```markdown
## 阶段状态
## 实施前状态
## 阶段目标
## 已完成
## 未完成与阻塞
## 验收与证据
## 下一阶段入口
```

Use Git commits and existing verification reports as evidence. Do not invent completion percentages where no task baseline exists.

- [ ] **Step 4: Validate entry links**

Run:

```powershell
node scripts/check_documentation_structure.mjs
```

Expected: required-entry failures disappear; duplicate/zero-byte and broken-link failures may remain until later tasks.

- [ ] **Step 5: Commit**

```powershell
git add docs/README.md docs/PROJECT_PROGRESS.zh-CN.md docs/phases
git commit -m "docs: add phased project progress index"
```

---

### Task 3: Move Authoritative Documents Into Phase and Reference Directories

**Files:**
- Move Phase Pre-0 documents from `docs/*.md` and `docs/evidence/{latest,production}`.
- Move Phase 0 documents from `docs/handle/`.
- Move Phase 1 plans, verification, audit, evidence, and Runbook from `docs/handle/` and `docs/evidence/frontend-phase1/`.
- Move reusable architecture, development, product, and protocol documents into `docs/reference/`.

**Interfaces:**
- Consumes: the existing tracked files without modifying their substantive historical contents.
- Produces: phase-owned paths referenced by the new progress indexes.

- [ ] **Step 1: Move Phase Pre-0 authoritative documents**

Use `git mv` for:

```text
docs/release-candidate.md
  -> docs/phases/phase-pre0-standalone/plans/release-candidate.md
docs/standalone-gap.md
  -> docs/phases/phase-pre0-standalone/reports/standalone-gap.md
docs/current-stage-report.md
  -> docs/phases/phase-pre0-standalone/reports/final-stage-report.md
docs/production-evidence.md
  -> docs/phases/phase-pre0-standalone/evidence/production-evidence.md
docs/production-evidence.json
  -> docs/phases/phase-pre0-standalone/evidence/production-evidence.json
docs/evidence/latest/
  -> docs/phases/phase-pre0-standalone/evidence/latest/
docs/evidence/production/
  -> docs/phases/phase-pre0-standalone/evidence/production/
```

- [ ] **Step 2: Move Phase 0 documents**

Move:

```text
2026-07-23-phase0-architecture-design.md -> design/
2026-07-23-phase0-implementation-plan.md -> plans/
PHASE0_CHECKLIST.zh-CN.md -> verification/
PHASE0_VERIFICATION.zh-CN.md -> verification/
REQUIREMENT_REGISTER.zh-CN.md -> registers/
RISK_REGISTER.zh-CN.md -> registers/
DEFECT_REGISTER.zh-CN.md -> registers/
RESOURCE_AND_SECRET_INVENTORY.zh-CN.md -> registers/
PRODUCTION_EVIDENCE_REGISTER.zh-CN.md -> registers/
```

under `docs/phases/phase-0-go-foundation/`.

- [ ] **Step 3: Move Phase 1 documents**

Move:

```text
docs/handle/2026-07-27-frontend-unification-foundation-implementation-plan.md
  -> docs/phases/phase-1-frontend-foundation/plans/
docs/handle/FRONTEND_PHASE1_VERIFICATION.zh-CN.md
  -> docs/phases/phase-1-frontend-foundation/verification/
docs/handle/FRONTEND_MIGRATION_RUNBOOK.zh-CN.md
  -> docs/phases/phase-1-frontend-foundation/plans/
docs/handle/frontend-audit/
  -> docs/phases/phase-1-frontend-foundation/audit/
docs/evidence/frontend-phase1/
  -> docs/phases/phase-1-frontend-foundation/evidence/
docs/superpowers/specs/2026-07-28-saas-admin-docker-publishing-design.md
  -> docs/phases/phase-1-frontend-foundation/plans/
```

- [ ] **Step 4: Move reusable references**

Move:

```text
docs/PROJECT_HANDOVER_GUIDE.zh-CN.md
  -> docs/reference/architecture/PROJECT_HANDOVER_GUIDE.zh-CN.md
docs/handle/ARCHITECTURE.zh-CN.md
  -> docs/reference/architecture/ARCHITECTURE.zh-CN.md
docs/handle/LOCAL_DEVELOPMENT.zh-CN.md
  -> docs/reference/development/LOCAL_DEVELOPMENT.zh-CN.md
docs/saas-admin-mvp-scope.md
  -> docs/reference/product/saas-admin-mvp-scope.md
docs/payment-settlement-bridge.md
  -> docs/reference/protocols/payment-settlement-bridge.md
```

- [ ] **Step 5: Review moves before any deletion**

Run:

```powershell
git status --short -- docs
git diff --summary -- docs
```

Expected: moves are detected as renames where content is unchanged; no untracked user directory appears.

- [ ] **Step 6: Commit**

```powershell
git add docs
git commit -m "docs: organize authoritative files by phase"
```

---

### Task 4: Update Repository Paths and Audit Tests

**Files:**
- Modify: `README.md`
- Modify: `MODIFICATIONS.md`
- Modify: `deploy/standalone/README.md`
- Modify: `scripts/audit_queue_annotation_coverage.sh`
- Modify: `scripts/audit_wework_callback_event_coverage.sh`
- Modify: `scripts/audit_legacy_frontend_inventory.mjs`
- Modify: `scripts/audit_legacy_frontend_inventory.test.mjs`
- Modify: `scripts/production_evidence_check.sh`

**Interfaces:**
- Consumes: new paths created in Task 3.
- Produces: repository scripts and documentation that no longer reference removed locations.

- [ ] **Step 1: Change the audit directory constant and test fixtures**

Replace every exact occurrence of:

```text
docs/handle/frontend-audit
```

with:

```text
docs/phases/phase-1-frontend-foundation/audit
```

in both the inventory implementation and its tests.

- [ ] **Step 2: Change standalone report paths**

Update:

```text
docs/standalone-gap.md
```

to:

```text
docs/phases/phase-pre0-standalone/reports/standalone-gap.md
```

in both audit shell scripts, `README.md`, and `MODIFICATIONS.md`.

- [ ] **Step 3: Update product, protocol, candidate, and stage-report links**

Change repository links to:

```text
docs/reference/product/saas-admin-mvp-scope.md
docs/reference/protocols/payment-settlement-bridge.md
docs/phases/phase-pre0-standalone/plans/release-candidate.md
docs/phases/phase-pre0-standalone/reports/final-stage-report.md
```

Preserve generated evidence-pack filenames inside an evidence directory; only the repository-owned default and documentation links move.

- [ ] **Step 4: Update production evidence defaults**

Change the documented repository output defaults in `scripts/production_evidence_check.sh` to:

```text
docs/phases/phase-pre0-standalone/evidence/production-evidence.md
docs/phases/phase-pre0-standalone/evidence/production-evidence.json
```

Do not change temporary output filenames used by evidence-pack scripts.

- [ ] **Step 5: Run targeted tests**

Run:

```powershell
node --test scripts/audit_legacy_frontend_inventory.test.mjs
node --test scripts/check_documentation_structure.test.mjs
```

Expected: both suites pass.

- [ ] **Step 6: Check for stale paths**

Run:

```powershell
rg -n "docs/(handle|standalone-gap\.md|release-candidate\.md|current-stage-report\.md|payment-settlement-bridge\.md|saas-admin-mvp-scope\.md)" README.md MODIFICATIONS.md deploy scripts .github
```

Expected: no stale tracked path references. References to filenames inside generated evidence-pack directories are allowed only when prefixed by a runtime output variable such as `$OUT_DIR`.

- [ ] **Step 7: Commit**

```powershell
git add README.md MODIFICATIONS.md deploy/standalone/README.md scripts
git commit -m "chore: update phased documentation paths"
```

---

### Task 5: Consolidate Session State and Remove Redundant Artifacts

**Files:**
- Delete after merging: `docs/handle/CURRENT_SESSION_HANDOFF.zh-CN.md`
- Delete after replacing: `docs/handle/README.md`
- Delete: duplicate `* 2.*` files under the moved production evidence tree.
- Delete: zero-byte log and snapshot files under the moved evidence tree.
- Archive only if independently valuable: superseded candidate summaries.

**Interfaces:**
- Consumes: phase README files from Task 2 and moved evidence from Task 3.
- Produces: a documentation tree with no duplicate or empty tracked artifacts.

- [ ] **Step 1: Merge handoff facts into Phase 1 and Phase 2**

Verify these facts are present before deleting the handoff:

```text
Phase 1 Task 1–10 status and verification commit;
1 React and 60 legacy Dashboard routes;
known real-account and production-evidence blockers;
bundle-size risk;
the page metadata override requirement before Dashboard batch 2.
```

- [ ] **Step 2: Resolve duplicate evidence pairs**

For every basename containing ` 2`, compare SHA-256 hashes or normalized content with its canonical counterpart. Delete the duplicate when identical or when the canonical file is the later complete version. If content differs materially, merge its unique conclusion into the canonical index before deletion.

- [ ] **Step 3: Delete zero-byte transient files**

Delete zero-byte `.log` and `.txt` files that contain no evidence. Keep non-empty status files and machine-readable results.

- [ ] **Step 4: Delete obsolete handle entry files**

Delete `docs/handle/CURRENT_SESSION_HANDOFF.zh-CN.md` and `docs/handle/README.md` only after their useful content and links are represented by the project and phase indexes. Remove the empty `docs/handle/` directory naturally after all moves.

- [ ] **Step 5: Run the structure gate**

Run:

```powershell
pnpm docs:check
```

Expected:

```text
documentation structure: ok
```

- [ ] **Step 6: Review the exact deletion set**

Run:

```powershell
git diff --name-status --diff-filter=D
git diff --check
```

Expected: only reviewed duplicate, zero-byte, or superseded documentation files are deleted; no product source is deleted.

- [ ] **Step 7: Commit**

```powershell
git add docs
git commit -m "docs: remove superseded documentation artifacts"
```

---

### Task 6: Final Verification and Git Synchronization

**Files:**
- Modify if needed: `docs/PROJECT_PROGRESS.zh-CN.md`
- Modify if needed: phase `README.md` files.

**Interfaces:**
- Consumes: the complete reorganized documentation tree.
- Produces: a verified `main` commit synchronized to `origin/main`.

- [ ] **Step 1: Reconcile progress facts with source**

Run:

```powershell
$manifest = Get-Content -Raw web/apps/dashboard/src/migration-routes.json | ConvertFrom-Json
$manifest | Group-Object target | Select-Object Name,Count
git log -10 --oneline
```

Expected: the progress document matches the actual route counts and relevant commits.

- [ ] **Step 2: Run complete documentation verification**

Run:

```powershell
pnpm docs:check
node --test scripts/check_documentation_structure.test.mjs
node --test scripts/audit_legacy_frontend_inventory.test.mjs
git diff --check
```

Expected: every command exits `0`.

- [ ] **Step 3: Verify repository state**

Run:

```powershell
git status --short
git log --oneline --decorate -8
```

Expected: only the pre-existing untracked `.workbuddy/` and `web/saas-admin/` remain; documentation work is committed.

- [ ] **Step 4: Push**

Run:

```powershell
git push origin main
```

Expected: push succeeds and `origin/main` points to the final documentation commit.

- [ ] **Step 5: Record synchronization**

Update `docs/PROJECT_PROGRESS.zh-CN.md` with the final documentation commit hash only if doing so does not create a self-referential commit loop. Otherwise report the pushed hash in the task handoff.
