// src/server/db/ingest.test.ts
// Fabricated Pair fixtures, no network. Exercises the real Prisma client
// against the local Postgres (docker-compose) instance.
import type { AppConfig } from "@/server/config";
import { afterAll, afterEach, beforeAll, describe, expect, it } from "vitest";
import { prisma } from "./client";
import { ingestIssuePair, type IngestContext, type Pair } from "./ingest";
import { slaConfigFromFile } from "./sla-config";

const TEST_APP: AppConfig = {
  repos: [
    {
      owner: "test-owner",
      name: "test-repo-ingest",
      githubProjectId: "PVT_test_ingest",
      projectTitle: "Test Project",
      issueQuery: 'label:"Origin/CS"',
    },
  ],
  taxonomy: {
    statuses: [
      { name: "Open", category: "PRODUCT_SIDE", accruesSla: true, isTerminal: false, sortOrder: 10 },
      { name: "In Progress", category: "PRODUCT_SIDE", accruesSla: true, isTerminal: false, sortOrder: 20 },
      { name: "Resolved", category: "OTHER", accruesSla: false, isTerminal: true, sortOrder: 30 },
    ],
    aliases: [],
  },
  budgets: [{ priority: "Critical(P1)", budgetHours: 24, coverage: "24x7", rank: 1 }],
  settings: {
    possibleThreshold: 0.75,
    recomputeIntervalMinutes: 10,
    syncOverlapMinutes: 15,
    snapshotHourUtc: 0,
    seedSnapshotDays: 90,
    seedClosedLookbackDays: 90,
  },
};

function pair(withLeadingEvent: boolean): Pair {
  return {
    node: {
      number: 42,
      state: "OPEN",
      url: "https://github.com/test-owner/test-repo-ingest/issues/42",
      createdAt: "2026-01-01T00:00:00.000Z",
      updatedAt: "2026-01-10T12:00:00.000Z",
      closedAt: null,
      labels: { nodes: [{ name: "Priority/Critical(P1)" }] },
    },
    detail: {
      number: 42,
      events: withLeadingEvent
        ? [{ createdAt: "2026-01-05T00:00:00.000Z", previousStatus: "Open", status: "In Progress" }]
        : [],
      projectStatuses: [
        {
          projectId: "PVT_test_ingest",
          status: "In Progress",
          statusUpdatedAt: "2026-01-05T00:00:00.000Z",
          itemCreatedAt: "2026-01-01T00:00:00.000Z",
        },
      ],
    },
  };
}

describe("ingestIssuePair", () => {
  const runtime = slaConfigFromFile(TEST_APP);
  const now = new Date("2026-01-10T12:00:00.000Z");
  let repositoryId: number;
  let projectId: number;

  beforeAll(async () => {
    const project = await prisma.project.create({
      data: { githubProjectId: "PVT_test_ingest", title: "Test Project", enabled: true },
    });
    projectId = project.id;
    const repository = await prisma.repository.create({
      data: {
        owner: "test-owner",
        name: "test-repo-ingest",
        issueQuery: 'label:"Origin/CS"',
        slaProjectId: projectId,
        enabled: true,
      },
    });
    repositoryId = repository.id;
  });

  afterEach(async () => {
    await prisma.slaSnapshot.deleteMany({ where: { repositoryId } });
    await prisma.issueSla.deleteMany({ where: { issue: { repositoryId } } });
    await prisma.issueStatusEvent.deleteMany({ where: { issue: { repositoryId } } });
    await prisma.issue.deleteMany({ where: { repositoryId } });
  });

  afterAll(async () => {
    await prisma.repository.delete({ where: { id: repositoryId } });
    await prisma.project.delete({ where: { id: projectId } });
    await prisma.$disconnect();
  });

  function ctx(source: "github" | "synthetic" = "github"): IngestContext {
    return {
      repositoryId,
      slaProjectId: projectId,
      repo: { owner: "test-owner", name: "test-repo-ingest", githubProjectId: "PVT_test_ingest" },
      runtime,
      source,
      now,
    };
  }

  it("is idempotent across repeated runs — same issue row, zero new events, IssueSla not duplicated", async () => {
    const p = pair(true);

    const first = await ingestIssuePair(p, ctx());
    expect(first.created).toBe(true);
    expect(first.eventsInserted).toBe(2); // leading "derived" + the one real event

    const second = await ingestIssuePair(p, ctx());
    expect(second.created).toBe(false);
    expect(second.issueId).toBe(first.issueId);
    expect(second.eventsInserted).toBe(0); // dedupe — no new rows

    expect(await prisma.issue.count({ where: { repositoryId } })).toBe(1);
    expect(await prisma.issueSla.count({ where: { issueId: first.issueId } })).toBe(1);
    expect(await prisma.issueStatusEvent.count({ where: { issueId: first.issueId } })).toBe(2);
  });

  it("creates the leading derived event only when the guard conditions hold, with a stable dedupeKey", async () => {
    const p = pair(true);

    const result = await ingestIssuePair(p, ctx());
    const events = await prisma.issueStatusEvent.findMany({
      where: { issueId: result.issueId },
      orderBy: { occurredAt: "asc" },
    });

    expect(events).toHaveLength(2);
    expect(events[0].source).toBe("derived");
    expect(events[0].previousStatus).toBeNull();
    expect(events[0].status).toBe("Open"); // firstEvent.previousStatus
    expect(events[1].source).toBe("github");

    const firstDedupeKey = events[0].dedupeKey;

    // Re-ingesting must reproduce the identical dedupeKey, not a new row.
    await ingestIssuePair(p, ctx());
    const eventsAfter = await prisma.issueStatusEvent.findMany({
      where: { issueId: result.issueId },
      orderBy: { occurredAt: "asc" },
    });
    expect(eventsAfter).toHaveLength(2);
    expect(eventsAfter[0].dedupeKey).toBe(firstDedupeKey);
  });

  it("does not synthesize a leading event when the timeline is empty", async () => {
    const result = await ingestIssuePair(pair(false), ctx());
    expect(result.eventsInserted).toBe(0);
    expect(await prisma.issueStatusEvent.count({ where: { issueId: result.issueId } })).toBe(0);
  });
});
