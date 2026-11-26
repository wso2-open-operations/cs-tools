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

import { fireEvent, render, screen, within } from "@testing-library/react";
import type { JSX } from "react";
import { MemoryRouter, Route, Routes, useLocation } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

interface Summary {
  id: string;
  name: string;
  owner: { id: string | null; email: string; name: string } | null;
  sharedViaTeam: { id: string; name: string; type: string } | null;
  sharedViaLink: boolean;
  sharedOn: string;
  updatedOn: string;
}
type ListState = { data?: Summary[]; isLoading?: boolean; isFetching?: boolean; isError?: boolean };
let canView = true;
let listState: ListState;
let codeListState: ListState;
let codeListCalls = 0;
let codeDetailState: { data?: unknown; isLoading?: boolean; isError?: boolean; error?: Error };
const codeDetailIds: (string | undefined)[] = [];

vi.mock("@features/csm-custom-dashboards/utils/advancedDashboardAccess", () => ({
  useCanViewAdvancedDashboards: () => canView,
}));
vi.mock("@features/csm-custom-dashboards/api/useDashboardShares", () => ({
  useGetSharedDashboards: () => listState,
  useGetSharedDashboard: () => ({ data: { dashboard: { widgets: [{ widgetId: "w1" }] } } }),
}));
vi.mock("@features/csm-custom-dashboards/api/useCodeDashboardShares", () => ({
  sharedCodeDashboardPath: (id: string) => `/dashboard/shared/code/${encodeURIComponent(id)}`,
  useGetSharedCodeDashboards: () => {
    codeListCalls++;
    return codeListState;
  },
  useGetSharedCodeDashboard: (id: string | undefined) => {
    codeDetailIds.push(id);
    return codeDetailState;
  },
}));
vi.mock("@features/csm-dashboard/components/DashboardWidgetGrid", () => ({
  default: ({ widgets }: { widgets: { widgetId: string }[] }) => <div>Grid: {widgets.map((w) => w.widgetId).join(", ")}</div>,
}));
// The frame runs the code against the portal; it is tested on its own.
vi.mock("@features/csm-custom-dashboards/components/CodeDashboardFrame", () => ({
  default: ({ source }: { source: string }) => <div>frame running {source}</div>,
}));

import SharedDashboardsPage from "@features/csm-custom-dashboards/pages/SharedDashboardsPage";

const jane = { id: "jane", email: "jane.doe@example.com", name: "Jane Doe" };
const ben = { id: "ben", email: "ben@example.com", name: "Ben Bee" };
function summary(id: string, name: string, owner = jane, sharedViaLink = false): Summary {
  return { id, name, owner, sharedViaTeam: null, sharedViaLink, sharedOn: "x", updatedOn: "y" };
}
const A = summary("5e636a12-0000-4000-8000-00000000000a", "Alpha");
const C = summary("5e636a12-0000-4000-8000-0000000000c1", "Cases Code", ben);
const D = summary("5e636a12-0000-4000-8000-0000000000d1", "Delta Code", jane, true);
const SOURCE = "async function render(portal, root) { root.textContent = 'hi'; }";

function Where(): JSX.Element {
  return <div data-testid="where">{useLocation().pathname}</div>;
}

function renderAt(path: string) {
  render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/dashboard/shared" element={<SharedDashboardsPage />} />
        <Route path="/dashboard/shared/:dashboardId" element={<SharedDashboardsPage />} />
        <Route path="/dashboard/shared/code/:codeDashboardId" element={<SharedDashboardsPage />} />
      </Routes>
      <Where />
    </MemoryRouter>,
  );
}

function where(): string {
  return screen.getByTestId("where").textContent ?? "";
}

/** The switcher's menu with nothing searched, as "# heading" and name lines in order. */
function menuLines(): string[] {
  fireEvent.mouseDown(screen.getByRole("combobox", { name: "Select dashboard" }));
  const listbox = screen.getByRole("listbox");
  return Array.from(listbox.children)
    .filter((el) => !el.querySelector("input"))
    .map((el) =>
      el.classList.contains("MuiListSubheader-root") ? `# ${el.textContent}` : (el.querySelector("span")?.textContent ?? ""),
    );
}

describe("SharedDashboardsPage with advanced dashboards", () => {
  beforeEach(() => {
    canView = true;
    listState = { data: [A] };
    codeListState = { data: [C, D] };
    codeListCalls = 0;
    codeDetailState = { data: { id: C.id, name: C.name, source: SOURCE } };
    codeDetailIds.length = 0;
  });

  it("lists the advanced ones under Advanced, after the others", () => {
    renderAt(`/dashboard/shared/${A.id}`);
    expect(menuLines()).toEqual(["Alpha", "# Advanced", "Cases Code", "Delta Code"]);
  });

  it("runs the advanced one the URL names, view only, with who shared it", () => {
    renderAt(`/dashboard/shared/code/${C.id}`);
    expect(codeDetailIds).toContain(C.id);
    expect(screen.getByText(`Cases Code · Shared by Ben Bee`)).toBeInTheDocument();
    expect(screen.getByText("View only")).toBeInTheDocument();
    const panel = screen.getByTestId("shared-code-dashboard-panel");
    expect(panel).toHaveStyle({ height: "calc(100dvh - 240px)", minHeight: "520px" });
    expect(within(panel).getByText(`frame running ${SOURCE}`)).toBeInTheDocument();
    expect(screen.queryByText(/^Grid:/)).not.toBeInTheDocument();
  });

  it("says when an advanced one reached the user only through its link", () => {
    codeDetailState = { data: { id: D.id, name: D.name, source: SOURCE } };
    renderAt(`/dashboard/shared/code/${D.id}`);
    expect(screen.getByText("Delta Code · Opened from a link shared by Jane Doe")).toBeInTheDocument();
  });

  it("opens an advanced one picked in the switcher, and a custom one again", () => {
    renderAt(`/dashboard/shared/${A.id}`);
    fireEvent.mouseDown(screen.getByRole("combobox", { name: "Select dashboard" }));
    fireEvent.click(screen.getByRole("option", { name: /Delta Code/ }));
    expect(where()).toBe(`/dashboard/shared/code/${D.id}`);
    fireEvent.mouseDown(screen.getByRole("combobox", { name: "Select dashboard" }));
    fireEvent.click(screen.getByRole("option", { name: /Alpha/ }));
    expect(where()).toBe(`/dashboard/shared/${A.id}`);
  });

  it("opens the first custom one when the URL names none", () => {
    renderAt("/dashboard/shared");
    expect(where()).toBe(`/dashboard/shared/${A.id}`);
  });

  it("opens the first advanced one when only advanced ones are shared", () => {
    listState = { data: [] };
    renderAt("/dashboard/shared");
    expect(where()).toBe(`/dashboard/shared/code/${C.id}`);
  });

  it("opens the first one for an advanced id that is not shared with the user", () => {
    renderAt("/dashboard/shared/code/5e636a12-0000-4000-8000-0000000000ff");
    expect(where()).toBe(`/dashboard/shared/${A.id}`);
  });

  it("waits for the advanced list while it is read, for an advanced id", () => {
    codeListState = { data: [], isFetching: true };
    renderAt(`/dashboard/shared/code/${C.id}`);
    expect(where()).toBe(`/dashboard/shared/code/${C.id}`);
    expect(screen.queryByTestId("shared-code-dashboard-panel")).not.toBeInTheDocument();
  });

  it("waits while the advanced list first loads", () => {
    codeListState = { isLoading: true };
    renderAt("/dashboard/shared");
    expect(where()).toBe("/dashboard/shared");
    expect(screen.queryByText("No shared dashboards yet")).not.toBeInTheDocument();
  });

  it("keeps the custom ones when only the advanced list fails, and says so", () => {
    codeListState = { isError: true };
    renderAt(`/dashboard/shared/${A.id}`);
    expect(screen.getByText("Grid: w1")).toBeInTheDocument();
    expect(screen.getByText("Could not load the advanced dashboards shared with you.")).toBeInTheDocument();
  });

  it("says so under the empty state when nothing loaded and the advanced list failed", () => {
    listState = { data: [] };
    codeListState = { isError: true };
    renderAt("/dashboard/shared");
    expect(screen.getByText("No shared dashboards yet")).toBeInTheDocument();
    expect(screen.getByText("Could not load the advanced dashboards shared with you.")).toBeInTheDocument();
  });

  it("shows the empty state only when neither kind is shared", () => {
    listState = { data: [] };
    codeListState = { data: [] };
    renderAt("/dashboard/shared");
    expect(screen.getByText("No shared dashboards yet")).toBeInTheDocument();
    expect(screen.queryByText(/Could not load/)).not.toBeInTheDocument();
  });

  it("says when an advanced one is no longer shared, or why it can't be shown", () => {
    codeDetailState = { data: null };
    renderAt(`/dashboard/shared/code/${C.id}`);
    expect(screen.getByText("This dashboard is no longer shared with you.")).toBeInTheDocument();
  });

  it("shows the backend's message when an advanced one can't be shown", () => {
    codeDetailState = { isError: true, error: new Error("The dashboard couldn't be loaded.") };
    renderAt(`/dashboard/shared/code/${C.id}`);
    expect(screen.getByText("The dashboard couldn't be loaded.")).toBeInTheDocument();
  });

  it("reads nothing advanced, and lists only custom ones, for a user who cannot view them", () => {
    canView = false;
    renderAt(`/dashboard/shared/code/${C.id}`);
    expect(codeListCalls).toBe(0);
    expect(codeDetailIds).toEqual([]);
    expect(where()).toBe(`/dashboard/shared/${A.id}`);
    expect(menuLines()).toEqual(["Alpha"]);
  });
});
