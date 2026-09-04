// src/server/lib/overview.test.ts
// F5/F6 regression: sparkQuery/productSideSparkQuery must exclude snapshot
// rows belonging to disabled repositories, same as every other query in this
// file. Real Postgres (see incremental.test.ts for the shared rationale).
import { afterEach, describe, expect, it } from "vitest";
import { prisma } from "@/server/db";
import { buildOverview } from "./overview";

describe("buildOverview — disabled repository exclusion (F5/F6 regression)", () => {
  const createdIssueIds: number[] = [];
  const createdRepoIds: number[] = [];
  const createdProjectIds: number[] = [];

  afterEach(async () => {
    for (const id of createdIssueIds.splice(0)) {
      await prisma.slaSnapshot.deleteMany({ where: { issueId: id } });
      await prisma.issueSla.deleteMany({ where: { issueId: id } });
      await prisma.issue.delete({ where: { id } }).catch(() => {});
    }
    for (const id of createdRepoIds.splice(0)) {
      await prisma.repository.delete({ where: { id } }).catch(() => {});
    }
    for (const id of createdProjectIds.splice(0)) {
      await prisma.project.delete({ where: { id } }).catch(() => {});
    }
  });

  it("never contributes a disabled repo's VIOLATED snapshot to the spark trend, filtered or not", async () => {
    const project = await prisma.project.create({
      data: { githubProjectId: "PVT_f5f6_test", title: "f5f6-test", enabled: false },
    });
    createdProjectIds.push(project.id);
    const repo = await prisma.repository.create({
      data: {
        owner: "f5f6-test-owner",
        name: "f5f6-test-repo",
        issueQuery: 'label:"Origin/CS"',
        slaProjectId: project.id,
        enabled: false, // decommissioned — the scenario from the finding
      },
    });
    createdRepoIds.push(repo.id);

    const before = await buildOverview(undefined, undefined);

    const issue = await prisma.issue.create({
      data: { repositoryId: repo.id, githubNumber: 1, state: "OPEN", priority: "Critical(P1)" },
    });
    createdIssueIds.push(issue.id);

    const today = new Date();
    today.setUTCHours(0, 0, 0, 0);
    await prisma.slaSnapshot.create({
      data: {
        snapshotDate: today,
        issueId: issue.id,
        repositoryId: repo.id,
        priority: "Critical(P1)",
        slaState: "VIOLATED",
      },
    });

    // Unfiltered load: today's violated-spark count must be unchanged by the
    // disabled repo's snapshot row (compares against a pre-fixture baseline
    // so this doesn't depend on what else is seeded in the dev DB).
    const after = await buildOverview(undefined, undefined);
    expect(after.hero.violated.spark[15]).toBe(before.hero.violated.spark[15]);

    // Explicit ?repo=<owner>/<name> for the disabled repo itself: pre-fix,
    // this returned the row directly regardless of `enabled`.
    const scoped = await buildOverview(`${repo.owner}/${repo.name}`, undefined);
    expect(scoped.hero.violated.spark[15]).toBe(0);
  });
});
