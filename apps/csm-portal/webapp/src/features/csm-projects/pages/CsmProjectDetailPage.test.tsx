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
import { MemoryRouter, Route, Routes, useLocation } from "react-router";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ProjectDetails } from "@features/csm-projects/types/csmProjects";

const mockUseGetProject = vi.fn();
vi.mock("@features/csm-projects/api/useGetProject", () => ({
  useGetProject: (...args: unknown[]) => mockUseGetProject(...args),
}));

vi.mock("@features/csm-projects/components/DeploymentsTab", () => ({
  default: ({ projectId }: { projectId: string }) => <div>Deployments for {projectId}</div>,
}));
vi.mock("@features/csm-projects/components/ProjectContactsTab", () => ({
  default: ({ projectId }: { projectId: string }) => <div>Contacts for {projectId}</div>,
}));
vi.mock("@features/csm-projects/components/WorkItemsTab", () => ({
  default: ({ projectId }: { projectId: string }) => <div>Work items for {projectId}</div>,
}));

// The backend client reads runtime config at module load, which isn't
// present under vitest. `UserRefLink` (used for the Onboarding Owner cell)
// resolves an unknown id through `useBackendApi` — same approach as
// ProjectContactsTab.test.tsx. Every test here passes a known `userId`, so
// the mocked `post` is never actually invoked.
vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: vi.fn() }),
}));

import CsmProjectDetailPage from "@features/csm-projects/pages/CsmProjectDetailPage";

const PROJECT: ProjectDetails = {
  id: "proj-1",
  account: {
    id: "acct-1",
    name: "Acme",
    activationDate: null,
    tier: "gold",
    agentEnabled: true,
    kbReferencesEnabled: true,
  },
  sfId: "SF-1",
  name: "Acme Subscription",
  key: "ACME",
  subscriptionType: "managed_cloud_subscription",
  startDate: null,
  endDate: null,
  createdOn: "2025-01-01T00:00:00Z",
  updatedOn: "2025-06-01T00:00:00Z",
  closureState: null,
};

/** Destination probe: shows the pathname+search and the state.from a click
 * actually navigated with, so tests assert on real router navigation. */
function LocationProbe() {
  const location = useLocation();
  return (
    <>
      <div data-testid="location-probe">{location.pathname + location.search}</div>
      <div data-testid="location-state-probe">{JSON.stringify(location.state ?? null)}</div>
    </>
  );
}

function renderPage(initialEntry = "/customers/projects/proj-1") {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[initialEntry]}>
        <Routes>
          <Route
            path="/customers/projects/:id"
            element={
              <>
                <CsmProjectDetailPage />
                <LocationProbe />
              </>
            }
          />
          <Route path="/cases/new" element={<LocationProbe />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("CsmProjectDetailPage — tab state", () => {
  beforeEach(() => {
    mockUseGetProject.mockReturnValue({
      data: PROJECT,
      isLoading: false,
      isError: false,
    });
  });

  it("defaults to the Overview tab when the URL carries no ?tab=", () => {
    renderPage();
    expect(screen.getByText("Project key")).toBeInTheDocument();
    expect(screen.queryByText("Work items for proj-1")).not.toBeInTheDocument();
  });

  it("renders the tab named in the URL's ?tab= param, not always Overview", () => {
    renderPage("/customers/projects/proj-1?tab=workItems");
    expect(screen.getByText("Work items for proj-1")).toBeInTheDocument();
    expect(screen.queryByText("Project key")).not.toBeInTheDocument();
  });

  // Regression test: activeTab used to be plain component state, so a
  // create-flow round trip back to this page always reset it to Overview
  // even if the engineer had been on Work items. It's now kept in the URL.
  it("switches tabs by updating the URL's ?tab= param, not local state", () => {
    renderPage();
    fireEvent.click(screen.getByRole("tab", { name: "Work items" }));
    expect(screen.getByText("Work items for proj-1")).toBeInTheDocument();
  });

  // Regression test: a sub-tab selected under Work items (`?subTab=`) only
  // means something under that tab — switching to a different top-level tab
  // must drop it, or revisiting Work items later would silently land back on
  // the stale sub-tab instead of its own default.
  it("drops a stale ?subTab= when switching to a different top-level tab", () => {
    renderPage("/customers/projects/proj-1?tab=workItems&subTab=engagements");

    fireEvent.click(screen.getByRole("tab", { name: "Deployments" }));

    expect(screen.getByTestId("location-probe")).toHaveTextContent(
      "/customers/projects/proj-1?tab=deployments",
    );
  });

  it("passes the current pathname+search as the create page's return state", () => {
    renderPage("/customers/projects/proj-1?tab=workItems&subTab=engagements");

    fireEvent.click(screen.getByRole("button", { name: /create/i }));
    fireEvent.click(screen.getByText("Create case"));

    expect(screen.getByTestId("location-probe")).toHaveTextContent("/cases/new");
    expect(screen.getByTestId("location-state-probe")).toHaveTextContent(
      JSON.stringify({
        from: "/customers/projects/proj-1?tab=workItems&subTab=engagements",
      }),
    );
  });
});

describe("CsmProjectDetailPage — Onboarding Owner", () => {
  it("renders the onboarding owner cell when onboarding is enabled and an owner is set", () => {
    mockUseGetProject.mockReturnValue({
      data: {
        ...PROJECT,
        onboardingStatus: "In-Progress",
        onboardingOwner: {
          id: "user-1",
          name: "Jane Doe",
          email: "jane.doe@example.com",
        },
      },
      isLoading: false,
      isError: false,
    });
    renderPage();
    expect(screen.getByText("Onboarding Owner")).toBeInTheDocument();
    expect(screen.getByText("Jane Doe")).toBeInTheDocument();
  });

  it("hides the onboarding owner cell when onboardingStatus is Not-Applicable", () => {
    mockUseGetProject.mockReturnValue({
      data: {
        ...PROJECT,
        onboardingStatus: "Not-Applicable",
        onboardingOwner: {
          id: "user-1",
          name: "Jane Doe",
          email: "jane.doe@example.com",
        },
      },
      isLoading: false,
      isError: false,
    });
    renderPage();
    expect(screen.queryByText("Onboarding Owner")).not.toBeInTheDocument();
  });

  it("hides the onboarding owner cell when there is no onboarding engagement at all", () => {
    mockUseGetProject.mockReturnValue({
      data: { ...PROJECT },
      isLoading: false,
      isError: false,
    });
    renderPage();
    expect(screen.queryByText("Onboarding Owner")).not.toBeInTheDocument();
  });
});
