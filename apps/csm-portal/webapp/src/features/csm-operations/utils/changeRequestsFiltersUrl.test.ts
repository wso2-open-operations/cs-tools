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
import { DEFAULT_CR_FILTERS } from "@features/csm-operations/utils/changeRequests";
import {
  readChangeRequestFiltersFromUrl,
  writeChangeRequestFiltersToUrl,
} from "./changeRequestsFiltersUrl";

describe("readChangeRequestFiltersFromUrl", () => {
  it("returns the defaults for an empty query string", () => {
    expect(readChangeRequestFiltersFromUrl(new URLSearchParams())).toEqual(
      DEFAULT_CR_FILTERS,
    );
  });

  it("parses a fully-populated query string", () => {
    const params = new URLSearchParams(
      "crQ=rollback&crStates=implement,review&crImpacts=high,low&crClosedFrom=2026-01-01&crClosedTo=2026-01-31&crSreTeams=team-apollo,team-atlas",
    );
    expect(readChangeRequestFiltersFromUrl(params)).toEqual({
      search: "rollback",
      states: ["implement", "review"],
      impacts: ["high", "low"],
      closedStartDate: "2026-01-01",
      closedEndDate: "2026-01-31",
      sreTeamIds: ["team-apollo", "team-atlas"],
    });
  });

  it("drops blank/whitespace SRE team entries", () => {
    const params = new URLSearchParams("crSreTeams=team-apollo,%20%20,,team-atlas");
    expect(readChangeRequestFiltersFromUrl(params).sreTeamIds).toEqual([
      "team-apollo",
      "team-atlas",
    ]);
  });

  it("drops values outside the allowed state/impact enums", () => {
    const params = new URLSearchParams(
      "crStates=implement,bogus&crImpacts=high,huge",
    );
    const f = readChangeRequestFiltersFromUrl(params);
    expect(f.states).toEqual(["implement"]);
    expect(f.impacts).toEqual(["high"]);
  });

  it("drops a malformed date instead of passing it through", () => {
    const params = new URLSearchParams(
      "crClosedFrom=not-a-date&crClosedTo=2026-13-99",
    );
    const f = readChangeRequestFiltersFromUrl(params);
    expect(f.closedStartDate).toBe("");
    expect(f.closedEndDate).toBe("");
  });

  it("does not read the incidents tab's own `inc...` params", () => {
    const params = new URLSearchParams("incQ=foo&incPriorities=HIGH");
    expect(readChangeRequestFiltersFromUrl(params)).toEqual(DEFAULT_CR_FILTERS);
  });
});

describe("writeChangeRequestFiltersToUrl", () => {
  it("omits default-valued fields to keep the URL clean", () => {
    expect(writeChangeRequestFiltersToUrl(DEFAULT_CR_FILTERS).toString()).toBe(
      "",
    );
  });

  it("round-trips a non-default filter set", () => {
    const filters = {
      search: "rollback",
      states: ["implement" as const],
      impacts: ["high" as const],
      closedStartDate: "2026-01-01",
      closedEndDate: "2026-01-31",
      sreTeamIds: ["team-apollo"],
    };
    const round = readChangeRequestFiltersFromUrl(
      writeChangeRequestFiltersToUrl(filters),
    );
    expect(round).toEqual(filters);
  });

  it("omits crSreTeams when no SRE team is selected", () => {
    const params = writeChangeRequestFiltersToUrl({
      ...DEFAULT_CR_FILTERS,
      sreTeamIds: [],
    });
    expect(params.has("crSreTeams")).toBe(false);
  });
});
