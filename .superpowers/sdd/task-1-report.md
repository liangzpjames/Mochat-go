# Phase 1 Task 1 report

## Result

- Commit: `511e610b2f78578386828ed4133d197e3830d3d6` (`docs: restore and audit legacy frontends`)
- Source ref: `origin/backup/pre-phase0-main-20260723` at `3dcd216c188df34f2c3ed489b8e8b9473e635488`
- Restored reference trees: `web/legacy/dashboard`, `web/legacy/sidebar`, and `web/legacy/operation`.
- `web/legacy/api-server` does not exist; PHP paths cited in `apis.csv` and this report were read only from the source ref.
- `SOURCE_MANIFEST.sha256` has 393 entries (all legacy files plus the legacy README, excluding the self-referential manifest).

## Files

- Added the exact legacy tree under `web/legacy/{dashboard,sidebar,operation}` and the required provenance README and SHA-256 manifest.
- Added all six matrices under `docs/handle/frontend-audit/`: pages, routes, APIs, permissions, assets, and dependencies.
- Added `licenses.md`, including the repository GPL-3.0 text, the NUWA Apache-2.0 text, dependency-verification procedure, and blocked/unproven images and fonts.
- Added `scripts/audit_legacy_frontend_inventory.mjs` and its Node test suite.

## Commands and evidence

1. Baseline/source checks:

   ```powershell
   git rev-parse --verify origin/backup/pre-phase0-main-20260723
   git status --short
   Test-Path web/legacy
   ```

   Result: source SHA was `3dcd216c188df34f2c3ed489b8e8b9473e635488`; only `.superpowers/` was untracked; `web/legacy` was `False`.

2. Restore:

   ```powershell
   git archive --format=tar --output=$archive origin/backup/pre-phase0-main-20260723 dashboard sidebar operation
   tar -xf $archive -C web/legacy
   ```

   Result: only the three required frontend directories were restored.

3. RED, before the checker existed:

   ```powershell
   node --test scripts/audit_legacy_frontend_inventory.test.mjs
   ```

   Result: 0/5 passed and 5/5 failed as expected. Every fixture received Node's `MODULE_NOT_FOUND` for `scripts/audit_legacy_frontend_inventory.mjs`, rather than the required `frontend-audit:` diagnostic. Fixtures cover an orphan route, API method/path omissions, missing dependency decision, empty asset license status, and a missing manifest source.

4. GREEN/final task-specific verification:

   ```powershell
   node --test scripts/audit_legacy_frontend_inventory.test.mjs
   node scripts/audit_legacy_frontend_inventory.mjs --check
   git diff --check
   ```

   Result: Node TAP passed 5/5 tests; audit output was `frontend-audit: ok (15 pages, 15 routes, 14 apis)`; and `git diff --check` exited 0.

5. Staged review:

   ```powershell
   git diff --cached --name-only
   ```

   Result: 403 staged files; zero paths containing `api-server`; the manifest had 393 lines. A further fresh `node scripts/audit_legacy_frontend_inventory.mjs --check` passed immediately before commit.

## Self-review

- CSV headers exactly match the task's six required schemas.
- The checker validates headers, required fields, duplicate keys, referenced-source existence, page/route and permission/route references, allowed page statuses, manifest file existence, and manifest SHA-256 values. Its errors use the required `frontend-audit: <file>:<row>: <message>` form.
- The audit tracks concrete router, API, permission/request-client, view, and Go/PHP comparison evidence from each legacy frontend. No page is marked `react`.
- All unproven legacy artwork is marked `blocked` in the inventory/license notes, preventing implicit reuse in React.

## Concerns

- The archive preserves existing trailing whitespace and blank-final-line diagnostics in a few legacy source files. They were not modified because doing so would invalidate the source-exact SHA-256 baseline. The original pre-staging `git diff --check` exit 0 covered only tracked changes and did **not** verify the untracked archive patch; the full staged/base-to-head archive diff reports inherited diagnostics. The Fix Review records the scoped verification that is actually claimed.
- The normal Windows `go test ./...` path/file-mode baseline exception was not run because it is explicitly outside this task; all task-specific Node/audit checks passed.

## Fix Review

### Commit and scope

- Fix commit: `deddbaadefa16e6b94dd9748add04bb494b3edda` (`fix: complete legacy frontend audit`).
- The audit now discovers and requires every static route declaration in `src/router/**/*.js`, API `url`/`method` declaration in `src/api/*.js`, `src/views/**/*.{vue,jsx}` page, `src/assets/**` or `src/static/**` asset, and runtime dependency in each legacy `package.json`.
- The regenerated matrices contain 221 pages, 89 routes, 373 unique API contracts, 75 assets, and 51 dependencies. This includes `/user/index`, `GET /workRoomGroup/index`, and all 33 Dashboard runtime dependencies.
- `go_evidence` now contains either a checked existing Go path under `internal/server`, `internal/dashboard`, or `internal/store`, or `-` when no matching Go evidence exists. PHP paths were removed; PHP remains read-only comparison material only.

### RED/GREEN evidence

1. RED: after adding the seven new fixture cases, `node --test scripts/audit_legacy_frontend_inventory.test.mjs` reported 5 pass / 7 fail. The missing checker behavior was explicit: router/API/dependency/page/asset omissions either exited 0 or did not produce the required `discovered … is missing` diagnostic; manifest missing/duplicate entries also exited 0.
2. GREEN: after scanner and manifest implementation, `node --test scripts/audit_legacy_frontend_inventory.test.mjs` passed 13/13. It includes deletion/omission cases for every discovered route, API, dependency, page, and asset; missing and duplicate manifest entries; and a clean `git archive` SHA-256 comparison with pinned source commit `3dcd216c188df34f2c3ed489b8e8b9473e635488`.

### Canonical source verification

- Added root `.gitattributes` entries forcing `-text` for the immutable source trees, preventing platform EOL conversion.
- Reconciled every restored source index mode with `git ls-tree -r origin/backup/pre-phase0-main-20260723 dashboard sidebar operation`; exact mode comparison result: `source_mode_mismatches=0`.
- `SOURCE_MANIFEST.sha256` now has 392 one-to-one source entries (the three archived frontend trees only). The checker rejects a missing entry, a duplicate entry, an unknown entry, a missing file, or a hash mismatch.

### Verification commands and exact results

```powershell
node --test scripts/audit_legacy_frontend_inventory.test.mjs
node scripts/audit_legacy_frontend_inventory.mjs --check
```

Result: 13/13 Node tests passed; audit printed `frontend-audit: ok (221 pages, 89 routes, 373 apis)`.

```powershell
git diff --check 511e610b2f78578386828ed4133d197e3830d3d6..deddbaadefa16e6b94dd9748add04bb494b3edda -- . ':(exclude)web/legacy/dashboard/**' ':(exclude)web/legacy/sidebar/**' ':(exclude)web/legacy/operation/**'
```

Result: exit 0. This is the intentionally scoped whitespace verification for task-authored files. The full base-to-head diff is **not** claimed clean: it reports inherited whitespace diagnostics inside immutable legacy source files, which are excluded so their canonical archive bytes and hashes remain intact.

## Second Fix Review

### Commit and semantic inventory

- Fix commit: `40e357c3f448492d56df5a0d5870b08d72a86db1` (`fix: derive semantic legacy audit evidence`).
- The refreshed audit has 137 view-backed pages, 89 routes, and 373 API contracts. Route records now use legacy client evidence (`ACCESS_TOKEN`, `Bearer cookie token`, or `no-auth-header`), mounted client scope, and the actual render target. `/user/index` is named `user` and maps to `web/legacy/dashboard/src/views/user/index.vue`.
- APIs record their request shape as `data:<expression>`, `params:<expression>`, or `none`, their real `response.data` client unwrapping, client auth, and mounted app prefix. Dependencies use an explicit React replacement or `no-direct-react-replacement` with `retire-or-reassess`, not a placeholder. Asset usage is resolved to legacy source files or explicitly marked `unreferenced-in-legacy-source` after a source scan.
- Permission actions are derived from component directives. For example `/corp/index` records `addwx;check;edit;search` from its `v-permission` directives.
- `go_evidence` now requires method plus full mounted route evidence. Ten contracts lack a Go match and remain `-`; all are listed as blocked migration gaps in `docs/handle/frontend-audit/go-evidence-gaps.md`, including the sidebar room calendar, room quality, and `PUT /sidebar/roomSop/setRoom` contracts. They no longer cite dashboard evidence.

### RED/GREEN and canonical sources

1. RED: the semantic fixture initially failed because refresh output mapped the example view to `-` and emitted generated placeholders. The first committed-HEAD canonical-source test then failed with four blob mismatches: dashboard `src/router/asyncRouter.js`, dashboard `src/views/chatTool/enhance.vue`, operation `src/router/index.js`, and sidebar `src/router/routes.js`.
2. GREEN: semantic extraction now passes the fixture, and the four files were restored with their exact pinned tree blobs/modes after `.gitattributes` became active. The committed-HEAD test passes.

### Final verification

```powershell
node --test scripts/audit_legacy_frontend_inventory.test.mjs
node scripts/audit_legacy_frontend_inventory.mjs --check
```

Result: 15/15 Node tests passed; audit printed `frontend-audit: ok (137 pages, 89 routes, 373 apis)`.

```powershell
# compare every legacy source mode/blob in HEAD with the pinned source tree
# and run the scoped whitespace check
git diff --check deddbaadefa16e6b94dd9748add04bb494b3edda..HEAD -- . ':(exclude)web/legacy/dashboard/**' ':(exclude)web/legacy/sidebar/**' ':(exclude)web/legacy/operation/**'
```

Result: `head_source_blob_mode_mismatches=0`; the scoped diff check exited 0; no `TBD`, `TODO`, or `unknown` matches were found in `docs/handle/frontend-audit` or `web/legacy/README.md`. The full immutable-source whitespace exception remains unchanged and is not claimed clean.

## Third review blocked assessment

No semantic-field commit was made for the third review because completing it safely requires source-of-truth decisions not present in the repository.

- Of 373 contracts, 366 legacy API exports only forward an opaque `params` or `data` wrapper. The exported function does not declare its object properties; reliable fields require a per-export call-site/data-flow analysis across Vue components and dynamic form state.
- All 373 current response entries are client transport unwrapping (`response.data`), not response schemas. Although 363 contracts have a matching Go registration/test path, those matches generally prove method/path dispatch only, not a response-field contract. A field list requires mapping every endpoint to its concrete Go handler result type or the pinned PHP controller/serializer, then reviewing wrapper conventions.
- Tenant scope is mixed: some calls carry `corpId`/`corp_id`, while many rely on the authenticated enterprise resolved server-side. Neither API declarations nor route mounts identify which behavior applies to each endpoint. This needs a reviewed scope taxonomy and handler/controller mapping before it can be represented honestly.

The remaining deterministic work is: (1) establish a reviewed endpoint-to-Go/PHP contract map; (2) define accepted field-schema and tenant-scope evidence rules; (3) add a per-export/call-site payload resolver; (4) regenerate and review the 373 rows; and (5) then add the requested route/auth, object-boundary, and wrong-method regressions against those decisions. The current report retains the previous exact source-byte, mode, gap, and scoped-whitespace verification evidence.

## Third Fix Review A

### Request, route, and Go contract evidence

- API request discovery now reads the direct `data` or `params` property from each exported request configuration object and resolves finite call-site payload evidence, including direct/computed objects, destructured parameters, local variables, Vue model roots, assignments, helper returns, and `FormData.append`.
- The refreshed `/corp/store` contract is `data:contactSecret;corpName;employeeSecret;wxCorpId`, derived from the `wechatDetail` call root and its four root-specific bindings. No edit-only fields leak into the create contract.
- The refreshed inventory contains zero forbidden generic `data:params`, `params:params`, or bare `none` request values. Unresolved evidence is emitted with an explicit `blocked[...]` reason, and `--check` recomputes `request_fields` rather than comparing API keys alone.
- Route discovery is bounded to direct properties of the owning route object. Sidebar navigation objects no longer shadow declarations in `src/router/routes.js`; Dashboard `/login` and Sidebar `/`, `/login`, `/auth`, and `/codeAuth` are public, while protected routes retain their guard-backed token requirement. Operation `/`, `/speed`, and `/explain` have the direct declaration name `-`.
- Go evidence validation requires the HTTP method and full mounted route in the same exact contract string, switch case, request constructor, or keyed table entry. A wrong-method fixture proves that an unrelated method token elsewhere in the file is rejected.
- `response_fields` and `corp_scope` were intentionally left unchanged.

### RED/GREEN evidence

1. RED: after adding the five focused regressions, `node --test scripts/audit_legacy_frontend_inventory.test.mjs` reported 15 pass / 5 fail. The failures covered request transport/property derivation, rejection of a generic request expression, route-object boundaries and guard auth, wrong-method Go evidence, and the updated semantic fixture.
2. GREEN: after the bounded parser fixes, the same command passed 20/20 tests.

### Final verification

```powershell
node --test scripts/audit_legacy_frontend_inventory.test.mjs
node scripts/audit_legacy_frontend_inventory.mjs --check
```

Result: 20/20 Node tests passed; the audit printed `frontend-audit: ok (135 pages, 89 routes, 373 apis)`.

The refreshed CSV inspection reported:

- `forbidden_generic_request_fields=0`
- `response_field_changes=0`
- `corp_scope_changes=0`
- `/corp/store=data:contactSecret;corpName;employeeSecret;wxCorpId`

The complete pinned-source tree comparison reported `pinned_entries=392`, `head_entries=392`, `blob_mode_mismatches=0`, and `working_tree_changes=0`. The scoped whitespace check excludes the immutable legacy trees, preserving the inherited source-byte exception documented above.

## Third Fix Review B

### Semantic response and enterprise-scope contracts

- Replaced every transport placeholder `response.data` with either finite `fields:<name;...>` evidence or an explicit `blocked[reason]@evidence` value. The refreshed census is 120 concrete and 253 blocked response contracts; no blanket transport value remains.
- Replaced `/dashboard`, `/sidebar`, and `/operation` scope placeholders with `explicit:<field>@evidence`, `server-current-enterprise@evidence`, `public-unscoped@evidence`, or `blocked[reason]@evidence`. The bounded census is 8 explicit, 79 server-current, 24 public/unscoped, and 262 blocked contracts.
- Added `api-contract-evidence.csv`, a one-to-one 373-row endpoint map across legacy declarations, current Go route evidence, pinned-PHP handlers read only with `git show`, and client consumers. Added `api-contract-gaps.csv`, which machine-validates every blocked response/scope reason and evidence reference; it has 515 gap rows.
- Endpoint aliases are reconciled before a `no-callsite` conclusion, including `/officialAccount/index`, `/roomWelcome/update`, and `/user/statusUpdate`. `/workMessage/toUsers` remains blocked because both declarations genuinely have no consumer. Three leading-slash alias pairs now share mounted-endpoint evidence while the required 373 raw inventory rows remain intact.
- Go contract matching strips comments before checking method/path constructs. Client response extraction excludes array operations such as `.map()`, and destructured request parameters contribute only fields that flow into the request payload.

### RED/GREEN evidence

1. RED: the first semantic regressions reported 20 pass / 5 fail, covering alias reconciliation, semantic response/scope generation, blanket-placeholder rejection, and the missing gap register.
2. RED: focused consumer regressions exposed `.map()` as a false response field and client `corpIds` as a false explicit override of server-current handler scope.
3. RED: a mounted leading-slash alias fixture reported 26 pass / 1 fail before normalized endpoint evidence was merged.
4. GREEN: the final Node suite passes all semantic, negative-comment, payload, route, and inventory regressions.

### Verification

`node scripts/audit_legacy_frontend_inventory.mjs --refresh --check` prints `frontend-audit: ok (135 pages, 89 routes, 373 apis)`. Placeholder scans report zero `response.data` values, zero mount-prefix scope values, and zero `TBD`, `TODO`, or `unknown` markers. The canonical 392-entry source blob/mode comparison and scoped non-legacy whitespace check remain required immediately before commit.
