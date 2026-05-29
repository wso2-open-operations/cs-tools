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
import { SortOrder } from "@/types/common";
import { CaseType } from "@features/support/constants/supportConstants";
import { EngagementsSortField } from "@features/engagements/types/engagements";
import {
  buildEngagementDetailPath,
  buildEngagementSearchRequest,
  computeEngagementsCasesAreaLoading,
  computeEngagementsInitialPageLoading,
  computeEngagementsStatValues,
  computeEngagementsStatsLoading,
  computeEngagementsTotalItems,
  computeEngagementsTotalPages,
  getEngagementsCurrentPageCases,
  parseEngagementsSortField,
} from "@features/engagements/utils/engagements";
import {
  SUPPORT_STATE_AWAITING_INFO,
  SUPPORT_STATE_CLOSED,
  SUPPORT_STATE_WAITING_ON_WSO2,
} from "@features/support/constants/supportConstants";

describe("parseEngagementsSortField", () => {
  it("returns UpdatedOn for unknown values", () => {
    expect(parseEngagementsSortField("unknown")).toBe(
      EngagementsSortField.UpdatedOn,
    );
  });

  it("passes through valid fields", () => {
    expect(parseEngagementsSortField(EngagementsSortField.State)).toBe(
      EngagementsSortField.State,
    );
  });

  it("maps legacy Severity sort to UpdatedOn", () => {
    expect(parseEngagementsSortField(EngagementsSortField.Severity)).toBe(
      EngagementsSortField.UpdatedOn,
    );
  });
});

describe("buildEngagementSearchRequest", () => {
  it("includes engagement case type and sort", () => {
    const req = buildEngagementSearchRequest(
      {},
      "",
      EngagementsSortField.UpdatedOn,
      SortOrder.DESC,
    );
    expect(req.filters?.caseTypes).toEqual([CaseType.ENGAGEMENT]);
    expect(req.sortBy?.field).toBe(EngagementsSortField.UpdatedOn);
    expect(req.sortBy?.order).toBe(SortOrder.DESC);
  });

  it("does not send severity filter", () => {
    const req = buildEngagementSearchRequest(
      { severityIds: ["99"] },
      "",
      EngagementsSortField.CreatedOn,
      SortOrder.DESC,
    );
    expect(req.filters?.severityIds).toBeUndefined();
  });

  it("normalizes legacy Severity sort field to UpdatedOn in API payload", () => {
    const req = buildEngagementSearchRequest(
      {},
      "",
      EngagementsSortField.Severity,
      SortOrder.DESC,
    );
    expect(req.sortBy?.field).toBe(EngagementsSortField.UpdatedOn);
  });

  it("maps status and engagement type filters", () => {
    const req = buildEngagementSearchRequest(
      { statusIds: ["1", "2"], engagementTypeKey: "10,20" },
      "  query ",
      EngagementsSortField.State,
      SortOrder.ASC,
    );
    expect(req.filters?.statusIds).toEqual([1, 2]);
    expect(req.filters?.engagementTypeKeys).toEqual([10, 20]);
    expect(req.filters?.searchQuery).toBe("query");
    expect(req.sortBy?.field).toBe(EngagementsSortField.State);
  });
});

describe("buildEngagementDetailPath", () => {
  it("builds project engagement route", () => {
    expect(buildEngagementDetailPath("p1", "c1")).toBe(
      "/projects/p1/engagements/c1",
    );
  });
});

describe("computeEngagementsTotalItems", () => {
  it("prefers API total when positive", () => {
    expect(computeEngagementsTotalItems(100, 5)).toBe(100);
  });

  it("falls back to row count when API total is zero", () => {
    expect(computeEngagementsTotalItems(0, 3)).toBe(3);
  });
});

describe("computeEngagementsStatValues", () => {
  it("returns empty object when stats undefined", () => {
    expect(computeEngagementsStatValues(undefined)).toEqual({});
  });

  it("maps total, active, completed, and on hold from state counts", () => {
    const values = computeEngagementsStatValues({
      totalCount: 10,
      activeCount: 4,
      stateCount: [
        { label: SUPPORT_STATE_CLOSED, count: 3 },
        { label: SUPPORT_STATE_AWAITING_INFO, count: 1 },
        { label: SUPPORT_STATE_WAITING_ON_WSO2, count: 2 },
      ],
    } as never);
    expect(values).toEqual({
      total: 10,
      active: 4,
      completed: 3,
      onHold: 3,
    });
  });
});

describe("loading helpers", () => {
  it("computeEngagementsStatsLoading when query loading or missing response", () => {
    expect(computeEngagementsStatsLoading(true, false, "p1")).toBe(true);
    expect(computeEngagementsStatsLoading(false, false, "p1")).toBe(true);
    expect(computeEngagementsStatsLoading(false, true, "p1")).toBe(false);
  });

  it("computeEngagementsCasesAreaLoading mirrors stats loading pattern", () => {
    expect(computeEngagementsCasesAreaLoading(true, false, "p1")).toBe(true);
    expect(computeEngagementsCasesAreaLoading(false, true, "p1")).toBe(false);
  });

  it("computeEngagementsInitialPageLoading when either area loads", () => {
    expect(computeEngagementsInitialPageLoading(true, false)).toBe(true);
    expect(computeEngagementsInitialPageLoading(false, true)).toBe(true);
    expect(computeEngagementsInitialPageLoading(false, false)).toBe(false);
  });
});

describe("getEngagementsCurrentPageCases", () => {
  it("returns cases for the requested 1-based page", () => {
    const data = {
      pages: [{ cases: [{ id: "a" }], totalRecords: 1 }],
      pageParams: [undefined],
    } as never;
    expect(getEngagementsCurrentPageCases(data, 1)).toEqual([{ id: "a" }]);
    expect(getEngagementsCurrentPageCases(data, 2)).toEqual([]);
  });

  it("returns empty array when data missing", () => {
    expect(getEngagementsCurrentPageCases(undefined, 1)).toEqual([]);
  });
});

describe("computeEngagementsTotalPages", () => {
  it("computes page count from total and page size", () => {
    expect(computeEngagementsTotalPages(25, 10)).toBe(3);
    expect(computeEngagementsTotalPages(0, 10)).toBe(0);
  });
});
