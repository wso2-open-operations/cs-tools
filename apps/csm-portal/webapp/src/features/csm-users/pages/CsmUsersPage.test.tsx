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

// usePortalAccess (which the Add User gating reads) derives from
// useCurrentUser's roles -- mocked the same way CsmAdminLayout.test.tsx does,
// with a mutable roles list a test can set before rendering. Defaults to no
// roles, matching every pre-existing test in this file (no CurrentUserProvider
// in their render tree previously either -- usePortalAccess's own doc comment
// says it reports no access rather than throwing in that case).
let mockRoles: string[] | undefined;
vi.mock("@context/current-user/CurrentUserContext", () => ({
  useCurrentUser: () => ({ user: { roles: mockRoles }, isLoading: false, isError: false }),
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
 * Same as {@link renderPage}, plus a marker route for where a row navigates
 * to — used to assert *which* route a click actually lands on, not just that
 * some navigation happened.
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
    mockRoles = undefined;
    authFetchMock.mockReset();
    postMock.mockReset();
    postMock.mockImplementation((path: string) => {
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

  it("combines name/email search, group, team and status into one request with every key set", async () => {
    renderPage(
      "/admin/users?search=jane&groups=11111111-1111-1111-1111-111111111111&teams=alpha&active=active",
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

  it("clears the selected team filter from its control", async () => {
    renderPage("/admin/users?teams=alpha");
    await waitFor(() => expect(authFetchMock).toHaveBeenCalled());

    fireEvent.click(screen.getByRole("button", { name: "Clear teams filter" }));
    await waitFor(() => {
      const body = JSON.parse(authFetchMock.mock.calls.at(-1)?.[1].body as string);
      expect(body.filters.teamIds).toBeUndefined();
    });
  });
});

describe("CsmUsersPage — type column and row navigation", () => {
  const INTERNAL_USER = {
    id: "user-1",
    userName: "jane.doe",
    name: "Jane Doe",
    email: "jane.doe@example.com",
    active: true,
    userType: "internal",
    createdOn: "2025-01-01T00:00:00Z",
    updatedOn: "2025-06-01T00:00:00Z",
  };
  const FEW_ROLES_USER = {
    id: "user-2",
    userName: "john.smith",
    name: "John Smith",
    email: "john.smith@example.com",
    active: true,
    userType: "customer",
    createdOn: "2025-01-01T00:00:00Z",
    updatedOn: "2025-06-01T00:00:00Z",
  };

  beforeEach(() => {
    mockRoles = undefined;
    authFetchMock.mockReset();
    postMock.mockReset();
    postMock.mockImplementation((path: string) => {
      if (path === "/teams/search") {
        return Promise.resolve({ teams: [], total: 0, limit: 50, offset: 0 });
      }
      return Promise.resolve({ groups: [], total: 0, limit: 20, offset: 0 });
    });
    authFetchMock.mockResolvedValue(
      jsonResponse({
        users: [INTERNAL_USER, FEW_ROLES_USER],
        total: 2,
        limit: 20,
        offset: 0,
      }),
    );
  });

  it("shows the user's type instead of a roles column", async () => {
    renderPage("/admin/users");

    await waitFor(() => expect(screen.getByText("Jane Doe")).toBeInTheDocument());
    const janeRow = screen.getByText("Jane Doe").closest("tr") as HTMLElement;
    const johnRow = screen.getByText("John Smith").closest("tr") as HTMLElement;

    expect(within(janeRow).getByText("Internal")).toBeInTheDocument();
    expect(within(johnRow).getByText("External (customer)")).toBeInTheDocument();
    expect(screen.queryByText("Roles")).not.toBeInTheDocument();
    expect(screen.getByText("Type")).toBeInTheDocument();
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

describe("CsmUsersPage — Add User (admin only)", () => {
  beforeEach(() => {
    mockRoles = undefined;
    authFetchMock.mockReset();
    postMock.mockReset();
    postMock.mockImplementation((path: string) => {
      if (path === "/teams/search") {
        return Promise.resolve({ teams: [], total: 0, limit: 50, offset: 0 });
      }
      if (path === "/users") {
        return Promise.resolve({ id: "new-user-1", email: "new.user@example.com" });
      }
      return Promise.resolve({ groups: [], total: 0, limit: 20, offset: 0 });
    });
    authFetchMock.mockResolvedValue(
      jsonResponse({ users: [], total: 0, limit: 20, offset: 0, hasMore: false }),
    );
  });

  it("is hidden for a caller with no admin role", async () => {
    mockRoles = ["cs_engineer"];
    renderPage("/admin/users");
    await waitFor(() => expect(authFetchMock).toHaveBeenCalled());
    expect(screen.queryByRole("button", { name: "Add user" })).not.toBeInTheDocument();
  });

  it("is hidden with no roles at all", async () => {
    renderPage("/admin/users");
    await waitFor(() => expect(authFetchMock).toHaveBeenCalled());
    expect(screen.queryByRole("button", { name: "Add user" })).not.toBeInTheDocument();
  });

  it("is shown for an admin, opens the form, and creates a user on submit", async () => {
    mockRoles = ["admin"];
    renderPage("/admin/users");

    const addButton = await screen.findByRole("button", { name: "Add user" });
    fireEvent.click(addButton);

    expect(await screen.findByRole("heading", { name: "Add user" })).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText("First name"), { target: { value: "Jane" } });
    fireEvent.change(screen.getByLabelText(/^Email/), { target: { value: "new.user@example.com" } });

    const submitButton = screen.getByRole("button", { name: "Add user" });
    await waitFor(() => expect(submitButton).not.toBeDisabled());
    fireEvent.click(submitButton);

    await waitFor(() =>
      expect(postMock).toHaveBeenCalledWith(
        "/users",
        expect.objectContaining({ firstName: "Jane", email: "new.user@example.com" }),
      ),
    );
    await waitFor(() =>
      expect(screen.queryByRole("heading", { name: "Add user" })).not.toBeInTheDocument(),
    );
  });

  it("disables submit until at least a name and a plausible email are entered", async () => {
    mockRoles = ["admin"];
    renderPage("/admin/users");

    fireEvent.click(await screen.findByRole("button", { name: "Add user" }));
    await screen.findByRole("heading", { name: "Add user" });

    const submitButton = screen.getByRole("button", { name: "Add user" });
    expect(submitButton).toBeDisabled();

    fireEvent.change(screen.getByLabelText("Last name"), { target: { value: "Doe" } });
    expect(submitButton).toBeDisabled();

    fireEvent.change(screen.getByLabelText(/^Email/), { target: { value: "not-an-email" } });
    expect(submitButton).toBeDisabled();

    fireEvent.change(screen.getByLabelText(/^Email/), { target: { value: "jane.doe@example.com" } });
    expect(submitButton).not.toBeDisabled();
    expect(postMock).not.toHaveBeenCalledWith("/users", expect.anything());
  });
});
