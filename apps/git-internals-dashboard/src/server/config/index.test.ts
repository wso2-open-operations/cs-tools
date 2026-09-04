// src/server/config/index.test.ts
import { mkdtempSync, writeFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { __resetConfigCacheForTests, loadConfig } from "./index";

const VALID_YAML = `
repos:
  - owner: acme
    name: widgets
    githubProjectId: PVT_test123
    projectTitle: Widgets
    issueQuery: 'label:"Origin/CS"'
taxonomy:
  statuses:
    - { name: "Open", category: PRODUCT_SIDE, accruesSla: true }
budgets:
  - { priority: "Critical(P1)", budgetHours: 24, coverage: "24x7", rank: 1 }
`;

describe("loadConfig", () => {
  let dir: string;

  beforeEach(() => {
    __resetConfigCacheForTests();
    dir = mkdtempSync(join(tmpdir(), "sla-config-test-"));
  });

  afterEach(() => {
    __resetConfigCacheForTests();
    delete process.env.SLA_CONFIG_PATH;
    rmSync(dir, { recursive: true, force: true });
  });

  it("SLA_CONFIG_PATH overrides the default location", () => {
    const path = join(dir, "custom.yaml");
    writeFileSync(path, VALID_YAML, "utf8");
    process.env.SLA_CONFIG_PATH = path;

    const cfg = loadConfig();
    expect(cfg.repos[0].owner).toBe("acme");
  });

  it("throws a descriptive error when the file is missing", () => {
    process.env.SLA_CONFIG_PATH = join(dir, "does-not-exist.yaml");
    expect(() => loadConfig()).toThrow(/SLA config not found at/);
  });

  it("memoizes the parsed config until reset", () => {
    const path = join(dir, "custom.yaml");
    writeFileSync(path, VALID_YAML, "utf8");
    process.env.SLA_CONFIG_PATH = path;

    const first = loadConfig();
    writeFileSync(path, VALID_YAML.replace("Widgets", "Gadgets"), "utf8");
    const second = loadConfig();
    expect(second).toBe(first); // memoized — the file change is not picked up

    __resetConfigCacheForTests();
    const third = loadConfig();
    expect(third.repos[0].projectTitle).toBe("Gadgets");
  });
});
