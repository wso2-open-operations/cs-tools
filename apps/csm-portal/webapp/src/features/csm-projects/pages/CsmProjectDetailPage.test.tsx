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
// the mocked `post` is never actually invoked. `get` backs
// `useProjectMetadata` (used only to gate the "Create service request" menu
// item) — resolving `null` exercises this page's fail-open path, same as an
// unconfirmed/unloaded metadata response would.
vi.mock("@api/backend/client", () => ({
  BackendApiError: class BackendApiError extends Error {
    status = 0;
  },
  useBackendApi: () => ({ post: vi.fn(), get: vi.fn(() => Promise.resolve(null)) }),
}));
vi.mock("@config/apiConfig", () => ({
  apiConfig: { backendUrl: "https://example.test" },
}));
// This page now reads usePortalAccess (to gate the "Create" split-button and
// the viewer-audience "Time Report" button), which transitively imports the
// real backend client/config -- mocked above. Default to full write access,
// no SPL audience, and the Work items staff view; the gating tests below
// override each independently.
let mockCanWrite = true;
let mockIsSplAudience = false;
let mockCanViewWorkItemsStaffView = true;
let mockCanUpdateProjectClosureState = false;
let mockCanEditProjectOnboarding = false;
vi.mock("@context/current-user/usePortalAccess", () => ({
  usePortalAccess: () => ({
    hasAnyRole: true,
    canEscalate: true,
    canDownloadAttachment: true,
    canUseOperations: true,
    canUseTimeCardsAndUpdates: true,
    canWrite: mockCanWrite,
    isSplAudience: mockIsSplAudience,
    canViewWorkItemsStaffView: mockCanViewWorkItemsStaffView,
    canUpdateProjectClosureState: mockCanUpdateProjectClosureState,
    canEditProjectOnboarding: mockCanEditProjectOnboarding,
  }),
}));

const mockPatchClosureStateMutate = vi.fn();
let mockPatchClosureStateIsPending = false;
let mockPatchClosureStateIsError = false;
vi.mock("@features/csm-projects/api/usePatchProjectClosureState", async () => {
  const actual = await vi.importActual<
    typeof import("@features/csm-projects/api/usePatchProjectClosureState")
  >("@features/csm-projects/api/usePatchProjectClosureState");
  return {
    ...actual,
    usePatchProjectClosureState: () => ({
      mutate: mockPatchClosureStateMutate,
      reset: vi.fn(),
      isPending: mockPatchClosureStateIsPending,
      isError: mockPatchClosureStateIsError,
      error: mockPatchClosureStateIsError ? new Error("failed") : null,
    }),
  };
});

const mockPatchOnboardingMutate = vi.fn();
let mockPatchOnboardingIsError = false;
vi.mock("@features/csm-projects/api/usePatchProjectOnboarding", async () => {
  const actual = await vi.importActual<
    typeof import("@features/csm-projects/api/usePatchProjectOnboarding")
  >("@features/csm-projects/api/usePatchProjectOnboarding");
  return {
    ...actual,
    usePatchProjectOnboarding: () => ({
      mutate: mockPatchOnboardingMutate,
      reset: vi.fn(),
      isPending: false,
      isError: mockPatchOnboardingIsError,
      error: mockPatchOnboardingIsError ? new Error("failed") : null,
    }),
  };
});
// The owner picker's directory search is exercised on its own in
// useSearchInternalUsers.test.tsx; here it just supplies options.
vi.mock("@features/csm-projects/api/useSearchInternalUsers", () => ({
  useSearchInternalUsers: () => ({
    data: [
      {
        id: "00000000-0000-0000-0000-000000000000",
        name: "Jane Doe",
        email: "jane.doe@example.com",
      },
    ],
    isFetching: false,
    isError: false,
  }),
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
    mockCanWrite = true;
    mockIsSplAudience = false;
    mockCanViewWorkItemsStaffView = true;
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

  it.each([
    ["managed_cloud_subscription", true],
    ["cloud_support", true],
    ["subscription", false],
    ["cloud_evaluation_support", false],
  ] as const)(
    "Create menu for a %s project shows 'Create service request': %s",
    (subscriptionType, shown) => {
      mockUseGetProject.mockReturnValue({
        data: { ...PROJECT, subscriptionType },
        isLoading: false,
        isError: false,
      });
      renderPage();
      fireEvent.click(screen.getByRole("button", { name: /create/i }));
      expect(screen.getByText("Create case")).toBeInTheDocument();
      if (shown) {
        expect(screen.getByText("Create service request")).toBeInTheDocument();
      } else {
        expect(screen.queryByText("Create service request")).not.toBeInTheDocument();
      }
    },
  );

  it("hides the Create split-button entirely for a caller without write access", () => {
    mockCanWrite = false;
    renderPage();
    expect(screen.queryByRole("button", { name: /create/i })).not.toBeInTheDocument();
  });

  // Regression: a caller without the Work items staff view (cs_engineer/
  // admin/timecard_approver) never sees Chats inside this tab (WorkItemsTab's
  // own canViewWorkItemsStaffView gate), so for them it's relabelled "Cases"
  // rather than "Work items".
  it("relabels the Work items tab as Cases for a caller without the staff view", () => {
    mockCanViewWorkItemsStaffView = false;
    renderPage();
    expect(screen.queryByRole("tab", { name: "Work items" })).not.toBeInTheDocument();
    expect(screen.getByRole("tab", { name: "Cases" })).toBeInTheDocument();
  });
});

describe("CsmProjectDetailPage — report buttons", () => {
  beforeEach(() => {
    mockCanViewWorkItemsStaffView = true;
    mockIsSplAudience = false;
    mockCanWrite = false;
    mockUseGetProject.mockReturnValue({
      data: PROJECT,
      isLoading: false,
      isError: false,
    });
  });

  it("shows Time and CS reports to anyone who can open the project, and keeps SLA disabled", () => {
    renderPage();
    expect(screen.getByRole("link", { name: "Time Report" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "CS Report" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "SLA Report — Coming soon" })).toBeDisabled();
  });
});

describe("CsmProjectDetailPage — Onboarding Status", () => {
  // Regression coverage for a real gap: onboardingStatus came back from the
  // API (confirmed on both the search row and this detail response) but the
  // page never rendered it anywhere — only the owner's name, and only when
  // an owner happened to be set too.
  it("renders the onboarding status cell independently of whether an owner is set", () => {
    mockUseGetProject.mockReturnValue({
      data: { ...PROJECT, onboardingStatus: "In-Progress", onboardingOwner: null },
      isLoading: false,
      isError: false,
    });
    renderPage();
    expect(screen.getByText("Onboarding Status")).toBeInTheDocument();
    expect(screen.getByText("In Progress")).toBeInTheDocument();
    expect(screen.queryByText("Onboarding Owner")).not.toBeInTheDocument();
  });

  it("hides the onboarding status cell when onboardingStatus is Not-Applicable", () => {
    mockUseGetProject.mockReturnValue({
      data: { ...PROJECT, onboardingStatus: "Not-Applicable", onboardingOwner: null },
      isLoading: false,
      isError: false,
    });
    renderPage();
    expect(screen.queryByText("Onboarding Status")).not.toBeInTheDocument();
  });

  it("hides the onboarding status cell when there is no onboarding engagement at all", () => {
    mockUseGetProject.mockReturnValue({
      data: { ...PROJECT },
      isLoading: false,
      isError: false,
    });
    renderPage();
    expect(screen.queryByText("Onboarding Status")).not.toBeInTheDocument();
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

// Regression coverage for the admin-only direct closure-state override: the
// edit affordance is a client-side convenience only (the backend enforces
// the real PermAdmin gate on PATCH /projects/{id}'s closureState field, see
// apps/csm-portal/backend's own CLAUDE.md) -- hiding it here just avoids
// offering an action that would 403 for everyone else.
describe("CsmProjectDetailPage — closure state edit", () => {
  beforeEach(() => {
    mockCanUpdateProjectClosureState = false;
    mockPatchClosureStateMutate.mockReset();
    mockPatchClosureStateIsPending = false;
    mockPatchClosureStateIsError = false;
    mockUseGetProject.mockReturnValue({
      data: { ...PROJECT, closureState: "Open" },
      isLoading: false,
      isError: false,
    });
  });

  it("hides the edit control for a caller without canUpdateProjectClosureState", () => {
    renderPage();
    expect(
      screen.queryByRole("button", { name: "Edit closure state" }),
    ).not.toBeInTheDocument();
  });

  it("shows the edit control for an admin and opens the dialog on click", () => {
    mockCanUpdateProjectClosureState = true;
    renderPage();

    fireEvent.click(screen.getByRole("button", { name: "Edit closure state" }));

    expect(screen.getByText("Edit closure state", { selector: "h2" })).toBeInTheDocument();
  });

  it("submits the newly selected state via the patch mutation", () => {
    mockCanUpdateProjectClosureState = true;
    renderPage();

    fireEvent.click(screen.getByRole("button", { name: "Edit closure state" }));
    fireEvent.mouseDown(screen.getByRole("combobox", { name: /closure state/i }));
    fireEvent.click(screen.getByRole("option", { name: "Suspended" }));
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    expect(mockPatchClosureStateMutate).toHaveBeenCalledWith(
      "SUSPENDED",
      expect.objectContaining({ onSuccess: expect.any(Function) }),
    );
  });
});

// The onboarding edit is gated client-side only to avoid offering an action
// that would 403 (the backend enforces onboarding_lead/admin on those two
// fields). For an editor it must stay reachable even when the project has no
// onboarding engagement yet, otherwise nobody could ever start one.
describe("CsmProjectDetailPage — onboarding edit", () => {
  beforeEach(() => {
    mockCanEditProjectOnboarding = false;
    mockPatchOnboardingMutate.mockReset();
    mockPatchOnboardingIsError = false;
    mockUseGetProject.mockReturnValue({
      data: {
        ...PROJECT,
        onboardingStatus: "In-Progress",
        onboardingOwner: { id: "owner-1", name: "John Roe", email: "john.roe@example.com" },
      },
      isLoading: false,
      isError: false,
    });
  });

  it("hides the edit control for a caller without canEditProjectOnboarding (e.g. cs_engineer only)", () => {
    renderPage();
    expect(screen.getByText("Onboarding Status")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Edit onboarding" })).not.toBeInTheDocument();
  });

  it("shows the edit control for an authorized caller and opens the dialog with controls", () => {
    mockCanEditProjectOnboarding = true;
    renderPage();

    fireEvent.click(screen.getByRole("button", { name: "Edit onboarding" }));

    expect(screen.getByText("Edit onboarding", { selector: "h2" })).toBeInTheDocument();
    expect(screen.getByRole("combobox", { name: /onboarding status/i })).toBeInTheDocument();
    expect(screen.getByRole("combobox", { name: /onboarding owner/i })).toBeInTheDocument();
    // The owner can be changed but never cleared.
    expect(screen.queryByRole("button", { name: /clear/i })).not.toBeInTheDocument();
  });

  it("stays reachable for an editor when the project has no onboarding status", () => {
    mockCanEditProjectOnboarding = true;
    mockUseGetProject.mockReturnValue({
      data: { ...PROJECT, onboardingStatus: "Not-Applicable", onboardingOwner: null },
      isLoading: false,
      isError: false,
    });
    renderPage();
    expect(screen.getByRole("button", { name: "Edit onboarding" })).toBeInTheDocument();
    expect(screen.getByText("Onboarding Owner")).toBeInTheDocument();
  });

  it("keeps the read-only display unchanged for a non-editor when there is no onboarding", () => {
    mockUseGetProject.mockReturnValue({
      data: { ...PROJECT, onboardingStatus: "Not-Applicable", onboardingOwner: null },
      isLoading: false,
      isError: false,
    });
    renderPage();
    expect(screen.queryByText("Onboarding Status")).not.toBeInTheDocument();
    expect(screen.queryByText("Onboarding Owner")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Edit onboarding" })).not.toBeInTheDocument();
  });

  it("submits only the changed status", () => {
    mockCanEditProjectOnboarding = true;
    renderPage();

    fireEvent.click(screen.getByRole("button", { name: "Edit onboarding" }));
    fireEvent.mouseDown(screen.getByRole("combobox", { name: /onboarding status/i }));
    fireEvent.click(screen.getByRole("option", { name: "On Hold" }));
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    expect(mockPatchOnboardingMutate).toHaveBeenCalledWith(
      { onboardingStatus: "On-Hold" },
      expect.objectContaining({ onSuccess: expect.any(Function) }),
    );
  });

  it("submits a newly picked owner by id", () => {
    mockCanEditProjectOnboarding = true;
    renderPage();

    fireEvent.click(screen.getByRole("button", { name: "Edit onboarding" }));
    const owner = screen.getByRole("combobox", { name: /onboarding owner/i });
    fireEvent.mouseDown(owner);
    fireEvent.click(screen.getByRole("option", { name: "Jane Doe" }));
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    expect(mockPatchOnboardingMutate).toHaveBeenCalledWith(
      { onboardingOwner: "00000000-0000-0000-0000-000000000000" },
      expect.objectContaining({ onSuccess: expect.any(Function) }),
    );
  });

  it("shows the save error inside the dialog", () => {
    mockCanEditProjectOnboarding = true;
    mockPatchOnboardingIsError = true;
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: "Edit onboarding" }));
    expect(screen.getByRole("alert")).toHaveTextContent(
      "Could not update the project's onboarding details",
    );
  });
});

describe("CsmProjectDetailPage — Onboarding tab", () => {
  beforeEach(() => {
    mockCanEditProjectOnboarding = false;
    mockPatchOnboardingMutate.mockReset();
    mockPatchOnboardingIsError = false;
    mockUseGetProject.mockReturnValue({
      data: {
        ...PROJECT,
        onboardingStatus: "In-Progress",
        onboardingOwner: { id: "owner-1", name: "John Roe", email: "john.roe@example.com" },
        onboardingStartDate: "2026-03-01T00:00:00Z",
        goLivePlanDate: "2026-04-15T00:00:00Z",
        goLiveDate: null,
        onboardingExpiryDate: "2026-12-31T00:00:00Z",
      },
      isLoading: false,
      isError: false,
    });
  });

  it("lists the Onboarding tab right after Overview, for everyone", () => {
    renderPage();
    const names = screen.getAllByRole("tab").map((t) => t.textContent);
    expect(names.slice(0, 2)).toEqual(["Overview", "Onboarding"]);
  });

  it("selects the tab from ?tab=onboarding and renders its fields read-only", () => {
    renderPage("/customers/projects/proj-1?tab=onboarding");
    expect(screen.queryByText("Project key")).not.toBeInTheDocument();
    expect(screen.getByText("Onboarding start date")).toBeInTheDocument();
    expect(screen.getByText("Mar 1, 2026")).toBeInTheDocument();
    expect(screen.getByText("Apr 15, 2026")).toBeInTheDocument();
    expect(screen.getByText("John Roe")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Edit" })).not.toBeInTheDocument();
  });

  it("clicking the tab updates ?tab=onboarding", () => {
    renderPage();
    fireEvent.click(screen.getByRole("tab", { name: "Onboarding" }));
    expect(screen.getByTestId("location-probe")).toHaveTextContent(
      "/customers/projects/proj-1?tab=onboarding",
    );
  });

  it("falls back to Overview for an unknown ?tab= value", () => {
    renderPage("/customers/projects/proj-1?tab=nonsense");
    expect(screen.getByText("Project key")).toBeInTheDocument();
  });

  it("opens the same onboarding dialog from the tab's Edit button for an editor", () => {
    mockCanEditProjectOnboarding = true;
    renderPage("/customers/projects/proj-1?tab=onboarding");
    fireEvent.click(screen.getByRole("button", { name: "Edit" }));
    expect(screen.getByText("Edit onboarding", { selector: "h2" })).toBeInTheDocument();
    expect(screen.getByRole("group", { name: "Go-live date" })).toBeInTheDocument();
  });

  it("opens the same dialog, with the dates, from the overview pencil", () => {
    mockCanEditProjectOnboarding = true;
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: "Edit onboarding" }));
    expect(screen.getByRole("group", { name: "Onboarding expiry date" })).toBeInTheDocument();
  });
});
