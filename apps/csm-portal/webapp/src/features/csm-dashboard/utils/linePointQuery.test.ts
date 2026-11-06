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
import { linePointQuery } from "@features/csm-dashboard/utils/linePointQuery";
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

  it("keeps the widget's own filters and replaces its own window on the same field", () => {
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
