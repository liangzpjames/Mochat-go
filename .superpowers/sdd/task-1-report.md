# Task 1 report: Yuanhu benchmark manifest

## Status

Completed. The repository now has a versioned Yuanhu benchmark manifest, an index-page observation archive, a non-mutating checker, focused Node tests, and a root pnpm command.

## Commit

- `25528bf25550568520bdf11d77f875492cbdd24f` — `docs: add yuanhu benchmark manifest`

## Files changed

- `docs/benchmark/yuanhu/manifest.json`
- `docs/benchmark/yuanhu/pages/index/spec.md`
- `docs/benchmark/yuanhu/pages/index/states.json`
- `scripts/check_yuanhu_benchmark_manifest.mjs`
- `scripts/check_yuanhu_benchmark_manifest.test.mjs`
- `package.json`

## Commands and results

1. `node --test scripts/check_yuanhu_benchmark_manifest.test.mjs`
   - Initial RED result: failed with `ERR_MODULE_NOT_FOUND` because `check_yuanhu_benchmark_manifest.mjs` did not yet exist.
   - Final GREEN result: 3 tests passed, 0 failed.
2. `pnpm check:yuanhu-benchmark`
   - Passed: `Yuanhu benchmark manifest passed (53 pages).`
3. `node -e "..."`
   - Confirmed 8 groups and 53 pages: P0 3, P1 9, P2 41.
4. `git diff --check`
   - Passed with no whitespace errors.

## Concerns

- The observed feature matrix currently records 53 routes, while the task brief describes “about 56”. The manifest deliberately uses the repository’s complete observed matrix rather than inventing three unobserved routes.
- No screenshot was recorded for this task. Every page uses `screenshotVersion: "unobserved"`; the index archive explicitly marks deep states as unobserved instead of inferring behavior.

## Self-review

- The manifest is the sole route, group, level, status, and screenshot-version source introduced by this task.
- The checker rejects missing required groups, duplicate group IDs, duplicate page paths, missing required page fields, invalid levels, unresolved group references, and a missing `/index` route. It only reads and validates; it never changes the manifest.
- Tests independently assert the eight required groups, the `/index` page, valid path/level records, and duplicate-route rejection.

## Follow-up repair: require an ungrouped index page

### Status

Completed. `/index` is now required to use `groupId: null`, matching its role as the top-level data overview rather than an item in one of the eight navigation groups.

### Commit

- `d8de3df063e75bebed430634ea09703359848d76` — `fix: require ungrouped Yuanhu index page`

### Files changed

- `scripts/check_yuanhu_benchmark_manifest.mjs`
- `scripts/check_yuanhu_benchmark_manifest.test.mjs`

### TDD evidence and verification

1. Added a regression test which clones the `/index` page and changes `groupId` to `conversation`.
2. Ran `node --test scripts/check_yuanhu_benchmark_manifest.test.mjs` before the checker change.
   - RED: 3 passed and the new fourth test failed with `Missing expected exception.`
3. Added the minimal checker rule: an `/index` page with any `groupId` other than `null` produces `index page must have groupId: null`.
4. Ran `node --test scripts/check_yuanhu_benchmark_manifest.test.mjs`.
   - GREEN: 4 passed, 0 failed.
5. Ran `pnpm check:yuanhu-benchmark`.
   - Passed: `Yuanhu benchmark manifest passed (53 pages).`
6. Ran `git diff --check`.
   - Passed with no whitespace errors.

### Follow-up self-review

- The valid manifest still demonstrates the passing case because its `/index` record has `groupId: null`.
- The regression test verifies the failing case by assigning `/index` to an actual navigation group.
- The checker emits a dedicated error rather than silently treating `/index` as an ordinary grouped page.
