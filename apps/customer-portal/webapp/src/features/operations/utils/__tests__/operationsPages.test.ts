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
import {
  buildChangeRequestSearchRequest,
  buildServiceRequestsPageCaseSearchRequest,
  flattenChangeRequestInfinitePages,
  formatOperationsOverviewServiceRequestsSubtitle,
  getOperationsNavSegment,
  resolveActionRequiredCrStateIds,
  resolveAllowedCrStateIds,
  resolveChangeRequestFilterListOptions,
  resolveClosedCrStateIds,
  resolveOutstandingCrStateIds,
  resolveScheduledCrStateIds,
} from "@features/operations/utils/operationsPages";
import { ChangeRequestFilterDefinitionId } from "@features/operations/types/changeRequests";
import {
  OperationsNavSegment,
  ServiceRequestCaseSortField,
} from "@features/operations/types/serviceRequests";

describe("getOperationsNavSegment", () => {
  it("returns operations when path includes /operations/", () => {
    expect(getOperationsNavSegment("/projects/x/operations/service-requests")).toBe(
      OperationsNavSegment.Operations,
    );
  });

  it("returns support otherwise", () => {
    expect(getOperationsNavSegment("/projects/x/support/service-requests")).toBe(
      OperationsNavSegment.Support,
    );
  });
});

describe("resolveOutstandingCrStateIds", () => {
  it("returns undefined when changeRequestStates is undefined", () => {
    expect(resolveOutstandingCrStateIds(undefined)).toBeUndefined();
  });

  it("excludes Rollback, Closed, and Canceled by label", () => {
    const states = [
      { id: "5", label: "Customer Approval" },
      { id: "-2", label: "Scheduled" },
      { id: "-1", label: "Implement" },
      { id: "0", label: "Review" },
      { id: "1", label: "Customer Review" },
      { id: "2", label: "Rollback" },
      { id: "3", label: "Closed" },
      { id: "4", label: "Canceled" },
    ];
    expect(resolveOutstandingCrStateIds(states)).toEqual([5, -2, -1, 0, 1]);
  });

  it("returns all IDs when no excluded labels are present", () => {
    const states = [
      { id: "-5", label: "New" },
      { id: "-4", label: "Assess" },
    ];
    expect(resolveOutstandingCrStateIds(states)).toEqual([-5, -4]);
  });
});

describe("resolveAllowedCrStateIds", () => {
  it("returns undefined when changeRequestStates is undefined", () => {
    expect(resolveAllowedCrStateIds(undefined)).toBeUndefined();
  });

  it("excludes New, Assess, and Authorize by label", () => {
    const states = [
      { id: "-5", label: "New" },
      { id: "-4", label: "Assess" },
      { id: "-3", label: "Authorize" },
      { id: "5", label: "Customer Approval" },
      { id: "-2", label: "Scheduled" },
      { id: "3", label: "Closed" },
    ];
    expect(resolveAllowedCrStateIds(states)).toEqual([5, -2, 3]);
  });
});

describe("resolveActionRequiredCrStateIds", () => {
  it("returns undefined when changeRequestStates is undefined", () => {
    expect(resolveActionRequiredCrStateIds(undefined)).toBeUndefined();
  });

  it("returns only Customer Approval and Customer Review IDs", () => {
    const states = [
      { id: "5", label: "Customer Approval" },
      { id: "-2", label: "Scheduled" },
      { id: "1", label: "Customer Review" },
      { id: "3", label: "Closed" },
    ];
    expect(resolveActionRequiredCrStateIds(states)).toEqual([5, 1]);
  });
});

describe("resolveScheduledCrStateIds", () => {
  it("returns undefined when changeRequestStates is undefined", () => {
    expect(resolveScheduledCrStateIds(undefined)).toBeUndefined();
  });

  it("returns only Scheduled ID", () => {
    const states = [
      { id: "5", label: "Customer Approval" },
      { id: "-2", label: "Scheduled" },
      { id: "3", label: "Closed" },
    ];
    expect(resolveScheduledCrStateIds(states)).toEqual([-2]);
  });
});

describe("resolveClosedCrStateIds", () => {
  it("returns undefined when changeRequestStates is undefined", () => {
    expect(resolveClosedCrStateIds(undefined)).toBeUndefined();
  });

  it("returns only the Closed state ID", () => {
    const states = [
      { id: "5", label: "Customer Approval" },
      { id: "-2", label: "Scheduled" },
      { id: "3", label: "Closed" },
      { id: "4", label: "Canceled" },
    ];
    expect(resolveClosedCrStateIds(states)).toEqual([3]);
  });
});

describe("buildChangeRequestSearchRequest", () => {
  const allStates = [
    { id: "-5", label: "New" },
    { id: "-4", label: "Assess" },
    { id: "-3", label: "Authorize" },
    { id: "5", label: "Customer Approval" },
    { id: "-2", label: "Scheduled" },
    { id: "-1", label: "Implement" },
    { id: "0", label: "Review" },
    { id: "1", label: "Customer Review" },
    { id: "2", label: "Rollback" },
    { id: "3", label: "Closed" },
    { id: "4", label: "Canceled" },
  ];

  it("maps filters and search using allowed states from metadata", () => {
    const req = buildChangeRequestSearchRequest(
      { stateId: "2", impactId: "1" },
      " q ",
      false, false, false,
      allStates,
    );
    expect(req.filters?.stateKeys).toEqual([2]);
    expect(req.filters?.impactKey).toBe(1);
    expect(req.filters?.searchQuery).toBe("q");
  });

  it("returns empty stateKeys when changeRequestStates is not loaded", () => {
    const req = buildChangeRequestSearchRequest({}, "", true);
    expect(req.filters?.stateKeys).toEqual([]);
  });

  it("resolves outstanding state IDs from metadata when outstandingOnly is true", () => {
    const req = buildChangeRequestSearchRequest({}, "", true, false, false, allStates);
    expect(req.filters?.stateKeys).toEqual([5, -2, -1, 0, 1]);
  });

  it("resolves action-required state IDs from metadata", () => {
    const req = buildChangeRequestSearchRequest({}, "", false, true, false, allStates);
    expect(req.filters?.stateKeys).toEqual([5, 1]);
  });

  it("resolves scheduled state IDs from metadata", () => {
    const req = buildChangeRequestSearchRequest({}, "", false, false, true, allStates);
    expect(req.filters?.stateKeys).toEqual([-2]);
  });

  it("excludes New, Assess, Authorize from default allowed states", () => {
    const req = buildChangeRequestSearchRequest({}, "", false, false, false, allStates);
    expect(req.filters?.stateKeys).not.toContain(-5);
    expect(req.filters?.stateKeys).not.toContain(-4);
    expect(req.filters?.stateKeys).not.toContain(-3);
  });
});

describe("resolveChangeRequestFilterListOptions", () => {
  it("uses switch on filter definition id for impact labels", () => {
    const options = resolveChangeRequestFilterListOptions(
      {
        id: ChangeRequestFilterDefinitionId.Impact,
        filterKey: "impactId",
        metadataKey: "changeRequestImpacts",
      },
      {
        changeRequestImpacts: [{ id: "1", label: "high" }],
      } as never,
    );
    expect(options[0]?.value).toBe("1");
    expect(options).toHaveLength(1);
  });

  it("passes through state options", () => {
    const options = resolveChangeRequestFilterListOptions(
      {
        id: ChangeRequestFilterDefinitionId.State,
        filterKey: "stateId",
        metadataKey: "changeRequestStates",
      },
      {
        changeRequestStates: [{ id: "9", label: "Scheduled" }],
      } as never,
    );
    expect(options).toEqual([{ label: "Scheduled", value: "9" }]);
  });
});

describe("flattenChangeRequestInfinitePages", () => {
  it("flattens pages", () => {
    expect(
      flattenChangeRequestInfinitePages([
        { changeRequests: [{ id: "a" } as never] },
        { changeRequests: [{ id: "b" } as never] },
      ]),
    ).toHaveLength(2);
  });
});

describe("buildServiceRequestsPageCaseSearchRequest", () => {
  it("sets service request case type and sort", () => {
    const req = buildServiceRequestsPageCaseSearchRequest(
      {},
      "",
      ServiceRequestCaseSortField.CreatedOn,
      SortOrder.DESC,
      false,
    );
    expect(req.filters?.caseTypes).toEqual([CaseType.SERVICE_REQUEST]);
    expect(req.sortBy?.field).toBe("createdOn");
  });

  it("does not send severity filter", () => {
    const req = buildServiceRequestsPageCaseSearchRequest(
      { severityId: "99" },
      "",
      ServiceRequestCaseSortField.CreatedOn,
      SortOrder.DESC,
      false,
    );
    expect(req.filters?.severityId).toBeUndefined();
  });

  it("normalizes legacy Severity sort field to CreatedOn in API payload", () => {
    const req = buildServiceRequestsPageCaseSearchRequest(
      {},
      "",
      ServiceRequestCaseSortField.Severity,
      SortOrder.DESC,
      false,
    );
    expect(req.sortBy?.field).toBe(ServiceRequestCaseSortField.CreatedOn);
  });
});

describe("formatOperationsOverviewServiceRequestsSubtitle", () => {
  it("includes limit", () => {
    expect(formatOperationsOverviewServiceRequestsSubtitle(5)).toBe(
      "Latest 5 service requests",
    );
  });
});
