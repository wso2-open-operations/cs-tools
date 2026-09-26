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

import type { BeDashboardWidget } from "@api/backend/types";

/** 12-column grid, matching each widget's own `gridWidth`; on very small
 * screens there's only room for 4 columns, so a wide widget there wraps to
 * (at most) one extra row rather than overflowing. Shared by
 * `DashboardWidgetGrid` (the real grid) and every page that shows a
 * same-shaped loading skeleton before that grid's own data has resolved. */
export const WIDGET_GRID_SX = {
  display: "grid",
  gap: 1.5,
  gridTemplateColumns: {
    xs: "repeat(4, minmax(0, 1fr))",
    sm: "repeat(12, minmax(0, 1fr))",
  },
} as const;

/** A denser alternative to `WIDGET_GRID_SX` for a section made up ENTIRELY
 * of `shape: "count"` widgets (see `isDenseSection`) — those tiles are just
 * an icon, a label and a number, so packing more of them per row (as many
 * as fit at a comfortable minimum width, via `auto-fill`) reads fine even
 * though it drops each widget's own `gridWidth`-driven relative sizing.
 * `gridWidth` still matters for a section with any pie/bar/list widget in
 * it (that shape genuinely needs the width its author gave it), which is
 * exactly the case `WIDGET_GRID_SX` above continues to cover — this export
 * only replaces it where every widget in the section opted out of that
 * distinction by being the plainest shape there is.
 *
 * `auto-fill`/`minmax` (rather than a wider fixed column COUNT, e.g.
 * `repeat(24, …)`) is deliberate: it scales with the real container width
 * at render time instead of assuming a specific viewport, so this needs no
 * viewport-specific breakpoint or magic pixel budget to stay correct as the
 * window resizes — on a narrow window it gracefully falls back to fewer,
 * still `minWidthPx`-wide columns (wrapping more rows) rather than
 * squeezing every tile down to illegibility the way a fixed column count
 * would.
 *
 * Below the `xl` breakpoint this falls back to the exact same tracks as
 * `WIDGET_GRID_SX` (not `auto-fill`) — the density trade-off this grid
 * makes (dropping each widget's own `gridWidth`) is only worth it once
 * tiles are actually cramped for space, which on a laptop-and-smaller
 * viewport they aren't. Every other dense-mode style in
 * `DashboardWidgetTile.tsx` is gated to `xl` the same way, for the same
 * reason — a section rendered at a narrower viewport (or embedded
 * somewhere the dashboard's own container is narrower than a full page,
 * e.g. a preview) should look identical whether or not it happens to be
 * all-`count`-shape. */
export function denseWidgetGridSx(minWidthPx = 168) {
  return {
    display: "grid",
    // Same xs-base/xl-override pattern as `gridTemplateColumns` below: the
    // tighter `1.25` gap is part of the dense treatment, so it only applies
    // once the tracks themselves actually go dense at `xl` — below that,
    // this must match `WIDGET_GRID_SX.gap` exactly, not just "look similar."
    gap: { xs: WIDGET_GRID_SX.gap, xl: 1.25 },
    gridTemplateColumns: {
      xs: "repeat(4, minmax(0, 1fr))",
      sm: "repeat(12, minmax(0, 1fr))",
      xl: `repeat(auto-fill, minmax(${minWidthPx}px, 1fr))`,
    },
  } as const;
}

/** The icon-shrink transform for a dense (all-count-section) count tile's
 * icon (see `DashboardWidgetTile.tsx`) — a pure function (rather than
 * inlined in that component's JSX) so its `xl`-only gating can be
 * unit-tested directly as a plain object, without depending on jsdom
 * correctly evaluating an emotion-generated `@media` rule (it does not —
 * confirmed while fixing the bug this replaces, where the shrink used to
 * apply at every breakpoint instead of only `xl` and above). `undefined`
 * for a non-dense tile — no icon-sizing change at all. */
export function denseWidgetIconSx(dense: boolean) {
  return dense ? { transform: { xl: "scale(0.875)" } } : undefined;
}

/** The label-wrap sx for a dense (all-count-section) count tile's title —
 * see `denseWidgetIconSx` above for why this is a separate, directly
 * testable function rather than inline JSX. Below `xl` this reproduces
 * MUI's own `noWrap` (single-line, ellipsis) by hand, since the `noWrap`
 * prop itself can't be made responsive; at `xl` and above it switches to a
 * 2-line `-webkit-line-clamp` instead, so a long label (e.g. "FDE -
 * InProgress Engagement Case") wraps instead of truncating mid-word.
 * `undefined` for a non-dense tile — the caller keeps using the plain
 * `noWrap` prop as before. */
export function denseWidgetLabelSx(dense: boolean) {
  if (!dense) return undefined;
  return {
    whiteSpace: { xs: "nowrap", xl: "normal" },
    overflow: "hidden",
    textOverflow: { xs: "ellipsis", xl: "clip" },
    fontSize: { xl: "0.7rem" },
    display: { xl: "-webkit-box" },
    WebkitLineClamp: { xl: 2 },
    WebkitBoxOrient: { xl: "vertical" },
    lineHeight: { xl: 1.2 },
    // Reserves the full 2-line height up front (rather than only as tall as
    // a shorter label needs) so every tile in the same dense grid row stays
    // the same height — `auto-fill`/`minmax` grid tracks don't otherwise
    // force sibling row items to match a taller neighbor's label. Only
    // applies at `xl`, where the 2-line clamp itself is active.
    minHeight: { xl: "2.4em" },
  } as const;
}

export interface WidgetGroup {
  /** `undefined` for the untitled/default group — every widget with no
   * `section` set lands here, rendered exactly as before this field
   * existed (no heading). */
  section?: string;
  widgets: BeDashboardWidget[];
}

/** True when every widget in `widgets` is `shape: "count"` — see
 * `denseWidgetGridSx`'s own doc comment for why that's the bar for opting a
 * section into the denser grid instead of the `gridWidth`-proportional one.
 * `false` for an empty list (nothing to render densely). */
export function isDenseSection(widgets: BeDashboardWidget[]): boolean {
  return widgets.length > 0 && widgets.every((w) => w.shape === "count");
}

/** Groups widgets by `section`, preserving the order each distinct section
 * value (including the untitled default) first appears among `widgets` —
 * see `BeDashboardWidget.section`. Exported (in its own non-component
 * module, not `DashboardWidgetGrid.tsx` itself) so the dashboard builder
 * can derive the same section-name list a live dashboard would render,
 * without duplicating this grouping rule. */
export function groupWidgetsBySection(widgets: BeDashboardWidget[]): WidgetGroup[] {
  const groups: WidgetGroup[] = [];
  const indexBySection = new Map<string | undefined, number>();
  for (const widget of widgets) {
    const key = widget.section || undefined;
    let index = indexBySection.get(key);
    if (index === undefined) {
      index = groups.length;
      indexBySection.set(key, index);
      groups.push({ section: key, widgets: [] });
    }
    groups[index].widgets.push(widget);
  }
  return groups;
}
