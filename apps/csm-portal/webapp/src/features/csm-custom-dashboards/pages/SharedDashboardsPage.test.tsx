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
import { MemoryRouter, Route, Routes } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

interface Summary {
  id: string;
  name: string;
  owner: { id: string | null; email: string; name: string } | null;
  sharedViaTeam: { id: string; name: string; type: string } | null;
  sharedOn: string;
  updatedOn: string;
}
let listState: { data?: Summary[]; isLoading?: boolean; isFetching?: boolean; isError?: boolean };
let detailState: { data?: unknown; isLoading?: boolean; isError?: boolean; error?: Error };
const detailIds: (string | undefined)[] = [];

vi.mock("@features/csm-custom-dashboards/api/useDashboardShares", () => ({
  useGetSharedDashboards: () => listState,
  useGetSharedDashboard: (id: string | undefined) => {
    detailIds.push(id);
    return detailState;
  },
}));
// The widgets fetch their own data; the grid is not under test here.
vi.mock("@features/csm-dashboard/components/DashboardWidgetGrid", () => ({
  default: ({ widgets }: { widgets: { widgetId: string }[] }) => <div>Grid: {widgets.map((w) => w.widgetId).join(", ")}</div>,
}));

import SharedDashboardsPage from "@features/csm-custom-dashboards/pages/SharedDashboardsPage";

const jane = { id: "jane", email: "jane.doe@example.com", name: "Jane Doe" };
const ben = { id: "ben", email: "ben@example.com", name: "Ben Bee" };
const A: Summary = { id: "5e636a12-0000-4000-8000-00000000000a", name: "Alpha", owner: jane, sharedViaTeam: null, sharedOn: "x", updatedOn: "y" };
const B: Summary = { id: "5e636a12-0000-4000-8000-00000000000b", name: "Beta", owner: ben, sharedViaTeam: null, sharedOn: "x", updatedOn: "y" };
const rigel = { id: "391737f4-0588-2f9f-9bdd-b18d39f83d9a", name: "Rigel", type: "cre-abt" };

function sharedWith(widgetId: string) {
  return {
    dashboard: { id: "any", displayName: "any", isDefault: false, isTeamBased: false, widgets: [{ widgetId }] },
    owner: jane,
    sharedOn: "x",
    updatedOn: "y",
  };
}

function renderAt(path: string) {
  render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/dashboard/shared" element={<SharedDashboardsPage />} />
        <Route path="/dashboard/shared/:dashboardId" element={<SharedDashboardsPage />} />
      </Routes>
    </MemoryRouter>,
  );
}

describe("SharedDashboardsPage", () => {
  beforeEach(() => {
    listState = { data: [A, B] };
    detailState = { data: sharedWith("w1") };
    detailIds.length = 0;
  });

  it("shows the empty state when nothing is shared", () => {
    listState = { data: [] };
    renderAt("/dashboard/shared");
    expect(screen.getByText("No shared dashboards yet")).toBeInTheDocument();
    expect(screen.getByText(/You can view them, not change them/)).toBeInTheDocument();
  });

  it("opens the first shared dashboard when the URL names none", () => {
    renderAt("/dashboard/shared");
    expect(screen.getByText(/Alpha · Shared by Jane Doe/)).toBeInTheDocument();
    expect(detailIds).toContain(A.id);
  });

  it("draws the one the URL names, view only, with who shared it", () => {
    detailState = { data: sharedWith("open-cases") };
    renderAt(`/dashboard/shared/${B.id}`);
    expect(screen.getByText(/Beta · Shared by Ben Bee/)).toBeInTheDocument();
    expect(screen.getByText("View only")).toBeInTheDocument();
    expect(screen.getByText("Grid: open-cases")).toBeInTheDocument();
  });

  it("offers no edit, share or delete", () => {
    renderAt(`/dashboard/shared/${A.id}`);
    for (const name of ["Edit with AI", "Share", "Generate Dashboard"]) {
      expect(screen.queryByRole("button", { name })).not.toBeInTheDocument();
    }
    fireEvent.mouseDown(screen.getByRole("combobox", { name: "Select dashboard" }));
    expect(screen.queryByRole("button", { name: /Delete dashboard/ })).not.toBeInTheDocument();
  });

  it("lists each entry with who shared it, and searches by owner too", () => {
    renderAt(`/dashboard/shared/${A.id}`);
    fireEvent.mouseDown(screen.getByRole("combobox", { name: "Select dashboard" }));
    expect(screen.getByText("Shared by Ben Bee")).toBeInTheDocument();

    fireEvent.change(screen.getByRole("textbox", { name: "Search dashboards" }), { target: { value: "ben" } });
    const box = screen.getByRole("textbox", { name: "Search dashboards" });
    const visible = screen
      .getAllByRole("option")
      .filter((o) => !o.contains(box) && o.getAttribute("aria-disabled") !== "true")
      .map((o) => o.textContent);
    expect(visible).toEqual(["BetaShared by Ben Bee"]);
  });

  it("names the team a dashboard reached the user through", () => {
    listState = { data: [{ ...A, sharedViaTeam: rigel }, B] };
    renderAt(`/dashboard/shared/${A.id}`);
    expect(screen.getByText(/Alpha · Shared with your team Rigel by Jane Doe/)).toBeInTheDocument();

    fireEvent.mouseDown(screen.getByRole("combobox", { name: "Select dashboard" }));
    expect(screen.getAllByText("Shared with your team Rigel by Jane Doe").length).toBeGreaterThan(0);
    expect(screen.getByText("Shared by Ben Bee")).toBeInTheDocument();

    // The team is searched too.
    fireEvent.change(screen.getByRole("textbox", { name: "Search dashboards" }), { target: { value: "rigel" } });
    const box = screen.getByRole("textbox", { name: "Search dashboards" });
    const visible = screen
      .getAllByRole("option")
      .filter((o) => !o.contains(box) && o.getAttribute("aria-disabled") !== "true")
      .map((o) => o.textContent);
    expect(visible).toEqual(["AlphaShared with your team Rigel by Jane Doe"]);
  });

  it("names a former user when the owner's user has been deleted", () => {
    listState = { data: [{ ...A, owner: null }, { ...B, owner: null, sharedViaTeam: rigel }] };
    renderAt(`/dashboard/shared/${A.id}`);
    expect(screen.getByText(/Alpha · Shared by a former user/)).toBeInTheDocument();

    fireEvent.mouseDown(screen.getByRole("combobox", { name: "Select dashboard" }));
    expect(screen.getByText("Shared with your team Rigel by a former user")).toBeInTheDocument();
  });

  it("opens a dashboard picked in the switcher", () => {
    renderAt(`/dashboard/shared/${A.id}`);
    fireEvent.mouseDown(screen.getByRole("combobox", { name: "Select dashboard" }));
    fireEvent.click(screen.getByRole("option", { name: /Beta/ }));
    expect(screen.getByText(/Beta · Shared by Ben Bee/)).toBeInTheDocument();
  });

  it("waits for the list to be re-read before giving up on an unknown id", () => {
    listState = { data: [A], isFetching: true };
    renderAt(`/dashboard/shared/${B.id}`);
    expect(detailIds).not.toContain(A.id);
  });

  it("opens the first one for an id that is not shared with the user", () => {
    listState = { data: [A] };
    renderAt(`/dashboard/shared/${B.id}`);
    expect(screen.getByText(/Alpha · Shared by Jane Doe/)).toBeInTheDocument();
  });

  it("shows the backend's message when the dashboard can't be shown", () => {
    detailState = { isError: true, error: new Error("This shared dashboard can't be shown any more. Ask its owner to update it.") };
    renderAt(`/dashboard/shared/${A.id}`);
    expect(screen.getByText("This shared dashboard can't be shown any more. Ask its owner to update it.")).toBeInTheDocument();
  });

  it("says so when it is no longer shared", () => {
    detailState = { data: null };
    renderAt(`/dashboard/shared/${A.id}`);
    expect(screen.getByText("This dashboard is no longer shared with you.")).toBeInTheDocument();
  });

  it("says so when the list can't be loaded", () => {
    listState = { isError: true };
    renderAt("/dashboard/shared");
    expect(screen.getByText("Could not load the dashboards shared with you.")).toBeInTheDocument();
  });
});
