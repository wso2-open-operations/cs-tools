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
import { resolveTeamScopeState } from "@features/csm-dashboard/utils/teamScopeState";

const base = {
  isTeamBased: true,
  selectedTeamId: "alpha",
  dashboardType: "cre" as string | undefined,
  teamsPending: false,
  teamsError: false,
  selectedTeamFound: true,
  selectedTeamHasCreGroup: true,
  selectedTeamHasSreGroup: false,
};

describe("resolveTeamScopeState", () => {
  it("is ready when the dashboard is not team-based or no team is selected", () => {
    expect(resolveTeamScopeState({ ...base, isTeamBased: false })).toBe("ready");
    expect(resolveTeamScopeState({ ...base, selectedTeamId: undefined })).toBe("ready");
  });

  it("treats All ABTs as ready even while teams load", () => {
    expect(
      resolveTeamScopeState({ ...base, selectedTeamId: "__all__", teamsPending: true }),
    ).toBe("ready");
  });

  it("holds while the teams list is loading, so no org-wide query fires", () => {
    expect(
      resolveTeamScopeState({ ...base, teamsPending: true, selectedTeamFound: false }),
    ).toBe("loading");
  });

  it("reports an error when the teams list failed", () => {
    expect(
      resolveTeamScopeState({ ...base, teamsError: true, selectedTeamFound: false }),
    ).toBe("error");
  });

  it("reports missing for an unknown team or one with no group for the discipline", () => {
    expect(resolveTeamScopeState({ ...base, selectedTeamFound: false })).toBe("missing");
    expect(resolveTeamScopeState({ ...base, dashboardType: "sre" })).toBe("missing");
    expect(resolveTeamScopeState({ ...base, selectedTeamHasCreGroup: false })).toBe("missing");
  });

  it("is ready once the team has the discipline's group id", () => {
    expect(resolveTeamScopeState(base)).toBe("ready");
    expect(
      resolveTeamScopeState({
        ...base,
        dashboardType: undefined,
        selectedTeamHasCreGroup: false,
        selectedTeamHasSreGroup: true,
      }),
    ).toBe("ready");
  });
});
