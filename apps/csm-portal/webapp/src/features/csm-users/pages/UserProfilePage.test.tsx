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
import { QueryClient, QueryClientProvider, type UseQueryResult } from "@tanstack/react-query";
import type { NormalizedUserDetail } from "@features/csm-users/types/csmUsers";

const useGetUserByIdMock = vi.fn();

// The backend client reads runtime config (`CSM_PORTAL_BACKEND_BASE_URL`) at
// module load, which isn't present under vitest. `QueryErrorState` imports
// `BackendApiError` from it directly, so stub the module with a real class
// (so `instanceof` still works) — same approach as
// CsmChangeRequestDetailPage.test.tsx. `useBackendApi` also needs a working
// `post`: `PermissionsCard` resolves role display names via `useSearchRoles`,
// which goes through this same client.
const backendPostMock = vi.fn();
vi.mock("@api/backend/client", () => ({
  BackendApiError: class BackendApiError extends Error {
    status: number;
    constructor(status: number, message: string) {
      super(message);
      this.status = status;
    }
  },
  useBackendApi: () => ({ post: backendPostMock }),
}));
vi.mock("@features/csm-users/api/useGetUserById", () => ({
  useGetUserById: () => useGetUserByIdMock(),
}));

// Imported after the mocks above so the module picks them up.
import UserProfilePage from "@features/csm-users/pages/UserProfilePage";

const INTERNAL_USER: NormalizedUserDetail = {
  id: "user-1",
  userName: "jane.doe",
  name: "Jane Doe",
  email: "jane.doe@example.com",
  timezone: "UTC",
  userType: "internal",
  active: true,
  roles: ["internal", "agent"],
  phone: "+10000000000",
  createdOn: "2025-01-01T00:00:00Z",
  updatedOn: "2025-06-01T00:00:00Z",
  groups: [{ id: "grp-1", name: "Tier 2 support" }],
  teams: [{ id: "team-1", name: "CRE", family: "CRE" }],
};

const BLOCKED_EXTERNAL_USER: NormalizedUserDetail = {
  id: "user-2",
  userName: "john.smith",
  name: "John Smith",
  email: "john.smith@example.com",
  timezone: null,
  userType: "external",
  active: true,
  roles: ["customer"],
  createdOn: "2025-01-01T00:00:00Z",
  updatedOn: "2025-06-01T00:00:00Z",
  projectAccess: [
    {
      projectId: "proj-1",
      projectName: "Payments Platform",
      projectKey: "PAYPLAT",
      contactEmail: "john.smith@example.com",
      contactRecordPresent: false,
      grantsCaseAccess: false,
    },
    {
      projectId: "proj-2",
      projectName: "Identity Platform",
      projectKey: "IDPLAT",
      contactEmail: "john.smith@example.com",
      contactRecordPresent: true,
      contactRecordEmail: "john.smith@example.com",
      registrationState: "registered",
      notificationsEnabled: true,
      roles: ["viewer"],
      grantsCaseAccess: true,
    },
  ],
};

function mockQueryResult(
  overrides: Partial<UseQueryResult<NormalizedUserDetail | null, Error>>,
): void {
  useGetUserByIdMock.mockReturnValue({
    data: null,
    isLoading: false,
    isError: false,
    error: null,
    ...overrides,
  });
}

/** Destination probe: renders wherever a Back click actually lands, showing
 * both the resulting path and the location.state that came with it — so
 * tests assert on real router navigation, not a mocked navigate function. */
function LocationProbe() {
  const location = useLocation();
  return (
    <>
      <div data-testid="location-probe">{location.pathname + location.search}</div>
      <div data-testid="location-state-probe">{JSON.stringify(location.state ?? null)}</div>
    </>
  );
}

function renderPage(
  routeState?: { from?: string; parentState?: unknown },
): ReturnType<typeof render> {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[{ pathname: "/people/user-1", state: routeState ?? null }]}>
        <Routes>
          <Route path="/people/:id" element={<UserProfilePage />} />
          <Route path="/admin/users" element={<LocationProbe />} />
          <Route path="/dashboard" element={<LocationProbe />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

const ROLES_RESPONSE = {
  roles: [
    { id: "internal", name: "Internal" },
    { id: "agent", name: "Agent" },
    { id: "customer", name: "Customer" },
  ],
  total: 3,
  limit: 50,
  offset: 0,
};

describe("UserProfilePage", () => {
  beforeEach(() => {
    backendPostMock.mockReset();
    backendPostMock.mockResolvedValue(ROLES_RESPONSE);
  });

  it("renders a loading skeleton while the query is pending", () => {
    mockQueryResult({ isLoading: true });
    const { container } = renderPage();
    expect(container.querySelectorAll(".MuiSkeleton-root").length).toBeGreaterThan(0);
  });

  it("renders an error state when the query fails", () => {
    mockQueryResult({ isError: true, error: new Error("boom") });
    renderPage();
    expect(screen.getByText("boom")).toBeInTheDocument();
  });

  it("renders a not-found state when the user is null", () => {
    mockQueryResult({ data: null });
    renderPage();
    expect(screen.getByText(/User not found/i)).toBeInTheDocument();
  });

  it("renders an internal user's team inline in the Overview card, plus groups and roles", async () => {
    mockQueryResult({ data: INTERNAL_USER });
    renderPage();
    expect(screen.getByText("Jane Doe")).toBeInTheDocument();
    expect(screen.getByText("+10000000000")).toBeInTheDocument();
    expect(screen.getByText("Tier 2 support")).toBeInTheDocument();
    expect(screen.getByText("CRE (CRE)")).toBeInTheDocument();

    // Role labels resolve through the roles catalogue, same as the users
    // list, so this reads "Agent" (and "Internal", appearing twice: once as
    // the account-type chip, once as a role chip) rather than the raw keys
    // "agent"/"internal".
    expect(await screen.findByText("Agent", { selector: ".MuiChip-label" })).toBeInTheDocument();
    expect(screen.getAllByText("Internal", { selector: ".MuiChip-label" }).length).toBeGreaterThan(
      0,
    );
    expect(screen.queryByText("agent", { selector: ".MuiChip-label" })).not.toBeInTheDocument();
  });

  it("renders 'Unassigned' rather than hiding the field when an internal user has no team", () => {
    mockQueryResult({ data: { ...INTERNAL_USER, teams: [] } });
    renderPage();
    expect(screen.getByText("Unassigned")).toBeInTheDocument();
  });

  it("does not render a team field or a User groups cluster for an external user", () => {
    mockQueryResult({ data: BLOCKED_EXTERNAL_USER });
    renderPage();
    expect(screen.queryByText("Team")).not.toBeInTheDocument();
    expect(screen.queryByText(/User groups/i)).not.toBeInTheDocument();
  });

  it("renders the project key and the blocking reason for a project that doesn't grant an external user case access", () => {
    mockQueryResult({ data: BLOCKED_EXTERNAL_USER });
    renderPage();

    // The blocked project surfaces its reason...
    expect(screen.getByText("Payments Platform")).toBeInTheDocument();
    expect(screen.getByText("PAYPLAT")).toBeInTheDocument();
    expect(screen.getByText("No access", { selector: ".MuiChip-label" })).toBeInTheDocument();
    expect(
      screen.getByText(/No contact record is linked to this project/i),
    ).toBeInTheDocument();

    // ...while the granted, registered project shows "Has access" and no reason.
    expect(screen.getByText("Identity Platform")).toBeInTheDocument();
    expect(screen.getByText("IDPLAT")).toBeInTheDocument();
    expect(screen.getByText("Has access", { selector: ".MuiChip-label" })).toBeInTheDocument();

    expect(screen.getByText(/Blocked on 1 of 2 projects/i)).toBeInTheDocument();
  });

  it("renders 'Invited' rather than 'Has access' for a granted row still pending registration", () => {
    mockQueryResult({
      data: {
        ...BLOCKED_EXTERNAL_USER,
        projectAccess: [
          {
            projectId: "proj-3",
            projectName: "Analytics Platform",
            projectKey: "ANALYTICS",
            contactEmail: "john.smith@example.com",
            contactRecordPresent: true,
            contactRecordEmail: "john.smith@example.com",
            registrationState: "invited",
            grantsCaseAccess: true,
          },
        ],
      },
    });
    renderPage();
    expect(screen.getByText("Invited", { selector: ".MuiChip-label" })).toBeInTheDocument();
    expect(screen.queryByText("Has access", { selector: ".MuiChip-label" })).not.toBeInTheDocument();
  });

  // Same two-causes distinction the project contacts tab makes: "No access"
  // on its own does not say what to fix.
  it("names both addresses when a row is linked but invited under a different address", () => {
    mockQueryResult({
      data: {
        ...BLOCKED_EXTERNAL_USER,
        projectAccess: [
          {
            projectId: "proj-4",
            projectName: "Query Platform",
            projectKey: "QUERYPLAT",
            contactEmail: "someone.else@example.com",
            contactRecordPresent: true,
            contactRecordEmail: "john.smith@example.com",
            registrationState: "invited",
            grantsCaseAccess: false,
          },
        ],
      },
    });
    renderPage();
    expect(screen.getByText("No access", { selector: ".MuiChip-label" })).toBeInTheDocument();
    expect(
      screen.getByText(
        /Invited as someone\.else@example\.com but linked to a contact whose own address is john\.smith@example\.com/i,
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByText(/No contact record is linked to this project/i),
    ).not.toBeInTheDocument();
  });

  it("renders 'No project access records found' rather than hiding the card for an external user with none", () => {
    mockQueryResult({ data: { ...BLOCKED_EXTERNAL_USER, projectAccess: [] } });
    renderPage();
    expect(
      screen.getByText(/No project access records found for this user/i),
    ).toBeInTheDocument();
  });

  it("renders a 'Locked out' chip when the account is locked out", () => {
    mockQueryResult({ data: { ...INTERNAL_USER, lockedOut: true } });
    renderPage();
    expect(screen.getByText("Locked out", { selector: ".MuiChip-label" })).toBeInTheDocument();
  });

  it("does not render a 'Locked out' chip when the account isn't locked out", () => {
    mockQueryResult({ data: { ...INTERNAL_USER, lockedOut: false } });
    renderPage();
    expect(screen.queryByText("Locked out", { selector: ".MuiChip-label" })).not.toBeInTheDocument();
  });

  it("shows only the 'Locked out' chip in the header — not 'Active' — for an active-but-locked-out user", () => {
    mockQueryResult({ data: { ...INTERNAL_USER, active: true, lockedOut: true } });
    renderPage();
    // Exactly one "Locked out" chip (header) and exactly one "Active" chip
    // (the Overview card's own, unconditional "Account status" field) — the
    // header itself must not also render a second "Active" chip.
    expect(screen.getAllByText("Locked out", { selector: ".MuiChip-label" })).toHaveLength(1);
    expect(screen.getAllByText("Active", { selector: ".MuiChip-label" })).toHaveLength(1);
  });

  it("still shows both Account status and Locked out as separate fields in the Overview card", () => {
    mockQueryResult({ data: { ...INTERNAL_USER, active: true, lockedOut: true } });
    renderPage();
    expect(screen.getByText("Account status")).toBeInTheDocument();
    // "Locked out" appears twice: the header chip's label and the Overview
    // field's caption — both are expected here, not a duplicate bug.
    expect(screen.getAllByText("Locked out").length).toBeGreaterThanOrEqual(2);
    // The header collapses to one "Locked out" chip; the Overview card still
    // shows "Active" for account status and "Yes" for locked-out separately.
    expect(screen.getByText("Active", { selector: ".MuiChip-label" })).toBeInTheDocument();
    expect(screen.getByText("Yes", { selector: ".MuiChip-label" })).toBeInTheDocument();
  });

  it("calls out an inactive account as blocking access to every project", () => {
    mockQueryResult({ data: { ...BLOCKED_EXTERNAL_USER, active: false } });
    renderPage();
    expect(screen.getByText(/account is inactive/i)).toBeInTheDocument();
  });

  it("renders the external account's exists/locked chips for an external user", () => {
    mockQueryResult({
      data: { ...BLOCKED_EXTERNAL_USER, externalAccount: { exists: true, locked: false } },
    });
    renderPage();
    expect(screen.getByText("External account")).toBeInTheDocument();
    expect(screen.getByText("Exists", { selector: ".MuiChip-label" })).toBeInTheDocument();
    expect(screen.getByText("Unlocked", { selector: ".MuiChip-label" })).toBeInTheDocument();
  });

  it("calls out a locked external account, both as a chip and as a blocking alert", () => {
    mockQueryResult({
      data: { ...BLOCKED_EXTERNAL_USER, externalAccount: { exists: true, locked: true } },
    });
    renderPage();
    expect(screen.getByText("Locked", { selector: ".MuiChip-label" })).toBeInTheDocument();
    expect(screen.getByText(/external account is locked/i)).toBeInTheDocument();
  });

  it("renders 'Not found' without a lock chip when the external account doesn't exist", () => {
    mockQueryResult({
      data: { ...BLOCKED_EXTERNAL_USER, externalAccount: { exists: false, locked: null } },
    });
    renderPage();
    expect(screen.getByText("Not found", { selector: ".MuiChip-label" })).toBeInTheDocument();
    expect(screen.queryByText("Locked", { selector: ".MuiChip-label" })).not.toBeInTheDocument();
    expect(screen.queryByText("Unlocked", { selector: ".MuiChip-label" })).not.toBeInTheDocument();
  });

  it("renders 'Unavailable' rather than a false 'Not found' when the SCIM lookup itself failed", () => {
    mockQueryResult({ data: { ...BLOCKED_EXTERNAL_USER, externalAccount: undefined } });
    renderPage();
    expect(screen.getByText("Unavailable")).toBeInTheDocument();
  });

  it("does not render an External account field for an internal user", () => {
    mockQueryResult({ data: INTERNAL_USER });
    renderPage();
    expect(screen.queryByText("External account")).not.toBeInTheDocument();
  });

  // A wso2.com contact can be tagged with a customer-facing userType/role in
  // ServiceNow (e.g. for testing) despite never being able to exist in the
  // SCIM "external" org, which is reserved for WSO2 staff. The field/alert
  // must stay hidden even when externalAccount data is present.
  it("does not render the External account field or locked alert for a wso2.com email, even if externalAccount data is present", () => {
    mockQueryResult({
      data: {
        ...BLOCKED_EXTERNAL_USER,
        email: "tester@wso2.com",
        externalAccount: { exists: true, locked: true },
      },
    });
    renderPage();
    expect(screen.queryByText("External account")).not.toBeInTheDocument();
    expect(screen.queryByText(/external account is locked/i)).not.toBeInTheDocument();
  });

  it("falls back to browser history when no origin was captured (e.g. a bookmarked/direct link)", () => {
    mockQueryResult({ data: INTERNAL_USER });
    // Two history entries (unlike renderPage's single-entry default) so
    // `navigate(-1)` has something real to pop back to — verified through
    // the actual router, not a mocked navigate function.
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter
          initialEntries={["/admin/users", "/people/user-1"]}
          initialIndex={1}
        >
          <Routes>
            <Route path="/people/:id" element={<UserProfilePage />} />
            <Route path="/admin/users" element={<LocationProbe />} />
          </Routes>
        </MemoryRouter>
      </QueryClientProvider>,
    );

    fireEvent.click(screen.getByRole("button", { name: "Back" }));

    expect(screen.getByTestId("location-probe")).toHaveTextContent("/admin/users");
  });

  it("returns to the captured origin (e.g. a dashboard widget or an admin users list) when one is known", () => {
    mockQueryResult({ data: INTERNAL_USER });
    renderPage({ from: "/admin/users" });

    fireEvent.click(screen.getByRole("button", { name: "Back" }));

    expect(screen.getByTestId("location-probe")).toHaveTextContent("/admin/users");
  });

  it("restores the users list's own dashboard-return state after a round trip (dashboard → users → profile → users → dashboard)", () => {
    mockQueryResult({ data: INTERNAL_USER });
    renderPage({ from: "/admin/users", parentState: { from: "/dashboard" } });

    fireEvent.click(screen.getByRole("button", { name: "Back" }));

    expect(screen.getByTestId("location-probe")).toHaveTextContent("/admin/users");
    expect(screen.getByTestId("location-state-probe")).toHaveTextContent(
      JSON.stringify({ from: "/dashboard" }),
    );
  });
});
