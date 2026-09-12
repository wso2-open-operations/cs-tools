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
  buildIncidentSearchFilters,
  countActiveIncidentFilters,
  DEFAULT_INCIDENT_FILTERS,
  getLegalNextIncidentStates,
  type IncidentFilters,
} from "@features/csm-operations/utils/incidents";
import type { BeIncidentState } from "@api/backend/types";

describe("getLegalNextIncidentStates", () => {
  it.each([
    ["NEW", ["NEW", "IN_PROGRESS", "CANCELLED"]],
    ["IN_PROGRESS", ["IN_PROGRESS", "ON_HOLD", "RESOLVED", "CANCELLED"]],
    ["ON_HOLD", ["ON_HOLD", "IN_PROGRESS", "CANCELLED"]],
    ["RESOLVED", ["RESOLVED", "CLOSED", "IN_PROGRESS"]],
    ["CLOSED", ["CLOSED"]],
    ["CANCELLED", ["CANCELLED"]],
  ] as const)("from %s returns %j", (current, expected) => {
    expect(getLegalNextIncidentStates(current as BeIncidentState)).toEqual(expected);
  });

  it("always includes the current state even for terminal states", () => {
    expect(getLegalNextIncidentStates("CLOSED")).toContain("CLOSED");
    expect(getLegalNextIncidentStates("CANCELLED")).toContain("CANCELLED");
  });

  it("treats CLOSED and CANCELLED as terminal (no outgoing transitions)", () => {
    expect(getLegalNextIncidentStates("CLOSED")).toHaveLength(1);
    expect(getLegalNextIncidentStates("CANCELLED")).toHaveLength(1);
  });
});

describe("buildIncidentSearchFilters", () => {
  it("returns an empty object for the default filters and no search", () => {
    expect(buildIncidentSearchFilters(DEFAULT_INCIDENT_FILTERS, "")).toEqual({});
  });

  it("omits slaViolated entirely when the toggle is off", () => {
    const filters: IncidentFilters = { ...DEFAULT_INCIDENT_FILTERS, slaViolated: false };
    const built = buildIncidentSearchFilters(filters, "");
    expect(built).not.toHaveProperty("slaViolated");
    expect(Object.keys(built)).toEqual([]);
  });

  it("sends slaViolated: true, never false, when the toggle is on", () => {
    const filters: IncidentFilters = { ...DEFAULT_INCIDENT_FILTERS, slaViolated: true };
    expect(buildIncidentSearchFilters(filters, "")).toEqual({ slaViolated: true });
  });

  it("produces exact inclusive UTC bounds for a whole-day range", () => {
    // Verified against the real data source (see the API description on
    // BeIncidentSearchPayload): an inclusive May 2026 range is
    // 2026-05-01T00:00:00Z .. 2026-05-31T23:59:59Z, not the next midnight —
    // the upstream date API silently truncates a bound to date-only, so
    // T00:00:00Z of 2026-06-01 would drop all of May 31st without erroring.
    const filters: IncidentFilters = {
      ...DEFAULT_INCIDENT_FILTERS,
      createdStartDate: "2026-05-01",
      createdEndDate: "2026-05-31",
    };
    expect(buildIncidentSearchFilters(filters, "")).toEqual({
      startCreatedDate: "2026-05-01T00:00:00Z",
      endCreatedDate: "2026-05-31T23:59:59Z",
    });
  });

  it("includes only the start bound when only createdStartDate is set", () => {
    const filters: IncidentFilters = { ...DEFAULT_INCIDENT_FILTERS, createdStartDate: "2026-05-01" };
    const built = buildIncidentSearchFilters(filters, "");
    expect(built).toEqual({ startCreatedDate: "2026-05-01T00:00:00Z" });
    expect(built).not.toHaveProperty("endCreatedDate");
  });

  it("includes productNames only when at least one product is selected", () => {
    expect(buildIncidentSearchFilters(DEFAULT_INCIDENT_FILTERS, "")).not.toHaveProperty(
      "productNames",
    );
    const filters: IncidentFilters = {
      ...DEFAULT_INCIDENT_FILTERS,
      products: ["Choreo", "Asgardeo"],
    };
    expect(buildIncidentSearchFilters(filters, "")).toEqual({
      productNames: ["Choreo", "Asgardeo"],
    });
  });

  it("includes priorities and searchQuery alongside the new filters", () => {
    const filters: IncidentFilters = {
      search: "",
      priorities: ["CRITICAL", "HIGH"],
      slaViolated: true,
      createdStartDate: "2026-05-01",
      createdEndDate: "2026-05-31",
      products: ["Choreo"],
      sreTeamIds: [],
    };
    expect(buildIncidentSearchFilters(filters, "timeout")).toEqual({
      searchQuery: "timeout",
      priorities: ["CRITICAL", "HIGH"],
      slaViolated: true,
      startCreatedDate: "2026-05-01T00:00:00Z",
      endCreatedDate: "2026-05-31T23:59:59Z",
      productNames: ["Choreo"],
    });
  });

  it("sends selected SRE teams as an assignmentGroupId/in generic filter entry", () => {
    const filters: IncidentFilters = {
      ...DEFAULT_INCIDENT_FILTERS,
      sreTeamIds: ["team-apollo", "team-atlas"],
    };
    expect(buildIncidentSearchFilters(filters, "")).toEqual({
      filters: [{ field: "assignmentGroupId", op: "in", values: ["team-apollo", "team-atlas"] }],
    });
  });

  it("omits the generic filters array entirely when no SRE team is selected", () => {
    expect(buildIncidentSearchFilters(DEFAULT_INCIDENT_FILTERS, "")).not.toHaveProperty(
      "filters",
    );
  });
});

describe("countActiveIncidentFilters", () => {
  it("counts 0 for the defaults", () => {
    expect(countActiveIncidentFilters(DEFAULT_INCIDENT_FILTERS)).toBe(0);
  });

  it("counts each of the new filters independently", () => {
    expect(
      countActiveIncidentFilters({ ...DEFAULT_INCIDENT_FILTERS, slaViolated: true }),
    ).toBe(1);
    expect(
      countActiveIncidentFilters({ ...DEFAULT_INCIDENT_FILTERS, createdStartDate: "2026-05-01" }),
    ).toBe(1);
    expect(
      countActiveIncidentFilters({ ...DEFAULT_INCIDENT_FILTERS, createdEndDate: "2026-05-31" }),
    ).toBe(1);
    expect(
      countActiveIncidentFilters({ ...DEFAULT_INCIDENT_FILTERS, products: ["Choreo"] }),
    ).toBe(1);
    expect(
      countActiveIncidentFilters({ ...DEFAULT_INCIDENT_FILTERS, sreTeamIds: ["team-apollo"] }),
    ).toBe(1);
    expect(
      countActiveIncidentFilters({
        ...DEFAULT_INCIDENT_FILTERS,
        slaViolated: true,
        createdStartDate: "2026-05-01",
        createdEndDate: "2026-05-31",
        products: ["Choreo"],
        priorities: ["HIGH"],
        sreTeamIds: ["team-apollo"],
      }),
    ).toBe(6);
  });
});
