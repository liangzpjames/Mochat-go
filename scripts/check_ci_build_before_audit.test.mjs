import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

test("MySQL 5.7 CI builds frontend before audit-based quick gate", async () => {
  const workflow = await readFile(new URL("../.github/workflows/mysql57-amd64.yml", import.meta.url), "utf8");
  const build = workflow.indexOf("- name: Frontend build gate");
  const quick = workflow.indexOf("- name: Frontend quick gate");
  assert.notEqual(build, -1);
  assert.notEqual(quick, -1);
  assert.ok(build < quick, "frontend build must precede the quick audit gate");
});

test("migration lifecycle gate is named after the discovered registry, not a stale version", async () => {
  const workflow = await readFile(new URL("../.github/workflows/mysql57-amd64.yml", import.meta.url), "utf8");
  assert.match(workflow, /Migration registry lifecycle gate/);
  assert.doesNotMatch(workflow, /Migration 0098 lifecycle gate/);
});
