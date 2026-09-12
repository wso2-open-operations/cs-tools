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

import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { describe, expect, it, vi, beforeEach } from "vitest";
import "@testing-library/jest-dom/vitest";
import { MemoryRouter, Route, Routes, useLocation } from "react-router";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";

const authFetchMock = vi.fn();
const postMock = vi.fn();

// useSearchUsers (csm-users/api) is on the older useAuthApiClient + apiConfig
// convention; useSearchRoles / useSearchTeams / the group picker's
// useSearchGroups are all on useBackendApi. Both read runtime config at
// module load, which isn't present under vitest, so stub both (same approach
// as useAccountProjects.test.tsx / useQuickCaseSearch.test.tsx).
vi.mock("@config/apiConfig", () => ({
  apiConfig: { backendUrl: "https://example.test" },
}));
vi.mock("@hooks/useAuthApiClient", () => ({
  useAuthApiClient: () => authFetchMock,
}));
vi.mock("@api/backend/client", () => ({
  BackendApiError: class BackendApiError extends Error {
    status: number;
    constructor(status: number, message: string) {
      super(message);
      this.status = status;
    }
  },
  useBackendApi: () => ({ post: postMock }),
}));

import CsmUsersPage from "@features/csm-users/pages/CsmUsersPage";

function jsonResponse(body: unknown): Response {
  return {
    ok: true,
    status: 200,
    statusText: "OK",
    json: async () => body,
    text: async () => JSON.stringify(body),
  } as unknown as Response;
}

function renderPage(initialPath: string): ReturnType<typeof render> {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[initialPath]}>
        <CsmUsersPage />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

/**
 * Same as {@link renderPage}, plus marker routes for the destinations a row
 * or a role chip can navigate to — used to assert *which* route a click
 * actually lands on, not just that some navigation happened.
 */
function renderPageWithDestinations(
  initialPath: string,
): ReturnType<typeof render> {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[initialPath]}>
        <Routes>
          <Route path="/admin/users" element={<CsmUsersPage />} />
          <Route path="/admin/roles/:id" element={<div>Role members page</div>} />
          <Route path="/people/:id" element={<div>User profile page</div>} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

/** Destination probe: renders wherever a navigation actually lands, showing
 * both the resulting path and the location.state that came with it — so
 * tests assert on real router navigation, not a static marker or a mocked
 * navigate function. */
function LocationProbe() {
  const location = useLocation();
  return (
    <>
      <div data-testid="location-probe">{location.pathname + location.search}</div>
      <div data-testid="location-state-probe">{JSON.stringify(location.state ?? null)}</div>
    </>
  );
}

/**
 * Same as {@link renderPageWithDestinations}, but `/people/:id` and
 * `/dashboard` both render {@link LocationProbe} instead of a static marker
 * — used to assert a navigation actually carries the right `location.state`
 * forward, not just that it landed on the right page.
 */
function renderPageWithLocationProbe(
  initialPath: string | { pathname: string; state?: unknown },
): ReturnType<typeof render> {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[initialPath]}>
        <Routes>
          <Route path="/admin/users" element={<CsmUsersPage />} />
          <Route path="/people/:id" element={<LocationProbe />} />
          <Route path="/dashboard" element={<LocationProbe />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("CsmUsersPage", () => {
  beforeEach(() => {
    authFetchMock.mockReset();
    postMock.mockReset();
    postMock.mockImplementation((path: string) => {
      if (path === "/roles/search") {
        return Promise.resolve({
          roles: [{ id: "agent", name: "Agent" }],
          total: 1,
          limit: 50,
          offset: 0,
        });
      }
      if (path === "/teams/search") {
        return Promise.resolve({
          teams: [{ id: "alpha", name: "Alpha" }],
          total: 1,
          limit: 50,
          offset: 0,
        });
      }
      return Promise.resolve({ groups: [], total: 0, limit: 20, offset: 0 });
    });
    authFetchMock.mockResolvedValue(
      jsonResponse({ users: [], total: 0, limit: 20, offset: 0, hasMore: false }),
    );
  });

  it("combines name/email search, role, group, team and status into one request with every key set", async () => {
    renderPage(
      "/admin/users?search=jane&roles=agent&groups=11111111-1111-1111-1111-111111111111&teams=alpha&active=active",
    );

    await waitFor(() => expect(authFetchMock).toHaveBeenCalled());

    // All filters land on a single /users/search call, combined (AND'd)
    // server-side — not split across separate requests per filter.
    expect(authFetchMock).toHaveBeenCalledTimes(1);
    const [url, requestInit] = authFetchMock.mock.calls[0];
    expect(url).toContain("/users/search");
    const body = JSON.parse(requestInit.body as string);
    expect(body.filters).toEqual({
      searchQuery: "jane",
      roleIds: ["agent"],
      groupIds: ["11111111-1111-1111-1111-111111111111"],
      teamIds: ["alpha"],
      active: true,
    });
  });

  it("omits every filter key when nothing is selected", async () => {
    renderPage("/admin/users");

    await waitFor(() => expect(authFetchMock).toHaveBeenCalled());
    const [, requestInit] = authFetchMock.mock.calls[0];
    const body = JSON.parse(requestInit.body as string);
    expect(body.filters).toEqual({});
  });

  it("clears selected role and team filters from their controls", async () => {
    renderPage("/admin/users?roles=agent&teams=alpha");
    await waitFor(() => expect(authFetchMock).toHaveBeenCalled());

    fireEvent.click(screen.getByRole("button", { name: "Clear roles filter" }));
    await waitFor(() => {
      const body = JSON.parse(authFetchMock.mock.calls.at(-1)?.[1].body as string);
      expect(body.filters.roleIds).toBeUndefined();
      expect(body.filters.teamIds).toEqual(["alpha"]);
    });

    fireEvent.click(screen.getByRole("button", { name: "Clear teams filter" }));
    await waitFor(() => {
      const body = JSON.parse(authFetchMock.mock.calls.at(-1)?.[1].body as string);
      expect(body.filters.teamIds).toBeUndefined();
    });
  });
});

describe("CsmUsersPage — role truncation and row navigation", () => {
  const MANY_ROLES_USER = {
    id: "user-1",
    userName: "jane.doe",
    name: "Jane Doe",
    email: "jane.doe@example.com",
    active: true,
    roles: ["agent", "admin", "commenter", "partner", "customer_admin"],
    createdOn: "2025-01-01T00:00:00Z",
    updatedOn: "2025-06-01T00:00:00Z",
  };
  const FEW_ROLES_USER = {
    id: "user-2",
    userName: "john.smith",
    name: "John Smith",
    email: "john.smith@example.com",
    active: true,
    roles: ["snc_internal", "admin"],
    createdOn: "2025-01-01T00:00:00Z",
    updatedOn: "2025-06-01T00:00:00Z",
  };

  beforeEach(() => {
    authFetchMock.mockReset();
    postMock.mockReset();
    postMock.mockImplementation((path: string) => {
      if (path === "/roles/search") {
        return Promise.resolve({
          roles: [
            { id: "agent", name: "Agent" },
            { id: "admin", name: "Admin" },
            { id: "commenter", name: "Commenter" },
            { id: "partner", name: "Partner" },
            { id: "customer_admin", name: "Customer Admin" },
            { id: "internal", name: "Internal" },
          ],
          total: 5,
          limit: 50,
          offset: 0,
        });
      }
      if (path === "/teams/search") {
        return Promise.resolve({ teams: [], total: 0, limit: 50, offset: 0 });
      }
      return Promise.resolve({ groups: [], total: 0, limit: 20, offset: 0 });
    });
    authFetchMock.mockResolvedValue(
      jsonResponse({
        users: [MANY_ROLES_USER, FEW_ROLES_USER],
        total: 2,
        limit: 20,
        offset: 0,
      }),
    );
  });

  it("keeps roles on one line and provides a more chip for roles that do not fit", async () => {
    renderPage("/admin/users");

    // Wait until the role-name catalogue has resolved the raw keys before asserting.
    await waitFor(() => expect(screen.getAllByText("Agent").length).toBeGreaterThan(0));

    const janeRow = screen.getByText("Jane Doe").closest("tr") as HTMLElement;
    const johnRow = screen.getByText("John Smith").closest("tr") as HTMLElement;
    const janeRoles = within(janeRow).getByTestId("role-measure").previousElementSibling as HTMLElement;
    const johnRoles = within(johnRow).getByTestId("role-measure").previousElementSibling as HTMLElement;

    // jsdom has no layout width, so the responsive list uses its safe
    // one-chip fallback and exposes the remainder through a legible chip.
    expect(within(janeRoles).getByText("Agent")).toBeInTheDocument();
    expect(within(janeRoles).getByText("+4 more")).toBeInTheDocument();
    expect(within(janeRoles).queryByText("Admin")).not.toBeInTheDocument();
    expect(within(janeRoles).queryByText("Commenter")).not.toBeInTheDocument();
    expect(within(janeRoles).queryByText("Partner")).not.toBeInTheDocument();
    expect(within(janeRoles).queryByText("Customer Admin")).not.toBeInTheDocument();

    // The same fallback remains a single line for a shorter role list.
    // Fully-qualified ServiceNow keys resolve through the same short-key
    // catalogue used by the role filter.
    expect(within(johnRoles).getByText("Internal")).toBeInTheDocument();
    expect(within(johnRoles).queryByText("Admin")).not.toBeInTheDocument();
    expect(within(johnRoles).getByText("+1 more")).toBeInTheDocument();
  });

  it("treats role chips as row content and navigates their row to the user profile", async () => {
    renderPageWithDestinations("/admin/users");

    await waitFor(() => expect(screen.getAllByText("Agent").length).toBeGreaterThan(0));
    const janeRow = screen.getByText("Jane Doe").closest("tr") as HTMLElement;
    const janeRoles = within(janeRow).getByTestId("role-measure").previousElementSibling as HTMLElement;

    // Role chips are informational in this table; clicking one follows the
    // containing row to the user rather than opening the role directory.
    fireEvent.click(within(janeRoles).getByText("Agent"));
    expect(await screen.findByText("User profile page")).toBeInTheDocument();
    expect(screen.queryByText("Role members page")).not.toBeInTheDocument();
  });

  it("navigates a whole-row click (outside any nested chip/link) to the user's profile", async () => {
    renderPageWithDestinations("/admin/users");

    await waitFor(() => expect(screen.getByText("John Smith")).toBeInTheDocument());

    // Click the timezone cell — plain text, not a nested interactive element.
    const row = screen.getByText("John Smith").closest("tr");
    expect(row).not.toBeNull();
    fireEvent.click(row as HTMLElement);
    expect(await screen.findByText("User profile page")).toBeInTheDocument();
  });

  it("is keyboard-activatable: Enter on a focused row navigates to the profile", async () => {
    renderPageWithDestinations("/admin/users");

    await waitFor(() => expect(screen.getByText("John Smith")).toBeInTheDocument());
    const row = screen.getByText("John Smith").closest("tr") as HTMLElement;
    row.focus();
    fireEvent.keyDown(row, { key: "Enter" });
    expect(await screen.findByText("User profile page")).toBeInTheDocument();
  });

  it("carries this page's own URL forward as `from` when a row navigates to a profile", async () => {
    renderPageWithLocationProbe("/admin/users");

    await waitFor(() => expect(screen.getByText("John Smith")).toBeInTheDocument());
    fireEvent.click(screen.getByText("John Smith").closest("tr") as HTMLElement);

    expect(await screen.findByTestId("location-state-probe")).toHaveTextContent(
      JSON.stringify({ from: "/admin/users", parentState: null }),
    );
  });

  it("shows 'Locked out' in the status column instead of 'Active', for a locked-out user even though they're active", async () => {
    authFetchMock.mockResolvedValue(
      jsonResponse({
        users: [{ ...FEW_ROLES_USER, active: true, lockedOut: true }],
        total: 1,
        limit: 20,
        offset: 0,
      }),
    );
    renderPage("/admin/users");

    await waitFor(() => expect(screen.getByText("John Smith")).toBeInTheDocument());
    const row = screen.getByText("John Smith").closest("tr") as HTMLElement;
    expect(within(row).getByText("Locked out")).toBeInTheDocument();
    expect(within(row).queryByText("Active")).not.toBeInTheDocument();
  });

  // This page no longer renders its own Back button -- `CsmAdminLayout`
  // (the parent route shell every `/admin/user-management/*` page is nested
  // under) sees this exact same `location.state` and is the single Back
  // button for every page it wraps (see `CsmAdminLayout.test.tsx` for that
  // behavior). This page still reads `location.state.from` itself, purely to
  // forward it as `parentState` when a row navigates to a person's profile,
  // so a dashboard → users → profile → Back → Back round trip restores
  // correctly instead of losing the dashboard origin partway through.
  it("carries its own dashboard-return state forward as `parentState` when a row navigates to a profile", async () => {
    renderPageWithLocationProbe({ pathname: "/admin/users", state: { from: "/dashboard" } });

    await waitFor(() => expect(screen.getByText("John Smith")).toBeInTheDocument());
    fireEvent.click(screen.getByText("John Smith").closest("tr") as HTMLElement);
    expect(await screen.findByTestId("location-state-probe")).toHaveTextContent(
      JSON.stringify({ from: "/admin/users", parentState: { from: "/dashboard" } }),
    );
  });
});
