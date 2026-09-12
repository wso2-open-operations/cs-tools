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
import { DEFAULT_INCIDENT_FILTERS } from "@features/csm-operations/utils/incidents";
import {
  readIncidentFiltersFromUrl,
  writeIncidentFiltersToUrl,
} from "./incidentsFiltersUrl";

describe("readIncidentFiltersFromUrl", () => {
  it("returns the defaults for an empty query string", () => {
    expect(readIncidentFiltersFromUrl(new URLSearchParams())).toEqual(
      DEFAULT_INCIDENT_FILTERS,
    );
  });

  it("parses a fully-populated query string", () => {
    const params = new URLSearchParams(
      "incQ=timeout&incPriorities=HIGH,LOW&incSlaViolated=1" +
        "&incCreatedFrom=2026-05-01&incCreatedTo=2026-05-31&incProducts=Choreo,Asgardeo" +
        "&incSreTeams=team-apollo,team-atlas",
    );
    expect(readIncidentFiltersFromUrl(params)).toEqual({
      search: "timeout",
      priorities: ["HIGH", "LOW"],
      slaViolated: true,
      createdStartDate: "2026-05-01",
      createdEndDate: "2026-05-31",
      products: ["Choreo", "Asgardeo"],
      sreTeamIds: ["team-apollo", "team-atlas"],
    });
  });

  it("drops blank/whitespace SRE team entries", () => {
    const params = new URLSearchParams("incSreTeams=team-apollo,%20%20,,team-atlas");
    expect(readIncidentFiltersFromUrl(params).sreTeamIds).toEqual([
      "team-apollo",
      "team-atlas",
    ]);
  });

  it("drops values outside the allowed priority enum", () => {
    const params = new URLSearchParams("incPriorities=HIGH,BOGUS");
    expect(readIncidentFiltersFromUrl(params).priorities).toEqual(["HIGH"]);
  });

  it("treats any value other than '1' as slaViolated=false", () => {
    expect(
      readIncidentFiltersFromUrl(new URLSearchParams("incSlaViolated=true"))
        .slaViolated,
    ).toBe(false);
    expect(
      readIncidentFiltersFromUrl(new URLSearchParams("incSlaViolated=0"))
        .slaViolated,
    ).toBe(false);
  });

  it("drops a malformed created-date bound", () => {
    const params = new URLSearchParams("incCreatedFrom=not-a-date&incCreatedTo=2026-13-40");
    expect(readIncidentFiltersFromUrl(params).createdStartDate).toBe("");
    expect(readIncidentFiltersFromUrl(params).createdEndDate).toBe("");
  });

  it("drops the end bound when the range is inverted, keeping the start", () => {
    // Both dates are individually valid, so each bound parses; only their
    // relative order is wrong. Reachable from a hand-edited or stale URL — the
    // pickers' own minDate/maxDate stop it happening through the UI.
    const params = new URLSearchParams(
      "incCreatedFrom=2026-05-31&incCreatedTo=2026-05-01",
    );
    const filters = readIncidentFiltersFromUrl(params);
    expect(filters.createdStartDate).toBe("2026-05-31");
    expect(filters.createdEndDate).toBe("");
  });

  it("keeps a range whose bounds are equal (single-day, inclusive)", () => {
    const params = new URLSearchParams(
      "incCreatedFrom=2026-05-01&incCreatedTo=2026-05-01",
    );
    const filters = readIncidentFiltersFromUrl(params);
    expect(filters.createdStartDate).toBe("2026-05-01");
    expect(filters.createdEndDate).toBe("2026-05-01");
  });

  it("drops blank/whitespace product entries", () => {
    const params = new URLSearchParams("incProducts=Choreo,%20%20,,Asgardeo");
    expect(readIncidentFiltersFromUrl(params).products).toEqual([
      "Choreo",
      "Asgardeo",
    ]);
  });

  it("does not read the change-requests tab's own `cr...` params", () => {
    const params = new URLSearchParams("crQ=foo&crStates=implement");
    expect(readIncidentFiltersFromUrl(params)).toEqual(
      DEFAULT_INCIDENT_FILTERS,
    );
  });
});

describe("writeIncidentFiltersToUrl", () => {
  it("omits default-valued fields to keep the URL clean", () => {
    expect(
      writeIncidentFiltersToUrl(DEFAULT_INCIDENT_FILTERS).toString(),
    ).toBe("");
  });

  it("omits incSlaViolated when the toggle is off", () => {
    const params = writeIncidentFiltersToUrl({
      ...DEFAULT_INCIDENT_FILTERS,
      slaViolated: false,
    });
    expect(params.has("incSlaViolated")).toBe(false);
  });

  it("round-trips a fully-populated filter set", () => {
    const filters: typeof DEFAULT_INCIDENT_FILTERS = {
      search: "timeout",
      priorities: ["HIGH"],
      slaViolated: true,
      createdStartDate: "2026-05-01",
      createdEndDate: "2026-05-31",
      products: ["Choreo", "Asgardeo"],
      sreTeamIds: ["team-apollo", "team-atlas"],
    };
    const round = readIncidentFiltersFromUrl(writeIncidentFiltersToUrl(filters));
    expect(round).toEqual(filters);
  });

  it("omits incSreTeams when no SRE team is selected", () => {
    const params = writeIncidentFiltersToUrl({
      ...DEFAULT_INCIDENT_FILTERS,
      sreTeamIds: [],
    });
    expect(params.has("incSreTeams")).toBe(false);
  });
});
