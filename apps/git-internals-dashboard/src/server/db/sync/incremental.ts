// src/server/db/sync/incremental.ts
// Manual incremental sync: one GitHub fetch pass per enabled repository,
// using the same shared ingest as the seed. No scheduling concerns here.
//
// Known, documented limitation: an issue that stops matching a repo's
// `issueQuery` (e.g. a label removed) no longer appears in incremental
// results and its row goes stale until the next full reseed reconciles it.
import { loadConfig } from "@/server/config";
import { prisma } from "../client";
import { slaConfigFromFile } from "../sla-config";
import { ingestIssuePair } from "../ingest";
import { fetchIssueDetail, searchAll } from "../github/client";

export class SyncTokenMissingError extends Error {}

export interface RepoSyncResult {
  repo: string; // "owner/name"
  status: "success" | "error";
  issuesProcessed: number;
  eventsInserted: number;
  error?: string;
}

export interface SyncSummary {
  startedAt: string;
  finishedAt: string;
  repos: RepoSyncResult[];
}

export interface MinimalLogger {
  info: (...args: unknown[]) => void;
  warn: (...args: unknown[]) => void;
  error: (...args: unknown[]) => void;
}

const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));

export async function runIncrementalSync(log?: MinimalLogger, triggeredBy?: string): Promise<SyncSummary> {
  const token = (process.env.SEED_GITHUB_TOKEN ?? "").trim();
  if (!token) {
    throw new SyncTokenMissingError(
      "SEED_GITHUB_TOKEN is not configured — manual sync needs a fine-grained PAT with Issues:Read + Projects:Read.",
    );
  }

  const app = loadConfig();
  const runtime = slaConfigFromFile(app);
  const overlapMs = app.settings.syncOverlapMinutes * 60_000;
  const lookbackMs = app.settings.seedClosedLookbackDays * 86_400_000;

  const startedAt = new Date();
  const repos = await prisma.repository.findMany({
    where: { enabled: true },
    include: { slaProject: true },
  });
  const results: RepoSyncResult[] = [];

  for (const repo of repos) {
    const repoLabel = `${repo.owner}/${repo.name}`;
    // Captured before any fetch — advances the watermark to "now" on success,
    // so anything updated mid-run is re-covered by the next sync's overlap.
    const syncStartedAt = new Date();
    const since = new Date(
      (repo.lastSyncedAt ? repo.lastSyncedAt.getTime() : syncStartedAt.getTime() - lookbackMs) - overlapMs,
    );

    try {
      if (repo.slaProjectId == null || repo.slaProject == null) {
        throw new Error(`Repository ${repoLabel} has no linked project — config sync should have set this.`);
      }

      const q = `repo:${repo.owner}/${repo.name} ${repo.issueQuery} updated:>=${since.toISOString()}`;
      const nodes = await searchAll(token, q);

      let issuesProcessed = 0;
      let eventsInserted = 0;
      for (const node of nodes) {
        const detail = await fetchIssueDetail(token, repo.owner, repo.name, node.number);
        if (detail) {
          const result = await ingestIssuePair(
            { node, detail },
            {
              repositoryId: repo.id,
              slaProjectId: repo.slaProjectId,
              repo: { owner: repo.owner, name: repo.name, githubProjectId: repo.slaProject.githubProjectId },
              runtime,
              source: "github",
              now: syncStartedAt,
            },
          );
          issuesProcessed++;
          eventsInserted += result.eventsInserted;
        }
        await sleep(150); // courtesy gap for the secondary rate limiter
      }

      await prisma.repository.update({ where: { id: repo.id }, data: { lastSyncedAt: syncStartedAt } });
      await prisma.syncRun.create({
        data: {
          repositoryId: repo.id,
          kind: "manual",
          sinceTs: since,
          startedAt: syncStartedAt,
          finishedAt: new Date(),
          status: "success",
          issuesProcessed,
          triggeredBy,
        },
      });

      results.push({ repo: repoLabel, status: "success", issuesProcessed, eventsInserted });
      log?.info(`[sync] ${repoLabel}: ${issuesProcessed} issues, ${eventsInserted} events`);
    } catch (err) {
      const message = (err as Error).message;
      await prisma.syncRun.create({
        data: {
          repositoryId: repo.id,
          kind: "manual",
          sinceTs: since,
          startedAt: syncStartedAt,
          finishedAt: new Date(),
          status: "error",
          error: message,
          triggeredBy,
        },
      });
      // Do not advance this repo's watermark — continue with the rest.
      results.push({ repo: repoLabel, status: "error", issuesProcessed: 0, eventsInserted: 0, error: message });
      log?.error(`[sync] ${repoLabel} failed: ${message}`);
    }
  }

  return { startedAt: startedAt.toISOString(), finishedAt: new Date().toISOString(), repos: results };
}
