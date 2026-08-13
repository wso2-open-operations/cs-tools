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
import {
  buildWidgetPreviewHref,
  describeWidgetFilters,
  parseWidgetPreviewFilters,
  resolveCurrentUserSentinels,
} from "./widgetPreviewUrl";

const CURRENT_USER_ID = "11111111-aaaa-bbbb-cccc-000000000001";

describe("widgetPreviewUrl", () => {
  it("encodes each filter field as its own readable query param, not one JSON blob", () => {
    const href = buildWidgetPreviewHref({
      previewSlug: "cases",
      widgetId: "my_critical_open",
      displayName: "My Critical & High Cases",
      filters: { severities: ["critical", "high"], states: ["open"] },
    });

    expect(href.startsWith("/dashboard/preview/cases?")).toBe(true);
    const params = new URLSearchParams(href.split("?")[1]);
    expect(params.get("w")).toBe("my_critical_open");
    expect(params.get("n")).toBe("My Critical & High Cases");
    expect(params.get("severities")).toBe("critical,high");
    expect(params.get("states")).toBe("open");
    expect(params.get("f")).toBeNull();
  });

  it("masks the current user's own id to @me instead of embedding it verbatim", () => {
    const href = buildWidgetPreviewHref({
      previewSlug: "cases",
      widgetId: "my_cases",
      displayName: "My Cases",
      filters: { assignedUserIds: [CURRENT_USER_ID] },
      currentUserId: CURRENT_USER_ID,
    });

    expect(href).not.toContain(CURRENT_USER_ID);
    const params = new URLSearchParams(href.split("?")[1]);
    expect(params.get("assignedUserIds")).toBe("@me");
  });

  it("round-trips filters through parseWidgetPreviewFilters + resolveCurrentUserSentinels", () => {
    const href = buildWidgetPreviewHref({
      previewSlug: "cases",
      widgetId: "my_cases",
      displayName: "My Cases",
      filters: { assignedUserIds: [CURRENT_USER_ID], severities: ["critical"] },
      currentUserId: CURRENT_USER_ID,
    });

    const searchParams = new URLSearchParams(href.split("?")[1]);
    const { filters, needsCurrentUser } = parseWidgetPreviewFilters(searchParams);
    expect(needsCurrentUser).toBe(true);
    expect(filters.severities).toEqual(["critical"]);
    expect(filters.assignedUserIds).toEqual(["@me"]);

    const resolved = resolveCurrentUserSentinels(filters, CURRENT_USER_ID);
    expect(resolved.assignedUserIds).toEqual([CURRENT_USER_ID]);
    expect(resolved.severities).toEqual(["critical"]);
  });

  it("leaves the @me sentinel in place when the current user id isn't known yet", () => {
    const resolved = resolveCurrentUserSentinels({ assignedUserIds: ["@me"] }, undefined);
    expect(resolved.assignedUserIds).toEqual(["@me"]);
  });

  it("ignores the reserved w/n params when parsing filters back", () => {
    const searchParams = new URLSearchParams({ w: "id", n: "Name", severities: "critical" });
    const { filters } = parseWidgetPreviewFilters(searchParams);
    expect(filters).toEqual({ severities: ["critical"] });
  });

  it("flattens a case widget's nested field/op/values filter array into readable query params", () => {
    const href = buildWidgetPreviewHref({
      previewSlug: "cases",
      widgetId: "my_critical_open",
      displayName: "My Critical & High Cases",
      filters: {
        filters: [
          { field: "severity", op: "in", values: ["critical", "high"] },
          { field: "state", op: "in", values: ["open"] },
        ],
      },
    });

    const params = new URLSearchParams(href.split("?")[1]);
    expect(params.get("severity")).toBe("critical,high");
    expect(params.get("state")).toBe("open");
    // No opaque JSON blob under the literal `filters` key.
    expect(params.get("filters")).toBeNull();
  });

  it("masks the current user's own id inside a case widget's nested filter array", () => {
    const href = buildWidgetPreviewHref({
      previewSlug: "cases",
      widgetId: "my_cases",
      displayName: "My Cases",
      filters: {
        filters: [{ field: "assignedUserId", op: "in", values: [CURRENT_USER_ID] }],
      },
      currentUserId: CURRENT_USER_ID,
    });

    expect(href).not.toContain(CURRENT_USER_ID);
    const params = new URLSearchParams(href.split("?")[1]);
    expect(params.get("assignedUserId")).toBe("@me");
  });

  it("round-trips a case widget's nested filter array through parse + resolveCurrentUserSentinels", () => {
    const href = buildWidgetPreviewHref({
      previewSlug: "cases",
      widgetId: "my_cases",
      displayName: "My Cases",
      filters: {
        filters: [
          { field: "assignedUserId", op: "in", values: [CURRENT_USER_ID] },
          { field: "severity", op: "in", values: ["critical"] },
        ],
      },
      currentUserId: CURRENT_USER_ID,
    });

    const searchParams = new URLSearchParams(href.split("?")[1]);
    const { filters, needsCurrentUser } = parseWidgetPreviewFilters(searchParams);
    expect(needsCurrentUser).toBe(true);
    expect(filters.filters).toEqual([
      { field: "assignedUserId", op: "in", values: ["@me"] },
      { field: "severity", op: "in", values: ["critical"] },
    ]);

    const resolved = resolveCurrentUserSentinels(filters, CURRENT_USER_ID);
    expect(resolved.filters).toEqual([
      { field: "assignedUserId", op: "in", values: [CURRENT_USER_ID] },
      { field: "severity", op: "in", values: ["critical"] },
    ]);
  });
});

/**
 * Regression: the preview URL used to drop each filter entry's `op`, so every
 * entry decoded back as `in`. That INVERTED `notIn` (a tag exclusion became a
 * tag filter) and dropped value-less ops entirely, silently widening
 * "Unassigned Cases" into "all cases". Found by live click-through testing,
 * not by any unit test -- hence this one.
 */
describe("widget preview URL — filter op round-trip", () => {
  function roundTrip(filters: { field: string; op: string; values?: string[] }[]) {
    const href = buildWidgetPreviewHref({
      previewSlug: "cases",
      widgetId: "w1",
      displayName: "W",
      filters: { filters },
    });
    const qs = href.split("?")[1] ?? "";
    return parseWidgetPreviewFilters(new URLSearchParams(qs));
  }

  it("preserves notIn instead of inverting it to in", () => {
    const parsed = roundTrip([{ field: "tag", op: "notIn", values: ["s_dip"] }]);
    const entries = (parsed.filters as { filters: { field: string; op: string }[] }).filters;
    expect(entries).toEqual([{ field: "tag", op: "notIn", values: ["s_dip"] }]);
  });

  it("preserves value-less ops rather than dropping them", () => {
    const parsed = roundTrip([{ field: "assignedUserId", op: "isEmpty", values: [] }]);
    const entries = (parsed.filters as { filters: { field: string; op: string }[] }).filters;
    expect(entries).toEqual([{ field: "assignedUserId", op: "isEmpty", values: [] }]);
  });

  it("keeps the bare field=values form for the default in op", () => {
    const href = buildWidgetPreviewHref({
      previewSlug: "cases",
      widgetId: "w1",
      displayName: "W",
      filters: { filters: [{ field: "state", op: "in", values: ["open"] }] },
    });
    expect(href).toContain("state=open");
    expect(href).not.toContain("~");
  });

  it("round-trips a mixed filter set faithfully", () => {
    const input = [
      { field: "state", op: "in", values: ["open", "reopened"] },
      { field: "tag", op: "notIn", values: ["s_dip", "patch"] },
      { field: "escalation", op: "isNotEmpty", values: [] },
    ];
    const parsed = roundTrip(input);
    const entries = (parsed.filters as { filters: unknown[] }).filters;
    expect(entries).toEqual(input);
  });
});

describe("describeWidgetFilters", () => {
  it("flattens the flat resourceType filter shape into readable field: value entries", () => {
    expect(
      describeWidgetFilters({ severities: ["critical", "high"], states: ["open"] }),
    ).toEqual([
      { field: "severities", value: "critical, high" },
      { field: "states", value: "open" },
    ]);
  });

  it("flattens the case field/op/values DSL shape, omitting the op for the default 'in'", () => {
    expect(
      describeWidgetFilters({
        filters: [
          { field: "state", op: "in", values: ["open"] },
          { field: "tag", op: "notIn", values: ["s_dip"] },
        ],
      }),
    ).toEqual([
      { field: "state", op: undefined, value: "open" },
      { field: "tag", op: "notIn", value: "s_dip" },
    ]);
  });

  it("still shows a value-less op (isEmpty/isNotEmpty) rather than silently dropping it", () => {
    expect(
      describeWidgetFilters({
        filters: [{ field: "escalation", op: "isNotEmpty", values: [] }],
      }),
    ).toEqual([{ field: "escalation", op: "isNotEmpty", value: "(no value)" }]);
  });

  it("shows an already-resolved team filter's real groupId value, not a placeholder", () => {
    expect(
      describeWidgetFilters({
        filters: [
          {
            field: "integrationCsTeam",
            op: "in",
            values: ["22222222-2222-2222-2222-222222222222"],
          },
        ],
      }),
    ).toEqual([
      {
        field: "integrationCsTeam",
        op: undefined,
        value: "22222222-2222-2222-2222-222222222222",
      },
    ]);
  });

  it("returns an empty list for empty/absent filters", () => {
    expect(describeWidgetFilters({})).toEqual([]);
  });
});
