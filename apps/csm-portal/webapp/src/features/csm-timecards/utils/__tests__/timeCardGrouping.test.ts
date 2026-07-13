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
import { groupTimeCards } from "@features/csm-timecards/utils/timeCardGrouping";
import type { CsmTimeCard } from "@features/csm-timecards/types/timeCards";

function card(overrides: Partial<CsmTimeCard>): CsmTimeCard {
  return {
    id: "card-1",
    caseId: "case-1",
    caseNumber: "CS0000001",
    projectId: "project-1",
    projectName: "Project",
    workDate: "2026-06-01",
    userId: "user-1",
    userName: "Test User",
    state: "submitted",
    billable: false,
    totalMinutes: 60,
    ...overrides,
  };
}

describe("groupTimeCards", () => {
  it("groups by case, summing minutes and collecting every card", () => {
    const groups = groupTimeCards(
      [
        card({ id: "a", caseId: "case-1", caseNumber: "CS001", totalMinutes: 30 }),
        card({ id: "b", caseId: "case-1", caseNumber: "CS001", totalMinutes: 15 }),
        card({ id: "c", caseId: "case-2", caseNumber: "CS002", totalMinutes: 60 }),
      ],
      "case",
    );
    expect(groups).toHaveLength(2);
    const cs001 = groups.find((g) => g.key === "case-1")!;
    expect(cs001.label).toBe("CS001");
    expect(cs001.cards).toHaveLength(2);
    expect(cs001.totalMinutes).toBe(45);
  });

  it("groups by engineer instead when asked", () => {
    const groups = groupTimeCards(
      [
        card({ id: "a", userId: "u1", userName: "Alice" }),
        card({ id: "b", userId: "u2", userName: "Bob" }),
      ],
      "engineer",
    );
    expect(groups.map((g) => g.label).sort()).toEqual(["Alice", "Bob"]);
  });

  it("sorts cards within a group newest work date first", () => {
    const groups = groupTimeCards(
      [
        card({ id: "old", caseId: "case-1", workDate: "2026-06-01" }),
        card({ id: "new", caseId: "case-1", workDate: "2026-06-15" }),
      ],
      "case",
    );
    expect(groups[0].cards.map((c) => c.id)).toEqual(["new", "old"]);
  });

  it("sorts groups by their own most recent card, newest first", () => {
    const groups = groupTimeCards(
      [
        card({ id: "a", caseId: "case-old", caseNumber: "OLD", workDate: "2026-06-01" }),
        card({ id: "b", caseId: "case-new", caseNumber: "NEW", workDate: "2026-06-20" }),
      ],
      "case",
    );
    expect(groups.map((g) => g.label)).toEqual(["NEW", "OLD"]);
  });

  it("returns an empty list for an empty input", () => {
    expect(groupTimeCards([], "case")).toEqual([]);
  });
});
