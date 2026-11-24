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
import { clusterTimelineEvents, elapsedLabel, ownershipTimeline } from "./slaTimelinePresentation";
import type { CaseAuditEntry } from "@features/csm-cases/types/csmCases";
const at = (hour: number) => new Date(Date.UTC(2026, 6, 1, hour)).toISOString();
const state = (hour: number, from: string, to: string): CaseAuditEntry => ({ id: `state-${hour}`, kind: "field_change", createdAt: at(hour), actor: "Engineer", changes: [{ field: "state", fieldLabel: "State", previousValue: from, newValue: to }] });

describe("ownershipTimeline", () => {
  it("keeps WSO2 ownership through ETA and workaround changes, merging unchanged-side states", () => {
    const events = buildSlaImpactTimeline([
      state(1, "Open", "Work In Progress"),
      { id: "eta", kind: "field_change", createdAt: at(2), actor: "Engineer", changes: [{ field: "most_likely_eta", fieldLabel: "Fix ETA", newValue: "2026-07-03" }] },
      state(3, "Work In Progress", "Awaiting Info"), state(5, "Awaiting Info", "Waiting on WSO2"),
    ]);
    const result = ownershipTimeline(events, at(0), "Waiting on WSO2", Date.parse(at(8)));
    expect(result.periods.map((p) => [p.side, elapsedLabel(p.end-p.start)])).toEqual([["wso2", "3h"], ["customer", "2h"], ["wso2", "3h"]]);
  });
  it("counts closed time before reopening and stops at the final closure", () => {
    const events = buildSlaImpactTimeline([state(2, "Open", "Closed"), state(4, "Closed", "Open"), state(7, "Open", "Closed")]);
    const result = ownershipTimeline(events, at(0), "Closed", Date.parse(at(9)));
    expect(result.end).toBe(Date.parse(at(7)));
    expect(result.periods.map((p) => [p.side, elapsedLabel(p.end-p.start)])).toEqual([["wso2", "2h"], ["closed", "2h"], ["wso2", "3h"]]);
  });
  it("uses the case closure timestamp even without a state audit", () => {
    const result = ownershipTimeline([], at(0), "Closed", Date.parse(at(9)), at(4));
    expect(result.end).toBe(Date.parse(at(4)));
    expect(result.periods).toEqual([{ side: "wso2", start: Date.parse(at(0)), end: Date.parse(at(4)) }]);
  });
  it("does not add a fabricated minimum day or a closed tail", () => {
    const result = ownershipTimeline(buildSlaImpactTimeline([state(1, "Open", "Closed")]), at(0), "Closed", Date.parse(at(5)));
    expect(result.end-result.start).toBe(3600000);
  });
});

describe("clusterTimelineEvents", () => {
  it("clusters nearby audit events without changing their original timestamps", () => {
    const events = buildSlaImpactTimeline([state(1, "Open", "Work In Progress"), state(2, "Work In Progress", "Awaiting Info"), state(8, "Awaiting Info", "Open")]);
    expect(clusterTimelineEvents(events, Date.parse(at(0)), Date.parse(at(9)), 320, 100).map((group) => group.length)).toEqual([2, 1]);
    expect(events[0].at).toBe(at(1));
  });
});
