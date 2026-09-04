// src/server/db/ingest.ts
// Shared per-issue ingest, used by both the seed and the incremental sync:
// priority extraction from labels (then discard), project-scoped current
// status, alias normalization, a guarded leading "derived" event,
// withCurrentStatusBoundary reconciliation, computeSla.
import { createHash } from "node:crypto";
import { prisma } from "./client";
import { computeSla, withCurrentStatusBoundary, type StatusEvent } from "./sla";
import type { RuntimeSlaConfig } from "./sla-config";
import type { GhIssueDetail, GhIssueNode } from "./github/client";

export type Pair = { node: GhIssueNode; detail: GhIssueDetail };

export interface IngestContext {
  repositoryId: number;
  slaProjectId: number; // DB Project.id
  repo: { owner: string; name: string; githubProjectId: string };
  runtime: RuntimeSlaConfig; // from slaConfigFromFile()
  source: "github" | "synthetic";
  now: Date;
}

export interface IngestResult {
  created: boolean;
  eventsInserted: number;
  issueId: number;
  priority: string | null;
  slaEvents: StatusEvent[];
  slaState: string;
  unknownStatuses: string[];
}

const PRIORITY_RE = /^Priority\/(.+)$/;
function extractPriority(labels: string[]): string | null {
  for (const l of labels) {
    const m = PRIORITY_RE.exec(l);
    if (m) return m[1];
  }
  return null;
}

// Stable across reseeds/incremental syncs: keyed on GitHub-side identifiers
// (repo/name/number/project + transition), not the local autoincrement id.
function dedupeKey(
  repoOwner: string,
  repoName: string,
  githubNumber: number,
  githubProjectId: string,
  occurredAt: string,
  prev: string | null,
  status: string | null,
): string {
  return createHash("sha1")
    .update(`${repoOwner}/${repoName}#${githubNumber}|${githubProjectId}|${occurredAt}|${prev ?? ""}|${status ?? ""}`)
    .digest("hex");
}

export async function ingestIssuePair(pair: Pair, ctx: IngestContext): Promise<IngestResult> {
  const { node, detail } = pair;
  const { runtime, repo, repositoryId, slaProjectId, source, now } = ctx;
  const normalize = runtime.normalize;

  // Labels are read transiently to derive priority, then discarded (PII rule).
  const priority = extractPriority(node.labels.nodes.map((l) => l.name));

  // Current status scoped to THIS repo's configured project.
  const scoped = detail.projectStatuses.find((p) => p.projectId === repo.githubProjectId) ?? null;
  const currentStatus = normalize(scoped?.status ?? null);
  const currentStatusAt = scoped?.statusUpdatedAt ?? null;

  // Normalize formatting variants (e.g. "Re-Opened" -> "Reopened") before
  // anything downstream sees them; canonical names are what gets persisted.
  const normalizedEvents = detail.events.map((e) => ({
    ...e,
    previousStatus: normalize(e.previousStatus),
    status: normalize(e.status),
  }));

  const unknownStatuses = new Set<string>();
  const trackStatus = (status: string | null) => {
    if (status != null && !runtime.knownNames.has(status)) unknownStatuses.add(status);
  };
  trackStatus(currentStatus);
  for (const e of normalizedEvents) {
    trackStatus(e.previousStatus);
    trackStatus(e.status);
  }

  // Synthesize a leading board-add event for the stretch before the first
  // recorded transition, when the first event demonstrably followed a prior
  // status. Guarded: absent/null itemCreatedAt or an itemCreatedAt that isn't
  // strictly before the first event => no-op (pre-first-event time stays
  // unaccounted).
  const firstEvent = normalizedEvents[0] as (typeof normalizedEvents)[number] | undefined;
  const itemCreatedAt = scoped?.itemCreatedAt ?? null;
  const leadingEvent =
    firstEvent != null &&
    firstEvent.previousStatus != null &&
    itemCreatedAt != null &&
    new Date(itemCreatedAt).getTime() < new Date(firstEvent.createdAt).getTime()
      ? { createdAt: itemCreatedAt, previousStatus: null as string | null, status: firstEvent.previousStatus }
      : null;

  // Events actually written to the log (source of truth): real/synthetic
  // timeline events plus the derived leading event, if any.
  const persistedEvents = leadingEvent ? [leadingEvent, ...normalizedEvents] : normalizedEvents;

  // SLA event list (ascending), reconciled with the project-scoped current
  // status — this is what both the current projection and the snapshot
  // replay walk.
  const slaEvents: StatusEvent[] = withCurrentStatusBoundary(
    persistedEvents.map((e) => ({ status: e.status, occurredAt: new Date(e.createdAt) })),
    currentStatus,
    currentStatusAt ? new Date(currentStatusAt) : null,
    now,
  );

  const existing = await prisma.issue.findUnique({
    where: { repositoryId_githubNumber: { repositoryId, githubNumber: node.number } },
    select: { id: true },
  });

  const issueData = {
    state: node.state, // "OPEN" | "CLOSED" — matches the IssueState enum
    htmlUrl: node.url,
    priority,
    currentStatus,
    currentStatusAt: currentStatusAt ? new Date(currentStatusAt) : null,
    githubCreatedAt: new Date(node.createdAt),
    githubClosedAt: node.closedAt ? new Date(node.closedAt) : null,
    githubUpdatedAt: new Date(node.updatedAt),
    lastSyncedAt: now,
  };

  const issue = await prisma.issue.upsert({
    where: { repositoryId_githubNumber: { repositoryId, githubNumber: node.number } },
    create: { repositoryId, githubNumber: node.number, ...issueData },
    update: issueData,
  });

  // Persist the event log (source of truth). skipDuplicates handles real GitHub
  // data where two events can share the same timestamp + transition, and makes
  // reseeds/incremental syncs safe to run repeatedly. The leading event (if
  // any) is marked source "derived" so every future consumer reproduces the
  // same numbers from the DB alone.
  let eventsInserted = 0;
  if (persistedEvents.length > 0) {
    const result = await prisma.issueStatusEvent.createMany({
      data: persistedEvents.map((e) => ({
        issueId: issue.id,
        projectId: slaProjectId,
        previousStatus: e.previousStatus,
        status: e.status,
        occurredAt: new Date(e.createdAt),
        source: e === leadingEvent ? "derived" : source,
        dedupeKey: dedupeKey(
          repo.owner,
          repo.name,
          node.number,
          repo.githubProjectId,
          e.createdAt,
          e.previousStatus,
          e.status,
        ),
      })),
      skipDuplicates: true,
    });
    eventsInserted = result.count;
  }

  // Current SLA projection.
  const r = computeSla(priority, slaEvents, currentStatus, runtime.cfg, now);
  await prisma.issueSla.upsert({
    where: { issueId: issue.id },
    create: {
      issueId: issue.id,
      priority,
      budgetHours: r.budgetHours,
      consumedHours: r.consumedHours,
      remainingHours: r.remainingHours,
      pctConsumed: r.pctConsumed,
      slaState: r.slaState,
      slaRunning: r.slaRunning,
      computedAt: now,
      computedThrough: now,
    },
    update: {
      priority,
      budgetHours: r.budgetHours,
      consumedHours: r.consumedHours,
      remainingHours: r.remainingHours,
      pctConsumed: r.pctConsumed,
      slaState: r.slaState,
      slaRunning: r.slaRunning,
      computedAt: now,
      computedThrough: now,
    },
  });

  return {
    created: existing == null,
    eventsInserted,
    issueId: issue.id,
    priority,
    slaEvents,
    slaState: r.slaState,
    unknownStatuses: [...unknownStatuses],
  };
}
