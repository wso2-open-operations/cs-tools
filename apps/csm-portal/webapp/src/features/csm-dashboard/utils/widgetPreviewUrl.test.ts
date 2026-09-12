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
  appendWidgetTitleParam,
  buildWidgetPreviewHref,
  describeWidgetFilters,
  parseWidgetPreviewFilters,
  readWidgetTitleParam,
  resolveCurrentUserSentinels,
  WIDGET_TITLE_PARAM,
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

  it("encodes a plain numeric filter value, not just string/string[]", () => {
    // Regression: the rating-distribution pie's slice query is
    // `{ rating: Math.round(avgRating) }` (see useCaseFeedbackTrendData) — a
    // number, not a string. Before this branch existed, a numeric filter
    // value was silently dropped from the URL entirely, so clicking a
    // rating slice landed on the unfiltered feedback list.
    const href = buildWidgetPreviewHref({
      previewSlug: "case-feedback",
      widgetId: "feedback_rating_distribution",
      displayName: "Rating Distribution",
      filters: { rating: 5 },
    });

    const params = new URLSearchParams(href.split("?")[1]);
    expect(params.get("rating")).toBe("5");

    const { filters } = parseWidgetPreviewFilters(params);
    expect(filters.rating).toEqual(["5"]);
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

/**
 * Regression (digiops-cs#2880): `anyOf` (cross-field OR branches) used to be
 * silently dropped entirely by this file -- neither serialized into the
 * preview URL nor read back out of it -- so a widget's "View more" click
 * (and, separately, `WIDGET_RESOURCE_CONFIG`'s own count-tile `buildHref`)
 * landed on a broader, unfiltered-by-`anyOf` result set than what the tile
 * itself had counted.
 */
describe("widget preview URL — anyOf round-trip", () => {
  const anyOf = [
    { filters: [{ field: "severity", op: "in", values: ["catastrophic", "critical"] }] },
    { filters: [{ field: "type", op: "in", values: ["security_report_analysis"] }] },
  ];

  it("round-trips anyOf branches through build + parse, alongside a flat filters object", () => {
    const href = buildWidgetPreviewHref({
      previewSlug: "cases",
      widgetId: "w1",
      displayName: "WOW P0/P1",
      filters: { severities: ["critical"], anyOf },
    });

    const searchParams = new URLSearchParams(href.split("?")[1]);
    const { filters } = parseWidgetPreviewFilters(searchParams);
    expect(filters.severities).toEqual(["critical"]);
    expect(filters.anyOf).toEqual(anyOf);
  });

  it("round-trips anyOf branches alongside the nested case field/op/values filter shape", () => {
    const href = buildWidgetPreviewHref({
      previewSlug: "cases",
      widgetId: "w1",
      displayName: "WOW P0/P1",
      filters: {
        filters: [{ field: "state", op: "in", values: ["open"] }],
        anyOf,
      },
    });

    const searchParams = new URLSearchParams(href.split("?")[1]);
    const { filters } = parseWidgetPreviewFilters(searchParams);
    expect(filters.filters).toEqual([{ field: "state", op: "in", values: ["open"] }]);
    expect(filters.anyOf).toEqual(anyOf);
  });

  it("masks the current user's own id inside an anyOf branch, and resolveCurrentUserSentinels restores it", () => {
    const href = buildWidgetPreviewHref({
      previewSlug: "cases",
      widgetId: "w1",
      displayName: "My anyOf widget",
      filters: {
        anyOf: [{ filters: [{ field: "assignedUserId", op: "in", values: [CURRENT_USER_ID] }] }],
      },
      currentUserId: CURRENT_USER_ID,
    });

    expect(href).not.toContain(CURRENT_USER_ID);
    const searchParams = new URLSearchParams(href.split("?")[1]);
    const { filters, needsCurrentUser } = parseWidgetPreviewFilters(searchParams);
    expect(needsCurrentUser).toBe(true);
    expect(filters.anyOf).toEqual([
      { filters: [{ field: "assignedUserId", op: "in", values: ["@me"] }] },
    ]);

    const resolved = resolveCurrentUserSentinels(filters, CURRENT_USER_ID);
    expect(resolved.anyOf).toEqual([
      { filters: [{ field: "assignedUserId", op: "in", values: [CURRENT_USER_ID] }] },
    ]);
  });

  it("drops a malformed anyOf param rather than throwing", () => {
    const searchParams = new URLSearchParams({ w: "id", n: "Name", _anyOf: "not json{{{" });
    expect(() => parseWidgetPreviewFilters(searchParams)).not.toThrow();
    const { filters } = parseWidgetPreviewFilters(searchParams);
    expect(filters.anyOf).toBeUndefined();
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
            field: "creTeam",
            op: "in",
            values: ["22222222-2222-2222-2222-222222222222"],
          },
        ],
      }),
    ).toEqual([
      {
        field: "creTeam",
        op: undefined,
        value: "22222222-2222-2222-2222-222222222222",
      },
    ]);
  });

  it("returns an empty list for empty/absent filters", () => {
    expect(describeWidgetFilters({})).toEqual([]);
  });
});

describe("appendWidgetTitleParam / readWidgetTitleParam", () => {
  it("appends the widget's displayName as WIDGET_TITLE_PARAM to a bare path", () => {
    const href = appendWidgetTitleParam("/engagements", "Total Outstanding");
    expect(href).toBe(`/engagements?${WIDGET_TITLE_PARAM}=Total+Outstanding`);
  });

  it("appends to a path that already has query params, without disturbing them", () => {
    const href = appendWidgetTitleParam(
      "/cases?states=open&severities=S1",
      "My Critical & High Cases",
    );
    const params = new URLSearchParams(href.split("?")[1]);
    expect(href.startsWith("/cases?")).toBe(true);
    expect(params.get("states")).toBe("open");
    expect(params.get("severities")).toBe("S1");
    expect(params.get(WIDGET_TITLE_PARAM)).toBe("My Critical & High Cases");
  });

  it("is a no-op when displayName is absent or empty", () => {
    expect(appendWidgetTitleParam("/engagements", undefined)).toBe("/engagements");
    expect(appendWidgetTitleParam("/engagements", "")).toBe("/engagements");
    expect(appendWidgetTitleParam("/cases?states=open", undefined)).toBe("/cases?states=open");
  });

  it("round-trips through readWidgetTitleParam", () => {
    const href = appendWidgetTitleParam("/engagements?types=engagement", "Migration Summary");
    const params = new URLSearchParams(href.split("?")[1]);
    expect(readWidgetTitleParam(params)).toBe("Migration Summary");
  });

  it("readWidgetTitleParam returns undefined when the param is absent or empty", () => {
    expect(readWidgetTitleParam(new URLSearchParams("states=open"))).toBeUndefined();
    expect(readWidgetTitleParam(new URLSearchParams(`${WIDGET_TITLE_PARAM}=`))).toBeUndefined();
  });
});
