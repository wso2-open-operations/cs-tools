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
import { buildSlaImpactTimeline, sideOfState } from "./slaImpactTimeline";
import type { CaseAuditEntry, CaseAuditFieldChange } from "@features/csm-cases/types/csmCases";

function entry(
  id: string,
  createdAt: string,
  changes: CaseAuditFieldChange[],
  actor = "Jane Doe",
): CaseAuditEntry {
  return { id, kind: "field_change", actor, createdAt, changes };
}

function change(field: string, previousValue: string | undefined, newValue: string | undefined) {
  return { field, fieldLabel: field, previousValue, newValue };
}

describe("sideOfState", () => {
  it("puts awaiting-info and solution-proposed on the customer side in any spelling", () => {
    expect(sideOfState("Awaiting Info")).toBe("customer");
    expect(sideOfState("AWAITING_INFO")).toBe("customer");
    expect(sideOfState("Solution proposed")).toBe("customer");
  });

  it("puts the other open states on the WSO2 side and closed on neither", () => {
    expect(sideOfState("Open")).toBe("wso2");
    expect(sideOfState("Work In Progress")).toBe("wso2");
    expect(sideOfState("Waiting on WSO2")).toBe("wso2");
    expect(sideOfState("Closed")).toBeNull();
    expect(sideOfState(undefined)).toBeNull();
  });
});

describe("buildSlaImpactTimeline", () => {
  it("returns nothing for no entries or only unrelated changes", () => {
    expect(buildSlaImpactTimeline([])).toEqual([]);
    expect(
      buildSlaImpactTimeline([
        entry("a", "2026-07-01T10:00:00Z", [
          change("assigned_to_id", "John Roe", "Jane Doe"),
          change("work_state", "Idle", "Active"),
          change("eta_shared_on", "", "2026-07-01T10:00:00Z"),
        ]),
      ]),
    ).toEqual([]);
  });

  it("maps state switches across the two sides, oldest first", () => {
    const events = buildSlaImpactTimeline([
      entry("b", "2026-07-02T10:00:00Z", [change("state", "Awaiting Info", "Work In Progress")]),
      entry("a", "2026-07-01T10:00:00Z", [change("state", "Work In Progress", "Awaiting Info")]),
    ]);
    expect(events.map((e) => e.id)).toEqual(["a:state", "b:state"]);
    expect(events[0]).toMatchObject({
      kind: "state",
      title: "Waiting on customer",
      side: "customer",
      previousSide: "wso2",
      actor: "Jane Doe",
      changes: [{ label: "State", from: "Work in progress", to: "Awaiting info" }],
    });
    expect(events[1]).toMatchObject({ title: "Back with WSO2", side: "wso2", previousSide: "customer" });
  });

  it("calls a same-side move a plain state change and skips a no-op", () => {
    const events = buildSlaImpactTimeline([
      entry("a", "2026-07-01T10:00:00Z", [change("state", "Open", "Work In Progress")]),
      entry("b", "2026-07-01T11:00:00Z", [change("state", "Open", "OPEN")]),
    ]);
    expect(events).toHaveLength(1);
    expect(events[0].title).toBe("State changed");
  });

  it("marks closing and reopening", () => {
    const events = buildSlaImpactTimeline([
      entry("a", "2026-07-01T10:00:00Z", [change("state", "Solution Proposed", "Closed")]),
      entry("b", "2026-07-02T10:00:00Z", [change("state", "Closed", "Open")]),
    ]);
    expect(events.map((e) => e.kind)).toEqual(["closed", "reopened"]);
    expect(events[0].side).toBeNull();
    expect(events[1].side).toBe("wso2");
  });

  it("collapses ETA dates saved together into one event and marks first-time set", () => {
    const events = buildSlaImpactTimeline([
      entry("a", "2026-07-01T10:00:00Z", [change("best_case_eta", undefined, "2026-08-01")]),
      entry("b", "2026-07-01T10:00:00.500Z", [change("most_likely_eta", undefined, "2026-08-10")]),
      entry("c", "2026-07-05T09:00:00Z", [change("best_case_eta", "2026-08-01", "2026-08-15")]),
    ]);
    expect(events).toHaveLength(2);
    expect(events[0]).toMatchObject({ kind: "eta", title: "Fix ETA set" });
    expect(events[0].changes).toHaveLength(2);
    expect(events[1]).toMatchObject({ title: "Fix ETA changed" });
    expect(events[1].changes[0]).toMatchObject({ from: "Aug 1, 2026", to: "Aug 15, 2026" });
  });

  it("maps workaround set and withdrawn", () => {
    const events = buildSlaImpactTimeline([
      entry("a", "2026-07-01T10:00:00Z", [change("workaround_provided_on", "", "2026-07-01T10:00:00Z")]),
      entry("b", "2026-07-02T10:00:00Z", [change("workaround_provided_on", "2026-07-01T10:00:00Z", "")]),
    ]);
    expect(events.map((e) => [e.kind, e.title])).toEqual([
      ["workaround", "Workaround provided"],
      ["workaround_withdrawn", "Workaround withdrawn"],
    ]);
  });

  it("maps severity and update released", () => {
    const events = buildSlaImpactTimeline([
      entry("a", "2026-07-01T10:00:00Z", [change("severity", "S3", "S1")]),
      entry("b", "2026-07-02T10:00:00Z", [change("fix_issued_on", "", "2026-07-02T10:00:00Z")]),
    ]);
    expect(events[0].changes).toEqual([{ label: "Severity", from: "Medium", to: "Critical" }]);
    expect(events[1].kind).toBe("update_released");
  });

  it("keeps several changes of one save as separate events with distinct ids", () => {
    const events = buildSlaImpactTimeline([
      entry("a", "2026-07-01T10:00:00Z", [
        change("state", "Open", "Awaiting Info"),
        change("severity", "S3", "S2"),
      ]),
    ]);
    expect(events.map((e) => e.id)).toEqual(["a:state", "a:severity"]);
  });
});
