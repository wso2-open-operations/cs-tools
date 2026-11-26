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
  buildProjectFilterChips,
  countActiveProjectFilters,
  EMPTY_PROJECT_FILTERS,
  projectFiltersToRequest,
  readProjectFilters,
  writeProjectFilters,
  type ProjectFilters,
} from "@features/csm-projects/utils/projectFilters";

const read = (qs: string): ProjectFilters => readProjectFilters(new URLSearchParams(qs));

describe("readProjectFilters", () => {
  it("is empty for a bare URL", () => {
    expect(read("")).toEqual(EMPTY_PROJECT_FILTERS);
  });

  it("drops unknown onboardingStatus values and keeps the valid ones", () => {
    expect(read("onboardingStatus=Bogus").onboardingStatus).toEqual([]);
    expect(read("onboardingStatus=Completed,Bogus%2CNot-Started").onboardingStatus).toEqual([
      "Completed",
      "Not-Started",
    ]);
  });

  it("keeps the dashboard click-through's onboardingStatus format working", () => {
    expect(read("onboardingStatus=In-Progress%2COnHold").onboardingStatus).toEqual(["In-Progress", "OnHold"]);
    expect(read("onboardingStatus=In-Progress,OnHold").onboardingStatus).toEqual(["In-Progress", "OnHold"]);
  });

  it("reads closure states, dropping values outside the enum", () => {
    expect(read("closureStates=Open,Bogus,Restricted").closureStates).toEqual(["Open", "Restricted"]);
    expect(read("excludeClosureStates=Suspended").excludeClosureStates).toEqual(["Suspended"]);
    expect(read("closureStatus=Open").closureStatus).toBe("Open");
    expect(read("closureStatus=Open,Suspended").closureStatus).toBe("");
  });

  it("reads each date bound and drops malformed or impossible dates", () => {
    const f = read(
      "goLiveDateFrom=2026-01-01&goLiveDateTo=2026-02-30&productionReadinessCompletionDateTo=2026-06-30&endDateFrom=yesterday",
    );
    expect(f.goLiveDateFrom).toBe("2026-01-01");
    expect(f.goLiveDateTo).toBe("");
    expect(f.productionReadinessCompletionDateTo).toBe("2026-06-30");
    expect(f.endDateFrom).toBe("");
  });

  it("reads regions and onboarding owners as plain lists, blanks dropped", () => {
    const f = read("regions=APAC,%20EU%20,&onboardingOwnerIds=u-1,u-2&onboardingOwnerEmails=owner@example.com");
    expect(f.regions).toEqual(["APAC", "EU"]);
    expect(f.onboardingOwnerIds).toEqual(["u-1", "u-2"]);
    expect(f.onboardingOwnerEmails).toEqual(["owner@example.com"]);
  });

  it("keeps a region holding a comma as one value through the URL", () => {
    const f: ProjectFilters = { ...EMPTY_PROJECT_FILTERS, regions: ["Europe, Middle East", "APAC", "100%"] };
    const params = writeProjectFilters(new URLSearchParams(), f);
    expect(readProjectFilters(params).regions).toEqual(["Europe, Middle East", "APAC", "100%"]);
    expect(read("regions=APAC,EU").regions).toEqual(["APAC", "EU"]);
  });

  it("drops an end date before its start", () => {
    const f = read("goLivePlanDateFrom=2026-03-01&goLivePlanDateTo=2026-02-01");
    expect(f.goLivePlanDateFrom).toBe("2026-03-01");
    expect(f.goLivePlanDateTo).toBe("");
  });
});

describe("writeProjectFilters / projectFiltersToRequest round trip", () => {
  const FULL: ProjectFilters = {
    onboardingStatus: ["In-Progress", "Completed"],
    closureStates: ["Open", "Suspended"],
    closureStatus: "Restricted",
    excludeClosureStates: ["Suspended"],
    regions: ["APAC", "EU"],
    onboardingOwnerIds: ["11111111-1111-1111-1111-111111111111"],
    onboardingOwnerEmails: ["owner@example.com"],
    endDateFrom: "2026-01-01",
    endDateTo: "2026-12-31",
    onboardingStartDateFrom: "2026-02-01",
    onboardingStartDateTo: "2026-02-28",
    goLivePlanDateFrom: "2026-03-01",
    goLivePlanDateTo: "2026-03-31",
    goLiveDateFrom: "2026-04-01",
    goLiveDateTo: "2026-04-30",
    onboardingExpiryDateFrom: "2026-05-01",
    onboardingExpiryDateTo: "2026-05-31",
    productionReadinessCompletionDateFrom: "2026-06-01",
    productionReadinessCompletionDateTo: "2026-06-30",
  };

  it("URL to state to URL is lossless for every field", () => {
    const params = writeProjectFilters(new URLSearchParams(), FULL);
    expect(readProjectFilters(params)).toEqual(FULL);
    expect(writeProjectFilters(new URLSearchParams(), readProjectFilters(params)).toString()).toBe(
      params.toString(),
    );
  });

  it("uses the request's own key names in the URL", () => {
    const params = writeProjectFilters(new URLSearchParams(), FULL);
    expect(params.get("onboardingStatus")).toBe("In-Progress,Completed");
    expect(params.get("closureStates")).toBe("Open,Suspended");
    expect(params.get("regions")).toBe("APAC,EU");
    expect(params.get("onboardingOwnerIds")).toBe("11111111-1111-1111-1111-111111111111");
    expect(params.get("onboardingOwnerEmails")).toBe("owner@example.com");
    expect(params.get("goLivePlanDateFrom")).toBe("2026-03-01");
    expect(params.get("productionReadinessCompletionDateTo")).toBe("2026-06-30");
  });

  it("maps state to the search request keys, omitting unset ones", () => {
    expect(projectFiltersToRequest(FULL)).toEqual(FULL);
    expect(projectFiltersToRequest(EMPTY_PROJECT_FILTERS)).toEqual({});
    expect(projectFiltersToRequest({ ...EMPTY_PROJECT_FILTERS, goLiveDateTo: "2026-04-30" })).toEqual({
      goLiveDateTo: "2026-04-30",
    });
  });

  it("clears owned params but leaves unrelated ones alone", () => {
    const params = writeProjectFilters(new URLSearchParams("keep=1&goLiveDateFrom=2026-01-01"), EMPTY_PROJECT_FILTERS);
    expect(params.toString()).toBe("keep=1");
  });
});

describe("countActiveProjectFilters", () => {
  it("counts each list once and each date column once, whichever bounds are set", () => {
    expect(countActiveProjectFilters(EMPTY_PROJECT_FILTERS)).toBe(0);
    expect(
      countActiveProjectFilters({
        ...EMPTY_PROJECT_FILTERS,
        closureStates: ["Open", "Suspended"],
        goLiveDateFrom: "2026-04-01",
        goLiveDateTo: "2026-04-30",
        goLivePlanDateTo: "2026-03-31",
      }),
    ).toBe(3);
    expect(
      countActiveProjectFilters({ ...EMPTY_PROJECT_FILTERS, regions: ["APAC", "EU"], onboardingOwnerIds: ["u-1"] }),
    ).toBe(2);
  });
});

describe("buildProjectFilterChips", () => {
  const f: ProjectFilters = { ...EMPTY_PROJECT_FILTERS, onboardingStatus: ["In-Progress", "OnHold"] };

  it("chips the onboarding status (it has no quick control) when the builder is closed, and removes it on its own", () => {
    const chips = buildProjectFilterChips(f, true);
    expect(chips.map((c) => c.label)).toEqual(["Onboarding status: In-Progress, OnHold"]);
    expect(chips[0].onRemove(f).onboardingStatus).toEqual([]);
  });

  it("leaves it to the builder row when the builder is open", () => {
    expect(buildProjectFilterChips(f, false)).toEqual([]);
  });

  it("chips the region and names the onboarding owners when their names are known", () => {
    const g: ProjectFilters = { ...EMPTY_PROJECT_FILTERS, regions: ["APAC"], onboardingOwnerIds: ["u-1", "u-2"] };
    const names = new Map([["u-1", "Jane Doe"], ["u-2", "John Roe"]]);
    const chips = buildProjectFilterChips(g, true, (id) => names.get(id));
    expect(chips.map((c) => c.label)).toEqual(["Region: APAC", "Onboarding owner: Jane Doe, John Roe"]);
    expect(chips[1].onRemove(g).onboardingOwnerIds).toEqual([]);
  });

  it("counts the onboarding owners while a name is unknown", () => {
    const g: ProjectFilters = { ...EMPTY_PROJECT_FILTERS, onboardingOwnerIds: ["u-1", "u-2"] };
    expect(buildProjectFilterChips(g, true, () => undefined).map((c) => c.label)).toEqual([
      "Onboarding owner: 2 selected",
    ]);
  });

  it("always chips an owner email from a dashboard click-through (the builder has no row for it)", () => {
    const g: ProjectFilters = { ...EMPTY_PROJECT_FILTERS, onboardingOwnerEmails: ["owner@example.com"] };
    expect(buildProjectFilterChips(g, false).map((c) => c.label)).toEqual(["Onboarding owner: owner@example.com"]);
  });
});
