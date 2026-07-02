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
import {
  cardActions,
  sheetActions,
  sheetStatus,
} from "@features/csm-timecards/utils/timeSheetState";
import type {
  CsmTimeCard,
  CsmTimeSheet,
  TimeCardState,
} from "@features/csm-timecards/types/timeCards";

const OWNER = { isOwner: true, isApprover: false, isAdmin: false };
const APPROVER = { isOwner: false, isApprover: true, isAdmin: false };
const ADMIN = { isOwner: false, isApprover: false, isAdmin: true };

function card(state: TimeCardState): CsmTimeCard {
  return {
    id: state,
    taskType: "case",
    activity: [],
    caseId: "c",
    caseNumber: "CS1",
    projectId: "proj-1",
    projectName: "Test Project",
    date: "2026-06-24",
    userId: "u",
    userName: "U",
    state,
    category: "Task work",
    billable: true,
    breakdown: {
      analysisDebugging: 0,
      reproduce: 0,
      settingUp: 0,
      providingSolution: 0,
      answering: 0,
    },
    totalHours: 0,
    workLogComment: "",
    issueComplexity: "N/A",
    approvers: [],
    submittedAt: "2026-06-24T09:00:00Z",
  };
}

describe("cardActions", () => {
  it("owner can edit + delete + submit a pending card", () => {
    expect(cardActions("pending", OWNER)).toEqual(["edit", "delete", "submit"]);
  });
  it("owner cannot act on a submitted card", () => {
    expect(cardActions("submitted", OWNER)).toEqual([]);
  });
  it("approver approves/rejects a submitted card", () => {
    expect(cardActions("submitted", APPROVER)).toEqual(["approve", "reject"]);
  });
  it("approver can recall an approved card; admin can also process", () => {
    expect(cardActions("approved", APPROVER)).toEqual(["recall"]);
    expect(cardActions("approved", ADMIN)).toEqual(["recall", "process"]);
  });
  it("owner can edit + delete + resubmit a rejected or recalled card", () => {
    expect(cardActions("rejected", OWNER)).toEqual(["edit", "delete", "resubmit"]);
    expect(cardActions("recalled", OWNER)).toEqual(["edit", "delete", "resubmit"]);
  });
  it("processed is terminal", () => {
    expect(cardActions("processed", ADMIN)).toEqual([]);
  });
});

describe("sheetStatus", () => {
  it("rolls up by priority: rejected > recalled > submitted > approved > open", () => {
    expect(sheetStatus([])).toBe("open");
    expect(sheetStatus([card("pending")])).toBe("open");
    expect(sheetStatus([card("submitted")])).toBe("submitted");
    expect(sheetStatus([card("approved"), card("processed")])).toBe("approved");
    expect(sheetStatus([card("recalled"), card("submitted")])).toBe("recalled");
    expect(sheetStatus([card("rejected"), card("approved")])).toBe("rejected");
  });
});

describe("sheetActions", () => {
  const sheet = (cards: CsmTimeCard[]): CsmTimeSheet => ({
    id: "u:2026-06-22",
    userId: "u",
    userName: "U",
    weekStart: "2026-06-22",
    weekEnd: "2026-06-28",
    state: sheetStatus(cards),
    cards,
    totalHours: 0,
  });

  it("owner can submit a week with editable cards", () => {
    expect(sheetActions(sheet([card("pending")]), OWNER)).toEqual(["submit"]);
  });
  it("approver can approve-remaining, reject and recall", () => {
    expect(
      sheetActions(sheet([card("submitted"), card("approved")]), APPROVER),
    ).toEqual(["approve", "reject", "recall"]);
  });
});
