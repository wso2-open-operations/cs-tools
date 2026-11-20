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

import { afterEach, describe, expect, it, vi } from "vitest";
import {
  buildSearchAccountsRequest,
  buildSearchProjectsRequest,
  toSearchAccountsResult,
  toSearchProjectsResult,
} from "@features/csm-custom-dashboards/api/codeDashboardCustomers";

describe("buildSearchAccountsRequest", () => {
  it("reads the first 10 accounts by default", () => {
    expect(buildSearchAccountsRequest(undefined)).toEqual({ filters: {}, pagination: { offset: 0, limit: 10 } });
  });

  it("passes every filter through, the team as abtTeam", () => {
    expect(
      buildSearchAccountsRequest({
        lifeCycle: "Customer",
        team: "rigel",
        risks: ["hasEolProduct", "hasRecentEscalations"],
        healthStatus: "at_risk",
        hasProject: true,
        active: false,
        limit: 1,
      }),
    ).toEqual({
      filters: {
        lifeCycle: "Customer",
        abtTeam: "rigel",
        risks: ["hasEolProduct", "hasRecentEscalations"],
        healthStatus: "at_risk",
        hasProject: true,
        active: false,
      },
      pagination: { offset: 0, limit: 1 },
    });
  });

  it.each([
    [{ lifeCycle: "customer" }, /lifeCycle must be one of Customer, Partner/],
    [{ lifeCycle: ["Customer"] }, /lifeCycle must be one of/],
    [{ team: ["rigel"] }, /team must be one team key/],
    [{ team: "" }, /team must be one team key/],
    [{ risks: [] }, /risks must be a non-empty list of hasNoGoLive/],
    [{ risks: ["atRisk"] }, /risks must be a non-empty list/],
    [{ healthStatus: "At Risk" }, /healthStatus must be one of at_risk, healthy, to_be_reviewed/],
    [{ active: "true" }, /active must be true or false/],
    [{ hasProject: 1 }, /hasProject must be true or false/],
    [{ limit: 0 }, /limit must be an integer from 1 to 50/],
    [{ sortBy: { field: "name" } }, /unknown argument "sortBy"/],
    [{ region: "NA" }, /unknown argument "region"/],
  ])("refuses %j", (args, message) => {
    expect(() => buildSearchAccountsRequest(args)).toThrow(message);
  });
});

describe("toSearchAccountsResult", () => {
  it("keeps plain values, people and teams by name", () => {
    expect(
      toSearchAccountsResult({
        total: 6,
        accounts: [
          {
            id: "a1",
            name: "Summit Enterprises",
            number: "ACC-2000",
            supportTier: null,
            region: "NA",
            creTeam: { name: "Rigel" },
            sreTeam: null,
            accountManager: { name: "Nico Rivera" },
          },
        ],
      }),
    ).toEqual({
      total: 6,
      items: [
        {
          id: "a1",
          name: "Summit Enterprises",
          number: "ACC-2000",
          tier: null,
          region: "NA",
          country: null,
          creTeam: "Rigel",
          sreTeam: null,
          accountManager: "Nico Rivera",
        },
      ],
    });
    expect(toSearchAccountsResult({ accounts: null })).toEqual({ total: 0, items: [] });
  });
});

describe("buildSearchProjectsRequest", () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  it("sends flat filters beside the page, and a date sort soonest first", () => {
    expect(
      buildSearchProjectsRequest({
        onboardingStatus: ["In-Progress"],
        excludeClosureStates: ["Suspended", "Restricted"],
        endDateFrom: "2026-11-01",
        endDateTo: "2026-12-31",
        sortBy: { field: "endDate" },
        limit: 20,
      }),
    ).toEqual({
      onboardingStatus: ["In-Progress"],
      excludeClosureStates: ["Suspended", "Restricted"],
      endDateFrom: "2026-11-01",
      endDateTo: "2026-12-31",
      sortBy: "endDate",
      sortOrder: "asc",
      pagination: { offset: 0, limit: 20 },
    });
    expect(buildSearchProjectsRequest({ closureStatus: "Open", closureStates: ["Suspended"] })).toEqual({
      closureStatus: "Open",
      closureStates: ["Suspended"],
      pagination: { offset: 0, limit: 10 },
    });
  });

  it("resolves date tokens to the dates the search takes", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date(2026, 10, 12, 12));
    expect(buildSearchProjectsRequest({ goLivePlanDateFrom: "__startOfMonth:0__", goLivePlanDateTo: "__endOfMonth:0__" })).toEqual({
      goLivePlanDateFrom: "2026-11-01",
      goLivePlanDateTo: "2026-11-30",
      pagination: { offset: 0, limit: 10 },
    });
  });

  it.each([
    [{ onboardingStatus: ["In Progress"] }, /onboardingStatus must be a non-empty list of Not-Started/],
    [{ onboardingStatus: "In-Progress" }, /onboardingStatus must be a non-empty list/],
    [{ closureStatus: "open" }, /closureStatus must be one of Open, Suspended, Restricted/],
    [{ closureStates: [] }, /closureStates must be a non-empty list/],
    [{ excludeClosureStates: ["Closed"] }, /excludeClosureStates must be a non-empty list/],
    [{ endDateFrom: "December" }, /endDateFrom must be a date/],
    [{ goLiveDateTo: 20261231 }, /goLiveDateTo must be a date/],
    [{ endDate: "2026-12-31" }, /unknown argument "endDate"/],
    [{ sortBy: { field: "onboardingStartDate" } }, /sortBy.field must be one of endDate, goLivePlanDate/],
    [{ sortBy: { field: "endDate", order: "soonest" } }, /sortBy.order must be/],
    [{ limit: 51 }, /limit must be an integer from 1 to 50/],
  ])("refuses %j", (args, message) => {
    expect(() => buildSearchProjectsRequest(args)).toThrow(message);
  });
});

describe("toSearchProjectsResult", () => {
  it("keeps plain values, the account by name and the active case count", () => {
    expect(
      toSearchProjectsResult({
        total: 7,
        projects: [
          {
            id: "p1",
            key: "GEN01-PROJ1",
            name: "Summit Enterprises Production",
            account: { name: "Summit Enterprises" },
            onboardingStatus: "In-Progress",
            closureState: null,
            startDate: "2025-11-06T00:00:00Z",
            activeCasesCount: 7,
          },
          { id: "p2" },
        ],
      }),
    ).toEqual({
      total: 7,
      items: [
        {
          id: "p1",
          key: "GEN01-PROJ1",
          name: "Summit Enterprises Production",
          account: "Summit Enterprises",
          onboardingStatus: "In-Progress",
          closureState: null,
          startDate: "2025-11-06T00:00:00Z",
          endDate: null,
          goLiveDate: null,
          activeCases: 7,
        },
        {
          id: "p2",
          key: null,
          name: null,
          account: null,
          onboardingStatus: null,
          closureState: null,
          startDate: null,
          endDate: null,
          goLiveDate: null,
          activeCases: 0,
        },
      ],
    });
  });
});
