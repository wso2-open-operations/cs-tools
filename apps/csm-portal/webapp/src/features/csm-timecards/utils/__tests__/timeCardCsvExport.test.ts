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
import { timeCardsToCsv } from "@features/csm-timecards/utils/timeCardCsvExport";
import type { CsmTimeCard } from "@features/csm-timecards/types/timeCards";

function card(overrides: Partial<CsmTimeCard> = {}): CsmTimeCard {
  return {
    id: "card-1",
    caseId: "case-1",
    caseNumber: "CS0352584",
    projectId: "proj-1",
    projectName: "Acme Corp",
    createdOn: "2026-06-27",
    userId: "user-1",
    userName: "Jane Doe",
    state: "submitted",
    billable: true,
    totalMinutes: 90,
    ...overrides,
  };
}

describe("timeCardsToCsv", () => {
  it("emits just the header for an empty list", () => {
    expect(timeCardsToCsv([])).toBe(
      "Date,Case,Project,Engineer,State,Billable,Minutes,Decided by",
    );
  });

  it("emits one row per card with human-readable state and billable", () => {
    const csv = timeCardsToCsv([card()]);
    const lines = csv.split("\r\n");
    expect(lines).toHaveLength(2);
    expect(lines[1]).toBe(
      "2026-06-27,CS0352584,Acme Corp,Jane Doe,Submitted,Yes,90,",
    );
  });

  it("includes the deciding approver once a card has one", () => {
    const csv = timeCardsToCsv([
      card({ state: "approved", approvedByName: "Lead Person" }),
    ]);
    expect(csv).toContain("Approved,Yes,90,Lead Person");
  });

  it("quotes a field that contains a comma", () => {
    const csv = timeCardsToCsv([card({ projectName: "Acme, Inc." })]);
    expect(csv).toContain('"Acme, Inc."');
  });

  it("escapes internal quotes by doubling them", () => {
    const csv = timeCardsToCsv([card({ userName: 'Jane "JD" Doe' })]);
    expect(csv).toContain('"Jane ""JD"" Doe"');
  });

  it("quotes a field with a bare carriage return, not just \\n", () => {
    const csv = timeCardsToCsv([card({ userName: "Jane\rDoe" })]);
    expect(csv).toContain('"Jane\rDoe"');
  });
});
