# Phase 2 Metadata and Dashboard Batch 2 Baseline Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove owner maintenance from the frontend audit, baseline every legacy page with a conservative risk and batch recommendation, and publish the ordered Dashboard Batch 2 route candidates.

**Architecture:** The generated inventory keeps discovered page facts, while `page-metadata-overrides.csv` stores only `status`, `risk`, and `batch` decisions keyed by `app`, `source_file`, and `route`. A deterministic initializer adds recommendations only for keys missing from the override file, never rewrites existing decisions, and the audit derives both the unbaselined-page gate and the Dashboard Batch 2 candidate report from the merged inventory.

**Tech Stack:** Node.js ESM, `node:test`, CSV audit artifacts, npm scripts

## Global Constraints

- Delete `owner` from generated pages, metadata overrides, reports, tests, and documentation.
- Existing override decisions must survive refresh and recommendation initialization byte-for-byte.
- Recommendation initialization may append missing page keys but must never alter an existing `status`, `risk`, or `batch`.
- Conservative defaults are required: uncertain Dashboard pages go to `dashboard-batch4`, not Batch 2.
- Pages requiring real external-platform evidence remain legacy and use `blocked-external`.
- Only routed Dashboard pages may appear in the Batch 2 candidate report.
- No runtime dependency may be added.

---

### Task 1: Remove owner from the page metadata contract

**Files:**
- Modify: `scripts/audit_legacy_frontend_inventory.mjs`
- Modify: `scripts/audit_legacy_frontend_inventory.test.mjs`
- Modify: `docs/phases/phase-1-frontend-foundation/audit/pages.csv`
- Modify: `docs/phases/phase-2-frontend-migration/audit/page-metadata-overrides.csv`
- Modify: `docs/phases/phase-2-frontend-migration/audit/unassigned-pages.csv`

**Interfaces:**
- Produces page schema: `app,source_file,route,status,risk,batch`
- Produces override schema: `app,source_file,route,status,risk,batch`
- Produces blocking report schema: `app,source_file,route,status,risk,batch,blocking_fields`

- [ ] **Step 1: Write failing schema and refresh-preservation tests**

Change the fixture constants to:

```js
const pageMetadataOverrideColumns = ['app', 'source_file', 'route', 'status', 'risk', 'batch'];
const columns = {
  'pages.csv': 'app,source_file,route,status,risk,batch',
  // existing non-page schemas remain unchanged
};
```

Change the fixture override to:

```csv
app,source_file,route,status,risk,batch
dashboard,web/legacy/dashboard/src/views/example/index.vue,/example,candidate,low,phase2-batch2
```

Update the preservation assertion to compare:

```js
assert.deepEqual(
  { status: page.status, risk: page.risk, batch: page.batch },
  { status: 'candidate', risk: 'low', batch: 'phase2-batch2' },
);
```

Add a test named `page schemas do not contain owner` that asserts all three committed headers omit `owner`.

- [ ] **Step 2: Run focused tests and verify RED**

Run:

```bash
node --test --test-name-pattern="page schemas do not contain owner|page metadata override" scripts/audit_legacy_frontend_inventory.test.mjs
```

Expected: FAIL because production schemas still require `owner`.

- [ ] **Step 3: Remove owner from production schemas and merge logic**

Use:

```js
const pageMetadataOverrideColumns = ['app', 'source_file', 'route', 'status', 'risk', 'batch'];

const specifications = {
  'pages.csv': {
    columns: ['app', 'source_file', 'route', 'status', 'risk', 'batch'],
    key: (row) => `${row.app}:${row.source_file}:${row.route}`,
    sourceColumns: ['source_file'],
  },
  // existing specifications remain unchanged
};
```

Generated pages become:

```js
'pages.csv': found.pages.map((item) => ({
  ...item,
  route: item.route ?? '-',
  status: 'legacy',
  risk: 'unassigned',
  batch: 'unassigned',
})),
```

Override values become:

```js
overrides.set(key, { status: row.status, risk: row.risk, batch: row.batch });
```

The blocking report selects and labels:

```js
function unassignedPages(pages) {
  return pages
    .filter((page) => page.risk === 'unassigned' || page.batch === 'unassigned')
    .map((page) => ({
      ...page,
      blocking_fields: [
        page.risk === 'unassigned' ? 'risk' : '',
        page.batch === 'unassigned' ? 'batch' : '',
      ].filter(Boolean).join(';'),
    }));
}
```

Allow `unassigned` only in generated/merged page rows; override rows must use `low`, `medium`, `high`, or `critical`.

- [ ] **Step 4: Refresh artifacts and run the complete inventory test**

Run:

```bash
npm run refresh:audit
node --test scripts/audit_legacy_frontend_inventory.test.mjs
```

Expected: inventory tests PASS; the refresh command may still expose only the separately recorded Windows legacy manifest hash baseline.

- [ ] **Step 5: Commit the owner-free schema**

```bash
git add scripts/audit_legacy_frontend_inventory.mjs scripts/audit_legacy_frontend_inventory.test.mjs docs/phases/phase-1-frontend-foundation/audit/pages.csv docs/phases/phase-2-frontend-migration/audit/page-metadata-overrides.csv docs/phases/phase-2-frontend-migration/audit/unassigned-pages.csv
git commit -m "refactor: remove page owner metadata"
```

### Task 2: Initialize conservative risk and batch recommendations

**Files:**
- Modify: `scripts/audit_legacy_frontend_inventory.mjs`
- Modify: `scripts/audit_legacy_frontend_inventory.test.mjs`
- Modify: `package.json`
- Modify: `docs/phases/phase-2-frontend-migration/audit/page-metadata-overrides.csv`

**Interfaces:**
- Produces: `recommendedPageMetadata(page): {status: 'legacy', risk: 'low' | 'medium' | 'high' | 'critical', batch: string}`
- Produces CLI: `node scripts/audit_legacy_frontend_inventory.mjs --initialize-page-metadata --check`
- Produces npm script: `initialize:page-metadata`

- [ ] **Step 1: Write failing recommendation classification tests**

Add table-driven tests for:

```js
const recommendations = [
  ['/contactField/index', 'medium', 'dashboard-batch2'],
  ['/department/index', 'medium', 'dashboard-batch2'],
  ['/menu/index', 'medium', 'dashboard-batch2'],
  ['/passwordUpdate/index', 'medium', 'dashboard-batch2'],
  ['/role/index', 'medium', 'dashboard-batch2'],
  ['/user/index', 'medium', 'dashboard-batch2'],
  ['/workContactTag/index', 'medium', 'dashboard-batch2'],
  ['/workEmployee/index', 'medium', 'dashboard-batch2'],
  ['/channelCode/index', 'high', 'dashboard-batch3'],
  ['/statistics/contact', 'high', 'dashboard-batch3'],
  ['/autoTag/dayPartCreate', 'high', 'dashboard-batch4'],
  ['/contactTransfer/workIndex', 'high', 'dashboard-batch4'],
  ['/officialAccount/index', 'critical', 'blocked-external'],
];
```

Also create Sidebar and Operation fixture pages and assert `high,sidebar` and `high,operation`.

- [ ] **Step 2: Run the classification tests and verify RED**

Run:

```bash
node --test --test-name-pattern="recommends conservative page metadata" scripts/audit_legacy_frontend_inventory.test.mjs
```

Expected: FAIL because the initializer and recommendation rules do not exist.

- [ ] **Step 3: Implement explicit allowlists and conservative fallbacks**

Implement:

```js
const dashboardBatch2Routes = new Set([
  '/contactField/index',
  '/department/index',
  '/menu/index',
  '/passwordUpdate/index',
  '/role/index',
  '/user/index',
  '/workContactTag/index',
  '/workEmployee/index',
]);

const dashboardBatch3Prefixes = [
  '/channelCode/',
  '/corpData/',
  '/greeting/',
  '/mediumGroup/',
  '/radar/',
  '/statistics/',
];

const blockedExternalPrefixes = [
  '/officialAccount/',
];

function recommendedPageMetadata(page) {
  if (page.app === 'sidebar') return { status: 'legacy', risk: 'high', batch: 'sidebar' };
  if (page.app === 'operation') return { status: 'legacy', risk: 'high', batch: 'operation' };
  if (blockedExternalPrefixes.some((prefix) => page.route.startsWith(prefix))) {
    return { status: 'legacy', risk: 'critical', batch: 'blocked-external' };
  }
  if (dashboardBatch2Routes.has(page.route)) {
    return { status: 'legacy', risk: 'medium', batch: 'dashboard-batch2' };
  }
  if (dashboardBatch3Prefixes.some((prefix) => page.route.startsWith(prefix))) {
    return { status: 'legacy', risk: 'high', batch: 'dashboard-batch3' };
  }
  return { status: 'legacy', risk: 'high', batch: 'dashboard-batch4' };
}
```

For non-routed component pages (`route === '-'`), classify by the nearest feature directory using the same prefixes after deriving a synthetic `/<feature>/` value from `source_file`.

- [ ] **Step 4: Write a failing no-overwrite initializer test**

The test must:

1. Keep the fixture `/example` override at `candidate,low,phase2-batch2`.
2. Add an unassigned `/department/index` page.
3. Run initialization twice.
4. Assert `/example` remains byte-identical.
5. Assert `/department/index` is appended once as `legacy,medium,dashboard-batch2`.

- [ ] **Step 5: Implement append-only initialization**

Add:

```js
function initializePageMetadata(root) {
  const pages = generatedArtifacts(root).inventory['pages.csv'];
  const errors = [];
  const overrides = readPageMetadataOverrides(root, pages, errors);
  if (errors.length) throw new Error(errors.join('\n'));
  const existingRows = parseCsv(readFileSync(rootFile(root, pageMetadataOverrideFile), 'utf8')).slice(1);
  const additions = pages
    .filter((page) => !overrides.has(specifications['pages.csv'].key(page)))
    .map((page) => ({ ...page, ...recommendedPageMetadata(page) }))
    .sort((left, right) => specifications['pages.csv'].key(left).localeCompare(specifications['pages.csv'].key(right)));
  const existing = existingRows.map((values) => Object.fromEntries(pageMetadataOverrideColumns.map((column, index) => [column, values[index] ?? ''])));
  writeFileSync(rootFile(root, pageMetadataOverrideFile), csvWithColumns(pageMetadataOverrideColumns, [...existing, ...additions]));
}
```

Run initialization before refresh when `--initialize-page-metadata` is present. Never call it implicitly from `--refresh`.

- [ ] **Step 6: Initialize the repository baseline and verify counts**

Run:

```bash
npm run initialize:page-metadata
npm run refresh:audit
```

Expected:

- `page-metadata-overrides.csv` contains 135 data rows.
- `/corp/index` remains `react,medium,phase1-batch1`.
- `unassigned-pages.csv` contains only its header.

- [ ] **Step 7: Commit the recommendation baseline**

```bash
git add package.json scripts/audit_legacy_frontend_inventory.mjs scripts/audit_legacy_frontend_inventory.test.mjs docs/phases/phase-2-frontend-migration/audit/page-metadata-overrides.csv docs/phases/phase-2-frontend-migration/audit/unassigned-pages.csv
git commit -m "feat: baseline phase2 page recommendations"
```

### Task 3: Publish ordered Dashboard Batch 2 candidates

**Files:**
- Create: `docs/phases/phase-2-frontend-migration/audit/dashboard-batch2-candidates.csv`
- Modify: `scripts/audit_legacy_frontend_inventory.mjs`
- Modify: `scripts/audit_legacy_frontend_inventory.test.mjs`
- Modify: `docs/phases/phase-2-frontend-migration/README.md`

**Interfaces:**
- Produces report schema: `order,route,source_file,risk,status,api_contract_count,go_evidence_count`
- Includes only `app=dashboard`, `batch=dashboard-batch2`, and routed pages

- [ ] **Step 1: Write a failing candidate report test**

Create fixture pages for `/department/index` and `/role/index`, initialize metadata, refresh, and assert the report:

```csv
order,route,source_file,risk,status,api_contract_count,go_evidence_count
1,/department/index,web/legacy/dashboard/src/views/department/index.vue,medium,legacy,0,0
2,/role/index,web/legacy/dashboard/src/views/role/index.vue,medium,legacy,0,0
```

Also assert `route=-`, `/login`, `/404`, and non-Dashboard pages are absent.

- [ ] **Step 2: Run the candidate test and verify RED**

Run:

```bash
node --test --test-name-pattern="Dashboard Batch 2 candidate report" scripts/audit_legacy_frontend_inventory.test.mjs
```

Expected: FAIL because the candidate report is not generated.

- [ ] **Step 3: Implement deterministic candidate ordering**

Build API counts from `apis.csv` by matching each candidate's top-level route feature to API paths. Sort by:

1. `risk`: low before medium.
2. Missing Go evidence count: zero before non-zero.
3. API contract count: fewer before more.
4. Route lexical order.

Exclude `-`, `*`, `/`, `/404`, and `/login`. Serialize the report on refresh and compare it during audit using normalized line endings.

- [ ] **Step 4: Refresh the repository report and document the first wave**

Run:

```bash
npm run refresh:audit
```

Expected first-wave candidates are the eight explicit Batch 2 routes:

```text
/contactField/index
/department/index
/menu/index
/passwordUpdate/index
/role/index
/user/index
/workContactTag/index
/workEmployee/index
```

Update the Phase 2 README:

```markdown
- 单人开发模式已删除 owner 元数据。
- 135 条页面已完成风险与批次基线。
- Dashboard Batch 2 首批候选见 [候选顺序](audit/dashboard-batch2-candidates.csv)。
- 下一项实施从候选报告第 1 条路由开始，逐路由通过交付门禁后切换 manifest。
```

- [ ] **Step 5: Run final verification**

Run:

```bash
npm test
node --test scripts/audit_legacy_frontend_inventory.test.mjs
git diff --check
```

Expected: all tests PASS and no whitespace errors. Run the audit separately and confirm it has no errors beyond the recorded Windows legacy manifest hash baseline.

- [ ] **Step 6: Commit the Batch 2 ordering**

```bash
git add scripts/audit_legacy_frontend_inventory.mjs scripts/audit_legacy_frontend_inventory.test.mjs docs/phases/phase-2-frontend-migration/README.md docs/phases/phase-2-frontend-migration/audit/dashboard-batch2-candidates.csv
git commit -m "docs: publish dashboard batch2 candidates"
```

