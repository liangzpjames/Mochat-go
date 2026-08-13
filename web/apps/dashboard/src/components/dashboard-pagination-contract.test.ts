import { readdirSync, readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

function productionTSXFiles(root: string): string[] {
  return readdirSync(root, { withFileTypes: true }).flatMap((entry) => {
    const path = join(root, entry.name);
    if (entry.isDirectory()) return productionTSXFiles(path);
    if (!entry.name.endsWith(".tsx") || entry.name.endsWith(".test.tsx")) return [];
    return [path];
  });
}

describe("Dashboard pagination contract", () => {
  it("routes every production pagination control through DashboardPagination", () => {
    const src = join(dirname(fileURLToPath(import.meta.url)), "..");
    const violations = productionTSXFiles(src)
      .filter((path) => !path.endsWith("dashboard-pagination.tsx"))
      .flatMap((path) => {
        const source = readFileSync(path, "utf8");
        const reasons = [
          />\s*上一页\s*</.test(source) ? "literal previous button" : "",
          />\s*下一页\s*</.test(source) ? "literal next button" : "",
          /pagination=\{\{/.test(source) ? "Ant Table pagination" : "",
          /className=["'][^"']*(?:benchmark-demo-pagination|conversation-global-pagination|employee-conversation-pagination|company-website-pagination)/.test(source)
            ? "legacy pagination class"
            : "",
        ].filter(Boolean);
        return reasons.map((reason) => `${path.replace(src, "src")}: ${reason}`);
      });
    expect(violations).toEqual([]);
  });
});
