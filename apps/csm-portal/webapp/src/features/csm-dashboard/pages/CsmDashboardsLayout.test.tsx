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
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import { MemoryRouter, Route, Routes, useLocation } from "react-router";
import type { JSX } from "react";
import type { PortalAccess } from "@context/current-user/portalAccess";
import { resetFeatureStatesForTests } from "@config/featureFlags";
import { writePreviewOverride } from "@config/featurePreview";

let mockCanWrite = true;
let mockIsAdmin = false;
const MOCK_USER_ID = "test-admin";
vi.mock("@context/current-user/usePortalAccess", () => ({
  usePortalAccess: () => ({ canWrite: mockCanWrite, isAdmin: mockIsAdmin }) as PortalAccess,
  useSafeCurrentUserId: () => MOCK_USER_ID,
}));

function setOverrides(value: unknown): void {
  window.config = {
    ...window.config,
    CSM_PORTAL_FEATURE_OVERRIDES: value,
  } as Window["config"];
  resetFeatureStatesForTests();
}

import CsmDashboardsLayout from "@features/csm-dashboard/pages/CsmDashboardsLayout";

function LocationProbe(): JSX.Element {
  return <div data-testid="location-probe">{useLocation().pathname}</div>;
}

function renderAt(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <LocationProbe />
      <Routes>
        <Route element={<CsmDashboardsLayout />}>
          <Route path="dashboard" element={<div>system page</div>} />
          <Route path="dashboard/:dashboardId" element={<div>system page</div>} />
          <Route path="dashboard/:dashboardId/:teamId" element={<div>system page</div>} />
          <Route path="dashboard/my" element={<div>my page</div>} />
          <Route path="dashboard/my/:dashboardId" element={<div>my page</div>} />
          <Route path="dashboard/shared" element={<div>shared page</div>} />
          <Route path="dashboard/shared/:dashboardId" element={<div>shared page</div>} />
        </Route>
      </Routes>
    </MemoryRouter>,
  );
}

function tabNames(): string[] {
  return screen.getAllByRole("tab").map((tab) => tab.textContent ?? "");
}

function selectedTab(): string {
  return screen.getByRole("tab", { selected: true }).textContent ?? "";
}

describe("CsmDashboardsLayout", () => {
  beforeEach(() => {
    mockCanWrite = true;
    mockIsAdmin = false;
    setOverrides(undefined);
    localStorage.clear();
  });

  afterEach(() => {
    setOverrides(undefined);
    localStorage.clear();
  });

  it("shows System, My and Shared Dashboards, in that order", () => {
    renderAt("/dashboard");

    expect(screen.getByRole("tablist", { name: "Dashboards tabs" })).toBeInTheDocument();
    expect(tabNames()).toEqual(["System Dashboards", "My Dashboards", "Shared Dashboards"]);
  });

  it.each([
    ["/dashboard", "System Dashboards", "system page"],
    ["/dashboard/some-dashboard", "System Dashboards", "system page"],
    ["/dashboard/some-dashboard/some-team", "System Dashboards", "system page"],
    ["/dashboard/my", "My Dashboards", "my page"],
    ["/dashboard/my/some-dashboard", "My Dashboards", "my page"],
    ["/dashboard/shared", "Shared Dashboards", "shared page"],
    ["/dashboard/shared/some-dashboard", "Shared Dashboards", "shared page"],
  ])("at %s selects %s and shows its page", (path, tab, page) => {
    renderAt(path);

    expect(selectedTab()).toBe(tab);
    expect(screen.getByText(page)).toBeInTheDocument();
  });

  it("opens a tab's page when it is clicked", () => {
    renderAt("/dashboard");

    fireEvent.click(screen.getByRole("tab", { name: "Shared Dashboards" }));

    expect(screen.getByTestId("location-probe")).toHaveTextContent("/dashboard/shared");
    expect(screen.getByText("shared page")).toBeInTheDocument();
  });

  it("offers System and Shared Dashboards, not My Dashboards, to someone without write access", () => {
    mockCanWrite = false;
    renderAt("/dashboard");

    expect(tabNames()).toEqual(["System Dashboards", "Shared Dashboards"]);
    expect(selectedTab()).toBe("System Dashboards");
  });

  it("stays hidden for a plain write-access user once the deployment marks it hidden, even though they'd otherwise qualify", () => {
    setOverrides({ "dashboard.my": "hidden" });
    renderAt("/dashboard");

    expect(tabNames()).toEqual(["System Dashboards", "Shared Dashboards"]);
  });

  it("stays hidden for an admin who has NOT switched on their own Feature Preview toggle", () => {
    mockIsAdmin = true;
    setOverrides({ "dashboard.my": "hidden" });
    renderAt("/dashboard");

    expect(tabNames()).toEqual(["System Dashboards", "Shared Dashboards"]);
  });

  it("shows My Dashboards, marked with a Preview chip, for an admin who has switched it on in Feature Preview -- and it stays fully usable", () => {
    mockIsAdmin = true;
    setOverrides({ "dashboard.my": "hidden" });
    writePreviewOverride(MOCK_USER_ID, "dashboard.my", true);
    renderAt("/dashboard");

    const myDashboardsTab = screen.getByRole("tab", { name: /My Dashboards/ });
    expect(myDashboardsTab).toBeInTheDocument();
    expect(myDashboardsTab).toHaveTextContent("Preview");
    expect(myDashboardsTab).not.toHaveAttribute("aria-disabled", "true");

    fireEvent.click(myDashboardsTab);
    expect(screen.getByTestId("location-probe")).toHaveTextContent("/dashboard/my");
    expect(screen.getByText("my page")).toBeInTheDocument();
  });
});
