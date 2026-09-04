// src/server/db/sla.test.ts
import { describe, expect, it } from "vitest";
import { computeSla, statusAsOf, withCurrentStatusBoundary, type SlaConfig, type StatusEvent } from "./sla";

const H = 3_600_000;
const t0 = new Date("2026-01-01T00:00:00Z");
const at = (h: number) => new Date(t0.getTime() + h * H);

const cfg: SlaConfig = {
  budgets: new Map([["High(P2)", 24]]),
  accrues: (s) => s === "Open" || s === "In Progress" || s === "WOW" || s === "Reopened",
  isTerminal: (s) => s === "Resolved" || s === "Duplicate",
  possibleThreshold: 0.75,
};

const ev = (status: string | null, h: number): StatusEvent => ({ status, occurredAt: at(h) });

describe("computeSla", () => {
  it("returns NO_SLA when priority has no budget", () => {
    const r = computeSla(null, [ev("Open", 0)], "Open", cfg, at(10));
    expect(r.slaState).toBe("NO_SLA");
    expect(r.budgetHours).toBeNull();
  });

  it("accrues only product-side intervals and pauses on CS side", () => {
    // Open 0-4h (accrues), WOC 4-10h (pauses), In Progress 10-14h (accrues) = 8h
    const events = [ev("Open", 0), ev("WOC", 4), ev("In Progress", 10)];
    const r = computeSla("High(P2)", events, "In Progress", cfg, at(14));
    expect(r.consumedHours).toBeCloseTo(8, 5);
    expect(r.slaState).toBe("OK");
    expect(r.slaRunning).toBe(true);
  });

  it("open interval accrues to now only when current status is product-side", () => {
    const paused = computeSla("High(P2)", [ev("Open", 0), ev("WOC", 5)], "WOC", cfg, at(100));
    expect(paused.consumedHours).toBeCloseTo(5, 5);
    expect(paused.slaRunning).toBe(false);
  });

  it("crosses AT_RISK at 75% and VIOLATED at 100%", () => {
    const atRisk = computeSla("High(P2)", [ev("Open", 0)], "Open", cfg, at(18)); // 18/24
    expect(atRisk.slaState).toBe("AT_RISK");
    const violated = computeSla("High(P2)", [ev("Open", 0)], "Open", cfg, at(25));
    expect(violated.slaState).toBe("VIOLATED");
  });

  it("never resets on reopen — accrual accumulates across the pause", () => {
    // Open 0-4h, WOC 4-28h, Reopened 28h..52h => 4 + 24 = 28h > 24h budget
    const events = [ev("Open", 0), ev("WOC", 4), ev("Reopened", 28)];
    const r = computeSla("High(P2)", events, "Reopened", cfg, at(52));
    expect(r.consumedHours).toBeCloseTo(28, 5);
    expect(r.slaState).toBe("VIOLATED");
  });

  it("terminal status wins regardless of pct consumed", () => {
    const r = computeSla("High(P2)", [ev("Open", 0), ev("Resolved", 30)], "Resolved", cfg, at(40));
    expect(r.slaState).toBe("TERMINAL");
    expect(r.slaRunning).toBe(false);
  });

  it("ignores events after `now` (historical snapshot reconstruction)", () => {
    const events = [ev("Open", 0), ev("Resolved", 50)];
    const r = computeSla("High(P2)", events, statusAsOf(events, at(10)), cfg, at(10));
    expect(r.consumedHours).toBeCloseTo(10, 5);
    expect(r.slaState).toBe("OK");
  });
});

describe("computeSla — 12x5 IST coverage", () => {
  // IST = UTC+5:30. 09:00 IST = 03:30 UTC; 21:00 IST = 15:30 UTC.
  // 2026-01-05 is a Monday, 2026-01-09 a Friday, 2026-01-12 a Monday.
  const cfg12x5: SlaConfig = {
    ...cfg,
    coverage: new Map([["High(P2)", "12x5_ist"]]),
  };
  const mon0105_09ist = new Date("2026-01-05T03:30:00Z"); // Mon 09:00 IST

  it("accrues only inside the 09:00–21:00 IST window (24h wall = 12h coverage)", () => {
    const open = { status: "Open", occurredAt: mon0105_09ist };
    // Mon 09:00 IST -> Tue 09:00 IST: only Mon 09–21 counts = 12h.
    const r = computeSla("High(P2)", [open], "Open", cfg12x5, new Date(mon0105_09ist.getTime() + 24 * H));
    expect(r.consumedHours).toBeCloseTo(12, 5);
  });

  it("burns zero budget across a weekend", () => {
    const fri21 = new Date("2026-01-09T15:30:00Z"); // Fri 21:00 IST (window just closed)
    const mon08 = new Date("2026-01-12T02:30:00Z"); // Mon 08:00 IST (before window opens)
    const r = computeSla("High(P2)", [{ status: "Open", occurredAt: fri21 }], "Open", cfg12x5, mon08);
    expect(r.consumedHours).toBeCloseTo(0, 5);
  });

  it("clips a partial window at the coverage boundary", () => {
    const mon20 = new Date("2026-01-05T14:30:00Z"); // Mon 20:00 IST
    const mon22 = new Date("2026-01-05T16:30:00Z"); // Mon 22:00 IST
    // Only 20:00–21:00 IST counts = 1h.
    const r = computeSla("High(P2)", [{ status: "Open", occurredAt: mon20 }], "Open", cfg12x5, mon22);
    expect(r.consumedHours).toBeCloseTo(1, 5);
  });

  it("slaRunning is false outside the coverage window even when the status accrues", () => {
    const sat = new Date("2026-01-10T06:00:00Z"); // Saturday
    const r = computeSla("High(P2)", [{ status: "Open", occurredAt: mon0105_09ist }], "Open", cfg12x5, sat);
    expect(r.slaRunning).toBe(false);
  });

  it("falls back to 24x7 when a priority has no coverage entry", () => {
    // cfg12x5 has no coverage for a hypothetical priority; use base cfg (no coverage map at all).
    const open = { status: "Open", occurredAt: mon0105_09ist };
    const r = computeSla("High(P2)", [open], "Open", cfg, new Date(mon0105_09ist.getTime() + 24 * H));
    expect(r.consumedHours).toBeCloseTo(24, 5); // full wall-clock — no business-hours mask
  });
});

describe("statusAsOf", () => {
  it("returns the status active at the instant, null before first event", () => {
    const events = [ev("Open", 5), ev("WOC", 10)];
    expect(statusAsOf(events, at(1))).toBeNull();
    expect(statusAsOf(events, at(7))).toBe("Open");
    expect(statusAsOf(events, at(10))).toBe("WOC");
    expect(statusAsOf(events, at(99))).toBe("WOC");
  });
});

describe("withCurrentStatusBoundary", () => {
  it("divergence: splits the final interval at currentStatusAt", () => {
    const events = [ev("Open", 0), ev("WOC", 4)]; // last event WOC@T4
    const out = withCurrentStatusBoundary(events, "In Progress", at(10), at(14));
    expect(out).toHaveLength(3);
    expect(out[2]).toEqual({ status: "In Progress", occurredAt: at(10) });

    const r = computeSla("High(P2)", out, "In Progress", cfg, at(14));
    // Open 0-4 (accrues) + In Progress 10-14 (accrues) = 8h; WOC 4-10 pauses.
    expect(r.consumedHours).toBeCloseTo(8, 5);
    expect(r.slaRunning).toBe(true);
  });

  it("empty timeline: accrues from currentStatusAt to now", () => {
    const out = withCurrentStatusBoundary([], "Open", at(0), at(5));
    expect(out).toEqual([{ status: "Open", occurredAt: at(0) }]);
    const r = computeSla("High(P2)", out, "Open", cfg, at(5));
    expect(r.consumedHours).toBeCloseTo(5, 5);
  });

  it("clamp: currentStatusAt before the last event lands the boundary at last.occurredAt", () => {
    const events = [ev("Open", 0), ev("WOC", 10)];
    const out = withCurrentStatusBoundary(events, "In Progress", at(3), at(20));
    expect(out).toHaveLength(3);
    expect(out[2]).toEqual({ status: "In Progress", occurredAt: at(10) });
    // Walk order preserved (ascending).
    for (let i = 1; i < out.length; i++) {
      expect(out[i].occurredAt.getTime()).toBeGreaterThanOrEqual(out[i - 1].occurredAt.getTime());
    }
  });

  it("agreement: no-op when the last event's status already equals currentStatus", () => {
    const events = [ev("Open", 0), ev("In Progress", 5)];
    const out = withCurrentStatusBoundary(events, "In Progress", at(5), at(20));
    expect(out).toEqual(events);
  });
});
