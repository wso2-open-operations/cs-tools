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

import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import { MemoryRouter, Route, Routes, useLocation } from "react-router";

const getMock = vi.fn();

vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ get: getMock }),
}));

import CsmDashboardBuilderListPage from "@features/csm-admin/dashboards/pages/CsmDashboardBuilderListPage";
import { saveDashboardDraft } from "@features/csm-admin/dashboards/utils/dashboardDraftsStorage";

function LocationProbe() {
  const location = useLocation();
  return <div data-testid="location-probe">{location.pathname}</div>;
}

function renderPage() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={["/admin/dashboards"]}>
        <Routes>
          <Route path="/admin/dashboards" element={<CsmDashboardBuilderListPage />} />
          <Route path="/admin/dashboards/:draftId" element={<LocationProbe />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("CsmDashboardBuilderListPage", () => {
  beforeEach(() => {
    getMock.mockReset();
    localStorage.clear();
  });

  it("lists every deployed dashboard and links each to its own editor route", async () => {
    getMock.mockResolvedValue([
      { id: "agents_pilot", displayName: "Engineer overview", isDefault: true, isTeamBased: false },
      { id: "team_perf", displayName: "Team performance", isDefault: false, isTeamBased: true },
    ]);

    renderPage();

    await waitFor(() => expect(screen.getByText("Engineer overview")).toBeInTheDocument());
    expect(screen.getByText("Team performance")).toBeInTheDocument();

    fireEvent.click(screen.getByText("Engineer overview"));

    await waitFor(() =>
      expect(screen.getByTestId("location-probe")).toHaveTextContent("/admin/dashboards/agents_pilot"),
    );
  });

  it("shows an empty state instead of a blank list when no dashboards are registered", async () => {
    getMock.mockResolvedValue([]);
    renderPage();
    expect(await screen.findByText(/no dashboards are registered/i)).toBeInTheDocument();
  });

  it("navigates to a freshly generated draft id when 'Create new dashboard' is clicked", async () => {
    getMock.mockResolvedValue([]);
    renderPage();
    await waitFor(() => expect(screen.getByText(/no dashboards are registered/i)).toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: /create new dashboard/i }));

    await waitFor(() => expect(screen.getByTestId("location-probe")).toBeInTheDocument());
    expect(screen.getByTestId("location-probe").textContent).toMatch(/^\/admin\/dashboards\/draft-/);
  });

  it("lists a local draft that has no matching deployed dashboard, and lets it be discarded", async () => {
    getMock.mockResolvedValue([]);
    saveDashboardDraft({
      id: "draft-orphan-1",
      displayName: "My new dashboard",
      isDefault: false,
      isTeamBased: false,
      widgets: [],
      emptySections: [],
    });

    renderPage();

    expect(await screen.findByText("My new dashboard")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /discard draft/i }));

    await waitFor(() => expect(screen.queryByText("My new dashboard")).not.toBeInTheDocument());
  });

  it("flags a deployed dashboard that has a local draft with unsaved-to-deployment changes", async () => {
    // The list response (`GET /dashboards`) and the drift chip's own detail
    // fetch (`GET /dashboards/agents_pilot`) share the same mocked `get` —
    // differentiate by path so the drift chip sees a REAL, materially
    // different live dashboard, not the list-shaped payload.
    getMock.mockImplementation((path: string) => {
      if (path === "/dashboards") {
        return Promise.resolve([
          { id: "agents_pilot", displayName: "Engineer overview", isDefault: true, isTeamBased: false },
        ]);
      }
      if (path === "/dashboards/agents_pilot") {
        return Promise.resolve({
          id: "agents_pilot",
          displayName: "Engineer overview",
          isDefault: true,
          isTeamBased: false,
          widgets: [],
        });
      }
      return Promise.resolve(null);
    });
    saveDashboardDraft({
      id: "agents_pilot",
      sourceDashboardId: "agents_pilot",
      displayName: "Engineer overview (renamed locally)",
      isDefault: true,
      isTeamBased: false,
      widgets: [],
      emptySections: [],
    });

    renderPage();

    expect(await screen.findByText("Engineer overview")).toBeInTheDocument();
    expect(await screen.findByText("Local draft")).toBeInTheDocument();
  });

  it("shows a 'clear local edits' action only for a deployed dashboard that has a local draft", async () => {
    getMock.mockImplementation((path: string) => {
      if (path === "/dashboards") {
        return Promise.resolve([
          { id: "agents_pilot", displayName: "Engineer overview", isDefault: true, isTeamBased: false },
          { id: "team_perf", displayName: "Team performance", isDefault: false, isTeamBased: true },
        ]);
      }
      if (path === "/dashboards/agents_pilot") {
        return Promise.resolve({
          id: "agents_pilot",
          displayName: "Engineer overview",
          isDefault: true,
          isTeamBased: false,
          widgets: [],
        });
      }
      return Promise.resolve(null);
    });
    saveDashboardDraft({
      id: "agents_pilot",
      sourceDashboardId: "agents_pilot",
      displayName: "Engineer overview (renamed locally)",
      isDefault: true,
      isTeamBased: false,
      widgets: [],
      emptySections: [],
    });

    renderPage();

    expect(await screen.findByText("Engineer overview")).toBeInTheDocument();
    expect(screen.getByText("Team performance")).toBeInTheDocument();

    expect(
      screen.getByRole("button", { name: /clear local edits for engineer overview/i }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /clear local edits for team performance/i }),
    ).not.toBeInTheDocument();
  });

  it("clears local edits for the clicked dashboard only, after confirming, and leaves other dashboards untouched", async () => {
    const confirmSpy = vi.spyOn(window, "confirm").mockReturnValue(true);
    getMock.mockImplementation((path: string) => {
      if (path === "/dashboards") {
        return Promise.resolve([
          { id: "agents_pilot", displayName: "Engineer overview", isDefault: true, isTeamBased: false },
          { id: "team_perf", displayName: "Team performance", isDefault: false, isTeamBased: true },
        ]);
      }
      if (path === "/dashboards/agents_pilot") {
        return Promise.resolve({
          id: "agents_pilot",
          displayName: "Engineer overview",
          isDefault: true,
          isTeamBased: false,
          widgets: [],
        });
      }
      if (path === "/dashboards/team_perf") {
        return Promise.resolve({
          id: "team_perf",
          displayName: "Team performance",
          isDefault: false,
          isTeamBased: true,
          widgets: [],
        });
      }
      return Promise.resolve(null);
    });
    saveDashboardDraft({
      id: "agents_pilot",
      sourceDashboardId: "agents_pilot",
      displayName: "Engineer overview (renamed locally)",
      isDefault: true,
      isTeamBased: false,
      widgets: [],
      emptySections: [],
    });
    saveDashboardDraft({
      id: "team_perf",
      sourceDashboardId: "team_perf",
      displayName: "Team performance (renamed locally)",
      isDefault: false,
      isTeamBased: true,
      widgets: [],
      emptySections: [],
    });

    renderPage();

    const clearButton = await screen.findByRole("button", {
      name: /clear local edits for engineer overview/i,
    });
    fireEvent.click(clearButton);

    expect(confirmSpy).toHaveBeenCalledTimes(1);
    await waitFor(() =>
      expect(
        screen.queryByRole("button", { name: /clear local edits for engineer overview/i }),
      ).not.toBeInTheDocument(),
    );
    // The other dashboard's draft, and its own clear action, must be untouched.
    expect(
      screen.getByRole("button", { name: /clear local edits for team performance/i }),
    ).toBeInTheDocument();

    confirmSpy.mockRestore();
  });

  it("does not clear local edits when the confirmation is declined", async () => {
    const confirmSpy = vi.spyOn(window, "confirm").mockReturnValue(false);
    getMock.mockImplementation((path: string) => {
      if (path === "/dashboards") {
        return Promise.resolve([
          { id: "agents_pilot", displayName: "Engineer overview", isDefault: true, isTeamBased: false },
        ]);
      }
      if (path === "/dashboards/agents_pilot") {
        return Promise.resolve({
          id: "agents_pilot",
          displayName: "Engineer overview",
          isDefault: true,
          isTeamBased: false,
          widgets: [],
        });
      }
      return Promise.resolve(null);
    });
    saveDashboardDraft({
      id: "agents_pilot",
      sourceDashboardId: "agents_pilot",
      displayName: "Engineer overview (renamed locally)",
      isDefault: true,
      isTeamBased: false,
      widgets: [],
      emptySections: [],
    });

    renderPage();

    const clearButton = await screen.findByRole("button", {
      name: /clear local edits for engineer overview/i,
    });
    fireEvent.click(clearButton);

    expect(confirmSpy).toHaveBeenCalledTimes(1);
    expect(
      screen.getByRole("button", { name: /clear local edits for engineer overview/i }),
    ).toBeInTheDocument();

    confirmSpy.mockRestore();
  });

  it("does NOT flag a deployed dashboard whose local draft is byte-identical to what's deployed", async () => {
    getMock.mockImplementation((path: string) => {
      if (path === "/dashboards") {
        return Promise.resolve([
          { id: "agents_pilot", displayName: "Engineer overview", isDefault: true, isTeamBased: false },
        ]);
      }
      if (path === "/dashboards/agents_pilot") {
        return Promise.resolve({
          id: "agents_pilot",
          displayName: "Engineer overview",
          isDefault: true,
          isTeamBased: false,
          widgets: [],
        });
      }
      return Promise.resolve(null);
    });
    // Same content as the live dashboard above — merely having a local
    // draft record must not, by itself, imply divergence.
    saveDashboardDraft({
      id: "agents_pilot",
      sourceDashboardId: "agents_pilot",
      displayName: "Engineer overview",
      isDefault: true,
      isTeamBased: false,
      widgets: [],
      emptySections: [],
    });

    renderPage();

    expect(await screen.findByText("Engineer overview")).toBeInTheDocument();
    await waitFor(() => expect(getMock).toHaveBeenCalledWith("/dashboards/agents_pilot"));
    expect(screen.queryByText("Local draft")).not.toBeInTheDocument();
  });
});
