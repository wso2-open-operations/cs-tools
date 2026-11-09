// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

import { describe, expect, it } from "vitest";
import { buildSlaImpactTimeline } from "./slaImpactTimeline";
import { layoutSlaTrack, MARKER_MIN_GAP_PX } from "./slaTrackLayout";
import type { CaseAuditEntry } from "@features/csm-cases/types/csmCases";

function entry(id: string, createdAt: string, field: string, from: string, to: string): CaseAuditEntry {
  return {
    id,
    kind: "field_change",
    actor: "Jane Doe",
    createdAt,
    changes: [{ field, fieldLabel: field, previousValue: from, newValue: to }],
  };
}

const NOW = Date.parse("2026-07-10T00:00:00Z");
const formatTick = (ms: number): string => new Date(ms).toISOString().slice(5, 10);

function layout(entries: CaseAuditEntry[], extra: Partial<Parameters<typeof layoutSlaTrack>[0]> = {}) {
  return layoutSlaTrack({ events: buildSlaImpactTimeline(entries), nowMs: NOW, formatTick, ...extra });
}

describe("layoutSlaTrack", () => {
  it("colours the track by side and covers it without gaps", () => {
    const l = layout(
      [
        entry("a", "2026-07-02T00:00:00Z", "state", "Open", "Awaiting Info"),
        entry("b", "2026-07-06T00:00:00Z", "state", "Awaiting Info", "Work In Progress"),
      ],
      { openedAt: "2026-07-01T00:00:00Z" },
    );
    expect(l.segments.map((s) => s.side)).toEqual(["wso2", "customer", "wso2"]);
    expect(l.segments[0].leftPct).toBeCloseTo(0);
    const end = l.segments.reduce((sum, s) => sum + s.widthPct, 0);
    expect(end).toBeCloseTo(100);
  });

  it("uses the current state for the colour when there is no state event", () => {
    expect(layout([], { currentState: "awaiting_info" }).segments.map((s) => s.side)).toEqual(["customer"]);
    expect(layout([]).segments.map((s) => s.side)).toEqual(["wso2"]);
  });

  it("ends a closed case at its closure and shows a neutral closed stretch", () => {
    const l = layout([entry("a", "2026-07-03T00:00:00Z", "state", "Open", "Closed")], {
      openedAt: "2026-07-01T00:00:00Z",
    });
    expect(l.segments.map((s) => s.side)).toEqual(["wso2", "closed"]);
    expect(l.markers[0].leftPct).toBeLessThan(100);
  });

  it("positions markers by time", () => {
    const l = layout(
      [
        entry("a", "2026-07-02T00:00:00Z", "severity", "S3", "S2"),
        entry("b", "2026-07-06T00:00:00Z", "severity", "S2", "S1"),
      ],
      { openedAt: "2026-07-01T00:00:00Z" },
    );
    expect(l.markers.map((m) => m.lane)).toEqual([0, 0]);
    expect(l.markers[1].leftPct).toBeGreaterThan(l.markers[0].leftPct);
  });

  it("keeps markers in one lane at least the minimum gap apart, spilling a same-moment cluster into lanes", () => {
    const entries = Array.from({ length: 8 }, (_, i) =>
      entry(`e${i}`, "2026-07-05T00:00:00Z", "severity", `S${i}`, `S${i + 1}`),
    );
    const l = layout(entries, { openedAt: "2026-07-01T00:00:00Z" });
    expect(l.maxLane).toBeGreaterThan(0);
    const byLane = new Map<number, number[]>();
    for (const m of l.markers) {
      byLane.set(m.lane, [...(byLane.get(m.lane) ?? []), (m.leftPct / 100) * l.baseWidthPx]);
    }
    for (const xs of byLane.values()) {
      for (let i = 1; i < xs.length; i += 1) {
        expect(xs[i] - xs[i - 1]).toBeGreaterThanOrEqual(MARKER_MIN_GAP_PX - 1e-6);
      }
    }
  });

  it("produces date ticks from start to end without repeating a label", () => {
    const l = layout([], { openedAt: "2026-07-01T00:00:00Z" });
    expect(l.ticks.length).toBeGreaterThanOrEqual(2);
    expect(l.ticks[0].leftPct).toBe(0);
    expect(new Set(l.ticks.map((t) => t.label)).size).toBe(l.ticks.length);
  });

  it("copes with no events, a single event and an unparseable time", () => {
    expect(layout([]).markers).toEqual([]);
    const one = layout([entry("a", "2026-07-02T00:00:00Z", "severity", "S3", "S2")]);
    expect(one.markers).toHaveLength(1);
    expect(Number.isFinite(one.markers[0].leftPct)).toBe(true);
    expect(layout([entry("a", "not a date", "severity", "S3", "S2")]).markers).toEqual([]);
  });
});
