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
import { groupIntoSheets, sheetStatus } from "@features/csm-timecards/utils/timeSheetGrouping";
import type { CsmTimeCard } from "@features/csm-timecards/types/timeCards";

function card(overrides: Partial<CsmTimeCard>): CsmTimeCard {
  return {
    id: "card-1",
    caseId: "case-1",
    caseNumber: "CS0000001",
    projectId: "project-1",
    projectName: "Project",
    createdOn: "2026-06-01",
    userId: "user-1",
    userName: "Test User",
    state: "submitted",
    billable: false,
    totalMinutes: 60,
    ...overrides,
  };
}

describe("sheetStatus", () => {
  it("is rejected if any card is rejected", () => {
    expect(sheetStatus([card({ state: "submitted" }), card({ state: "rejected" })])).toBe(
      "rejected",
    );
  });

  it("is approved only when every card is approved or processed", () => {
    expect(sheetStatus([card({ state: "approved" }), card({ state: "processed" })])).toBe(
      "approved",
    );
  });

  it("is submitted otherwise", () => {
    expect(sheetStatus([card({ state: "submitted" }), card({ state: "approved" })])).toBe(
      "submitted",
    );
  });
});

describe("groupIntoSheets", () => {
  it("groups cards into weekly sheets", () => {
    const sheets = groupIntoSheets(
      [card({ id: "a", createdOn: "2026-06-01" }), card({ id: "b", createdOn: "2026-06-02" })],
      "user-1",
      "Test User",
    );
    expect(sheets).toHaveLength(1);
    expect(sheets[0].cards).toHaveLength(2);
  });

  it("splits cards from different weeks into separate sheets, newest first", () => {
    const sheets = groupIntoSheets(
      [card({ id: "a", createdOn: "2026-06-01" }), card({ id: "b", createdOn: "2026-06-15" })],
      "user-1",
      "Test User",
    );
    expect(sheets).toHaveLength(2);
    expect(sheets[0].cards[0].id).toBe("b");
  });

  it("skips a card with an unparseable createdOn instead of throwing, keeping the rest", () => {
    const sheets = groupIntoSheets(
      [
        card({ id: "good-1", createdOn: "2026-06-01" }),
        card({ id: "bad", createdOn: "" }),
        card({ id: "good-2", createdOn: "2026-06-02" }),
      ],
      "user-1",
      "Test User",
    );
    const ids = sheets.flatMap((s) => s.cards.map((c) => c.id));
    expect(ids).toEqual(expect.arrayContaining(["good-1", "good-2"]));
    expect(ids).not.toContain("bad");
    expect(ids).toHaveLength(2);
  });

  it("returns an empty list when every card has an unparseable createdOn", () => {
    const sheets = groupIntoSheets([card({ id: "bad", createdOn: "" })], "user-1", "Test User");
    expect(sheets).toEqual([]);
  });
});
