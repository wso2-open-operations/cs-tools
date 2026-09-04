// src/server/jobs/recompute.ts
// In-process recompute + daily-snapshot job.
//
// What this does NOT do: no GitHub calls. `currentStatus`, priority, and the
// event log stay frozen at their last-synced values; this job only advances
// the open interval of already-known state (the IssueSla projection and
// today's sla_snapshots row). SyncRun/watermark belong to the sync.
import { loadConfig } from "@/server/config";
import {
  prisma,
  slaConfigFromFile,
  withCurrentStatusBoundary,
  computeSla,
  type RuntimeSlaConfig,
  type StatusEvent,
} from "@/server/db";
import { jobLock } from "./lock";
import type { MinimalLogger } from "@/server/db/sync/incremental";

const ENABLED = (process.env.RECOMPUTE_ENABLED ?? "1").trim() !== "0";
const PAGE_SIZE = 200;

const startOfUtcDay = (d: Date) => new Date(Date.UTC(d.getUTCFullYear(), d.getUTCMonth(), d.getUTCDate()));

// Config is immutable for the process lifetime — build the runtime config
// once and reuse it every tick.
let cachedRuntime: RuntimeSlaConfig | null = null;
function getRuntime(): RuntimeSlaConfig {
  if (!cachedRuntime) cachedRuntime = slaConfigFromFile();
  return cachedRuntime;
}

/** One recompute pass. Exported so the manual-sync route can invoke it directly. */
export async function runTickOnce(log: MinimalLogger): Promise<void> {
  const start = Date.now();
  const now = new Date();
  const snapshotDate = startOfUtcDay(now);

  const { cfg, normalize, knownNames } = getRuntime();
  const unknownStatuses = new Map<string, number>();
  const trackStatus = (status: string | null) => {
    if (status == null || knownNames.has(status)) return;
    unknownStatuses.set(status, (unknownStatuses.get(status) ?? 0) + 1);
  };

  const stateCounts: Record<string, number> = {};
  let processed = 0;
  let skip = 0;

  while (true) {
    const page = await prisma.issue.findMany({
      where: { repository: { is: { enabled: true } } },
      select: {
        id: true,
        repositoryId: true,
        priority: true,
        currentStatus: true,
        currentStatusAt: true,
        events: { orderBy: { occurredAt: "asc" }, select: { status: true, occurredAt: true } },
      },
      orderBy: { id: "asc" },
      skip,
      take: PAGE_SIZE,
    });
    if (page.length === 0) break;

    for (const issue of page) {
      const currentStatus = normalize(issue.currentStatus);
      trackStatus(currentStatus);

      const events: StatusEvent[] = issue.events.map((e) => {
        const status = normalize(e.status);
        trackStatus(status);
        return { status, occurredAt: e.occurredAt };
      });

      const slaEvents = withCurrentStatusBoundary(events, currentStatus, issue.currentStatusAt, now);
      const result = computeSla(issue.priority, slaEvents, currentStatus, cfg, now);

      await prisma.issueSla.update({
        where: { issueId: issue.id },
        data: {
          priority: issue.priority,
          budgetHours: result.budgetHours,
          consumedHours: result.consumedHours,
          remainingHours: result.remainingHours,
          pctConsumed: result.pctConsumed,
          slaState: result.slaState,
          slaRunning: result.slaRunning,
          computedAt: now,
          computedThrough: now,
        },
      });

      // Recomputing today's row on every tick is idempotent and keeps the
      // hero spark delta live all day; historical rows are never touched.
      await prisma.slaSnapshot.upsert({
        where: { snapshotDate_issueId: { snapshotDate, issueId: issue.id } },
        create: {
          snapshotDate,
          issueId: issue.id,
          repositoryId: issue.repositoryId,
          priority: issue.priority,
          currentStatus,
          budgetHours: result.budgetHours,
          consumedHours: result.consumedHours,
          remainingHours: result.remainingHours,
          pctConsumed: result.pctConsumed,
          slaState: result.slaState,
          slaRunning: result.slaRunning,
        },
        update: {
          priority: issue.priority,
          currentStatus,
          budgetHours: result.budgetHours,
          consumedHours: result.consumedHours,
          remainingHours: result.remainingHours,
          pctConsumed: result.pctConsumed,
          slaState: result.slaState,
          slaRunning: result.slaRunning,
        },
      });

      stateCounts[result.slaState] = (stateCounts[result.slaState] ?? 0) + 1;
      processed++;
    }

    skip += PAGE_SIZE;
    if (page.length < PAGE_SIZE) break;
  }

  const durationMs = Date.now() - start;
  log.info(
    { processed, stateCounts, durationMs, unknownStatusCount: unknownStatuses.size },
    "[recompute] tick complete",
  );
  if (unknownStatuses.size > 0) {
    log.warn(
      { unknownStatuses: Object.fromEntries(unknownStatuses) },
      "[recompute] unknown statuses encountered — defaulting to pause + non-terminal",
    );
  }
}

export async function startRecomputeJob(log: MinimalLogger): Promise<void> {
  if (!ENABLED) {
    log.info("[recompute] disabled via RECOMPUTE_ENABLED=0");
    return;
  }

  const intervalMinutes = loadConfig().settings.recomputeIntervalMinutes;
  const intervalMs = intervalMinutes * 60_000;

  // Routed through the shared job lock so a tick never interleaves with a
  // manual sync, on this replica or any other. A busy lock just skips this tick.
  const tick = () =>
    jobLock.tryRun(() => runTickOnce(log))?.catch((err) => {
      log.error(err, "[recompute] tick failed");
    }) ?? log.info("tick skipped: job lock busy");

  void tick(); // run one tick immediately on boot — self-heals a server started days after seeding
  setInterval(() => void tick(), intervalMs);
  log.info(`[recompute] started, interval=${intervalMinutes}m`);
}
