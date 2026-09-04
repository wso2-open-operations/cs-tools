// src/server/config/schema.test.ts
import { describe, expect, it } from "vitest";
import { AppConfigSchema } from "./schema";

function validFixture() {
  return {
    repos: [
      {
        owner: "wso2-enterprise",
        name: "wso2-iam-internal",
        githubProjectId: "PVT_kwDOATtEas4ADJ4n",
        projectTitle: "IAM Internals",
        issueQuery: 'label:"Origin/CS"',
      },
    ],
    taxonomy: {
      statuses: [
        { name: "Open", category: "PRODUCT_SIDE", accruesSla: true },
        { name: "Resolved", category: "OTHER", accruesSla: false, isTerminal: true },
      ],
      aliases: [{ alias: "Re-Opened", canonical: "Open" }],
    },
    budgets: [{ priority: "Critical(P1)", budgetHours: 24, coverage: "24x7", rank: 1 }],
  };
}

describe("AppConfigSchema", () => {
  it("parses a valid fixture and applies defaults", () => {
    const result = AppConfigSchema.safeParse(validFixture());
    expect(result.success).toBe(true);
    if (!result.success) return;
    expect(result.data.taxonomy.statuses[0].isTerminal).toBe(false);
    expect(result.data.taxonomy.statuses[0].sortOrder).toBe(0);
    expect(result.data.settings.possibleThreshold).toBe(0.75);
    expect(result.data.settings.recomputeIntervalMinutes).toBe(10);
    expect(result.data.settings.syncOverlapMinutes).toBe(15);
    expect(result.data.settings.snapshotHourUtc).toBe(0);
    expect(result.data.settings.seedSnapshotDays).toBe(90);
    expect(result.data.settings.seedClosedLookbackDays).toBe(90);
  });

  it("rejects duplicate repo owner/name", () => {
    const fixture = validFixture();
    fixture.repos.push({ ...fixture.repos[0] });
    const result = AppConfigSchema.safeParse(fixture);
    expect(result.success).toBe(false);
    if (result.success) return;
    expect(result.error.issues.some((i) => i.message.includes("duplicate repo"))).toBe(true);
  });

  it("rejects duplicate githubProjectId", () => {
    const fixture = validFixture();
    fixture.repos.push({ ...fixture.repos[0], owner: "other-owner", name: "other-name" });
    const result = AppConfigSchema.safeParse(fixture);
    expect(result.success).toBe(false);
    if (result.success) return;
    expect(result.error.issues.some((i) => i.message.includes("duplicate githubProjectId"))).toBe(true);
  });

  it("rejects an alias mapping to an unknown canonical status", () => {
    const fixture = validFixture();
    fixture.taxonomy.aliases.push({ alias: "Weird", canonical: "Nonexistent Status" });
    const result = AppConfigSchema.safeParse(fixture);
    expect(result.success).toBe(false);
    if (result.success) return;
    expect(result.error.issues.some((i) => i.message.includes('unknown canonical "Nonexistent Status"'))).toBe(true);
  });

  it("rejects a bad coverage value", () => {
    const fixture = validFixture();
    fixture.budgets[0].coverage = "9x5_utc";
    const result = AppConfigSchema.safeParse(fixture);
    expect(result.success).toBe(false);
  });

  it("rejects an empty repos array", () => {
    const fixture = validFixture();
    fixture.repos = [];
    const result = AppConfigSchema.safeParse(fixture);
    expect(result.success).toBe(false);
  });
});
