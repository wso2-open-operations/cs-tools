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
      "scope=all_customers&q=timeout&severities=S0,S2&states=open,closed&sla=at_risk&assignees=alice,bob&projects=apim&products=mi",
    );
    expect(readCasesFiltersFromUrl(params)).toEqual({
      scope: "all_customers",
      search: "timeout",
      severities: ["S0", "S2"],
      states: ["open", "closed"],
      sla: "at_risk",
      assignees: ["alice", "bob"],
      projects: ["apim"],
      products: ["mi"],
    });
  });

  it("drops values outside the allowed enums", () => {
    const params = new URLSearchParams(
      "scope=bogus&severities=S0,S9,wat&states=open,nonsense&sla=invalid",
    );
    const f = readCasesFiltersFromUrl(params);
    expect(f.scope).toBe("my_abt"); // fallback
    expect(f.severities).toEqual(["S0"]);
    expect(f.states).toEqual(["open"]);
    expect(f.sla).toBe("any"); // fallback
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
      scope: "all_customers",
      search: "disk full",
      severities: ["S1"],
      states: ["work_in_progress"],
      sla: "breached",
      assignees: ["carol"],
      projects: ["streaming"],
      products: ["si"],
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
    expect(casesHref({ scope: "all_customers" })).toBe(
      "/cases?scope=all_customers",
    );
  });
});
