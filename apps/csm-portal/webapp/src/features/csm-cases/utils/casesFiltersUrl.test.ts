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
import type { AdvancedFilterRow } from "@features/csm-cases/utils/advancedFilters";
import type { AnyOfBranch } from "@features/csm-cases/utils/anyOfFilters";
import {
  casesHref,
  countActiveFilters,
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
      "search=timeout&severities=S0,S2&states=open,work_in_progress,closed&types=case,engagement&assignees=alice@example.com,@me&workStates=ongoing,paused&projects=apim&products=API%20Manager,Asgardeo",
    );
    expect(readCasesFiltersFromUrl(params)).toEqual({
      ...DEFAULT_CASES_FILTERS,
      search: "timeout",
      severities: ["S0", "S2"],
      states: ["open", "work_in_progress", "closed"],
      caseTypes: ["case", "engagement"],
      assignees: ["alice@example.com", "@me"],
      // `work_in_progress` is one of three selected states here, not the
      // sole one -- workStates can't apply server-side in that shape, so it
      // parses back out as empty. See the exact-match tests below.
      workStates: [],
      projects: ["apim"],
      productNames: ["API Manager", "Asgardeo"],
    });
  });

  it("parses `tags` — a live param again, not the stale no-op it used to be", () => {
    const params = new URLSearchParams("tags=micro-gw,ws-policy");
    expect(readCasesFiltersFromUrl(params)).toEqual({
      ...DEFAULT_CASES_FILTERS,
      tags: ["micro-gw", "ws-policy"],
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

  it("drops work states when `work_in_progress` is selected alongside another state", () => {
    const params = new URLSearchParams(
      "states=work_in_progress,open&workStates=ongoing,paused",
    );
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
      ...DEFAULT_CASES_FILTERS,
      search: "disk full",
      severities: ["S1"],
      states: ["work_in_progress"],
      caseTypes: ["service_request"],
      assignees: ["carol@example.com"],
      workStates: ["paused"],
      projects: ["streaming"],
      productNames: ["Identity Server", "Asgardeo"],
    };
    const round = readCasesFiltersFromUrl(writeCasesFiltersToUrl(filters));
    expect(round).toEqual(filters);
  });
});

/**
 * Regression coverage for the exact bug class `widgetPreviewUrl.ts` shipped
 * (see `6a9059789`): an op silently decoding back as a different op, or a
 * value-less op being dropped for having no `values` to serialize. This
 * codec avoids the `field~op` mechanism that bug required fixing (see
 * `writeCasesFiltersToUrl`'s doc comment) by giving every op its own named
 * field — these tests exist to prove that actually holds, not just to
 * restate the design.
 */
describe("op-awareness (regression: the widgetPreviewUrl field~op bug)", () => {
  it("`tags` (op:in) and `excludeTags` (op:notIn) never conflate on a round trip", () => {
    const filters: CasesFilters = {
      ...DEFAULT_CASES_FILTERS,
      excludeTags: ["s_dip"],
    };
    const round = readCasesFiltersFromUrl(writeCasesFiltersToUrl(filters));
    // The bug this guards against: `tag notIn [s_dip]` decoding back as
    // `tag in [s_dip]` — an EXCLUSION becoming a FILTER. Assert both halves:
    // the exclusion survived, and it did NOT leak into `tags` (inclusion).
    expect(round.excludeTags).toEqual(["s_dip"]);
    expect(round.tags).toEqual([]);
  });

  it("`tags` and `excludeTags` survive together, independently, when both are set", () => {
    const filters: CasesFilters = {
      ...DEFAULT_CASES_FILTERS,
      tags: ["patch"],
      excludeTags: ["s_dip"],
    };
    const round = readCasesFiltersFromUrl(writeCasesFiltersToUrl(filters));
    expect(round.tags).toEqual(["patch"]);
    expect(round.excludeTags).toEqual(["s_dip"]);
  });

  it("`states` (op:in) and `excludeStates` (op:notIn) never conflate on a round trip", () => {
    const filters: CasesFilters = { ...DEFAULT_CASES_FILTERS, excludeStates: ["closed"] };
    const round = readCasesFiltersFromUrl(writeCasesFiltersToUrl(filters));
    expect(round.excludeStates).toEqual(["closed"]);
    expect(round.states).toEqual([]);
  });

  it("`states` and `excludeStates` survive together, independently, when both are set", () => {
    const filters: CasesFilters = {
      ...DEFAULT_CASES_FILTERS,
      states: ["open", "work_in_progress"],
      excludeStates: ["closed"],
    };
    const round = readCasesFiltersFromUrl(writeCasesFiltersToUrl(filters));
    expect(round.states).toEqual(["open", "work_in_progress"]);
    expect(round.excludeStates).toEqual(["closed"]);
  });

  // `onboardingStatuses` has no `excludeOnboardingStatuses` counterpart --
  // unlike `states`/`tags`, its domain is the 4 fixed values in
  // `onboardingStatus.ts`, so a dashboard widget's `notIn` filter is folded
  // into this same field's complement at the translation boundary
  // (`translateCaseDashboardFilters`, see its own doc comment) rather than a
  // second field/param this codec would need to keep distinct.
  it("`onboardingStatuses` round-trips losslessly through the URL", () => {
    const filters: CasesFilters = {
      ...DEFAULT_CASES_FILTERS,
      onboardingStatuses: ["Not-Started", "Completed", "Not-Applicable"],
    };
    const round = readCasesFiltersFromUrl(writeCasesFiltersToUrl(filters));
    expect(round.onboardingStatuses).toEqual(["Not-Started", "Completed", "Not-Applicable"]);
  });

  it("a value-less op (`hasEscalation` / escalation isNotEmpty) survives rather than being dropped", () => {
    const filters: CasesFilters = { ...DEFAULT_CASES_FILTERS, hasEscalation: true };
    const href = writeCasesFiltersToUrl(filters);
    // Assert the param is actually present, not just that the round trip
    // happens to produce the right value some other way.
    expect(href.get("escalation")).toBe("yes");
    expect(readCasesFiltersFromUrl(href).hasEscalation).toBe(true);
  });

  it("the other value-less state (`hasEscalation: false` / isEmpty) also survives", () => {
    const filters: CasesFilters = { ...DEFAULT_CASES_FILTERS, hasEscalation: false };
    const href = writeCasesFiltersToUrl(filters);
    expect(href.get("escalation")).toBe("no");
    expect(readCasesFiltersFromUrl(href).hasEscalation).toBe(false);
  });

  it("a gte+lte range on one field round-trips with both bounds intact", () => {
    const filters: CasesFilters = {
      ...DEFAULT_CASES_FILTERS,
      slaElapsedPctGte: 50,
      slaElapsedPctLte: 100,
      createdOnGte: "2026-01-01",
      createdOnLte: "2026-03-31",
    };
    const round = readCasesFiltersFromUrl(writeCasesFiltersToUrl(filters));
    expect(round.slaElapsedPctGte).toBe(50);
    expect(round.slaElapsedPctLte).toBe(100);
    expect(round.createdOnGte).toBe("2026-01-01");
    expect(round.createdOnLte).toBe("2026-03-31");
  });

  it("a one-sided range only sets the bound that was given", () => {
    const filters: CasesFilters = { ...DEFAULT_CASES_FILTERS, slaElapsedPctGte: 90 };
    const round = readCasesFiltersFromUrl(writeCasesFiltersToUrl(filters));
    expect(round.slaElapsedPctGte).toBe(90);
    expect(round.slaElapsedPctLte).toBeNull();
  });
});

describe("advanced filters (`af` param)", () => {
  it("round-trips a multi-value `in` row through the URL", () => {
    const advancedFilters: AdvancedFilterRow[] = [
      { field: "deploymentId", op: "in", values: ["dep-1", "dep-2"] },
    ];
    const filters: CasesFilters = { ...DEFAULT_CASES_FILTERS, advancedFilters };
    const round = readCasesFiltersFromUrl(writeCasesFiltersToUrl(filters));
    expect(round.advancedFilters).toEqual(advancedFilters);
  });

  // `escalation` now has a typed `CasesFilters` slot (`hasEscalation`, see
  // `filterFieldAdapters.ts`'s typed-adapter registry) — an `af` row
  // targeting it is folded (`normalizeCasesFilters`) into that real property
  // on read rather than staying a dangling `advancedFilters` entry, so the
  // round-trip is still lossless, just visible on a different property now.
  it("round-trips a value-less op (`escalation isNotEmpty`) into the typed `hasEscalation` field, not a dangling `af` row", () => {
    const advancedFilters: AdvancedFilterRow[] = [
      { field: "escalation", op: "isNotEmpty", values: [] },
    ];
    const filters: CasesFilters = { ...DEFAULT_CASES_FILTERS, advancedFilters };
    const href = writeCasesFiltersToUrl(filters);
    expect(href.get("af")).not.toBeNull();
    const round = readCasesFiltersFromUrl(href);
    expect(round.hasEscalation).toBe(true);
    expect(round.advancedFilters).toEqual([]);
  });

  // `createdOn`/`updatedOn`/`closedOn` also now have typed slots
  // (`updatedOnGte`/`updatedOnLte`, ...) — same normalization as above.
  it("round-trips a date `gte`/`lte` pair on the same field into the typed `updatedOnGte`/`updatedOnLte` fields", () => {
    const advancedFilters: AdvancedFilterRow[] = [
      { field: "updatedOn", op: "gte", values: ["2026-01-01"] },
      { field: "updatedOn", op: "lte", values: ["2026-03-31"] },
    ];
    const filters: CasesFilters = { ...DEFAULT_CASES_FILTERS, advancedFilters };
    const round = readCasesFiltersFromUrl(writeCasesFiltersToUrl(filters));
    expect(round.updatedOnGte).toBe("2026-01-01");
    expect(round.updatedOnLte).toBe("2026-03-31");
    expect(round.advancedFilters).toEqual([]);
  });

  it("round-trips an in-progress row (field/op picked, no value yet) rather than dropping it", () => {
    // Regression test for the "Add filter" bug: `CsmIssuesView` treats the
    // URL as the single source of truth (`filters =
    // readCasesFiltersFromUrl(searchParams)`), so a freshly-added row with no
    // value yet used to vanish within the same tick — `writeAdvancedFiltersParam`
    // filtered it out before it ever reached the URL, and the very next
    // re-read from that URL then rendered zero rows. An incomplete row must
    // survive this URL round trip (it's still correctly excluded from the
    // `/cases/search` request payload — see `caseSearchPayload.test.ts`).
    const advancedFilters: AdvancedFilterRow[] = [
      { field: "number", op: "eq", values: [] },
    ];
    const filters: CasesFilters = { ...DEFAULT_CASES_FILTERS, advancedFilters };
    const href = writeCasesFiltersToUrl(filters);
    expect(href.get("af")).not.toBeNull();
    const round = readCasesFiltersFromUrl(href);
    expect(round.advancedFilters).toEqual(advancedFilters);
  });

  it("round-trips a row whose op takes a value but none has been typed yet (empty `values` array)", () => {
    const advancedFilters: AdvancedFilterRow[] = [
      { field: "internalId", op: "eq", values: [] },
    ];
    const filters: CasesFilters = { ...DEFAULT_CASES_FILTERS, advancedFilters };
    const round = readCasesFiltersFromUrl(writeCasesFiltersToUrl(filters));
    expect(round.advancedFilters).toEqual(advancedFilters);
  });

  it("does not count an in-progress (incomplete) row toward the active-filter badge, even though it now round-trips", () => {
    const advancedFilters: AdvancedFilterRow[] = [
      { field: "internalId", op: "eq", values: ["a"] },
      { field: "number", op: "eq", values: [] },
    ];
    expect(
      countActiveFilters({ ...DEFAULT_CASES_FILTERS, advancedFilters }),
    ).toBe(1);
  });

  it("silently drops an unknown field/op on a hand-edited or stale `af` param instead of throwing", () => {
    const params = new URLSearchParams();
    params.set(
      "af",
      JSON.stringify([
        ["not_a_real_field", "in", ["x"]],
        ["internalId", "not_a_real_op", ["y"]],
        ["internalId", "eq", ["ok"]],
      ]),
    );
    expect(readCasesFiltersFromUrl(params).advancedFilters).toEqual([
      { field: "internalId", op: "eq", values: ["ok"] },
    ]);
  });

  it("silently returns no advanced filters for garbage JSON in `af`", () => {
    const params = new URLSearchParams();
    params.set("af", "not json{{{");
    expect(readCasesFiltersFromUrl(params).advancedFilters).toEqual([]);
  });

  it("counts each complete advanced-filter row individually toward the active-filter count", () => {
    const advancedFilters: AdvancedFilterRow[] = [
      { field: "internalId", op: "eq", values: ["a"] },
      { field: "number", op: "eq", values: ["CS0441174"] },
    ];
    expect(
      countActiveFilters({ ...DEFAULT_CASES_FILTERS, advancedFilters }),
    ).toBe(2);
  });
});

describe("OR groups (`anyOf` param)", () => {
  it("round-trips a two-branch, single-condition-each anyOf through the URL", () => {
    const anyOfBranches: AnyOfBranch[] = [
      { filters: [{ field: "type", values: ["case"] }] },
      { filters: [{ field: "severity", values: ["critical"] }] },
    ];
    const filters: CasesFilters = { ...DEFAULT_CASES_FILTERS, anyOfBranches };
    const round = readCasesFiltersFromUrl(writeCasesFiltersToUrl(filters));
    expect(round.anyOfBranches).toEqual(anyOfBranches);
  });

  it("round-trips an in-progress branch (a row with field picked, no value yet) rather than dropping it", () => {
    const anyOfBranches: AnyOfBranch[] = [{ filters: [{ field: "type", values: [] }] }];
    const filters: CasesFilters = { ...DEFAULT_CASES_FILTERS, anyOfBranches };
    const href = writeCasesFiltersToUrl(filters);
    expect(href.get("anyOf")).not.toBeNull();
    const round = readCasesFiltersFromUrl(href);
    expect(round.anyOfBranches).toEqual(anyOfBranches);
  });

  it("round-trips a branch with multiple ANDed conditions", () => {
    const anyOfBranches: AnyOfBranch[] = [
      {
        filters: [
          { field: "type", values: ["case"] },
          { field: "severity", values: ["critical", "high"] },
        ],
      },
    ];
    const filters: CasesFilters = { ...DEFAULT_CASES_FILTERS, anyOfBranches };
    const round = readCasesFiltersFromUrl(writeCasesFiltersToUrl(filters));
    expect(round.anyOfBranches).toEqual(anyOfBranches);
  });

  it("does not count a branch with no complete condition toward the active-filter badge, even though it round-trips", () => {
    const anyOfBranches: AnyOfBranch[] = [
      { filters: [{ field: "type", values: ["case"] }] },
      { filters: [{ field: "severity", values: [] }] },
    ];
    expect(countActiveFilters({ ...DEFAULT_CASES_FILTERS, anyOfBranches })).toBe(1);
  });

  it("silently drops an unknown branch field on a hand-edited or stale `anyOf` param instead of throwing", () => {
    const params = new URLSearchParams();
    params.set(
      "anyOf",
      JSON.stringify([[["not_a_real_field", ["x"]], ["type", ["case"]]]]),
    );
    expect(readCasesFiltersFromUrl(params).anyOfBranches).toEqual([
      { filters: [{ field: "type", values: ["case"] }] },
    ]);
  });

  it("silently returns no OR groups for garbage JSON in `anyOf`", () => {
    const params = new URLSearchParams();
    params.set("anyOf", "not json{{{");
    expect(readCasesFiltersFromUrl(params).anyOfBranches).toEqual([]);
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
