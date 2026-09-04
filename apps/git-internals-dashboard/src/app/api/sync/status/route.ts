import { prisma } from "@/server/db";
import { requireAuth } from "@/server/auth";
import { jobLock } from "@/server/jobs/lock";

// GET /api/sync/status — 
// Return: Current lock state (this replica) + per-repo watermark + last run.
export const GET = requireAuth(async () => {
  const [repos, lastRun] = await Promise.all([
    prisma.repository.findMany({
      where: { enabled: true },
      select: { owner: true, name: true, lastSyncedAt: true },
    }),
    prisma.syncRun.findFirst({ orderBy: { startedAt: "desc" } }),
  ]);

  return Response.json({
    running: jobLock.running,
    repos: repos.map((r) => ({ repo: `${r.owner}/${r.name}`, lastSyncedAt: r.lastSyncedAt })),
    lastRun: lastRun
      ? {
          kind: lastRun.kind,
          status: lastRun.status,
          finishedAt: lastRun.finishedAt,
          issuesProcessed: lastRun.issuesProcessed,
          error: lastRun.error,
          triggeredBy: lastRun.triggeredBy,
        }
      : null,
  });
});
