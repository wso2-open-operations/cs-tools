// src/server/db/seed/seed.ts
// =============================================================================
// Idempotent seed. Resets the tables it owns, then rebuilds from real GitHub
// data (if SEED_GITHUB_TOKEN is set) or synthetic fixtures (if not).
// Run via:  npm run db:seed
// PRIVACY: persists no titles, assignees, openers, labels, or actors.
// =============================================================================

import { loadConfig } from "@/server/config";
import { prisma } from "../client";
import { computeSla, statusAsOf } from "../sla";
import { slaConfigFromFile, type ConfigRepo, type RuntimeSlaConfig } from "../sla-config";
import { syncConfigToDb } from "../config-sync";
import { ingestIssuePair, type IngestResult, type Pair } from "../ingest";
import { fetchIssueDetail, fetchRepoIssues } from "../github/client";
import { syntheticRepoIssues } from "./synthetic";

// ---------------------------------------------------------------------------
// Env
// ---------------------------------------------------------------------------
const TOKEN = (process.env.SEED_GITHUB_TOKEN ?? "").trim();
const STRICT_TAXONOMY = (process.env.SEED_STRICT_TAXONOMY ?? "").trim() === "1";

const DAY_MS = 86_400_000;

const startOfUtcDay = (d: Date) => new Date(Date.UTC(d.getUTCFullYear(), d.getUTCMonth(), d.getUTCDate()));
const endOfUtcDay = (d: Date) =>
  new Date(Date.UTC(d.getUTCFullYear(), d.getUTCMonth(), d.getUTCDate(), 23, 59, 59, 999));

const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));

// ---------------------------------------------------------------------------
// Reset (FK-safe order: children before parents). Repositories/projects are
// file-synced derived tables now — syncConfigToDb() owns their lifecycle, so
// they are no longer reset here.
// ---------------------------------------------------------------------------
async function resetSeededTables() {
  await prisma.slaSnapshot.deleteMany();
  await prisma.issueSla.deleteMany();
  await prisma.issueStatusEvent.deleteMany();
  await prisma.issue.deleteMany();
  await prisma.syncRun.deleteMany();
}

// ---------------------------------------------------------------------------
// Issue source: real GitHub or synthetic
// ---------------------------------------------------------------------------
async function gatherRepoIssues(cfg: ConfigRepo, repoIndex: number, closedLookbackDays: number): Promise<Pair[]> {
  if (!TOKEN) return syntheticRepoIssues(cfg, repoIndex);

  const nodes = await fetchRepoIssues(TOKEN, cfg.owner, cfg.name, cfg.issueQuery, closedLookbackDays);
  const out: Pair[] = [];
  for (const node of nodes) {
    const detail = await fetchIssueDetail(TOKEN, cfg.owner, cfg.name, node.number);
    if (detail) out.push({ node, detail });
    await sleep(150); // courtesy gap for the secondary rate limiter
  }
  return out;
}

// ---------------------------------------------------------------------------
// Snapshot reconstruction — seed-only backfill: the sync never writes
// historical snapshots, so this stays here rather than in the shared ingest.
// ---------------------------------------------------------------------------
async function writeSnapshots(
  pair: Pair,
  result: IngestResult,
  repositoryId: number,
  now: Date,
  snapshotDays: number,
  runtime: RuntimeSlaConfig,
) {
  const { slaEvents, priority, issueId } = result;
  const firstMs = slaEvents.length ? slaEvents[0].occurredAt.getTime() : new Date(pair.node.createdAt).getTime();
  const windowStartMs = now.getTime() - (snapshotDays - 1) * DAY_MS;
  let day = startOfUtcDay(new Date(Math.max(firstMs, windowStartMs)));
  const today = startOfUtcDay(now);

  const snapshots: Array<{
    snapshotDate: Date; issueId: number; repositoryId: number; priority: string | null;
    currentStatus: string | null; budgetHours: number | null; consumedHours: number;
    remainingHours: number | null; pctConsumed: number | null; slaState: string; slaRunning: boolean;
  }> = [];
  while (day.getTime() <= today.getTime()) {
    const through = endOfUtcDay(day);
    const statusThatDay = statusAsOf(slaEvents, through);
    const rr = computeSla(priority, slaEvents, statusThatDay, runtime.cfg, through);
    snapshots.push({
      snapshotDate: day,
      issueId,
      repositoryId,
      priority,
      currentStatus: statusThatDay,
      budgetHours: rr.budgetHours,
      consumedHours: rr.consumedHours,
      remainingHours: rr.remainingHours,
      pctConsumed: rr.pctConsumed,
      slaState: rr.slaState,
      slaRunning: rr.slaRunning,
    });
    day = new Date(day.getTime() + DAY_MS);
  }
  if (snapshots.length) {
    await prisma.slaSnapshot.createMany({ data: snapshots });
  }
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------
async function main() {
  const now = new Date();
  const app = loadConfig();
  const runtime = slaConfigFromFile(app);
  const stateCounts: Record<string, number> = {};
  const unknownStatuses = new Map<string, number>();

  console.log(`\n[seed] mode = ${TOKEN ? "REAL GitHub data" : "SYNTHETIC fixtures (no SEED_GITHUB_TOKEN)"}`);
  console.log(`[seed] repos: ${app.repos.map((r) => `${r.owner}/${r.name}`).join(", ")}`);

  console.log("[seed] resetting seeded tables...");
  await resetSeededTables();

  console.log("[seed] syncing config to db...");
  const syncSummary = await syncConfigToDb(app);
  console.log(`[seed] config sync: ${syncSummary.activeRepos} repos active, ${syncSummary.disabledRepos} disabled`);

  let totalIssues = 0;
  let repoIndex = 0;
  for (const r of app.repos) {
    const repo = await prisma.repository.findUniqueOrThrow({
      where: { owner_name: { owner: r.owner, name: r.name } },
    });
    if (repo.slaProjectId == null) {
      throw new Error(`Repository ${r.owner}/${r.name} has no linked project — config sync should have set this.`);
    }
    const slaProjectId = repo.slaProjectId;

    const pairs = await gatherRepoIssues(r, repoIndex, app.settings.seedClosedLookbackDays);
    console.log(`[seed]   ${r.owner}/${r.name}: ${pairs.length} issues`);

    for (const pair of pairs) {
      const result = await ingestIssuePair(pair, {
        repositoryId: repo.id,
        slaProjectId,
        repo: { owner: r.owner, name: r.name, githubProjectId: r.githubProjectId },
        runtime,
        source: TOKEN ? "github" : "synthetic",
        now,
      });
      totalIssues++;
      stateCounts[result.slaState] = (stateCounts[result.slaState] ?? 0) + 1;
      for (const s of result.unknownStatuses) {
        unknownStatuses.set(s, (unknownStatuses.get(s) ?? 0) + 1);
      }

      // Reconstruct daily snapshots by replaying the event log as of each past day.
      await writeSnapshots(pair, result, repo.id, now, app.settings.seedSnapshotDays, runtime);
    }

    await prisma.repository.update({ where: { id: repo.id }, data: { lastSyncedAt: now } });
    repoIndex++;
  }

  const snapshotCount = await prisma.slaSnapshot.count();
  console.log(
    `\n[seed] done.\n` +
      `  issues:        ${totalIssues}\n` +
      `  by SLA state:  ${JSON.stringify(stateCounts)}\n` +
      `  snapshots:     ${snapshotCount}\n`,
  );

  if (unknownStatuses.size > 0) {
    console.warn(`\n[seed] ⚠⚠⚠ UNKNOWN STATUSES — not present in the config taxonomy ⚠⚠⚠`);
    console.warn(`  status → issue occurrence count`);
    for (const [status, count] of unknownStatuses) {
      console.warn(`  ${JSON.stringify(status)} → ${count}`);
    }
    console.warn(`  unknown statuses default to pause + non-terminal\n`);
    if (STRICT_TAXONOMY) {
      console.error("[seed] SEED_STRICT_TAXONOMY=1 — failing due to unknown statuses above.");
      process.exit(1);
    }
  }
}

main()
  .then(async () => {
    await prisma.$disconnect();
  })
  .catch(async (err) => {
    console.error("[seed] FAILED:", err);
    await prisma.$disconnect();
    process.exit(1);
  });
