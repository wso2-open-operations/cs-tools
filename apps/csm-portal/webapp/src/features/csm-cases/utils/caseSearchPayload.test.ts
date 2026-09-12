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
import { DEFAULT_CASES_FILTERS } from "@features/csm-cases/utils/casesFiltersUrl";
import type { AdvancedFilterRow } from "@features/csm-cases/utils/advancedFilters";
import type { BeCaseSearchView } from "@api/backend/types";
import { buildCaseSearchFilters, mapCaseSearchViewToRow } from "./caseSearchPayload";

function filterOf(filters: CasesFilters, field: string) {
  return (buildCaseSearchFilters(filters, "", undefined).filters ?? []).filter(
    (f) => f.field === field,
  );
}

describe("mapCaseSearchViewToRow — Customer column", () => {
  it("reads the customer name straight off the search response's embedded account", () => {
    const view: BeCaseSearchView = {
      id: "c1",
      account: { id: "acc-1", name: "Acme Corp" },
    };
    expect(mapCaseSearchViewToRow(view, undefined).customer).toBe("Acme Corp");
    expect(mapCaseSearchViewToRow(view, undefined).accountId).toBe("acc-1");
  });

  it("falls back to a placeholder when the search response has no account (e.g. no linked account)", () => {
    const view: BeCaseSearchView = { id: "c1" };
    expect(mapCaseSearchViewToRow(view, undefined).customer).toBe("-");
    expect(mapCaseSearchViewToRow(view, undefined).accountId).toBe("");
  });
});

describe("mapCaseSearchViewToRow — escalationLevel", () => {
  it("carries the raw escalation-level id through unmapped", () => {
    const view: BeCaseSearchView = { id: "c1", escalationLevel: "2" };
    expect(mapCaseSearchViewToRow(view, undefined).escalationLevel).toBe("2");
  });

  it("defaults to null when the search response omits escalationLevel", () => {
    const view: BeCaseSearchView = { id: "c1" };
    expect(mapCaseSearchViewToRow(view, undefined).escalationLevel).toBeNull();
  });
});

describe("buildCaseSearchFilters — new advanced-filter fields", () => {
  it("emits creTeam op:in for csTeams", () => {
    const filters: CasesFilters = { ...DEFAULT_CASES_FILTERS, csTeams: ["team-a"] };
    expect(filterOf(filters, "creTeam")).toEqual([
      { field: "creTeam", op: "in", values: ["team-a"] },
    ]);
  });

  it("emits sreTeam op:in for sreTeams, independently of creTeam", () => {
    const filters: CasesFilters = {
      ...DEFAULT_CASES_FILTERS,
      csTeams: ["team-a"],
      sreTeams: ["team-sre-b"],
    };
    expect(filterOf(filters, "creTeam")).toEqual([
      { field: "creTeam", op: "in", values: ["team-a"] },
    ]);
    expect(filterOf(filters, "sreTeam")).toEqual([
      { field: "sreTeam", op: "in", values: ["team-sre-b"] },
    ]);
  });

  it("emits tag op:in and tag op:notIn as two independent entries", () => {
    const filters: CasesFilters = {
      ...DEFAULT_CASES_FILTERS,
      tags: ["patch"],
      excludeTags: ["s_dip"],
    };
    const tagEntries = filterOf(filters, "tag");
    expect(tagEntries).toEqual([
      { field: "tag", op: "in", values: ["patch"] },
      { field: "tag", op: "notIn", values: ["s_dip"] },
    ]);
  });

  it("does NOT invert excludeTags into an `in` entry", () => {
    // Regression: the equivalent bug in widgetPreviewUrl.ts inverted `tag
    // notIn` into `tag in` because it dropped the op. This asserts the
    // payload builder never produces an `in` entry when only excludeTags is
    // set.
    const filters: CasesFilters = { ...DEFAULT_CASES_FILTERS, excludeTags: ["s_dip"] };
    const tagEntries = filterOf(filters, "tag");
    expect(tagEntries).toEqual([{ field: "tag", op: "notIn", values: ["s_dip"] }]);
    expect(tagEntries?.some((e) => e.op === "in")).toBe(false);
  });

  it("emits projectOnboardingStatus op:in", () => {
    const filters: CasesFilters = {
      ...DEFAULT_CASES_FILTERS,
      onboardingStatuses: ["in_progress"],
    };
    expect(filterOf(filters, "projectOnboardingStatus")).toEqual([
      { field: "projectOnboardingStatus", op: "in", values: ["in_progress"] },
    ]);
  });

  it("emits state op:in and state op:notIn as two independent entries", () => {
    const filters: CasesFilters = {
      ...DEFAULT_CASES_FILTERS,
      states: ["open"],
      excludeStates: ["closed"],
    };
    expect(filterOf(filters, "state")).toEqual([
      { field: "state", op: "in", values: ["open"] },
      { field: "state", op: "notIn", values: ["closed"] },
    ]);
  });

  it("does NOT invert excludeStates into an `in` entry", () => {
    const filters: CasesFilters = { ...DEFAULT_CASES_FILTERS, excludeStates: ["closed"] };
    const stateEntries = filterOf(filters, "state");
    expect(stateEntries).toEqual([{ field: "state", op: "notIn", values: ["closed"] }]);
    expect(stateEntries?.some((e) => e.op === "in")).toBe(false);
  });

  // Unlike `state`/`tag`, `projectOnboardingStatus` has no `excludeOnboarding
  // Statuses` field/op:notIn entry of its own -- its domain is the 4 fixed
  // values in `onboardingStatus.ts`, so a dashboard widget's `notIn` filter
  // is folded into `onboardingStatuses`' own complement at the translation
  // boundary (`translateCaseDashboardFilters`), and this builder only ever
  // sees (and only ever emits) an `in` entry for it.
  it("emits projectOnboardingStatus as a single op:in entry, never notIn", () => {
    const filters: CasesFilters = {
      ...DEFAULT_CASES_FILTERS,
      onboardingStatuses: ["Completed", "Not-Applicable"],
    };
    expect(filterOf(filters, "projectOnboardingStatus")).toEqual([
      { field: "projectOnboardingStatus", op: "in", values: ["Completed", "Not-Applicable"] },
    ]);
  });

  it("emits taskSLABusinessElapsedPercent gte and lte as separate entries", () => {
    const filters: CasesFilters = {
      ...DEFAULT_CASES_FILTERS,
      slaElapsedPctGte: 50,
      slaElapsedPctLte: 100,
    };
    expect(filterOf(filters, "taskSLABusinessElapsedPercent")).toEqual([
      { field: "taskSLABusinessElapsedPercent", op: "gte", values: ["50"] },
      { field: "taskSLABusinessElapsedPercent", op: "lte", values: ["100"] },
    ]);
  });

  it("emits escalation isNotEmpty with no `values` when hasEscalation is true", () => {
    const filters: CasesFilters = { ...DEFAULT_CASES_FILTERS, hasEscalation: true };
    expect(filterOf(filters, "escalation")).toEqual([
      { field: "escalation", op: "isNotEmpty" },
    ]);
  });

  it("emits escalation isEmpty with no `values` when hasEscalation is false", () => {
    const filters: CasesFilters = { ...DEFAULT_CASES_FILTERS, hasEscalation: false };
    expect(filterOf(filters, "escalation")).toEqual([{ field: "escalation", op: "isEmpty" }]);
  });

  it("omits escalation entirely when hasEscalation is null (unfiltered)", () => {
    const filters: CasesFilters = { ...DEFAULT_CASES_FILTERS, hasEscalation: null };
    expect(filterOf(filters, "escalation")).toEqual([]);
  });

  it("emits escalationLevel and projectType op:in", () => {
    const filters: CasesFilters = {
      ...DEFAULT_CASES_FILTERS,
      escalationLevels: ["L1"],
      projectTypes: ["enterprise"],
    };
    expect(filterOf(filters, "escalationLevel")).toEqual([
      { field: "escalationLevel", op: "in", values: ["L1"] },
    ]);
    expect(filterOf(filters, "projectType")).toEqual([
      { field: "projectType", op: "in", values: ["enterprise"] },
    ]);
  });

  it("emits createdOn/updatedOn/closedOn gte+lte as independent entries per field", () => {
    const filters: CasesFilters = {
      ...DEFAULT_CASES_FILTERS,
      createdOnGte: "2026-01-01",
      createdOnLte: "2026-03-31",
      updatedOnGte: "2026-02-01",
      closedOnLte: "2026-06-30",
    };
    expect(filterOf(filters, "createdOn")).toEqual([
      { field: "createdOn", op: "gte", values: ["2026-01-01"] },
      { field: "createdOn", op: "lte", values: ["2026-03-31"] },
    ]);
    expect(filterOf(filters, "updatedOn")).toEqual([
      { field: "updatedOn", op: "gte", values: ["2026-02-01"] },
    ]);
    expect(filterOf(filters, "closedOn")).toEqual([
      { field: "closedOn", op: "lte", values: ["2026-06-30"] },
    ]);
  });

  it("emits nothing beyond the default filters when all new fields are unset", () => {
    const result = buildCaseSearchFilters(DEFAULT_CASES_FILTERS, "", undefined);
    expect(result.filters).toBeUndefined();
  });
});

// A typed case number / WSO2 case id must go through as an exact-match field
// filter, not the free-text `searchQuery` scan. `searchQuery` is a CONTAINS/OR
// scan that also covers the description upstream, so an exact case number
// matched other cases merely *mentioning* it -- searching one case number
// surfaced a different case entirely.
describe("buildCaseSearchFilters — exact case-number / WSO2-id search", () => {
  it("routes a CS case number to an exact `number` filter, not searchQuery", () => {
    const result = buildCaseSearchFilters(DEFAULT_CASES_FILTERS, "CS0346083", undefined);

    expect(result.searchQuery).toBeUndefined();
    expect(result.filters).toEqual([
      { field: "number", op: "eq", values: ["CS0346083"] },
    ]);
  });

  it("routes a WSO2 case id to an exact `internalId` filter, not searchQuery", () => {
    const result = buildCaseSearchFilters(
      DEFAULT_CASES_FILTERS,
      "AXACOLPATRIASUB-484",
      undefined,
    );

    expect(result.searchQuery).toBeUndefined();
    expect(result.filters).toEqual([
      { field: "internalId", op: "eq", values: ["AXACOLPATRIASUB-484"] },
    ]);
  });

  it("still uses free-text searchQuery for anything that isn't an identifier", () => {
    const result = buildCaseSearchFilters(DEFAULT_CASES_FILTERS, "printer jam", undefined);

    expect(result.searchQuery).toBe("printer jam");
    expect(result.filters).toBeUndefined();
  });

  it("treats a partial/malformed case number as free text, so typing stays usable", () => {
    // Mid-typing (6 digits) and an over-long 8-digit string are both free text.
    expect(
      buildCaseSearchFilters(DEFAULT_CASES_FILTERS, "CS034608", undefined).searchQuery,
    ).toBe("CS034608");
    expect(
      buildCaseSearchFilters(DEFAULT_CASES_FILTERS, "CS03460834", undefined).searchQuery,
    ).toBe("CS03460834");
  });

  it("combines the exact identifier filter with the other active filters", () => {
    const result = buildCaseSearchFilters(
      { ...DEFAULT_CASES_FILTERS, caseTypes: ["case"] },
      "CS0346083",
      undefined,
    );

    expect(result.searchQuery).toBeUndefined();
    expect(result.filters).toEqual([
      { field: "type", op: "in", values: ["case"] },
      { field: "number", op: "eq", values: ["CS0346083"] },
    ]);
  });

  it("forceFreeText opts an identifier query back into the searchQuery scan", () => {
    // The cases list runs this leg alongside the exact one (see useGetCsmCases),
    // so a case that only *mentions* the number stays findable.
    const result = buildCaseSearchFilters(
      DEFAULT_CASES_FILTERS,
      "CS0346083",
      undefined,
      { forceFreeText: true },
    );

    expect(result.searchQuery).toBe("CS0346083");
    expect(result.filters).toBeUndefined();
  });

  it("emits no search filter at all for an empty query", () => {
    const result = buildCaseSearchFilters(DEFAULT_CASES_FILTERS, "", undefined);

    expect(result.searchQuery).toBeUndefined();
    expect(result.filters).toBeUndefined();
  });
});

describe("buildCaseSearchFilters — workState only applies when state is exactly work_in_progress", () => {
  it("emits workState when work_in_progress is the sole selected state", () => {
    const filters: CasesFilters = {
      ...DEFAULT_CASES_FILTERS,
      states: ["work_in_progress"],
      workStates: ["ongoing", "paused"],
    };

    expect(filterOf(filters, "workState")).toEqual([
      { field: "workState", op: "in", values: ["ongoing", "paused"] },
    ]);
  });

  // Regression guard: a stale `workStates` value reaching this builder any
  // way other than the filter bar's own onChange (a saved view, a
  // dashboard/pinned-view URL that predates the exact-match fix, a future
  // caller) must never silently narrow results to just in-progress/paused
  // cases once another state is also selected.
  it("drops workState from the payload when another state is selected alongside work_in_progress", () => {
    const filters: CasesFilters = {
      ...DEFAULT_CASES_FILTERS,
      states: ["work_in_progress", "open"],
      workStates: ["ongoing", "paused"],
    };

    expect(filterOf(filters, "workState")).toEqual([]);
    // The state filter itself still applies normally.
    expect(filterOf(filters, "state")).toEqual([
      { field: "state", op: "in", values: ["work_in_progress", "open"] },
    ]);
  });

  it("drops workState from the payload when no state is selected", () => {
    const filters: CasesFilters = {
      ...DEFAULT_CASES_FILTERS,
      states: [],
      workStates: ["ongoing"],
    };

    expect(filterOf(filters, "workState")).toEqual([]);
  });
});

describe("mapCaseSearchViewToRow — issueType and createdBy (reporter)", () => {
  const BASE: BeCaseSearchView = { id: "case-1", number: "CS0000001", subject: "Cluster degraded" };

  it("carries issueType through unchanged when present", () => {
    const row = mapCaseSearchViewToRow({ ...BASE, issueType: "total_outage" }, undefined);
    expect(row.issueType).toBe("total_outage");
  });

  it("leaves issueType undefined when the response doesn't carry one", () => {
    const row = mapCaseSearchViewToRow(BASE, undefined);
    expect(row.issueType).toBeUndefined();
  });

  it("prefers the creator's display name for createdBy", () => {
    const row = mapCaseSearchViewToRow(
      { ...BASE, createdBy: { id: null, email: "jane.doe@example.com", name: "Jane Doe" } },
      undefined,
    );
    expect(row.createdBy).toBe("Jane Doe");
  });

  it("falls back to the creator's email when there's no name", () => {
    const row = mapCaseSearchViewToRow(
      { ...BASE, createdBy: { id: null, email: "jane.doe@example.com", name: "" } },
      undefined,
    );
    expect(row.createdBy).toBe("jane.doe@example.com");
  });

  it("falls back to 'Unknown' when the response carries no creator at all", () => {
    const row = mapCaseSearchViewToRow(BASE, undefined);
    expect(row.createdBy).toBe("Unknown");
  });
});

describe("buildCaseSearchFilters — advanced filter rows", () => {
  it("emits a multi-value `in` row as a BeCaseFieldFilter with the field's values array", () => {
    const advancedFilters: AdvancedFilterRow[] = [
      { field: "deploymentId", op: "in", values: ["dep-1", "dep-2"] },
    ];
    const filters: CasesFilters = { ...DEFAULT_CASES_FILTERS, advancedFilters };
    expect(filterOf(filters, "deploymentId")).toEqual([
      { field: "deploymentId", op: "in", values: ["dep-1", "dep-2"] },
    ]);
  });

  it("emits an issueType `in` row (fixed multi-select) as a BeCaseFieldFilter", () => {
    const advancedFilters: AdvancedFilterRow[] = [
      { field: "issueType", op: "in", values: ["error", "total_outage"] },
    ];
    const filters: CasesFilters = { ...DEFAULT_CASES_FILTERS, advancedFilters };
    expect(filterOf(filters, "issueType")).toEqual([
      { field: "issueType", op: "in", values: ["error", "total_outage"] },
    ]);
  });

  it("emits a value-less op (`resolutionNotes` isEmpty) with no `values` at all", () => {
    const advancedFilters: AdvancedFilterRow[] = [
      { field: "resolutionNotes", op: "isEmpty", values: [] },
    ];
    const filters: CasesFilters = { ...DEFAULT_CASES_FILTERS, advancedFilters };
    expect(filterOf(filters, "resolutionNotes")).toEqual([
      { field: "resolutionNotes", op: "isEmpty" },
    ]);
  });

  it("emits a date `gte` row's literal value straight through (relative-date resolution happens upstream in useGetCsmCases)", () => {
    const advancedFilters: AdvancedFilterRow[] = [
      { field: "createdOn", op: "gte", values: ["2026-01-01"] },
    ];
    const filters: CasesFilters = { ...DEFAULT_CASES_FILTERS, advancedFilters };
    expect(filterOf(filters, "createdOn")).toEqual([
      { field: "createdOn", op: "gte", values: ["2026-01-01"] },
    ]);
  });

  it("drops an incomplete row (a field/op with no value entered) rather than sending an empty predicate", () => {
    const advancedFilters: AdvancedFilterRow[] = [
      { field: "number", op: "eq", values: [] },
    ];
    const filters: CasesFilters = { ...DEFAULT_CASES_FILTERS, advancedFilters };
    expect(filterOf(filters, "number")).toEqual([]);
  });

  it("emits the `createdBy eq` current-user placeholder value regardless of what's stored in `values`", () => {
    const advancedFilters: AdvancedFilterRow[] = [
      { field: "createdBy", op: "eq", values: [] },
    ];
    const filters: CasesFilters = { ...DEFAULT_CASES_FILTERS, advancedFilters };
    expect(filterOf(filters, "createdBy")).toEqual([
      { field: "createdBy", op: "eq", values: ["__current_user_email__"] },
    ]);
  });

  it("appends advanced rows alongside the dedicated-control fields, not in place of them", () => {
    const advancedFilters: AdvancedFilterRow[] = [
      { field: "deploymentId", op: "in", values: ["dep-1"] },
    ];
    const filters: CasesFilters = {
      ...DEFAULT_CASES_FILTERS,
      severities: ["S1"],
      advancedFilters,
    };
    const result = buildCaseSearchFilters(filters, "", undefined).filters ?? [];
    expect(result).toEqual([
      { field: "severity", op: "in", values: ["critical"] },
      { field: "deploymentId", op: "in", values: ["dep-1"] },
    ]);
  });
});

describe("buildCaseSearchFilters — anyOf (OR groups)", () => {
  it("emits one anyOf entry per branch with a complete condition", () => {
    const filters: CasesFilters = {
      ...DEFAULT_CASES_FILTERS,
      anyOfBranches: [
        { filters: [{ field: "type", values: ["case"] }] },
        { filters: [{ field: "severity", values: ["critical"] }] },
      ],
    };
    const result = buildCaseSearchFilters(filters, "", undefined);
    expect(result.anyOf).toEqual([
      { filters: [{ field: "type", op: "in", values: ["case"] }] },
      { filters: [{ field: "severity", op: "in", values: ["critical"] }] },
    ]);
  });

  it("ANDs multiple conditions within one branch", () => {
    const filters: CasesFilters = {
      ...DEFAULT_CASES_FILTERS,
      anyOfBranches: [
        {
          filters: [
            { field: "type", values: ["case"] },
            { field: "severity", values: ["critical", "high"] },
          ],
        },
      ],
    };
    const result = buildCaseSearchFilters(filters, "", undefined);
    expect(result.anyOf).toEqual([
      {
        filters: [
          { field: "type", op: "in", values: ["case"] },
          { field: "severity", op: "in", values: ["critical", "high"] },
        ],
      },
    ]);
  });

  it("drops a branch with no complete conditions rather than emitting an empty one", () => {
    const filters: CasesFilters = {
      ...DEFAULT_CASES_FILTERS,
      anyOfBranches: [
        { filters: [{ field: "type", values: [] }] },
        { filters: [{ field: "severity", values: ["critical"] }] },
      ],
    };
    const result = buildCaseSearchFilters(filters, "", undefined);
    expect(result.anyOf).toEqual([
      { filters: [{ field: "severity", op: "in", values: ["critical"] }] },
    ]);
  });

  it("omits anyOf entirely when no branch has a complete condition", () => {
    const filters: CasesFilters = {
      ...DEFAULT_CASES_FILTERS,
      anyOfBranches: [{ filters: [{ field: "type", values: [] }] }],
    };
    const result = buildCaseSearchFilters(filters, "", undefined);
    expect(result.anyOf).toBeUndefined();
  });
});
