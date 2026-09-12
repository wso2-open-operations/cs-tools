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

import { render, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import { MemoryRouter } from "react-router";
import type { ReactNode } from "react";
import type { BeDashboardWidget } from "@api/backend/types";

// Stubs the real tile out entirely — this test is only about
// `DashboardWidgetGrid`'s own wiring of `hideRefreshButton` alongside
// `renderWidgetAction`, not about anything the real tile fetches/renders.
// Exposes both as plain text so the assertions below can read them straight
// out of the DOM rather than needing a spy.
vi.mock("@features/csm-dashboard/components/DashboardWidgetTile", () => ({
  default: ({
    widgetId,
    hideRefreshButton,
  }: {
    widgetId: string;
    hideRefreshButton?: boolean;
  }) => (
    <div data-testid={`tile-${widgetId}`}>
      {!hideRefreshButton && (
        <button type="button" aria-label={`Refresh ${widgetId}`}>
          refresh
        </button>
      )}
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
});
