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
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import type { JSX } from "react";
import {
  MemoryRouter,
  Route,
  Routes,
  useLocation,
  useNavigate,
  type NavigateOptions,
  type To,
} from "react-router";
import type { UseQueryResult } from "@tanstack/react-query";
import type { ProjectDetails } from "@features/csm-projects/types/csmProjects";

const navigateMock = vi.fn();
const useGetProjectMock = vi.fn();

vi.mock("@hooks/useNavTransition", () => ({
  useNavTransition: () => navigateMock,
}));
vi.mock("@features/csm-projects/api/useGetProject", () => ({
  useGetProject: () => useGetProjectMock(),
}));
vi.mock("@features/csm-projects/components/DeploymentsTab", () => ({
  default: ({ projectId }: { projectId: string }) => <div>Deployments for {projectId}</div>,
}));
vi.mock("@features/csm-projects/components/ProjectContactsTab", () => ({
  default: ({ projectId }: { projectId: string }) => <div>Contacts for {projectId}</div>,
}));
vi.mock("@features/csm-projects/components/WorkItemsTab", () => ({
  default: ({ projectId, basePath }: { projectId: string; basePath: string }) => (
    <div>
      Work items for {projectId} at {basePath}
    </div>
  ),
}));

// Imported after the mocks above so the module picks them up.
import CsmProjectDetailPage from "@features/csm-projects/pages/CsmProjectDetailPage";

// `useNavTransition` is mocked module-wide (above); this page's tab strip is
// a real URL path segment (`usePathTabs`), so the mock is bridged to the real
// `useNavigate` from this render tree — same pattern as
// CsmChangeRequestDetailPage.test.tsx's `NavigateBridge` — so a simulated tab
// click actually drives the router instead of being a no-op.
function NavigateBridge(): null {
  const navigate = useNavigate();
  navigateMock.mockImplementation((to: To, options?: NavigateOptions) =>
    navigate(to, options),
  );
  return null;
}

function LocationProbe(): JSX.Element {
  const location = useLocation();
  return <div data-testid="location-probe">{location.pathname}</div>;
}

function renderPage(
  initialEntry = "/customers/projects/proj-1",
): ReturnType<typeof render> {
  return render(
    <MemoryRouter initialEntries={[initialEntry]}>
      <NavigateBridge />
      <LocationProbe />
      <Routes>
        <Route
          path="/customers/projects/:id/:tab?"
          element={<CsmProjectDetailPage />}
        />
        <Route
          path="/customers/projects/:id/work-items/:tab?"
          element={<CsmProjectDetailPage />}
        />
      </Routes>
    </MemoryRouter>,
  );
}

const BASE_PROJECT: ProjectDetails = {
  id: "proj-1",
  account: {
    id: "acc-1",
    name: "Acme Corp",
    activationDate: "2020-01-01T00:00:00Z",
    tier: "Enterprise",
    agentEnabled: true,
    kbReferencesEnabled: true,
  },
  sfId: "SF-001",
  name: "Acme Platform",
  key: "ACME-PLAT",
  subscriptionType: "subscription",
  startDate: "2020-01-01T00:00:00Z",
  endDate: null,
  createdOn: "2020-01-01T00:00:00Z",
  updatedOn: "2026-01-01T00:00:00Z",
  closureState: "open",
};

function mockQueryResult(
  overrides: Partial<UseQueryResult<ProjectDetails | null, Error>>,
): void {
  useGetProjectMock.mockReturnValue({
    data: null,
    isLoading: false,
    isError: false,
    error: null,
    ...overrides,
  });
}

beforeEach(() => {
  navigateMock.mockClear();
  useGetProjectMock.mockReset();
  mockQueryResult({ data: BASE_PROJECT });
});

describe("CsmProjectDetailPage — tab is a real URL path segment", () => {
  function goToTab(name: string): void {
    fireEvent.click(screen.getByRole("tab", { name }));
  }

  it("defaults to the Overview tab when the URL carries no tab segment", () => {
    renderPage();
    expect(screen.getByRole("tab", { name: "Overview" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    expect(screen.getByText("Project key")).toBeInTheDocument();
  });

  it("clicking a tab navigates to that tab's own URL", () => {
    renderPage();

    goToTab("Deployments");

    expect(screen.getByTestId("location-probe")).toHaveTextContent(
      "/customers/projects/proj-1/deployments",
    );
    expect(screen.getByRole("tab", { name: "Deployments" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    expect(screen.getByText("Deployments for proj-1")).toBeInTheDocument();
  });

  it("loading the page directly at a tab's URL opens on that tab", () => {
    renderPage("/customers/projects/proj-1/contacts");
    expect(screen.getByRole("tab", { name: "Project contacts" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    expect(screen.getByText("Contacts for proj-1")).toBeInTheDocument();
  });

  it("falls back to Overview for an unknown tab segment, without redirecting the URL", () => {
    renderPage("/customers/projects/proj-1/bogus-tab");

    expect(screen.getByRole("tab", { name: "Overview" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    expect(screen.getByTestId("location-probe")).toHaveTextContent(
      "/customers/projects/proj-1/bogus-tab",
    );
  });
});

describe("CsmProjectDetailPage — Work items nested sub-tab route", () => {
  it("clicking Work items navigates to the nested work-items base path", () => {
    renderPage();

    fireEvent.click(screen.getByRole("tab", { name: "Work items" }));

    expect(screen.getByTestId("location-probe")).toHaveTextContent(
      "/customers/projects/proj-1/work-items",
    );
    expect(
      screen.getByText("Work items for proj-1 at /customers/projects/proj-1/work-items"),
    ).toBeInTheDocument();
  });

  it("loading the page directly at a nested work-items sub-tab URL shows the Work items tab", () => {
    renderPage("/customers/projects/proj-1/work-items/conversations");

    expect(screen.getByRole("tab", { name: "Work items" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    expect(
      screen.getByText("Work items for proj-1 at /customers/projects/proj-1/work-items"),
    ).toBeInTheDocument();
  });

  it("loading the page directly at the bare work-items path shows the Work items tab", () => {
    renderPage("/customers/projects/proj-1/work-items");

    expect(screen.getByRole("tab", { name: "Work items" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    expect(
      screen.getByText("Work items for proj-1 at /customers/projects/proj-1/work-items"),
    ).toBeInTheDocument();
  });
});

describe("CsmProjectDetailPage — loading/error/not-found states", () => {
  it("shows a skeleton while loading", () => {
    mockQueryResult({ isLoading: true });
    renderPage();
    expect(screen.queryByRole("tab")).not.toBeInTheDocument();
  });

  it("shows an error message when the query fails", () => {
    mockQueryResult({ isError: true });
    renderPage();
    expect(screen.getByText(/could not load project/i)).toBeInTheDocument();
  });

  it("shows a not-found message when there is no data", () => {
    mockQueryResult({ data: null });
    renderPage();
    expect(screen.getByText("Project not found")).toBeInTheDocument();
  });
});
