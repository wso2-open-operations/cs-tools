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
  autoGenerateCards,
  computeReports,
  copyPreviousWeek,
  createTimeCard,
  decideTimeCard,
  deleteCard,
  listDeletionAudit,
  listMyTimeSheets,
  recallCard,
  rejectSheet,
  submitSheet,
} from "@features/csm-timecards/api/timeCardStore";
import { weekStartOf } from "@features/csm-timecards/utils/timeSheetWeek";
import { emptyBreakdown } from "@features/csm-timecards/utils/timeCardTotals";
import type { CreateTimeCardInput } from "@features/csm-timecards/types/timeCards";

const ENGINEER = { id: "test-engineer", name: "Test Engineer" };
const DATE = "2026-06-24";

function input(): CreateTimeCardInput {
  return {
    taskType: "case",
    caseId: "test-case",
    caseNumber: "CS9999",
    projectId: "proj-test",
    projectName: "Test Project",
    date: DATE,
    category: "Task work",
    billable: true,
    breakdown: { ...emptyBreakdown(), analysisDebugging: 2 },
    workLogComment: "Worked on it.",
    issueComplexity: "Low",
    approver: { id: "a", name: "Approver" },
  };
}

function myCardState(cardId: string): string | undefined {
  for (const sheet of listMyTimeSheets(ENGINEER.id)) {
    const found = sheet.cards.find((c) => c.id === cardId);
    if (found) return found.state;
  }
  return undefined;
}

describe("timeCardStore lifecycle", () => {
  it("walks a card pending → submitted → approved → recalled", () => {
    const card = createTimeCard(input(), ENGINEER);
    expect(card.state).toBe("pending");
    expect(card.totalHours).toBe(2);

    const sheets = listMyTimeSheets(ENGINEER.id);
    expect(sheets).toHaveLength(1);
    expect(sheets[0].state).toBe("open");

    submitSheet(ENGINEER.id, weekStartOf(DATE));
    expect(myCardState(card.id)).toBe("submitted");

    decideTimeCard({ cardId: card.id, state: "approved" }, "Lead");
    expect(myCardState(card.id)).toBe("approved");

    recallCard(card.id, "Lead");
    expect(myCardState(card.id)).toBe("recalled");
  });

  it("records an activity audit trail", () => {
    const card = createTimeCard(input(), { id: "audit-eng", name: "Audit Eng" });
    expect(card.activity.map((a) => a.action)).toEqual(["created"]);
    submitSheet("audit-eng", weekStartOf(DATE));
    decideTimeCard({ cardId: card.id, state: "approved" }, "Lead");
    const after = listMyTimeSheets("audit-eng")
      .flatMap((s) => s.cards)
      .find((c) => c.id === card.id);
    expect(after?.activity.map((a) => a.action)).toEqual([
      "created",
      "submitted",
      "approved",
    ]);
  });

  it("auto-generates pending cards for assigned tasks", () => {
    const week = weekStartOf("2026-06-01");
    const n = autoGenerateCards("gen-eng", "Gen Eng", week);
    expect(n).toBeGreaterThan(0);
    const cards = listMyTimeSheets("gen-eng").flatMap((s) => s.cards);
    expect(cards.length).toBe(n);
    expect(cards.every((c) => c.state === "pending")).toBe(true);
  });

  it("copies the previous week's tasks into a later week", () => {
    createTimeCard(
      { ...input(), date: "2026-06-10" },
      { id: "copy-eng", name: "Copy Eng" },
    );
    const target = weekStartOf("2026-06-17");
    expect(copyPreviousWeek("copy-eng", target)).toBeGreaterThanOrEqual(1);
    const targetCards = listMyTimeSheets("copy-eng")
      .find((s) => s.weekStart === target)
      ?.cards;
    expect(targetCards?.every((c) => c.state === "pending")).toBe(true);
  });

  it("rejects a submitted sheet with a comment", () => {
    const card = createTimeCard(
      { ...input(), date: "2026-05-04" },
      { id: "rej-eng", name: "Rej Eng" },
    );
    const week = weekStartOf("2026-05-04");
    submitSheet("rej-eng", week);
    rejectSheet("rej-eng", week, "Lead", "Please fix the hours.");
    const after = listMyTimeSheets("rej-eng")
      .flatMap((s) => s.cards)
      .find((c) => c.id === card.id);
    expect(after?.state).toBe("rejected");
    expect(after?.leadComment).toBe("Please fix the hours.");
  });

  it("computeReports returns coherent aggregates", () => {
    const r = computeReports();
    expect(r.totalHours).toBeGreaterThanOrEqual(0);
    expect(r.submissionRate).toBeGreaterThanOrEqual(0);
    expect(r.submissionRate).toBeLessThanOrEqual(1);
    expect(Array.isArray(r.byCategory)).toBe(true);
    expect(Array.isArray(r.exceptions)).toBe(true);
  });

  it("splits hours into billable and non-billable, summing to the total", () => {
    createTimeCard(
      { ...input(), billable: true, breakdown: { ...emptyBreakdown(), analysisDebugging: 2 } },
      { id: "bill-eng", name: "Bill Eng" },
    );
    createTimeCard(
      { ...input(), billable: false, breakdown: { ...emptyBreakdown(), answering: 1 } },
      { id: "bill-eng", name: "Bill Eng" },
    );
    const r = computeReports();
    expect(r.billableHours + r.nonBillableHours).toBeCloseTo(r.totalHours);
    expect(r.billableHours).toBeGreaterThanOrEqual(2);
    expect(r.nonBillableHours).toBeGreaterThanOrEqual(1);
  });

  it("deletes an editable (pending) card but refuses a submitted one", () => {
    const stateOf = (userId: string, cardId: string): string | undefined =>
      listMyTimeSheets(userId)
        .flatMap((s) => s.cards)
        .find((c) => c.id === cardId)?.state;

    const card = createTimeCard(
      { ...input(), date: "2026-07-06" },
      { id: "del-eng", name: "Del Eng" },
    );
    expect(stateOf("del-eng", card.id)).toBe("pending");
    deleteCard(card.id, "Del Eng");
    expect(stateOf("del-eng", card.id)).toBeUndefined();

    const card2 = createTimeCard(
      { ...input(), date: "2026-07-13" },
      { id: "del-eng", name: "Del Eng" },
    );
    submitSheet("del-eng", weekStartOf("2026-07-13"));
    expect(() => deleteCard(card2.id, "Del Eng")).toThrow();
    expect(stateOf("del-eng", card2.id)).toBe("submitted");
  });

  it("keeps a deletion audit record (actor, hours, case) after delete", () => {
    const card = createTimeCard(
      { ...input(), date: "2026-08-03", breakdown: { ...emptyBreakdown(), analysisDebugging: 2 } },
      { id: "audit-del", name: "Audit Del" },
    );
    deleteCard(card.id, "Admin Lead");
    const record = listDeletionAudit().find((d) => d.cardId === card.id);
    expect(record).toBeDefined();
    expect(record?.deletedBy).toBe("Admin Lead");
    expect(record?.userName).toBe("Audit Del");
    expect(record?.totalHours).toBe(2);
    expect(computeReports().deletions.some((d) => d.cardId === card.id)).toBe(true);
  });

  it("excludes rejected hours from report totals and rollups", () => {
    createTimeCard(
      { ...input(), breakdown: { ...emptyBreakdown(), analysisDebugging: 3 } },
      { id: "rep-eng", name: "Rep Eng" },
    );
    const rejected = createTimeCard(
      { ...input(), date: "2026-09-07", breakdown: { ...emptyBreakdown(), analysisDebugging: 5 } },
      { id: "rep-eng", name: "Rep Eng" },
    );
    const week = weekStartOf("2026-09-07");
    submitSheet("rep-eng", week);
    rejectSheet("rep-eng", week, "Lead", "No.");
    expect(rejected.totalHours).toBe(5);

    const r = computeReports();
    const repUser = r.byUser.find((u) => u.userId === "rep-eng");
    // The 5h rejected card is excluded; only the 3h card counts for this user.
    expect(repUser?.hours).toBe(3);
  });
});
