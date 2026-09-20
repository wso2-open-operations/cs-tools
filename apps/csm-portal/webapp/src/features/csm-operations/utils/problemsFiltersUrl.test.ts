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
import { DEFAULT_PROBLEM_FILTERS } from "@features/csm-operations/utils/problems";
import {
  readProblemFiltersFromUrl,
  writeProblemFiltersToUrl,
} from "./problemsFiltersUrl";

describe("readProblemFiltersFromUrl", () => {
  it("returns the defaults for an empty query string", () => {
    expect(readProblemFiltersFromUrl(new URLSearchParams())).toEqual(
      DEFAULT_PROBLEM_FILTERS,
    );
  });

  it("parses a fully-populated query string", () => {
    const params = new URLSearchParams(
      "probQ=timeout&probStates=NEW,ASSESS&probSreTeams=team-apollo,team-atlas",
    );
    expect(readProblemFiltersFromUrl(params)).toEqual({
      search: "timeout",
      states: ["NEW", "ASSESS"],
      sreTeamIds: ["team-apollo", "team-atlas"],
    });
  });

  it("drops values outside the allowed state enum", () => {
    const params = new URLSearchParams("probStates=NEW,BOGUS");
    expect(readProblemFiltersFromUrl(params).states).toEqual(["NEW"]);
  });

  it("drops blank/whitespace SRE team entries", () => {
    const params = new URLSearchParams("probSreTeams=team-apollo,%20%20,,team-atlas");
    expect(readProblemFiltersFromUrl(params).sreTeamIds).toEqual([
      "team-apollo",
      "team-atlas",
    ]);
  });

  it("does not read the incidents tab's own `inc...` params", () => {
    const params = new URLSearchParams("incQ=foo&incPriorities=HIGH");
    expect(readProblemFiltersFromUrl(params)).toEqual(DEFAULT_PROBLEM_FILTERS);
  });
});

describe("writeProblemFiltersToUrl", () => {
  it("omits default-valued fields to keep the URL clean", () => {
    expect(writeProblemFiltersToUrl(DEFAULT_PROBLEM_FILTERS).toString()).toBe("");
  });

  it("round-trips a fully-populated filter set", () => {
    const filters: typeof DEFAULT_PROBLEM_FILTERS = {
      search: "timeout",
      states: ["NEW", "ASSESS"],
      sreTeamIds: ["team-apollo", "team-atlas"],
    };
    const round = readProblemFiltersFromUrl(writeProblemFiltersToUrl(filters));
    expect(round).toEqual(filters);
  });

  it("omits probSreTeams when no SRE team is selected", () => {
    const params = writeProblemFiltersToUrl({
      ...DEFAULT_PROBLEM_FILTERS,
      sreTeamIds: [],
    });
    expect(params.has("probSreTeams")).toBe(false);
  });
});
