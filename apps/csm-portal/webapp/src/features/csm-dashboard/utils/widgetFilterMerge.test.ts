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
import { mergeWidgetFilters } from "./widgetFilterMerge";

describe("mergeWidgetFilters", () => {
  it("merges non-case filters as a plain object spread, slice keys winning", () => {
    const base = { states: ["open"], severities: ["critical"] };
    const slice = { severities: ["catastrophic"] };

    expect(mergeWidgetFilters(base, slice)).toEqual({
      states: ["open"],
      severities: ["catastrophic"],
    });
  });

  it("merges case filter arrays by field, keeping base entries the slice doesn't override", () => {
    const base = {
      filters: [
        { field: "state", op: "in", values: ["open", "work_in_progress"] },
        { field: "severity", op: "in", values: ["critical"] },
      ],
    };
    const slice = {
      filters: [{ field: "severity", op: "in", values: ["catastrophic"] }],
    };

    expect(mergeWidgetFilters(base, slice)).toEqual({
      filters: [
        { field: "state", op: "in", values: ["open", "work_in_progress"] },
        { field: "severity", op: "in", values: ["catastrophic"] },
      ],
    });
  });

  it("keeps the base's case filters when the slice's own filters array is empty", () => {
    const base = { filters: [{ field: "state", op: "in", values: ["open"] }] };
    const slice = { filters: [] };

    expect(mergeWidgetFilters(base, slice)).toEqual({
      filters: [{ field: "state", op: "in", values: ["open"] }],
    });
  });

  // Regression test: DASHBOARDS_CONFIG is a raw JSON env var, not
  // schema-validated beyond basic decoding — a widget's base `filters` or a
  // pie/bar slice's own `filters` can be genuinely absent at runtime despite
  // the wire type declaring both required. This used to crash with "Cannot
  // read properties of undefined (reading 'filters')" three calls down from
  // DashboardWidgetTile/useWidgetPieData.
  it("treats an undefined base or slice as empty rather than throwing", () => {
    expect(mergeWidgetFilters(undefined, { states: ["open"] })).toEqual({ states: ["open"] });
    expect(mergeWidgetFilters({ states: ["open"] }, undefined)).toEqual({ states: ["open"] });
    expect(mergeWidgetFilters(undefined, undefined)).toEqual({});
  });

  it("lets a slice override a base entry for the same field", () => {
    const merged = mergeWidgetFilters(
      { filters: [{ field: "state", op: "in", values: ["open"] }] },
      { filters: [{ field: "state", op: "in", values: ["closed"] }] },
    );

    expect(merged.filters).toEqual([{ field: "state", op: "in", values: ["closed"] }]);
  });

  it("lets a slice override a base entry for the same field", () => {
    const merged = mergeWidgetFilters(
      { filters: [{ field: "state", op: "in", values: ["open"] }] },
      { filters: [{ field: "state", op: "in", values: ["closed"] }] },
    );

    expect(merged.filters).toEqual([{ field: "state", op: "in", values: ["closed"] }]);
  });

  it("keeps a non-case flat criteria record's base keys", () => {
    const merged = mergeWidgetFilters({ states: ["pending"] }, { approverIds: ["u1"] });

    expect(merged).toEqual({ states: ["pending"], approverIds: ["u1"] });
  });

  // A slices-only widget (no top-level `query`, e.g. a shape "bar" widget
  // where every slice supplies its own complete criteria) marshals its base
  // as JSON `null`, not `{}` -- BeDashboardWidget.query is legally absent
  // (see its own doc comment). Before this null-safety fix, `base.filters`
  // threw straight out of `DashboardWidgetTile`.
  it("treats a null or undefined base the same as an empty object", () => {
    expect(mergeWidgetFilters(null, { filters: [{ field: "state", op: "in", values: ["open"] }] })).toEqual({
      filters: [{ field: "state", op: "in", values: ["open"] }],
    });
    expect(mergeWidgetFilters(undefined, { filters: [{ field: "state", op: "in", values: ["open"] }] })).toEqual({
      filters: [{ field: "state", op: "in", values: ["open"] }],
    });
  });

  it("treats a null/undefined slice the same as an empty object", () => {
    expect(mergeWidgetFilters({ states: ["pending"] }, null)).toEqual({ states: ["pending"] });
    expect(mergeWidgetFilters({ states: ["pending"] }, undefined)).toEqual({ states: ["pending"] });
  });

  // A plain object spread drops the base's `anyOf` wholesale the moment a
  // slice sets its own. That is not a hypothetical shape: the backend loader
  // actively PRODUCES `anyOf` by migrating the legacy `orGroups` key, so a
  // migrated widget with an OR group plus any slice that also uses one loses
  // every base branch and silently widens that slice's count.
  it("does not drop the base's `anyOf` branches when the slice also sets `anyOf`", () => {
    const merged = mergeWidgetFilters(
      {
        anyOf: [
          { filters: [{ field: "state", op: "in", values: ["open"] }] },
          { filters: [{ field: "state", op: "in", values: ["work_in_progress"] }] },
        ],
      },
      { anyOf: [{ filters: [{ field: "severity", op: "in", values: ["critical"] }] }] },
    );

    // (open OR wip) AND (critical) distributes into two branches, each
    // carrying both constraints -- never just the slice's branch alone.
    expect(merged.anyOf).toEqual([
      {
        filters: [
          { field: "state", op: "in", values: ["open"] },
          { field: "severity", op: "in", values: ["critical"] },
        ],
      },
      {
        filters: [
          { field: "state", op: "in", values: ["work_in_progress"] },
          { field: "severity", op: "in", values: ["critical"] },
        ],
      },
    ]);
  });

  it("lets a slice branch override the base branch on the same field", () => {
    const merged = mergeWidgetFilters(
      { anyOf: [{ filters: [{ field: "state", op: "in", values: ["open"] }] }] },
      { anyOf: [{ filters: [{ field: "state", op: "in", values: ["closed"] }] }] },
    );

    expect(merged.anyOf).toEqual([
      { filters: [{ field: "state", op: "in", values: ["closed"] }] },
    ]);
  });

  it("keeps the base's `anyOf` untouched when the slice sets none", () => {
    const base = { anyOf: [{ filters: [{ field: "state", op: "in", values: ["open"] }] }] };
    const merged = mergeWidgetFilters(base, {
      filters: [{ field: "severity", op: "in", values: ["critical"] }],
    });

    expect(merged.anyOf).toEqual(base.anyOf);
    expect(merged.filters).toEqual([{ field: "severity", op: "in", values: ["critical"] }]);
  });

  it("takes the slice's `anyOf` when the base sets none", () => {
    const merged = mergeWidgetFilters(
      { filters: [{ field: "state", op: "in", values: ["open"] }] },
      { anyOf: [{ filters: [{ field: "severity", op: "in", values: ["critical"] }] }] },
    );

    expect(merged.anyOf).toEqual([
      { filters: [{ field: "severity", op: "in", values: ["critical"] }] },
    ]);
    expect(merged.filters).toEqual([{ field: "state", op: "in", values: ["open"] }]);
  });

  // An unrecognisable `anyOf` (not an array of {filters: [...]}) must not be
  // silently mangled into something the backend would accept but mean
  // differently -- last-writer-wins is the honest fallback there.
  it("falls back to the slice's value when `anyOf` is not the expected branch shape", () => {
    const merged = mergeWidgetFilters(
      { anyOf: "nonsense" as unknown as [] },
      { anyOf: [{ filters: [{ field: "state", op: "in", values: ["open"] }] }] },
    );

    expect(merged.anyOf).toEqual([
      { filters: [{ field: "state", op: "in", values: ["open"] }] },
    ]);
  });
});
