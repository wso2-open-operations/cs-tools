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
import { DEFAULT_CASES_FILTERS } from "@features/csm-cases/utils/casesFiltersUrl";
import type { CasesFilters } from "@features/csm-cases/components/CasesFilterBar";
import {
  addBlankUnifiedRow,
  filtersToAdvancedRows,
  isSimpleRepresentable,
  normalizeCasesFilters,
  removeUnifiedRow,
  updateUnifiedRow,
} from "./filterFieldAdapters";

describe("isSimpleRepresentable", () => {
  it("is true for the untouched default filters", () => {
    expect(isSimpleRepresentable(DEFAULT_CASES_FILTERS)).toBe(true);
  });

  it("stays true for every field the Simple grid's own dedicated controls cover", () => {
    const filters: CasesFilters = {
      ...DEFAULT_CASES_FILTERS,
      severities: ["S1"],
      states: ["open"],
      caseTypes: ["case"],
      assignees: ["@me"],
      workStates: ["ongoing"],
      projects: ["p1"],
      engagementTypes: ["onboarding"],
      productNames: ["API Manager"],
      csTeams: ["g1"],
      onboardingStatuses: ["Completed"],
    };
    expect(isSimpleRepresentable(filters)).toBe(true);
  });

  const gatingOverrides: [string, Partial<CasesFilters>][] = [
    ["tags", { tags: ["urgent"] }],
    ["excludeTags", { excludeTags: ["spam"] }],
    // The Simple-mode "State" control used to be a tri-state that read/wrote
    // `excludeStates` directly (digiops-cs#2907); now that it's a plain
    // include-only multi-select, `excludeStates` is only representable via
    // the Advanced-mode "State"/"is not one of" row, same as `excludeTags`.
    ["excludeStates", { excludeStates: ["closed"] }],
    ["sreTeams", { sreTeams: ["g1"] }],
    ["projectTypes", { projectTypes: ["Subscription"] }],
    ["escalationLevels", { escalationLevels: ["1"] }],
    ["hasEscalation", { hasEscalation: true }],
    ["slaElapsedPctGte", { slaElapsedPctGte: 80 }],
    ["slaElapsedPctLte", { slaElapsedPctLte: 100 }],
    ["createdOnGte", { createdOnGte: "2026-01-01" }],
    ["createdOnLte", { createdOnLte: "2026-01-31" }],
    ["updatedOnGte", { updatedOnGte: "2026-01-01" }],
    ["updatedOnLte", { updatedOnLte: "2026-01-31" }],
    ["closedOnGte", { closedOnGte: "2026-01-01" }],
    ["closedOnLte", { closedOnLte: "2026-01-31" }],
    ["advancedFilters", { advancedFilters: [{ field: "number", op: "eq", values: ["CS1"] }] }],
    ["anyOfBranches", { anyOfBranches: [{ filters: [{ field: "type", values: ["case"] }] }] }],
  ];
  it.each(gatingOverrides)("is false once %s is set", (_name, override) => {
    const filters: CasesFilters = { ...DEFAULT_CASES_FILTERS, ...override };
    expect(isSimpleRepresentable(filters)).toBe(false);
  });
});

describe("filtersToAdvancedRows / updateUnifiedRow / removeUnifiedRow — round trip", () => {
  it("produces no rows for the untouched default filters", () => {
    expect(filtersToAdvancedRows(DEFAULT_CASES_FILTERS)).toEqual([]);
  });

  it("shows a typed row for a non-empty Simple-grid field, and it round-trips through the real CasesFilters property", () => {
    const filters: CasesFilters = { ...DEFAULT_CASES_FILTERS, severities: ["S1", "S2"] };
    const rows = filtersToAdvancedRows(filters);
    expect(rows).toEqual([
      { origin: "typed", field: "severity", op: "in", values: ["S1", "S2"] },
    ]);
  });

  it("editing a typed row's values writes straight back to the same CasesFilters property (lossless Simple<->Advanced)", () => {
    const filters: CasesFilters = { ...DEFAULT_CASES_FILTERS, severities: ["S1"] };
    const [row] = filtersToAdvancedRows(filters);
    const next = updateUnifiedRow(filters, row, { field: "severity", op: "in", values: ["S1", "S3"] });
    expect(next.severities).toEqual(["S1", "S3"]);
    // Nothing leaked into the untyped escape hatch.
    expect(next.advancedFilters).toEqual([]);
  });

  it("removing a typed row clears the real CasesFilters property", () => {
    const filters: CasesFilters = { ...DEFAULT_CASES_FILTERS, productNames: ["API Manager"] };
    const [row] = filtersToAdvancedRows(filters);
    const next = removeUnifiedRow(filters, row);
    expect(next.productNames).toEqual([]);
  });

  it("changing a typed row's field to a different typed field moves the value to the new field's property", () => {
    const filters: CasesFilters = { ...DEFAULT_CASES_FILTERS, severities: ["S1"] };
    const [row] = filtersToAdvancedRows(filters);
    // Field-picker reset: values clear to [] on a field change (same as the
    // pre-existing per-row field dropdown behavior).
    const afterFieldChange = updateUnifiedRow(filters, row, {
      field: "type",
      op: "in",
      values: [],
    });
    expect(afterFieldChange.severities).toEqual([]);
    expect(afterFieldChange.caseTypes).toEqual([]);
    // Still incomplete (no value yet) -- lives in the untyped array so the
    // in-progress row survives a render, exactly like an untyped field would.
    expect(afterFieldChange.advancedFilters).toEqual([
      { field: "type", op: "in", values: [] },
    ]);

    const afterValue = updateUnifiedRow(
      filters,
      { origin: "array", field: "type", op: "in", values: [], arrayIndex: 0 },
      { field: "type", op: "in", values: ["case"] },
    );
    expect(afterValue.caseTypes).toEqual(["case"]);
    expect(afterValue.advancedFilters).toEqual([]);
  });

  it("an untyped field (no CasesFilters slot) stays in the advancedFilters array end to end", () => {
    let filters: CasesFilters = { ...DEFAULT_CASES_FILTERS };
    filters = addBlankUnifiedRow(filters);
    expect(filters.advancedFilters).toEqual([
      { field: filters.advancedFilters[0].field, op: filters.advancedFilters[0].op, values: [] },
    ]);

    const [row] = filtersToAdvancedRows(filters);
    const next = updateUnifiedRow(filters, row, {
      field: "internalId",
      op: "eq",
      values: ["WSO2-1"],
    });
    expect(next.advancedFilters).toEqual([{ field: "internalId", op: "eq", values: ["WSO2-1"] }]);
  });

  it("editing an in-progress array row's value in place does not reorder it relative to a sibling row", () => {
    let filters: CasesFilters = { ...DEFAULT_CASES_FILTERS };
    filters = {
      ...filters,
      advancedFilters: [
        { field: "internalId", op: "eq", values: ["first"] },
        { field: "parentId", op: "eq", values: [] },
      ],
    };
    const rows = filtersToAdvancedRows(filters);
    const secondRow = rows[1];
    const next = updateUnifiedRow(filters, secondRow, {
      field: "parentId",
      op: "eq",
      values: ["p"],
    });
    expect(next.advancedFilters).toEqual([
      { field: "internalId", op: "eq", values: ["first"] },
      { field: "parentId", op: "eq", values: ["p"] },
    ]);
  });

  it("the value-less escalation ops set/clear the tri-state hasEscalation field, not a truthy row value", () => {
    let filters: CasesFilters = { ...DEFAULT_CASES_FILTERS };
    filters = addBlankUnifiedRow(filters);
    filters = updateUnifiedRow(filters, filtersToAdvancedRows(filters)[0], {
      field: "escalation",
      op: "isNotEmpty",
      values: [],
    });
    expect(filters.hasEscalation).toBe(true);
    expect(filtersToAdvancedRows(filters)).toEqual([
      { origin: "typed", field: "escalation", op: "isNotEmpty", values: [] },
    ]);

    const [row] = filtersToAdvancedRows(filters);
    const cleared = removeUnifiedRow(filters, row);
    expect(cleared.hasEscalation).toBeNull();
    expect(filtersToAdvancedRows(cleared)).toEqual([]);
  });

  it("state/in pruning: switching states away from a sole work_in_progress clears workStates", () => {
    const filters: CasesFilters = {
      ...DEFAULT_CASES_FILTERS,
      states: ["work_in_progress"],
      workStates: ["ongoing"],
    };
    const [row] = filtersToAdvancedRows(filters);
    expect(row.field).toBe("state");
    const next = updateUnifiedRow(filters, row, {
      field: "state",
      op: "in",
      values: ["work_in_progress", "open"],
    });
    expect(next.states).toEqual(["work_in_progress", "open"]);
    expect(next.workStates).toEqual([]);
  });
});

describe("normalizeCasesFilters", () => {
  it("folds a complete advancedFilters row targeting a now-typed field into the real property", () => {
    const filters: CasesFilters = {
      ...DEFAULT_CASES_FILTERS,
      advancedFilters: [{ field: "severity", op: "in", values: ["S1", "S2"] }],
    };
    const next = normalizeCasesFilters(filters);
    expect(next.severities).toEqual(["S1", "S2"]);
    expect(next.advancedFilters).toEqual([]);
  });

  it("leaves an incomplete row targeting a typed field alone (nothing to fold yet)", () => {
    const filters: CasesFilters = {
      ...DEFAULT_CASES_FILTERS,
      advancedFilters: [{ field: "severity", op: "in", values: [] }],
    };
    const next = normalizeCasesFilters(filters);
    expect(next.severities).toEqual([]);
    expect(next.advancedFilters).toEqual([{ field: "severity", op: "in", values: [] }]);
  });

  it("leaves a row targeting an untyped field untouched", () => {
    const filters: CasesFilters = {
      ...DEFAULT_CASES_FILTERS,
      advancedFilters: [{ field: "internalId", op: "eq", values: ["WSO2-1"] }],
    };
    expect(normalizeCasesFilters(filters)).toEqual(filters);
  });
});
