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

import { fireEvent, render, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import { MemoryRouter } from "react-router";
import type { ReactNode } from "react";
import type { BeDashboardWidget } from "@api/backend/types";
import type { PieSliceResult } from "@features/csm-dashboard/api/useWidgetPieData";

// `DashboardWidgetGrid` now imports `WidgetInlineDrilldownPanel` directly
// (unmocked, its own real module graph reaches `widgetListConfig.tsx`,
// which pulls in `useTimeSheets.ts` — time_card's mapper — which reads
// `window.config` at load via `@config/apiConfig`, unavailable under
// vitest). See `DashboardWidgetTile.test.tsx`'s identical mock/comment.
vi.mock("@config/apiConfig", () => ({
  apiConfig: { backendUrl: "https://example.test" },
}));

// Every widget id's own most-recently-received `filters`/`onExpandChange`
// prop, captured on every mock render — lets this file's own tests assert
// directly on prop IDENTITY (not just value) across an unrelated widget's
// expand/collapse, which is the actual mechanism (see
// `DashboardWidgetGrid`'s own `getResolvedFilters`/`getOnExpandChange`
// caches) that lets the real `DashboardWidgetTile`'s `React.memo` bail out
// of re-rendering a widget nowhere near the one that was clicked. Reset
// per-test via `beforeEach` so one test's captures can't leak into another.
let capturedTileProps: Record<
  string,
  { filters: unknown; onExpandChange: unknown; renderCount: number }
> = {};

// Stubs the real tile out entirely — this test is only about
// `DashboardWidgetGrid`'s own wiring of `hideRefreshButton` alongside
// `renderWidgetAction`, not about anything the real tile fetches/renders.
// Exposes both as plain text so the assertions below can read them straight
// out of the DOM rather than needing a spy. Also exposes `expandedSlice`/
// `onExpandChange` as a plain button so this file's own tests can assert on
// the LIFTED expand/collapse behavior `DashboardWidgetGrid` now owns,
// without depending on the real chart/slice-click machinery
// `DashboardWidgetTile.test.tsx` already covers in full.
vi.mock("@features/csm-dashboard/components/DashboardWidgetTile", () => ({
  default: ({
    widgetId,
    hideRefreshButton,
    expandedSlice,
    onExpandChange,
    filters,
  }: {
    widgetId: string;
    hideRefreshButton?: boolean;
    expandedSlice?: PieSliceResult | null;
    onExpandChange?: (slice: PieSliceResult | null) => void;
    filters?: unknown;
  }) => {
    const previous = capturedTileProps[widgetId];
    capturedTileProps[widgetId] = {
      filters,
      onExpandChange,
      renderCount: (previous?.renderCount ?? 0) + 1,
    };
    return (
      <div data-testid={`tile-${widgetId}`}>
        {!hideRefreshButton && (
          <button type="button" aria-label={`Refresh ${widgetId}`}>
            refresh
          </button>
        )}
        {onExpandChange && (
          <button
            type="button"
            aria-label={`Expand a slice on ${widgetId}`}
            onClick={() =>
              onExpandChange(
                expandedSlice
                  ? null
                  : { label: `${widgetId}-slice`, value: 1, query: {} },
              )
            }
          >
            toggle slice
          </button>
        )}
      </div>
    );
  },
}));

// Stubs the real panel out too — this file only asserts on WHERE/WHEN it
// renders (full-width sibling, singular across the grid), not on its own
// data-fetching/rendering, which `WidgetInlineDrilldownPanel.test.tsx`
// covers directly.
vi.mock("@features/csm-dashboard/components/WidgetInlineDrilldownPanel", () => ({
  default: ({
    widgetId,
    slice,
    onClose,
  }: {
    widgetId: string;
    slice: PieSliceResult;
    onClose: () => void;
  }) => (
    <div data-testid={`panel-${widgetId}`}>
      <span>{slice.label}</span>
      <button type="button" aria-label={`Close panel for ${widgetId}`} onClick={onClose}>
        close
      </button>
    </div>
  ),
}));

import DashboardWidgetGrid from "@features/csm-dashboard/components/DashboardWidgetGrid";

function renderGrid(
  widgets: BeDashboardWidget[],
  renderWidgetAction?: (widget: BeDashboardWidget) => ReactNode,
) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>
        <DashboardWidgetGrid widgets={widgets} renderWidgetAction={renderWidgetAction} />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

function makeWidget(overrides: Partial<BeDashboardWidget> = {}): BeDashboardWidget {
  return {
    widgetId: "my_patches",
    displayName: "My Patches",
    resourceType: "case",
    shape: "count",
    gridWidth: 3,
    query: {},
    ...overrides,
  } as BeDashboardWidget;
}

describe("DashboardWidgetGrid", () => {
  beforeEach(() => {
    capturedTileProps = {};
  });

  it("renders every tile's own refresh button as before when no renderWidgetAction is passed (live dashboard, unaffected)", () => {
    renderGrid([makeWidget()]);

    expect(screen.getByRole("button", { name: "Refresh my_patches" })).toBeInTheDocument();
  });

  it("suppresses a widget's own refresh button (and renders the builder action instead) when renderWidgetAction returns a non-null action for it", () => {
    renderGrid([makeWidget()], (widget) => (
      <div>
        <button type="button" aria-label={`Edit ${widget.widgetId}`}>
          edit
        </button>
        <button type="button" aria-label={`Remove ${widget.widgetId}`}>
          remove
        </button>
      </div>
    ));

    // The builder's own Edit/Remove actions render...
    expect(screen.getByRole("button", { name: "Edit my_patches" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Remove my_patches" })).toBeInTheDocument();
    // ...and the tile's own refresh button is suppressed, so there's no
    // longer any overlap for the two to fight over in the same corner.
    expect(
      screen.queryByRole("button", { name: "Refresh my_patches" }),
    ).not.toBeInTheDocument();
  });

  it("keeps a widget's own refresh button when renderWidgetAction returns nothing for that specific widget", () => {
    renderGrid(
      [makeWidget({ widgetId: "no_action_widget" })],
      () => null,
    );

    expect(
      screen.getByRole("button", { name: "Refresh no_action_widget" }),
    ).toBeInTheDocument();
  });

  it("renders a section's widgets in the config's own array order, not grouped/reordered by shape", () => {
    // Deliberately lists a bar-shape widget ahead of a list-shape widget
    // within the same (default/untitled) section — the old implementation
    // hardcoded every list/count tile ahead of every pie/bar tile
    // regardless of array position, so reordering this config array had no
    // visible effect. This asserts the fix: DOM order follows array order.
    renderGrid([
      makeWidget({ widgetId: "trend_widget", shape: "bar", groupBy: { field: "status" } }),
      makeWidget({ widgetId: "list_widget", shape: "list" }),
      makeWidget({ widgetId: "count_widget", shape: "count" }),
    ]);

    const tileIds = screen
      .getAllByTestId(/^tile-/)
      .map((el) => el.getAttribute("data-testid"));

    expect(tileIds).toEqual(["tile-trend_widget", "tile-list_widget", "tile-count_widget"]);
  });

  it("expands a widget's slice into a full-width panel rendered as a sibling of that widget's own tile, not nested inside it", () => {
    renderGrid([makeWidget({ widgetId: "cases_by_severity", shape: "pie" })]);

    expect(screen.queryByTestId("panel-cases_by_severity")).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Expand a slice on cases_by_severity" }));

    const panel = screen.getByTestId("panel-cases_by_severity");
    expect(panel).toBeInTheDocument();
    expect(screen.getByText("cases_by_severity-slice")).toBeInTheDocument();
    // A sibling of the tile's own wrapper, not a descendant of it.
    const tile = screen.getByTestId("tile-cases_by_severity");
    expect(tile.contains(panel)).toBe(false);
  });

  it("collapses the panel when the tile reports null (e.g. clicking the same slice again, or the panel's own close control)", () => {
    renderGrid([makeWidget({ widgetId: "cases_by_severity", shape: "pie" })]);

    const toggle = screen.getByRole("button", { name: "Expand a slice on cases_by_severity" });
    fireEvent.click(toggle);
    expect(screen.getByTestId("panel-cases_by_severity")).toBeInTheDocument();

    fireEvent.click(toggle);
    expect(screen.queryByTestId("panel-cases_by_severity")).not.toBeInTheDocument();
  });

  it("closing the panel via its own onClose collapses it", () => {
    renderGrid([makeWidget({ widgetId: "cases_by_severity", shape: "pie" })]);

    fireEvent.click(screen.getByRole("button", { name: "Expand a slice on cases_by_severity" }));
    expect(screen.getByTestId("panel-cases_by_severity")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Close panel for cases_by_severity" }));
    expect(screen.queryByTestId("panel-cases_by_severity")).not.toBeInTheDocument();
  });

  it("expanding a different widget's slice replaces the previously expanded panel — only one panel is ever open at a time", () => {
    renderGrid([
      makeWidget({ widgetId: "widget_a", shape: "pie" }),
      makeWidget({ widgetId: "widget_b", shape: "pie" }),
    ]);

    fireEvent.click(screen.getByRole("button", { name: "Expand a slice on widget_a" }));
    expect(screen.getByTestId("panel-widget_a")).toBeInTheDocument();
    expect(screen.queryByTestId("panel-widget_b")).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Expand a slice on widget_b" }));
    expect(screen.queryByTestId("panel-widget_a")).not.toBeInTheDocument();
    expect(screen.getByTestId("panel-widget_b")).toBeInTheDocument();
  });

  it("renders the expanded panel once, AFTER every tile in its own section — not as a sibling inserted right after the clicked widget — so unrelated tiles never get pushed onto a new grid row", () => {
    // Three widgets in the SAME (default/untitled) section; the one that
    // gets expanded ("widget_b") sits in the MIDDLE of that section's own
    // array, which is exactly the layout that broke before this fix: a
    // full-width panel inserted as `widget_b`'s own next DOM sibling forced
    // `widget_c` (queued right after it in the same CSS grid row) onto a
    // new row, even though nothing about `widget_c` changed.
    renderGrid([
      makeWidget({ widgetId: "widget_a", shape: "pie" }),
      makeWidget({ widgetId: "widget_b", shape: "pie" }),
      makeWidget({ widgetId: "widget_c", shape: "pie" }),
    ]);

    fireEvent.click(screen.getByRole("button", { name: "Expand a slice on widget_b" }));

    const domOrder = screen
      .getAllByTestId(/^(tile|panel)-/)
      .map((el) => el.getAttribute("data-testid"));
    // Every tile — including widget_c, which comes AFTER the expanded
    // widget_b in array order — still renders before the single panel,
    // which lands at the very end of the section rather than wedged
    // between widget_b and widget_c.
    expect(domOrder).toEqual([
      "tile-widget_a",
      "tile-widget_b",
      "tile-widget_c",
      "panel-widget_b",
    ]);
  });

  it("does not give an unrelated widget's tile a new filters/onExpandChange prop identity when a different widget's slice is expanded — the mechanism that lets DashboardWidgetTile's own React.memo skip re-rendering it", () => {
    renderGrid([
      makeWidget({ widgetId: "widget_a", shape: "pie" }),
      makeWidget({ widgetId: "widget_b", shape: "pie" }),
    ]);

    const beforeFilters = capturedTileProps.widget_b.filters;
    const beforeOnExpandChange = capturedTileProps.widget_b.onExpandChange;
    const beforeRenderCount = capturedTileProps.widget_b.renderCount;

    fireEvent.click(screen.getByRole("button", { name: "Expand a slice on widget_a" }));

    // widget_b's own mock DID re-render (this test file's mock isn't
    // wrapped in React.memo, so it can't itself demonstrate the bail-out) —
    // but its `filters`/`onExpandChange` props kept the EXACT SAME object/
    // function identity across that re-render, which is what actually lets
    // the real, memoized `DashboardWidgetTile` skip doing so.
    expect(capturedTileProps.widget_b.renderCount).toBeGreaterThan(beforeRenderCount);
    expect(capturedTileProps.widget_b.filters).toBe(beforeFilters);
    expect(capturedTileProps.widget_b.onExpandChange).toBe(beforeOnExpandChange);
  });

  it("still recomputes a widget's own resolvedFilters identity when its own query actually changes across a re-render (the cache doesn't over-cache)", () => {
    const { rerender } = render(
      <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
        <MemoryRouter>
          <DashboardWidgetGrid
            widgets={[makeWidget({ widgetId: "widget_a", shape: "pie", query: { status: "open" } })]}
          />
        </MemoryRouter>
      </QueryClientProvider>,
    );
    const beforeFilters = capturedTileProps.widget_a.filters;

    rerender(
      <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
        <MemoryRouter>
          <DashboardWidgetGrid
            widgets={[makeWidget({ widgetId: "widget_a", shape: "pie", query: { status: "closed" } })]}
          />
        </MemoryRouter>
      </QueryClientProvider>,
    );

    expect(capturedTileProps.widget_a.filters).not.toBe(beforeFilters);
    expect(capturedTileProps.widget_a.filters).toEqual({ status: "closed" });
  });
});
