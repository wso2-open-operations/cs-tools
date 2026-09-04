// src/server/db/sla.ts
// Pure SLA engine. No I/O, no Prisma — unit-testable in isolation (see sla.test.ts).
export type SlaState = "NO_SLA" | "OK" | "AT_RISK" | "VIOLATED" | "TERMINAL";

/**
 * When the SLA clock is allowed to tick.
 *  - "24x7"     — around the clock, every day (e.g. P1).
 *  - "12x5_ist" — business hours only: Mon–Fri, 09:00–21:00 IST (e.g. P2/P3).
 * Unknown/missing coverage falls back to "24x7" (the historical behaviour).
 */
export type SlaCoverage = "24x7" | "12x5_ist";

export interface StatusEvent {
  status: string | null; // status active starting at occurredAt
  occurredAt: Date;
}

export interface SlaConfig {
  budgets: Map<string, number>; // priority -> budget hours
  coverage?: Map<string, SlaCoverage>; // priority -> coverage window; absent => "24x7"
  accrues: (status: string | null) => boolean; // true only for product-side statuses
  isTerminal: (status: string | null) => boolean; // true for Resolved / Duplicate
  possibleThreshold: number; // AT_RISK threshold, e.g. 0.75
}

export interface SlaResult {
  budgetHours: number | null;
  consumedHours: number;
  remainingHours: number | null;
  pctConsumed: number | null;
  slaState: SlaState;
  slaRunning: boolean;
}

const MS_PER_HOUR = 3_600_000;
const MS_PER_DAY = 86_400_000;

// IST (Asia/Kolkata) is a fixed UTC+5:30 offset with no daylight saving, so a
// constant shift converts UTC <-> IST wall-clock exactly. Adding the offset and
// reading the result with getUTC* yields the IST calendar day / hour.
const IST_OFFSET_MS = 5 * MS_PER_HOUR + 30 * 60_000;
const COVERAGE_START_HOUR = 9; // 09:00 IST
const COVERAGE_END_HOUR = 21; // 21:00 IST (9pm)

/** True when instant `ms` falls inside the 12x5 window (Mon–Fri, 09:00–21:00 IST). */
function within12x5Ist(ms: number): boolean {
  const ist = new Date(ms + IST_OFFSET_MS);
  const dow = ist.getUTCDay(); // 0=Sun … 6=Sat, in IST wall-clock terms
  if (dow === 0 || dow === 6) return false;
  const hour = ist.getUTCHours();
  return hour >= COVERAGE_START_HOUR && hour < COVERAGE_END_HOUR;
}

/** Milliseconds of [startMs, endMs) that land inside the 12x5 IST window. */
function covered12x5IstMs(startMs: number, endMs: number): number {
  if (endMs <= startMs) return 0;
  let total = 0;
  const firstDay = Math.floor((startMs + IST_OFFSET_MS) / MS_PER_DAY);
  const lastDay = Math.floor((endMs + IST_OFFSET_MS) / MS_PER_DAY);
  for (let day = firstDay; day <= lastDay; day++) {
    const dow = new Date(day * MS_PER_DAY).getUTCDay();
    if (dow === 0 || dow === 6) continue; // weekend: no coverage
    const midnightUtc = day * MS_PER_DAY - IST_OFFSET_MS; // 00:00 IST for this day, in UTC ms
    const lo = Math.max(startMs, midnightUtc + COVERAGE_START_HOUR * MS_PER_HOUR);
    const hi = Math.min(endMs, midnightUtc + COVERAGE_END_HOUR * MS_PER_HOUR);
    if (hi > lo) total += hi - lo;
  }
  return total;
}

/** Milliseconds of [startMs, endMs) during which the SLA clock ticks for `coverage`. */
function coveredMs(startMs: number, endMs: number, coverage: SlaCoverage): number {
  if (coverage === "12x5_ist") return covered12x5IstMs(startMs, endMs);
  return Math.max(0, endMs - startMs); // 24x7
}

/** Is the SLA clock currently ticking at instant `ms` under `coverage`? */
function isWithinCoverage(ms: number, coverage: SlaCoverage): boolean {
  return coverage === "12x5_ist" ? within12x5Ist(ms) : true;
}

/**
 * Accumulated time the issue spent in a product-side status, never reset.
 * Pass the FULL event list; `now` bounds the open interval (pass a past instant to
 * reconstruct a historical snapshot). Events after `now` are ignored.
 */
export function computeSla(
  priority: string | null,
  events: StatusEvent[],
  currentStatus: string | null,
  cfg: SlaConfig,
  now: Date = new Date(),
): SlaResult {
  const budget = priority ? cfg.budgets.get(priority) : undefined;
  if (budget === undefined) {
    return {
      budgetHours: null,
      consumedHours: 0,
      remainingHours: null,
      pctConsumed: null,
      slaState: "NO_SLA",
      slaRunning: false,
    };
  }

  const coverage: SlaCoverage = (priority ? cfg.coverage?.get(priority) : undefined) ?? "24x7";

  const nowMs = now.getTime();
  const sorted = events
    .filter((e) => e.occurredAt.getTime() <= nowMs)
    .sort((a, b) => a.occurredAt.getTime() - b.occurredAt.getTime());

  let consumedMs = 0;
  for (let i = 0; i < sorted.length; i++) {
    const start = sorted[i].occurredAt.getTime();
    const end = i + 1 < sorted.length ? sorted[i + 1].occurredAt.getTime() : nowMs;
    // Only time inside the priority's coverage window burns SLA budget.
    if (cfg.accrues(sorted[i].status)) consumedMs += coveredMs(start, end, coverage);
  }

  const consumedHours = consumedMs / MS_PER_HOUR;
  const pct = budget > 0 ? consumedHours / budget : 0;

  let slaState: SlaState;
  if (cfg.isTerminal(currentStatus)) slaState = "TERMINAL";
  else if (pct >= 1.0) slaState = "VIOLATED";
  else if (pct >= cfg.possibleThreshold) slaState = "AT_RISK";
  else slaState = "OK";

  return {
    budgetHours: budget,
    consumedHours,
    remainingHours: budget - consumedHours,
    pctConsumed: pct,
    slaState,
    // The clock is "running" only when the status accrues AND we are inside the
    // coverage window (a P2/P3 issue mid-transition at 2am IST is paused).
    slaRunning: cfg.accrues(currentStatus) && isWithinCoverage(nowMs, coverage),
  };
}

/** Status active at instant `at`. Assumes events sorted ascending. */
export function statusAsOf(events: StatusEvent[], at: Date): string | null {
  const atMs = at.getTime();
  let s: string | null = null;
  for (const e of events) {
    if (e.occurredAt.getTime() <= atMs) s = e.status;
    else break;
  }
  return s;
}

/**
 * Reconcile the event walk with the project-scoped current status (the
 * authoritative "final open interval" source — see SLA domain rules).
 * Appends an in-memory boundary event when the last event's status disagrees
 * with currentStatus, splitting the final interval at currentStatusAt.
 * Never persisted: derived from Issue.currentStatus/currentStatusAt on demand.
 */
export function withCurrentStatusBoundary(
  events: StatusEvent[],
  currentStatus: string | null,
  currentStatusAt: Date | null,
  now: Date,
): StatusEvent[] {
  const sorted = [...events].sort((a, b) => a.occurredAt.getTime() - b.occurredAt.getTime());
  const last = sorted[sorted.length - 1];

  if (!last) {
    // Empty timeline: the clock starts when the current status was set (exact —
    // currentStatusAt is the field's updatedAt), or contributes nothing if unknown.
    return currentStatus != null
      ? [{ status: currentStatus, occurredAt: currentStatusAt ?? now }]
      : [];
  }
  if (last.status === currentStatus) return sorted;

  // Divergence: clamp the boundary into [last.occurredAt, now] so contradictory
  // timestamps (currentStatusAt older than the last event) can't reorder the walk.
  const at = Math.min(
    Math.max((currentStatusAt ?? now).getTime(), last.occurredAt.getTime()),
    now.getTime(),
  );
  return [...sorted, { status: currentStatus, occurredAt: new Date(at) }];
}
