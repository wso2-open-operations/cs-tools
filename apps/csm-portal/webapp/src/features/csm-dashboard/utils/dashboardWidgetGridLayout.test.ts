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
import type { BeDashboardWidget } from "@api/backend/types";
import {
  denseWidgetGridSx,
  denseWidgetIconSx,
  denseWidgetLabelSx,
  groupWidgetsBySection,
  isDenseSection,
  WIDGET_GRID_SX,
} from "@features/csm-dashboard/utils/dashboardWidgetGridLayout";

function makeWidget(overrides: Partial<BeDashboardWidget> = {}): BeDashboardWidget {
  return {
    widgetId: "widget_1",
    displayName: "Widget",
    resourceType: "case",
    shape: "count",
    gridWidth: 4,
    query: {},
    ...overrides,
  } as BeDashboardWidget;
}

describe("isDenseSection", () => {
  it("is false for an empty widget list", () => {
    expect(isDenseSection([])).toBe(false);
  });

  it("is true when every widget in the list is shape 'count'", () => {
    expect(
      isDenseSection([
        makeWidget({ widgetId: "a" }),
        makeWidget({ widgetId: "b" }),
        makeWidget({ widgetId: "c" }),
      ]),
    ).toBe(true);
  });

  it("is false when at least one widget isn't shape 'count', regardless of position", () => {
    expect(
      isDenseSection([
        makeWidget({ widgetId: "a" }),
        makeWidget({ widgetId: "b", shape: "bar", slices: [] }),
      ]),
    ).toBe(false);
    expect(
      isDenseSection([
        makeWidget({ widgetId: "a", shape: "list" }),
        makeWidget({ widgetId: "b" }),
      ]),
    ).toBe(false);
  });

  it("is false for a single non-count widget", () => {
    expect(isDenseSection([makeWidget({ shape: "pie", slices: [] })])).toBe(false);
  });
});

describe("denseWidgetGridSx", () => {
  it("defaults to a 168px auto-fill minimum column width, only at xl and above", () => {
    const sx = denseWidgetGridSx();
    expect(sx.display).toBe("grid");
    expect(sx.gridTemplateColumns.xl).toBe("repeat(auto-fill, minmax(168px, 1fr))");
  });

  it("honors a custom minimum column width, only at xl and above", () => {
    const sx = denseWidgetGridSx(200);
    expect(sx.gridTemplateColumns.xl).toBe("repeat(auto-fill, minmax(200px, 1fr))");
  });

  it("falls back to the exact same tracks and gap as WIDGET_GRID_SX below xl", () => {
    const sx = denseWidgetGridSx();
    expect(sx.gridTemplateColumns.xs).toBe(WIDGET_GRID_SX.gridTemplateColumns.xs);
    expect(sx.gridTemplateColumns.sm).toBe(WIDGET_GRID_SX.gridTemplateColumns.sm);
    expect(sx.gap.xs).toBe(WIDGET_GRID_SX.gap);
  });

  it("only tightens the gap at xl and above", () => {
    const sx = denseWidgetGridSx();
    expect(sx.gap.xl).toBe(1.25);
  });
});

describe("denseWidgetLabelSx", () => {
  it("is undefined when not dense — no styling change at all", () => {
    expect(denseWidgetLabelSx(false)).toBeUndefined();
  });

  it("gates the 2-line clamp (and the single-line fallback below it) to xl and above", () => {
    // A full DOM render can't verify this directly — jsdom does not
    // reliably evaluate an emotion-generated `@media` rule against a
    // simulated viewport width (confirmed while fixing the bug this
    // replaces), so this asserts the sx object itself, the same way the
    // `denseWidgetGridSx` tests above do for the grid tracks.
    const sx = denseWidgetLabelSx(true);
    // Below xl (the base/"xs" value): reproduces `noWrap`'s single-line
    // ellipsis truncation by hand.
    expect(sx?.whiteSpace.xs).toBe("nowrap");
    expect(sx?.textOverflow.xs).toBe("ellipsis");
    // At xl and above: switches to a 2-line clamp instead.
    expect(sx?.whiteSpace.xl).toBe("normal");
    expect(sx?.WebkitLineClamp.xl).toBe(2);
    expect(sx?.display.xl).toBe("-webkit-box");
  });
});

describe("denseWidgetIconSx", () => {
  it("is undefined when not dense — no icon-sizing change at all", () => {
    expect(denseWidgetIconSx(false)).toBeUndefined();
  });

  it("only shrinks the icon at xl and above", () => {
    expect(denseWidgetIconSx(true)?.transform.xl).toBe("scale(0.875)");
  });
});

// `groupWidgetsBySection` itself is pre-existing (not part of this change),
// but exercised here alongside `isDenseSection` since a real caller
// (`DashboardWidgetGrid`) always evaluates the latter against the former's
// own output, one group at a time — this documents that exact composition.
describe("groupWidgetsBySection + isDenseSection composition", () => {
  it("classifies each of a multi-section dashboard's own groups independently", () => {
    const widgets = [
      makeWidget({ widgetId: "cre_a", section: "CRE" }),
      makeWidget({ widgetId: "cre_b", section: "CRE" }),
      makeWidget({ widgetId: "sre_a", section: "SRE", shape: "list" }),
      makeWidget({ widgetId: "sre_b", section: "SRE" }),
    ];
    const groups = groupWidgetsBySection(widgets);
    expect(groups).toHaveLength(2);

    const [creGroup, sreGroup] = groups;
    expect(creGroup.section).toBe("CRE");
    expect(isDenseSection(creGroup.widgets)).toBe(true);
    expect(sreGroup.section).toBe("SRE");
    expect(isDenseSection(sreGroup.widgets)).toBe(false);
  });
});
