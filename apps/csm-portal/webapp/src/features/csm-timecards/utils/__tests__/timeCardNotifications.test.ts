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
import { timeCardNotices } from "@features/csm-timecards/utils/timeCardNotifications";
import { emptyBreakdown } from "@features/csm-timecards/utils/timeCardTotals";
import type {
  CsmTimeCard,
  CsmTimeSheet,
  TimeCardState,
} from "@features/csm-timecards/types/timeCards";

function card(
  id: string,
  state: TimeCardState,
  extra: Partial<CsmTimeCard> = {},
): CsmTimeCard {
  return {
    id,
    taskType: "case",
    caseId: id,
    caseNumber: `CS-${id}`,
    projectId: "proj-1",
    projectName: "Test Project",
    date: "2026-06-24",
    userId: "u1",
    userName: "Alex Engineer",
    state,
    category: "Task work",
    billable: true,
    breakdown: emptyBreakdown(),
    totalHours: 0,
    workLogComment: "",
    issueComplexity: "N/A",
    approvers: [],
    submittedAt: "2026-06-24T09:00:00Z",
    activity: [],
    ...extra,
  };
}

function sheet(cards: CsmTimeCard[]): CsmTimeSheet {
  return {
    id: "u1:2026-06-22",
    userId: "u1",
    userName: "Alex Engineer",
    weekStart: "2026-06-22",
    weekEnd: "2026-06-28",
    state: "rejected",
    cards,
    totalHours: 0,
  };
}

describe("timeCardNotices", () => {
  it("raises a notice for rejected and recalled cards only", () => {
    const notices = timeCardNotices([
      sheet([
        card("1", "rejected", { leadComment: "Fix the hours." }),
        card("2", "approved"),
        card("3", "recalled", { decidedBy: "Lead" }),
        card("4", "pending"),
      ]),
    ]);
    expect(notices.map((n) => n.cardId).sort()).toEqual(["1", "3"]);
  });

  it("includes the lead's comment in a rejection notice", () => {
    const [notice] = timeCardNotices([
      sheet([
        card("1", "rejected", {
          leadComment: "Fix the hours.",
          decidedAt: "2026-06-25T09:00:00Z",
        }),
      ]),
    ]);
    expect(notice.severity).toBe("error");
    expect(notice.message).toContain("Fix the hours.");
  });

  it("returns nothing when no cards need rework", () => {
    expect(timeCardNotices([sheet([card("1", "approved")])])).toEqual([]);
  });
});
