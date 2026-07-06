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
import type { CasesFilters } from "@features/csm-cases/components/CasesFilterBar";
import {
  casesHref,
  DEFAULT_CASES_FILTERS,
  readCasesFiltersFromUrl,
  writeCasesFiltersToUrl,
} from "./casesFiltersUrl";

describe("readCasesFiltersFromUrl", () => {
  it("returns the defaults for an empty query string", () => {
    expect(readCasesFiltersFromUrl(new URLSearchParams())).toEqual(
      DEFAULT_CASES_FILTERS,
    );
  });

  it("parses a fully-populated query string", () => {
    const params = new URLSearchParams(
      "q=timeout&severities=S0,S2&states=open,work_in_progress,closed&types=case,engagement&assignees=alice@example.com,@me&workStates=ongoing,paused&projects=apim&products=API%20Manager,Asgardeo",
    );
    expect(readCasesFiltersFromUrl(params)).toEqual({
      search: "timeout",
      severities: ["S0", "S2"],
      states: ["open", "work_in_progress", "closed"],
      caseTypes: ["case", "engagement"],
      engagementTypes: [],
      assignees: ["alice@example.com", "@me"],
      workStates: ["ongoing", "paused"],
      projects: ["apim"],
      productNames: ["API Manager", "Asgardeo"],
    });
  });

  it("drops values outside the allowed enums", () => {
    const params = new URLSearchParams(
      "severities=S0,S9,wat&states=open,nonsense&types=case,bogus_type",
    );
    const f = readCasesFiltersFromUrl(params);
    expect(f.severities).toEqual(["S0"]);
    expect(f.states).toEqual(["open"]);
    expect(f.caseTypes).toEqual(["case"]);
  });

  it("drops work-state values outside the allowed enum", () => {
    const params = new URLSearchParams(
      "states=work_in_progress&workStates=ongoing,bogus,2",
    );
    expect(readCasesFiltersFromUrl(params).workStates).toEqual(["ongoing"]);
  });

  it("drops work states when `work_in_progress` is not in the state filter", () => {
    const params = new URLSearchParams("states=open&workStates=ongoing,paused");
    expect(readCasesFiltersFromUrl(params).workStates).toEqual([]);
  });

  it("strips empties and over-long free-form entries", () => {
    const long = "x".repeat(121);
    const params = new URLSearchParams();
    params.set("assignees", `alice, ,${long}`);
    expect(readCasesFiltersFromUrl(params).assignees).toEqual(["alice"]);
  });
});

describe("writeCasesFiltersToUrl", () => {
  it("omits default-valued fields to keep the URL clean", () => {
    expect(writeCasesFiltersToUrl(DEFAULT_CASES_FILTERS).toString()).toBe("");
  });

  it("round-trips a non-default filter set", () => {
    const filters: CasesFilters = {
      search: "disk full",
      severities: ["S1"],
      states: ["work_in_progress"],
      caseTypes: ["service_request"],
      assignees: ["carol@example.com"],
      workStates: ["paused"],
      projects: ["streaming"],
      engagementTypes: [],
      productNames: ["Identity Server", "Asgardeo"],
    };
    const round = readCasesFiltersFromUrl(writeCasesFiltersToUrl(filters));
    expect(round).toEqual(filters);
  });
});

describe("casesHref", () => {
  it("returns the bare path when overrides reduce to defaults", () => {
    expect(casesHref({})).toBe("/cases");
  });

  it("builds a query string from a partial override", () => {
    expect(casesHref({ severities: ["S1"] })).toBe("/cases?severities=S1");
  });
});
