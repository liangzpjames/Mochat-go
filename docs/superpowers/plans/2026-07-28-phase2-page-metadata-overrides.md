# Phase 2 Page Metadata Overrides Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Preserve manually governed page migration metadata across legacy inventory refreshes and publish a deterministic report of pages that still block Phase 2 batch planning.

**Architecture:** Keep discovered page facts in the existing generator, move human decisions into a separate CSV keyed by `app`, `source_file`, and `route`, and merge those decisions only when materializing `pages.csv`. Validate the override file independently so malformed, duplicate, or stale decisions fail the audit instead of disappearing silently; derive the unassigned-page report from the merged page inventory.

**Tech Stack:** Node.js ESM, `node:test`, CSV audit artifacts, npm scripts

## Global Constraints

- Generated facts and manual decisions must remain in separate files.
- `--refresh` may update discovered page facts but must never overwrite `status`, `owner`, `risk`, or `batch` values stored in the override file.
- A page is identified by the existing `pages.csv` key: `app:source_file:route`.
- No new runtime dependency is allowed; reuse the script's CSV parser and serializer.
- The existing `pages.csv` schema remains `app,source_file,route,status,owner,risk,batch`.
- Phase 2 progress remains unpublished while any page has `owner=unassigned` or `batch=unassigned`.

---

### Task 1: Define and validate the page metadata override boundary

**Files:**
- Create: `docs/phases/phase-2-frontend-migration/audit/page-metadata-overrides.csv`
- Modify: `scripts/audit_legacy_frontend_inventory.mjs`
- Test: `scripts/audit_legacy_frontend_inventory.test.mjs`

**Interfaces:**
- Consumes: discovered page rows keyed by `app`, `source_file`, and `route`
- Produces: `readPageMetadataOverrides(root, pages, errors): Map<string, {status: string, owner: string, risk: string, batch: string}>`
- Produces: CSV schema `app,source_file,route,status,owner,risk,batch`

- [ ] **Step 1: Extend the fixture with a valid override file and add failing schema tests**

Add the Phase 2 audit path and fixture row:

```js
const pageMetadataOverrideFile = 'docs/phases/phase-2-frontend-migration/audit/page-metadata-overrides.csv';
const pageMetadataOverrideColumns = ['app', 'source_file', 'route', 'status', 'owner', 'risk', 'batch'];
```

In `createFixture()`, write:

```js
write(
  root,
  pageMetadataOverrideFile,
  `${pageMetadataOverrideColumns.join(',')}\ndashboard,${view},/example,candidate,frontend-platform,low,phase2-batch2\n`,
);
```

Add tests that replace the header with an invalid header, duplicate the fixture row, change `/example` to `/missing`, and set `risk` to `urgent`. Each audit must fail with these respective messages:

```text
frontend-audit: page-metadata-overrides.csv:1: columns must be app,source_file,route,status,owner,risk,batch
frontend-audit: page-metadata-overrides.csv:3: duplicate key dashboard:web/legacy/dashboard/src/views/example/index.vue:/example
frontend-audit: page-metadata-overrides.csv:2: override does not match a discovered page
frontend-audit: page-metadata-overrides.csv:2: risk must be low, medium, high, or critical
```

- [ ] **Step 2: Run the focused tests and verify the new tests fail**

Run:

```bash
node --test --test-name-pattern="page metadata override" scripts/audit_legacy_frontend_inventory.test.mjs
```

Expected: FAIL because the audit does not read or validate `page-metadata-overrides.csv`.

- [ ] **Step 3: Implement strict override parsing and validation**

Add `pageMetadataOverrideFile`, `pageMetadataOverrideColumns`, and:

```js
function readPageMetadataOverrides(root, pages, errors) {
  if (!fileExists(root, pageMetadataOverrideFile)) {
    errors.push('frontend-audit: page-metadata-overrides.csv:1: file is required');
    return new Map();
  }
  const rows = parseCsv(readFileSync(rootFile(root, pageMetadataOverrideFile), 'utf8'));
  const header = rows.shift() ?? [];
  if (header.join(',') !== pageMetadataOverrideColumns.join(',')) {
    errors.push(`frontend-audit: page-metadata-overrides.csv:1: columns must be ${pageMetadataOverrideColumns.join(',')}`);
    return new Map();
  }
  const discovered = new Set(pages.map(specifications['pages.csv'].key));
  const overrides = new Map();
  rows.forEach((values, index) => {
    const rowNumber = index + 2;
    const row = Object.fromEntries(pageMetadataOverrideColumns.map((column, position) => [column, (values[position] ?? '').trim()]));
    if (values.length !== pageMetadataOverrideColumns.length) errors.push(`frontend-audit: page-metadata-overrides.csv:${rowNumber}: expected ${pageMetadataOverrideColumns.length} columns`);
    for (const column of pageMetadataOverrideColumns) if (!row[column]) errors.push(`frontend-audit: page-metadata-overrides.csv:${rowNumber}: ${column} is required`);
    const key = specifications['pages.csv'].key(row);
    if (overrides.has(key)) errors.push(`frontend-audit: page-metadata-overrides.csv:${rowNumber}: duplicate key ${key}`);
    if (!discovered.has(key)) errors.push(`frontend-audit: page-metadata-overrides.csv:${rowNumber}: override does not match a discovered page`);
    if (!['legacy', 'candidate', 'react', 'blocked'].includes(row.status)) errors.push(`frontend-audit: page-metadata-overrides.csv:${rowNumber}: status must be legacy, candidate, react, or blocked`);
    if (!['low', 'medium', 'high', 'critical'].includes(row.risk)) errors.push(`frontend-audit: page-metadata-overrides.csv:${rowNumber}: risk must be low, medium, high, or critical`);
    overrides.set(key, { status: row.status, owner: row.owner, risk: row.risk, batch: row.batch });
  });
  return overrides;
}
```

Call this function from both `audit()` and `refresh()` using the discovered page rows. The initial committed file contains only its header, so all 134 unresolved pages continue to use generated defaults until owners approve entries.

- [ ] **Step 4: Run the focused tests and the existing audit suite**

Run:

```bash
node --test --test-name-pattern="page metadata override" scripts/audit_legacy_frontend_inventory.test.mjs
npm run check:audit
```

Expected: both commands PASS.

- [ ] **Step 5: Commit the validation boundary**

```bash
git add scripts/audit_legacy_frontend_inventory.mjs scripts/audit_legacy_frontend_inventory.test.mjs docs/phases/phase-2-frontend-migration/audit/page-metadata-overrides.csv
git commit -m "feat: validate page metadata overrides"
```

### Task 2: Merge manual page decisions without refresh overwrite

**Files:**
- Modify: `scripts/audit_legacy_frontend_inventory.mjs`
- Test: `scripts/audit_legacy_frontend_inventory.test.mjs`

**Interfaces:**
- Consumes: `readPageMetadataOverrides(...)`
- Produces: `mergePageMetadata(pages, overrides): Array<PageRow>`
- Preserves: generated `app`, `source_file`, and `route`
- Overrides: manual `status`, `owner`, `risk`, and `batch`

- [ ] **Step 1: Add a failing refresh regression test**

Create a test named `page metadata override survives refresh while discovered facts still update`. It must:

1. Create the fixture, whose view override is `candidate,frontend-platform,low,phase2-batch2`.
2. Run `runRefresh(root)`.
3. Assert the materialized view row has the four override values.
4. Change the legacy route declaration from `/example` to `/renamed`.
5. Assert `runRefresh(root)` fails because the old override is stale.
6. Change the override route to `/renamed`, refresh again, and assert the new route fact appears while the four manual values remain unchanged.
7. Assert the override file content before and after the successful refresh is byte-for-byte identical.

Use:

```js
const before = readFileSync(join(root, pageMetadataOverrideFile), 'utf8');
runRefresh(root);
const after = readFileSync(join(root, pageMetadataOverrideFile), 'utf8');
assert.equal(after, before);
```

- [ ] **Step 2: Run the regression test and verify it fails**

Run:

```bash
node --test --test-name-pattern="survives refresh" scripts/audit_legacy_frontend_inventory.test.mjs
```

Expected: FAIL because generated defaults still replace the fixture's manual values.

- [ ] **Step 3: Implement the merge before materialization**

Add:

```js
function mergePageMetadata(pages, overrides) {
  return pages.map((page) => ({
    ...page,
    ...(overrides.get(specifications['pages.csv'].key(page)) ?? {}),
  }));
}
```

In `refresh(root)`, validate overrides before writing any artifact so a stale override makes refresh atomic with respect to audit CSV outputs:

```js
function refresh(root) {
  const artifacts = generatedArtifacts(root);
  const errors = [];
  const overrides = readPageMetadataOverrides(root, artifacts.inventory['pages.csv'], errors);
  if (errors.length) throw new Error(errors.join('\n'));
  const inventory = {
    ...artifacts.inventory,
    'pages.csv': mergePageMetadata(artifacts.inventory['pages.csv'], overrides),
  };
  // Existing writes continue here; never write pageMetadataOverrideFile.
}
```

In `audit(root)`, compare discovery coverage using generated rows, then validate the committed `pages.csv` against the same merged result so manual values are accepted but divergent values are rejected.

- [ ] **Step 4: Run focused and full inventory tests**

Run:

```bash
node --test --test-name-pattern="page metadata override|survives refresh" scripts/audit_legacy_frontend_inventory.test.mjs
node --test scripts/audit_legacy_frontend_inventory.test.mjs
npm run check:audit
```

Expected: all commands PASS and the repository audit still reports 135 pages.

- [ ] **Step 5: Commit refresh preservation**

```bash
git add scripts/audit_legacy_frontend_inventory.mjs scripts/audit_legacy_frontend_inventory.test.mjs
git commit -m "fix: preserve page decisions during audit refresh"
```

### Task 3: Publish the Phase 2 unassigned-page gate

**Files:**
- Create: `docs/phases/phase-2-frontend-migration/audit/unassigned-pages.csv`
- Modify: `scripts/audit_legacy_frontend_inventory.mjs`
- Modify: `docs/phases/phase-2-frontend-migration/README.md`
- Test: `scripts/audit_legacy_frontend_inventory.test.mjs`

**Interfaces:**
- Consumes: merged `pages.csv` rows
- Produces: `unassigned-pages.csv` with columns `app,source_file,route,status,owner,risk,batch,blocking_fields`
- Produces: one row for each page where `owner === 'unassigned' || batch === 'unassigned'`

- [ ] **Step 1: Add a failing deterministic-report test**

In the fixture, leave the router page unassigned and keep the view page assigned by the override. After refresh, assert:

```js
const rows = csvObjects(root, 'docs/phases/phase-2-frontend-migration/audit/unassigned-pages.csv');
assert.equal(rows.length, 1);
assert.equal(rows[0].source_file, 'web/legacy/dashboard/src/router/asyncRouter.js');
assert.equal(rows[0].blocking_fields, 'owner;batch');
```

Also assert a second refresh produces identical bytes.

- [ ] **Step 2: Run the report test and verify it fails**

Run:

```bash
node --test --test-name-pattern="unassigned page report" scripts/audit_legacy_frontend_inventory.test.mjs
```

Expected: FAIL because `unassigned-pages.csv` is not generated.

- [ ] **Step 3: Generate and audit the blocking report**

Add:

```js
const unassignedPageFile = 'docs/phases/phase-2-frontend-migration/audit/unassigned-pages.csv';
const unassignedPageColumns = [...specifications['pages.csv'].columns, 'blocking_fields'];

function unassignedPages(pages) {
  return pages
    .filter((page) => page.owner === 'unassigned' || page.batch === 'unassigned')
    .map((page) => ({
      ...page,
      blocking_fields: [
        page.owner === 'unassigned' ? 'owner' : '',
        page.batch === 'unassigned' ? 'batch' : '',
      ].filter(Boolean).join(';'),
    }));
}
```

During refresh, serialize the result with `csvWithColumns(unassignedPageColumns, unassignedPages(inventory['pages.csv']))`. During audit, compare the committed report byte-for-byte with the expected serialization and emit:

```text
frontend-audit: unassigned-pages.csv: report is stale; run npm run refresh:audit
```

Add the package script:

```json
"refresh:audit": "node scripts/audit_legacy_frontend_inventory.mjs --refresh --check"
```

- [ ] **Step 4: Refresh repository artifacts and document the gate**

Run:

```bash
npm run refresh:audit
```

Expected: PASS, `unassigned-pages.csv` contains exactly 134 data rows, and `page-metadata-overrides.csv` remains header-only.

Update the Phase 2 README state to:

```markdown
- page metadata override 与防覆盖测试已实现。
- 当前 134 条阻塞页面见 [未分配页面清单](audit/unassigned-pages.csv)。
- 在 owner、risk 和 batch 决策录入 override 文件前，不发布阶段完成比例，也不确定 Dashboard Batch 2 首批路由。
```

- [ ] **Step 5: Run the complete verification set**

Run:

```bash
node --test scripts/audit_legacy_frontend_inventory.test.mjs
npm run check:audit
git diff --check
```

Expected: all tests PASS, the audit reports 135 pages, and `git diff --check` emits no output.

- [ ] **Step 6: Commit the Phase 2 startup gate**

```bash
git add package.json scripts/audit_legacy_frontend_inventory.mjs scripts/audit_legacy_frontend_inventory.test.mjs docs/phases/phase-2-frontend-migration/README.md docs/phases/phase-2-frontend-migration/audit/unassigned-pages.csv
git commit -m "docs: publish phase2 page assignment gate"
```

