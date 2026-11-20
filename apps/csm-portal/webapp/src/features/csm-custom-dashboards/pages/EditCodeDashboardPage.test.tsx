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

let allowed = true;
let getResult: { data?: unknown; isLoading?: boolean; isError?: boolean; error?: Error } = {};
const getCalls: (string | undefined)[] = [];

// The real client reads runtime config at module load, which isn't present under vitest.
vi.mock("@api/backend/client", () => ({
  BackendApiError: class BackendApiError extends Error {
    status: number;
    constructor(status: number, message: string) {
      super(message);
      this.status = status;
    }
  },
}));
vi.mock("@features/csm-custom-dashboards/utils/advancedDashboardAccess", () => ({
  useCanGenerateAdvancedDashboards: () => allowed,
}));
vi.mock("@features/csm-custom-dashboards/api/useCodeDashboards", () => ({
  savedCodeDashboardPath: (id: string) => `/dashboard/my/code/${encodeURIComponent(id)}`,
  useGetCodeDashboard: (id: string | undefined) => {
    getCalls.push(id);
    return getResult;
  },
  usePutCodeDashboard: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));
vi.mock("@features/csm-custom-dashboards/api/useGenerateCodeDashboard", () => ({
  useGenerateCodeDashboard: () => vi.fn(),
}));
vi.mock("@context/success-banner/SuccessBannerContext", () => ({ useSuccessBanner: () => ({ showSuccess: vi.fn() }) }));
vi.mock("@context/error-banner/ErrorBannerContext", () => ({ useErrorBanner: () => ({ showError: vi.fn() }) }));
// The frame runs the code against the portal; it is tested on its own.
vi.mock("@features/csm-custom-dashboards/components/CodeDashboardFrame", () => ({
  default: ({ source }: { source: string }) => <div>frame running {source}</div>,
}));

import EditCodeDashboardPage from "@features/csm-custom-dashboards/pages/EditCodeDashboardPage";

const ID = "5e636a12-0000-4000-8000-000000000001";
const SOURCE = "async function render(portal, root) { root.textContent = 'hi'; }";
const saved = { id: ID, name: "Open Cases", source: SOURCE, createdOn: "a", updatedOn: "b" };

function renderAt(path: string) {
  render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/dashboard/my" element={<div>My Dashboards</div>} />
        <Route path="/dashboard/my/code/:dashboardId/edit" element={<EditCodeDashboardPage />} />
        <Route path="/404" element={<div>Not found</div>} />
      </Routes>
    </MemoryRouter>,
  );
}

describe("EditCodeDashboardPage", () => {
  beforeEach(() => {
    allowed = true;
    getResult = { data: saved };
    getCalls.length = 0;
  });

  it("opens the advanced page with the saved code", () => {
    renderAt(`/dashboard/my/code/${ID}/edit`);
    expect(getCalls).toContain(ID);
    expect(screen.getByText("Edit Dashboard (Advanced)")).toBeInTheDocument();
    expect(screen.getByText(`frame running ${SOURCE}`)).toBeInTheDocument();
  });

  it("is the 404 page, and loads nothing, for a user who cannot use advanced dashboards", () => {
    allowed = false;
    renderAt(`/dashboard/my/code/${ID}/edit`);
    expect(screen.getByText("Not found")).toBeInTheDocument();
    expect(getCalls.every((id) => id === undefined)).toBe(true);
  });

  it("says when the dashboard is gone", () => {
    getResult = { data: null };
    renderAt(`/dashboard/my/code/${ID}/edit`);
    expect(screen.getByText("This dashboard no longer exists.")).toBeInTheDocument();
  });

  it("shows the load error's message, with Back to My Dashboards", () => {
    getResult = { isError: true, error: new Error("The dashboard couldn't be loaded. Try again in a moment.") };
    renderAt(`/dashboard/my/code/${ID}/edit`);
    expect(screen.getByText("The dashboard couldn't be loaded. Try again in a moment.")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Back to My Dashboards" }));
    expect(screen.getByText("My Dashboards")).toBeInTheDocument();
  });
});
