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
import {
  ChangeRequestFilterDefinitionId,
  ChangeRequestSortField,
} from "@features/operations/types/changeRequests";
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

  it("offers every state the server's filters carry, Authorize included, and hides none itself", () => {
    // What GET /projects/{id}/filters sends: New and Assess are left out by the
    // server (nothing a customer can see is ever in them), Authorize is in.
    const states = [
      { id: "-3", label: "Authorize" },
      { id: "5", label: "Customer Approval" },
      { id: "-2", label: "Scheduled" },
      { id: "3", label: "Closed" },
    ];
    expect(resolveAllowedCrStateIds(states)).toEqual([-3, 5, -2, 3]);
  });

  it("does not second-guess the server: a New or Assess entry in the filters is passed on, not dropped", () => {
    const states = [
      { id: "-5", label: "New" },
      { id: "-4", label: "Assess" },
      { id: "5", label: "Customer Approval" },
    ];
    expect(resolveAllowedCrStateIds(states)).toEqual([-5, -4, 5]);
  });

  it("never sends an id that is not a number (JSON turns NaN into null, which the API reads as state 0)", () => {
    const states = [
      { id: "AUTHORIZE", label: "AUTHORIZE" },
      { id: "5", label: "Customer Approval" },
    ];
    expect(resolveAllowedCrStateIds(states)).toEqual([5]);
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
      { stateIds: ["2"], impactIds: ["1"] },
      " q ",
      false, false, false,
      allStates,
    );
    expect(req.filters?.stateKeys).toEqual([2]);
    expect(req.filters?.impactKeys).toEqual([1]);
    expect(req.filters?.searchQuery).toBe("q");
  });

  it("returns empty stateKeys when changeRequestStates is not loaded", () => {
    const req = buildChangeRequestSearchRequest({}, "", true);
    expect(req.filters?.stateKeys).toEqual([]);
  });

  it("resolves outstanding state IDs from metadata when outstandingOnly is true, Authorize among them", () => {
    const req = buildChangeRequestSearchRequest({}, "", true, false, false, allStates);
    expect(req.filters?.stateKeys).toEqual([-5, -4, -3, 5, -2, -1, 0, 1]);
    // A change request waiting in Authorize after the customer proposed a new time
    // is still outstanding for them: not Closed, Canceled or Rollback.
    expect(req.filters?.stateKeys).toContain(-3);
  });

  it("resolves action-required state IDs from metadata", () => {
    const req = buildChangeRequestSearchRequest({}, "", false, true, false, allStates);
    expect(req.filters?.stateKeys).toEqual([5, 1]);
  });

  it("resolves scheduled state IDs from metadata", () => {
    const req = buildChangeRequestSearchRequest({}, "", false, false, true, allStates);
    expect(req.filters?.stateKeys).toEqual([-2]);
  });

  it("asks for every state the filters carry by default, hiding none (a designated change request in Authorize must show)", () => {
    const req = buildChangeRequestSearchRequest({}, "", false, false, false, allStates);
    expect(req.filters?.stateKeys).toEqual([-5, -4, -3, 5, -2, -1, 0, 1, 2, 3, 4]);
    expect(req.filters?.stateKeys).toContain(-3);
  });

  it("with the filters the server really sends (no New, no Assess) the default view is every state a customer's change request can be in", () => {
    const offered = allStates.filter((s) => s.label !== "New" && s.label !== "Assess");
    const req = buildChangeRequestSearchRequest({}, "", false, false, false, offered);
    expect(req.filters?.stateKeys).toEqual([-3, 5, -2, -1, 0, 1, 2, 3, 4]);
  });

  it("lets the customer filter by Authorize and keeps only selections the filters offer", () => {
    const offered = allStates.filter((s) => s.label !== "New" && s.label !== "Assess");
    const authorizeOnly = buildChangeRequestSearchRequest({ stateIds: ["-3"] }, "", false, false, false, offered);
    expect(authorizeOnly.filters?.stateKeys).toEqual([-3]);
    // A stale selection of a state the filters no longer carry is dropped, not sent.
    const stale = buildChangeRequestSearchRequest({ stateIds: ["-5", "5"] }, "", false, false, false, offered);
    expect(stale.filters?.stateKeys).toEqual([5]);
  });

  it("with the filters the previous system's data source sends (no New, Assess or Authorize) no view ever names one of the three", () => {
    // The page no longer hides New / Assess / Authorize itself: the API's filter options
    // leave them out wherever a customer is never shown them (every change request is
    // visible there except in those three states), so what the page asks for is what the
    // server may return. The server applies the same line to a request that names or
    // omits a state differently (entity-service, the section on the previous system's data source).
    const offered = allStates.filter((s) => !["New", "Assess", "Authorize"].includes(s.label));
    const hidden = [-5, -4, -3];
    const views: Array<[string, boolean, boolean, boolean]> = [
      ["default", false, false, false],
      ["outstanding", true, false, false],
      ["action required", false, true, false],
      ["scheduled", false, false, true],
    ];
    for (const [name, outstanding, actionRequired, scheduled] of views) {
      const req = buildChangeRequestSearchRequest({}, "", outstanding, actionRequired, scheduled, offered);
      expect(req.filters?.stateKeys?.some((k) => hidden.includes(k)), name).toBe(false);
    }
    // A stale selection of Authorize (kept from a deployment that offered it) is dropped, not sent.
    const stale = buildChangeRequestSearchRequest({ stateIds: ["-3", "5"] }, "", false, false, false, offered);
    expect(stale.filters?.stateKeys).toEqual([5]);
  });

  it("sorts by updatedOn descending by default", () => {
    const req = buildChangeRequestSearchRequest({}, "", false, false, false, allStates);
    expect(req.sortBy?.field).toBe("updatedOn");
    expect(req.sortBy?.order).toBe("desc");
  });

  it("applies custom sort field and order", () => {
    const req = buildChangeRequestSearchRequest(
      {},
      "",
      false,
      false,
      false,
      allStates,
      ChangeRequestSortField.CreatedOn,
      SortOrder.ASC,
    );
    expect(req.sortBy?.field).toBe("createdOn");
    expect(req.sortBy?.order).toBe("asc");
  });
});

describe("resolveChangeRequestFilterListOptions", () => {
  it("uses switch on filter definition id for impact labels", () => {
    const options = resolveChangeRequestFilterListOptions(
      {
        id: ChangeRequestFilterDefinitionId.Impact,
        filterKey: "impactIds",
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
        filterKey: "stateIds",
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
      ServiceRequestCaseSortField.UpdatedOn,
      SortOrder.DESC,
      false,
    );
    expect(req.filters?.caseTypes).toEqual([CaseType.SERVICE_REQUEST]);
    expect(req.sortBy?.field).toBe("updatedOn");
  });

  it("does not send severity filter", () => {
    const req = buildServiceRequestsPageCaseSearchRequest(
      { severityIds: ["99"] },
      "",
      ServiceRequestCaseSortField.CreatedOn,
      SortOrder.DESC,
      false,
    );
    expect(req.filters?.severityIds).toBeUndefined();
  });

  it("normalizes legacy Severity sort field to UpdatedOn in API payload", () => {
    const req = buildServiceRequestsPageCaseSearchRequest(
      {},
      "",
      ServiceRequestCaseSortField.Severity,
      SortOrder.DESC,
      false,
    );
    expect(req.sortBy?.field).toBe(ServiceRequestCaseSortField.UpdatedOn);
  });
});

describe("formatOperationsOverviewServiceRequestsSubtitle", () => {
  it("includes limit", () => {
    expect(formatOperationsOverviewServiceRequestsSubtitle(5)).toBe(
      "Latest 5 service requests",
    );
  });
});
