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
  computeEngagementsStatValues,
  computeEngagementsTotalItems,
  parseEngagementsSortField,
} from "@features/engagements/utils/engagements";

describe("parseEngagementsSortField", () => {
  it("returns CreatedOn for unknown values", () => {
    expect(parseEngagementsSortField("unknown")).toBe(
      EngagementsSortField.CreatedOn,
    );
  });

  it("passes through valid fields", () => {
    expect(parseEngagementsSortField(EngagementsSortField.State)).toBe(
      EngagementsSortField.State,
    );
  });

  it("maps legacy Severity sort to CreatedOn", () => {
    expect(parseEngagementsSortField(EngagementsSortField.Severity)).toBe(
      EngagementsSortField.CreatedOn,
    );
  });
});

describe("buildEngagementSearchRequest", () => {
  it("includes engagement case type and sort", () => {
    const req = buildEngagementSearchRequest(
      {},
      "",
      EngagementsSortField.CreatedOn,
      SortOrder.DESC,
    );
    expect(req.filters?.caseTypes).toEqual([CaseType.ENGAGEMENT]);
    expect(req.sortBy?.field).toBe(EngagementsSortField.CreatedOn);
    expect(req.sortBy?.order).toBe(SortOrder.DESC);
  });

  it("does not send severity filter", () => {
    const req = buildEngagementSearchRequest(
      { severityId: "99" },
      "",
      EngagementsSortField.CreatedOn,
      SortOrder.DESC,
    );
    expect(req.filters?.severityId).toBeUndefined();
  });

  it("normalizes legacy Severity sort field to CreatedOn in API payload", () => {
    const req = buildEngagementSearchRequest(
      {},
      "",
      EngagementsSortField.Severity,
      SortOrder.DESC,
    );
    expect(req.sortBy?.field).toBe(EngagementsSortField.CreatedOn);
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
});
