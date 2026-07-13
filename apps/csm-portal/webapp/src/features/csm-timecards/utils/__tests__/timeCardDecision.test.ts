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
import { decisionSummary } from "@features/csm-timecards/utils/timeCardDecision";
import type { CsmTimeCard } from "@features/csm-timecards/types/timeCards";

function card(overrides: Partial<CsmTimeCard> = {}): CsmTimeCard {
  return {
    id: "card-1",
    caseId: "case-1",
    caseNumber: "CS0352584",
    projectId: "proj-1",
    projectName: "Acme Corp",
    workDate: "2026-07-13",
    userId: "user-1",
    userName: "Jane Doe",
    state: "submitted",
    billable: true,
    totalMinutes: 60,
    ...overrides,
  };
}

describe("decisionSummary", () => {
  it("names the approver on an approved card", () => {
    expect(decisionSummary(card({ state: "approved", approvedByName: "Lead Person" }))).toBe(
      "Approved by Lead Person",
    );
  });

  // ServiceNow records no rejecter identity — only the approver's comment — so a
  // rejection surfaces the reason instead of a name.
  it("shows the reason on a rejected card", () => {
    expect(
      decisionSummary(card({ state: "rejected", rejectionReason: "Break down the debug time." })),
    ).toBe("Rejected: Break down the debug time.");
  });

  it("still reports a rejected card with no reason as rejected", () => {
    expect(decisionSummary(card({ state: "rejected" }))).toBe("Rejected");
  });

  it("still reports an approved card with no approver name as approved", () => {
    expect(decisionSummary(card({ state: "approved" }))).toBe("Approved");
  });

  it("returns null for an undecided card", () => {
    expect(decisionSummary(card({ state: "submitted" }))).toBeNull();
  });

  // Regression guard for the old behaviour: a rejected card used to read its
  // (always-null) `approvedBy`, so it rendered nothing at all.
  it("does not fall back to the approver name on a rejected card", () => {
    expect(
      decisionSummary(card({ state: "rejected", approvedByName: "Lead Person" })),
    ).toBe("Rejected");
  });
});
