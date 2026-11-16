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
import { linePointFilters, linePointQuery } from "@features/csm-dashboard/utils/linePointQuery";
import { mergeWidgetFilters } from "@features/csm-dashboard/utils/widgetFilterMerge";

describe("linePointQuery", () => {
  it("an Opened point is the cases created on that day", () => {
    expect(linePointQuery("2026-11-03", "opened")).toEqual({
      filters: [
        { field: "createdOn", op: "gte", values: ["2026-11-03"] },
        { field: "createdOn", op: "lte", values: ["2026-11-03"] },
      ],
    });
  });

  it("a Closed point is the cases closed on that day", () => {
    expect(linePointQuery("2026-11-03", "closed")).toEqual({
      filters: [
        { field: "closedOn", op: "gte", values: ["2026-11-03"] },
        { field: "closedOn", op: "lte", values: ["2026-11-03"] },
      ],
    });
  });

  it("a week or month point is the cases from its first day to its last", () => {
    expect(linePointQuery("2025-12-29", "opened", "2025-12-31")).toEqual({
      filters: [
        { field: "createdOn", op: "gte", values: ["2025-12-29"] },
        { field: "createdOn", op: "lte", values: ["2025-12-31"] },
      ],
    });
  });

  it("mergeWidgetFilters alone replaces the widget's own window on the same field", () => {
    const widget = {
      filters: [
        { field: "type", op: "in", values: ["case"] },
        { field: "createdOn", op: "gte", values: ["__daysAgo:30__"] },
      ],
    };
    expect(mergeWidgetFilters(widget, linePointQuery("2026-11-03", "opened"))).toEqual({
      filters: [
        { field: "type", op: "in", values: ["case"] },
        { field: "createdOn", op: "gte", values: ["2026-11-03"] },
        { field: "createdOn", op: "lte", values: ["2026-11-03"] },
      ],
    });
    expect(mergeWidgetFilters(widget, linePointQuery("2026-11-03", "closed"))).toEqual({
      filters: [
        { field: "type", op: "in", values: ["case"] },
        { field: "createdOn", op: "gte", values: ["__daysAgo:30__"] },
        { field: "closedOn", op: "gte", values: ["2026-11-03"] },
        { field: "closedOn", op: "lte", values: ["2026-11-03"] },
      ],
    });
  });
});

describe("linePointFilters", () => {
  // Saturday 7 November 2026, noon UTC.
  const NOW = new Date(Date.UTC(2026, 10, 7, 12, 0, 0));
  const type = { field: "type", op: "in", values: ["case"] };

  it("takes the point's own bounds when the widget's are wider", () => {
    const widget = { filters: [type, { field: "createdOn", op: "gte", values: ["__daysAgo:30__"] }] };
    expect(linePointFilters(widget, "2026-11-03", "opened", undefined, NOW)).toEqual({
      filters: [
        type,
        { field: "createdOn", op: "gte", values: ["2026-11-03"] },
        { field: "createdOn", op: "lte", values: ["2026-11-03"] },
      ],
    });
  });

  it("keeps the widget's own lte when it ends inside the point's day", () => {
    const lte = { field: "createdOn", op: "lte", values: ["2026-11-03T12:00:00Z"] };
    expect(linePointFilters({ filters: [type, lte] }, "2026-11-03", "opened", undefined, NOW)).toEqual({
      filters: [type, { field: "createdOn", op: "gte", values: ["2026-11-03"] }, lte],
    });
  });

  it("keeps the widget's own gte when it starts inside a week point", () => {
    const gte = { field: "createdOn", op: "gte", values: ["2025-12-30"] };
    expect(linePointFilters({ filters: [gte] }, "2025-12-29", "opened", "2025-12-31", NOW)).toEqual({
      filters: [gte, { field: "createdOn", op: "lte", values: ["2025-12-31"] }],
    });
  });

  it("keeps a tighter placeholder as a placeholder, for the case list", () => {
    // __endOfMonth:-1__ is 31 October: inside a week point from 26 October.
    const lte = { field: "closedOn", op: "lte", values: ["__endOfMonth:-1__"] };
    expect(linePointFilters({ filters: [lte] }, "2026-10-26", "closed", "2026-11-01", NOW)).toEqual({
      filters: [{ field: "closedOn", op: "gte", values: ["2026-10-26"] }, lte],
    });
  });

  it("leaves another field's bounds alone", () => {
    const createdLte = { field: "createdOn", op: "lte", values: ["2026-11-01"] };
    expect(linePointFilters({ filters: [createdLte] }, "2026-11-03", "closed", undefined, NOW)).toEqual({
      filters: [
        createdLte,
        { field: "closedOn", op: "gte", values: ["2026-11-03"] },
        { field: "closedOn", op: "lte", values: ["2026-11-03"] },
      ],
    });
  });

  it("takes the point's bound when the widget's cannot be read as a date", () => {
    const widget = { filters: [{ field: "createdOn", op: "lte", values: ["not a date"] }] };
    expect(linePointFilters(widget, "2026-11-03", "opened", undefined, NOW)).toEqual({
      filters: [
        { field: "createdOn", op: "gte", values: ["2026-11-03"] },
        { field: "createdOn", op: "lte", values: ["2026-11-03"] },
      ],
    });
  });

  it("works on a widget without a query", () => {
    expect(linePointFilters(null, "2026-11-03", "opened", undefined, NOW)).toEqual(linePointQuery("2026-11-03", "opened"));
  });

  // rksk's case: the list counts what the point counted. The point is the
  // widget's filters AND the point's day; the list is the merged filters as
  // entity-service applies them, one gte and one lte per field (a later entry
  // replaces an earlier one).
  it("lists exactly the cases the point counted when the widget has a same-field lte", () => {
    const created = [
      "2026-11-03T01:00:00Z",
      "2026-11-03T11:59:00Z",
      "2026-11-03T12:01:00Z",
      "2026-11-03T23:00:00Z",
      "2026-11-04T09:00:00Z",
    ].map(Date.parse);
    const widget = { filters: [{ field: "createdOn", op: "lte", values: ["2026-11-03T12:00:00Z"] }] };
    const dayStart = Date.parse("2026-11-03T00:00:00.000Z");
    const dayEnd = Date.parse("2026-11-03T23:59:59.999Z");
    const widgetLte = Date.parse("2026-11-03T12:00:00Z");
    const pointCount = created.filter((t) => t <= widgetLte && t >= dayStart && t <= dayEnd).length;

    const bounds: { gte?: number; lte?: number } = {};
    for (const f of linePointFilters(widget, "2026-11-03", "opened", undefined, NOW).filters as {
      field: string;
      op: "gte" | "lte";
      values: string[];
    }[]) {
      const v = f.values[0];
      const dateOnly = /^\d{4}-\d{2}-\d{2}$/.test(v);
      bounds[f.op] = Date.parse(dateOnly ? `${v}${f.op === "gte" ? "T00:00:00.000Z" : "T23:59:59.999Z"}` : v);
    }
    const listCount = created.filter((t) => t >= bounds.gte! && t <= bounds.lte!).length;

    expect(pointCount).toBe(2);
    expect(listCount).toBe(pointCount);
  });
});
