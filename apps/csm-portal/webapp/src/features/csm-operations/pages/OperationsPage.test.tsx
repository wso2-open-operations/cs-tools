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

/**
 * Exercises OperationsPage's tab strip against the `/operations` and
 * `/operations/:tab` routes exactly as declared in App.tsx, plus the legacy
 * `?tab=` redirect — without pulling in App.tsx's full provider/lazy-loading
 * tree. Each tab's own content component is mocked to a marker div, since
 * this page's job is picking which one renders, not what they render.
 */

import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { MemoryRouter, Route, Routes, useLocation } from "react-router";
import "@testing-library/jest-dom/vitest";
import { resetFeatureStatesForTests } from "@config/featureFlags";

function setOverrides(value: unknown): void {
  window.config = {
    ...window.config,
    CSM_PORTAL_FEATURE_OVERRIDES: value,
  } as Window["config"];
  resetFeatureStatesForTests();
}

vi.mock("@features/csm-cases/components/CsmIssuesView", () => ({
  default: () => <div>Service requests content</div>,
}));
vi.mock("@features/csm-operations/components/ChangeRequestsTab", () => ({
  default: () => <div>Change requests content</div>,
}));
vi.mock("@features/csm-operations/components/IncidentsTab", () => ({
  default: () => <div>Incidents content</div>,
}));
vi.mock("@features/csm-operations/components/ProblemsTab", () => ({
  default: () => <div>Problems content</div>,
}));

import OperationsPage from "@features/csm-operations/pages/OperationsPage";

function LocationProbe() {
  const location = useLocation();
  return <div data-testid="location-probe">{location.pathname}</div>;
}

function renderAt(initialEntry: string) {
  return render(
    <MemoryRouter initialEntries={[initialEntry]}>
      <LocationProbe />
      <Routes>
        <Route path="/operations" element={<OperationsPage />} />
        <Route path="/operations/:tab" element={<OperationsPage />} />
      </Routes>
    </MemoryRouter>,
  );
}

describe("OperationsPage", () => {
  beforeEach(() => {
    setOverrides(undefined);
  });

  afterEach(() => {
    setOverrides(undefined);
  });

  it("renders the requested tab's content off a known URL segment", () => {
    renderAt("/operations/change_requests");
    expect(screen.getByText("Change requests content")).toBeInTheDocument();
  });

  it("defaults to the service requests tab with no segment", () => {
    renderAt("/operations");
    expect(screen.getByText("Service requests content")).toBeInTheDocument();
  });

  it("switching tabs updates the URL to the new path segment", () => {
    renderAt("/operations/service_requests");

    fireEvent.click(screen.getByRole("tab", { name: "Incidents" }));

    expect(screen.getByTestId("location-probe")).toHaveTextContent(
      "/operations/incidents",
    );
    expect(screen.getByText("Incidents content")).toBeInTheDocument();
  });

  it("renders a WIP tab disabled with its chip, and falls back rendering to the first enabled tab", () => {
    setOverrides({ "operations.change-requests": "wip" });
    renderAt("/operations/change_requests");

    const wipTab = screen.getByRole("tab", { name: /change requests/i });
    expect(wipTab).toBeDisabled();
    expect(screen.getByText("WIP")).toBeInTheDocument();
    // Requesting a WIP tab by URL renders the first enabled tab instead.
    expect(screen.getByText("Service requests content")).toBeInTheDocument();
  });

  it("omits a hidden tab from the strip entirely and falls back rendering to the first enabled tab", () => {
    setOverrides({ "operations.change-requests": "hidden" });
    renderAt("/operations/change_requests");

    expect(
      screen.queryByRole("tab", { name: /change requests/i }),
    ).not.toBeInTheDocument();
    expect(screen.getByText("Service requests content")).toBeInTheDocument();
  });

  it("falls back to the first enabled tab for an unknown URL segment", () => {
    renderAt("/operations/not-a-real-tab");
    expect(screen.getByText("Service requests content")).toBeInTheDocument();
  });

  it("redirects a legacy ?tab= URL to the equivalent path-segment URL", async () => {
    renderAt("/operations?tab=change_requests");

    await waitFor(() =>
      expect(screen.getByTestId("location-probe")).toHaveTextContent(
        "/operations/change_requests",
      ),
    );
    expect(screen.getByText("Change requests content")).toBeInTheDocument();
  });

  it("does not redirect when there is no legacy ?tab= query", () => {
    renderAt("/operations");
    expect(screen.getByTestId("location-probe")).toHaveTextContent("/operations");
  });
});
