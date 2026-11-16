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

import { describe, expect, it, vi } from "vitest";
import {
  buildSearchCustomerHealthRequest,
  createNameLookup,
  toSearchCustomerHealthResult,
} from "@features/csm-custom-dashboards/api/codeDashboardCustomerHealth";

const ME = "jane.doe@example.com";

describe("buildSearchCustomerHealthRequest", () => {
  it("reads the first 10 accounts by default", () => {
    expect(buildSearchCustomerHealthRequest(undefined, ME)).toEqual({ offset: 0, limit: 10 });
  });

  it("passes every filter through, in the summary's own names", () => {
    expect(
      buildSearchCustomerHealthRequest(
        {
          mine: true,
          risks: ["hasEolProduct", "hasRecentEscalations"],
          healthStatus: "at_risk",
          product: "API Gateway",
          team: "rigel",
          subRegions: ["ANZ", "NA - EAST"],
          limit: 50,
        },
        ME,
      ),
    ).toEqual({
      email: ME,
      risks: "hasEolProduct,hasRecentEscalations",
      healthStatus: "at_risk",
      product: "API Gateway",
      abtTeam: "rigel",
      region: ["ANZ", "NA - EAST"],
      offset: 0,
      limit: 50,
    });
  });

  it("sends no email when mine is false", () => {
    expect(buildSearchCustomerHealthRequest({ mine: false, healthStatus: "healthy" }, ME)).toEqual({
      healthStatus: "healthy",
      offset: 0,
      limit: 10,
    });
  });

  it("refuses to_be_reviewed, saying how to count it", () => {
    expect(() => buildSearchCustomerHealthRequest({ healthStatus: "to_be_reviewed" }, ME)).toThrow(
      "searchCustomerHealth: healthStatus cannot be to_be_reviewed; count all accounts less at_risk and healthy",
    );
  });

  it.each([
    [{ owner: "x" }, 'searchCustomerHealth: unknown argument "owner"'],
    [{ mine: "yes" }, "mine must be true or false"],
    [{ risks: [] }, "risks must be a non-empty list of hasNoGoLive, hasGoneLive"],
    [{ risks: "hasEolProduct" }, "risks must be a non-empty list"],
    [{ risks: ["atRisk"] }, "risks must be a non-empty list"],
    [{ healthStatus: "At Risk" }, "healthStatus must be one of at_risk, healthy"],
    [{ product: ["API Gateway"] }, "product must be one name"],
    [{ product: " " }, "product must be one name"],
    [{ team: "" }, "team must be one name"],
    [{ subRegions: "ANZ" }, "subRegions must be a non-empty list of AFRICA, ANZ"],
    [{ subRegions: [] }, "subRegions must be a non-empty list"],
    [{ subRegions: ["APAC"] }, "subRegions must be a non-empty list"],
    [{ limit: 0 }, "limit must be an integer from 1 to 50"],
    [{ limit: 51 }, "limit must be an integer from 1 to 50"],
    [{ limit: 2.5 }, "limit must be an integer from 1 to 50"],
  ])("refuses %j", (args, message) => {
    expect(() => buildSearchCustomerHealthRequest(args, ME)).toThrow(message);
  });

  it("needs the signed-in user for mine", () => {
    expect(() => buildSearchCustomerHealthRequest({ mine: true }, undefined)).toThrow(
      "searchCustomerHealth: the signed-in user is not loaded",
    );
  });
});

describe("createNameLookup", () => {
  const TEAMS = ["Apollo", "Rigel", "SaaS Night cover"];

  it("returns the list's own spelling for a name in any case or punctuation", async () => {
    const lookup = createNameLookup(async () => TEAMS);
    await expect(lookup("searchCustomerHealth", "team", "rigel")).resolves.toBe("Rigel");
    await expect(lookup("searchCustomerHealth", "team", "saas-night-cover")).resolves.toBe("SaaS Night cover");
  });

  it("refuses a name not on the list, with the names that are", async () => {
    const lookup = createNameLookup(async () => TEAMS);
    await expect(lookup("searchCustomerHealth", "team", "Orion")).rejects.toThrow(
      "searchCustomerHealth: no team is called Orion; the teams are Apollo, Rigel, SaaS Night cover",
    );
  });

  it("reads the list once", async () => {
    const read = vi.fn().mockResolvedValue(TEAMS);
    const lookup = createNameLookup(read);
    await Promise.all([lookup("f", "team", "rigel"), lookup("f", "team", "apollo")]);
    await lookup("f", "team", "Rigel");
    expect(read).toHaveBeenCalledTimes(1);
  });

  it("reads the list again after a failed read", async () => {
    const read = vi.fn().mockRejectedValueOnce(new Error("Forbidden")).mockResolvedValueOnce(TEAMS);
    const lookup = createNameLookup(read);
    await expect(lookup("f", "team", "rigel")).rejects.toThrow("Forbidden");
    await expect(lookup("f", "team", "rigel")).resolves.toBe("Rigel");
    expect(read).toHaveBeenCalledTimes(2);
  });

  it("reads an empty answer as no names", async () => {
    await expect(createNameLookup(async () => null)("f", "product", "API Gateway")).rejects.toThrow(
      "f: no product is called API Gateway; the products are ",
    );
  });
});

describe("toSearchCustomerHealthResult", () => {
  it("flattens each account to its status and risk flags", () => {
    expect(
      toSearchCustomerHealthResult({
        totalCount: 5,
        data: [
          {
            accountSysId: "00000000-0000-0000-0000-000000000301",
            accountName: "Example Corp",
            hasNoGoLive: { status: "Hasn't Gone Live in 6 months", isRisk: true },
            hasRecentCases: true,
            hasEolProduct: true,
            hasAbandonedMigrations: true,
            hasMigrationDelays: true,
            hasRecentEscalations: true,
            noSupportCases6mo: true,
            healthStatus: "at_risk",
          },
        ],
      }),
    ).toEqual({
      total: 5,
      items: [
        {
          id: "00000000-0000-0000-0000-000000000301",
          name: "Example Corp",
          healthStatus: "at_risk",
          goLive: "Hasn't Gone Live in 6 months",
          noGoLive: true,
          hasRecentCases: true,
          noSupportCases6mo: true,
          hasEolProduct: true,
          hasAbandonedMigrations: true,
          hasMigrationDelays: true,
          hasRecentEscalations: true,
        },
      ],
    });
  });

  it("keeps each flag to its own field, and a go-live state that is no risk as no risk", () => {
    const [item] = toSearchCustomerHealthResult({
      data: [
        {
          accountSysId: "a2",
          hasNoGoLive: { status: "Has Gone Live", isRisk: false },
          hasRecentCases: true,
          hasEolProduct: false,
          hasAbandonedMigrations: true,
          hasMigrationDelays: false,
          hasRecentEscalations: true,
          noSupportCases6mo: false,
          healthStatus: "healthy",
        },
      ],
    }).items;
    expect(item).toMatchObject({
      goLive: "Has Gone Live",
      noGoLive: false,
      hasRecentCases: true,
      hasEolProduct: false,
      hasAbandonedMigrations: true,
      hasMigrationDelays: false,
      hasRecentEscalations: true,
      noSupportCases6mo: false,
    });
  });

  it("reads missing values as null or false, and an empty response as empty", () => {
    expect(toSearchCustomerHealthResult({ data: [{ accountSysId: "a1" }] })).toEqual({
      total: 1,
      items: [
        {
          id: "a1",
          name: null,
          healthStatus: null,
          goLive: null,
          noGoLive: false,
          hasRecentCases: false,
          noSupportCases6mo: false,
          hasEolProduct: false,
          hasAbandonedMigrations: false,
          hasMigrationDelays: false,
          hasRecentEscalations: false,
        },
      ],
    });
    expect(toSearchCustomerHealthResult({})).toEqual({ total: 0, items: [] });
  });
});
