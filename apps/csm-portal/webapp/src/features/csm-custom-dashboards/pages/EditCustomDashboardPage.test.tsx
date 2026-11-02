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
import { MemoryRouter, Route, Routes } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import type { BeDashboard } from "@api/backend/types";

let detailState: { data?: unknown; isLoading?: boolean; isError?: boolean; error?: Error };
const detailIds: (string | undefined)[] = [];
const received: (BeDashboard | undefined)[] = [];

vi.mock("@features/csm-custom-dashboards/api/useCustomDashboards", () => ({
  useGetCustomDashboard: (id: string | undefined) => {
    detailIds.push(id);
    return detailState;
  },
}));
// The generate page has its own tests; here only what it is handed matters.
vi.mock("@features/csm-custom-dashboards/pages/GenerateDashboardPage", () => ({
  default: ({ initialDashboard }: { initialDashboard?: BeDashboard }) => {
    received.push(initialDashboard);
    return <div>Generate page for {initialDashboard?.displayName}</div>;
  },
}));

import EditCustomDashboardPage from "@features/csm-custom-dashboards/pages/EditCustomDashboardPage";

const ID = "5e636a12-0000-4000-8000-000000000001";
const dashboard: BeDashboard = { id: ID, displayName: "My Cases", isDefault: false, isTeamBased: false, widgets: [] };

function renderAt(path: string) {
  render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/dashboard/my/:dashboardId/edit" element={<EditCustomDashboardPage />} />
      </Routes>
    </MemoryRouter>,
  );
}

describe("EditCustomDashboardPage", () => {
  beforeEach(() => {
    detailIds.length = 0;
    received.length = 0;
  });

  it("hands the saved dashboard to the generate page", () => {
    detailState = { data: { dashboard, createdOn: "a", updatedOn: "b" } };
    renderAt(`/dashboard/my/${ID}/edit`);
    expect(detailIds).toContain(ID);
    expect(screen.getByText("Generate page for My Cases")).toBeInTheDocument();
    expect(received.at(-1)).toEqual(dashboard);
  });

  it("shows a skeleton and Back while loading", () => {
    detailState = { isLoading: true };
    renderAt(`/dashboard/my/${ID}/edit`);
    expect(screen.getByRole("button", { name: "Back to My Dashboards" })).toBeInTheDocument();
    expect(received).toHaveLength(0);
  });

  it("shows the backend's message when the dashboard can't be edited", () => {
    detailState = { isError: true, error: new Error("This saved dashboard can't be shown any more. Generate it again.") };
    renderAt(`/dashboard/my/${ID}/edit`);
    expect(screen.getByText("This saved dashboard can't be shown any more. Generate it again.")).toBeInTheDocument();
  });

  it("says so when the dashboard no longer exists", () => {
    detailState = { data: null };
    renderAt(`/dashboard/my/${ID}/edit`);
    expect(screen.getByText("This dashboard no longer exists.")).toBeInTheDocument();
  });
});
