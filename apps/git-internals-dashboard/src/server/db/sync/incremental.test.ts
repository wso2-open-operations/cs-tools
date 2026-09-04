// src/server/db/sync/incremental.test.ts
// GitHub fetchers mocked (no network); real Postgres for the ingest side.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { prisma } from "../client";
import type { GhIssueDetail, GhIssueNode } from "../github/client";

vi.mock("../github/client", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../github/client")>();
  return { ...actual, searchAll: vi.fn(), fetchIssueDetail: vi.fn() };
});

import { fetchIssueDetail, searchAll } from "../github/client";
import { runIncrementalSync, SyncTokenMissingError } from "./incremental";

const searchAllMock = vi.mocked(searchAll);
const fetchIssueDetailMock = vi.mocked(fetchIssueDetail);

const OWNER = "test-sync";

function node(number: number): GhIssueNode {
  return {
    number,
    state: "OPEN",
    url: `https://github.com/${OWNER}/repo/issues/${number}`,
    createdAt: "2026-01-01T00:00:00.000Z",
    updatedAt: "2026-01-05T00:00:00.000Z",
    closedAt: null,
    labels: { nodes: [{ name: "Priority/Critical(P1)" }] },
  };
}

function detail(number: number, projectId: string): GhIssueDetail {
  return {
    number,
    events: [{ createdAt: "2026-01-02T00:00:00.000Z", previousStatus: "Open", status: "In Progress" }],
    projectStatuses: [
      {
        projectId,
        status: "In Progress",
        statusUpdatedAt: "2026-01-02T00:00:00.000Z",
        itemCreatedAt: "2026-01-01T00:00:00.000Z",
      },
    ],
  };
}

async function makeRepo(name: string, githubProjectId: string, lastSyncedAt: Date | null) {
  const project = await prisma.project.create({ data: { githubProjectId, title: name, enabled: true } });
  const repo = await prisma.repository.create({
    data: {
      owner: OWNER,
      name,
      issueQuery: 'label:"Origin/CS"',
      slaProjectId: project.id,
      enabled: true,
      lastSyncedAt,
    },
  });
  return { project, repo };
}

describe("runIncrementalSync", () => {
  const originalToken = process.env.SEED_GITHUB_TOKEN;
  const createdRepoIds: number[] = [];
  const createdProjectIds: number[] = [];

  beforeEach(() => {
    process.env.SEED_GITHUB_TOKEN = "test-token";
    searchAllMock.mockReset();
    fetchIssueDetailMock.mockReset();
  });

  afterEach(async () => {
    process.env.SEED_GITHUB_TOKEN = originalToken;
    for (const id of createdRepoIds.splice(0)) {
      await prisma.slaSnapshot.deleteMany({ where: { repositoryId: id } });
      await prisma.issueSla.deleteMany({ where: { issue: { repositoryId: id } } });
      await prisma.issueStatusEvent.deleteMany({ where: { issue: { repositoryId: id } } });
      await prisma.issue.deleteMany({ where: { repositoryId: id } });
      await prisma.syncRun.deleteMany({ where: { repositoryId: id } });
      await prisma.repository.delete({ where: { id } });
    }
    for (const id of createdProjectIds.splice(0)) {
      await prisma.project.delete({ where: { id } });
    }
  });

  it("throws SyncTokenMissingError when the token is empty", async () => {
    process.env.SEED_GITHUB_TOKEN = "";
    await expect(runIncrementalSync()).rejects.toBeInstanceOf(SyncTokenMissingError);
  });

  it("computes the watermark as lastSyncedAt minus the overlap when lastSyncedAt is set", async () => {
    const lastSyncedAt = new Date("2026-01-10T00:00:00.000Z");
    const { repo } = await makeRepo("watermark-repo", "PVT_watermark", lastSyncedAt);
    createdRepoIds.push(repo.id);
    createdProjectIds.push(repo.slaProjectId!);

    searchAllMock.mockResolvedValue([]);
    await runIncrementalSync();

    const call = searchAllMock.mock.calls.find(([, q]) => q.includes("watermark-repo"));
    expect(call).toBeDefined();
    const since = new Date(/updated:>=(.+)$/.exec(call![1])![1]);
    expect(since.getTime()).toBe(lastSyncedAt.getTime() - 15 * 60_000); // default syncOverlapMinutes: 15
  });

  it("falls back to the lookback window when lastSyncedAt is null", async () => {
    const { repo } = await makeRepo("lookback-repo", "PVT_lookback", null);
    createdRepoIds.push(repo.id);
    createdProjectIds.push(repo.slaProjectId!);

    searchAllMock.mockResolvedValue([]);
    const before = Date.now();
    await runIncrementalSync();
    const after = Date.now();

    const call = searchAllMock.mock.calls.find(([, q]) => q.includes("lookback-repo"));
    const since = new Date(/updated:>=(.+)$/.exec(call![1])![1]).getTime();
    // default settings: seedClosedLookbackDays=90, syncOverlapMinutes=15
    expect(since).toBeGreaterThanOrEqual(before - 90 * 86_400_000 - 15 * 60_000 - 1000);
    expect(since).toBeLessThanOrEqual(after - 90 * 86_400_000 - 15 * 60_000 + 1000);
  });

  it("isolates per-repo failures — a failing repo keeps its watermark and records an error run; other repos still succeed", async () => {
    const { repo: repoA } = await makeRepo("fail-repo", "PVT_fail", null);
    const { repo: repoB } = await makeRepo("ok-repo", "PVT_ok", null);
    createdRepoIds.push(repoA.id, repoB.id);
    createdProjectIds.push(repoA.slaProjectId!, repoB.slaProjectId!);

    searchAllMock.mockImplementation(async (_token: string, q: string) => {
      if (q.includes("fail-repo")) throw new Error("boom");
      if (q.includes("ok-repo")) return [node(1)];
      return [];
    });
    fetchIssueDetailMock.mockImplementation(async (_token, _owner, _name, number) => detail(number, "PVT_ok"));

    const summary = await runIncrementalSync();

    const resultA = summary.repos.find((r) => r.repo === `${OWNER}/fail-repo`);
    const resultB = summary.repos.find((r) => r.repo === `${OWNER}/ok-repo`);
    expect(resultA?.status).toBe("error");
    expect(resultB?.status).toBe("success");
    expect(resultB?.issuesProcessed).toBe(1);

    const refreshedA = await prisma.repository.findUniqueOrThrow({ where: { id: repoA.id } });
    const refreshedB = await prisma.repository.findUniqueOrThrow({ where: { id: repoB.id } });
    expect(refreshedA.lastSyncedAt).toBeNull(); // unchanged — no advance on failure
    expect(refreshedB.lastSyncedAt).not.toBeNull(); // advanced on success

    const errorRun = await prisma.syncRun.findFirst({ where: { repositoryId: repoA.id } });
    expect(errorRun?.status).toBe("error");
    expect(errorRun?.error).toContain("boom");

    const successRun = await prisma.syncRun.findFirst({ where: { repositoryId: repoB.id } });
    expect(successRun?.status).toBe("success");
    expect(successRun?.issuesProcessed).toBe(1);
  });

  it("is idempotent — running twice with identical mock data inserts zero new events the second time", async () => {
    const { repo } = await makeRepo("idem-repo", "PVT_idem", null);
    createdRepoIds.push(repo.id);
    createdProjectIds.push(repo.slaProjectId!);

    searchAllMock.mockImplementation(async (_token: string, q: string) =>
      q.includes("idem-repo") ? [node(42)] : [],
    );
    fetchIssueDetailMock.mockImplementation(async (_token, _owner, _name, number) => detail(number, "PVT_idem"));

    const first = await runIncrementalSync();
    const second = await runIncrementalSync();

    expect(first.repos.find((r) => r.repo === `${OWNER}/idem-repo`)?.eventsInserted).toBeGreaterThan(0);
    expect(second.repos.find((r) => r.repo === `${OWNER}/idem-repo`)?.eventsInserted).toBe(0);
  });

  it("records the triggering user's sub on the SyncRun row when provided", async () => {
    const { repo } = await makeRepo("audit-repo", "PVT_audit", null);
    createdRepoIds.push(repo.id);
    createdProjectIds.push(repo.slaProjectId!);

    searchAllMock.mockResolvedValue([]);
    await runIncrementalSync(undefined, "user-42@example.com");

    const run = await prisma.syncRun.findFirst({ where: { repositoryId: repo.id } });
    expect(run?.triggeredBy).toBe("user-42@example.com");
  });

  it("leaves triggeredBy null when not provided (e.g. stub mode)", async () => {
    const { repo } = await makeRepo("no-trigger-repo", "PVT_no_trigger", null);
    createdRepoIds.push(repo.id);
    createdProjectIds.push(repo.slaProjectId!);

    searchAllMock.mockResolvedValue([]);
    await runIncrementalSync();

    const run = await prisma.syncRun.findFirst({ where: { repositoryId: repo.id } });
    expect(run?.triggeredBy).toBeNull();
  });
});
