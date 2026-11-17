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

import { render, waitFor } from "@testing-library/react";
import { describe, expect, it, vi, beforeEach } from "vitest";
import "@testing-library/jest-dom/vitest";
import { MemoryRouter, Route, Routes } from "react-router";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactElement } from "react";

const authFetchMock = vi.fn();

vi.mock("@config/apiConfig", () => ({
  apiConfig: { backendUrl: "https://example.test" },
}));
vi.mock("@hooks/useAuthApiClient", () => ({
  useAuthApiClient: () => authFetchMock,
}));
// useSearchRoles (for role display names) goes through this client; give
// `post` a real resolved value so react-query doesn't warn about an
// undefined query result.
const backendPostMock = vi.fn().mockResolvedValue({ roles: [], total: 0, limit: 50, offset: 0 });
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

vi.mock("@context/current-user/CurrentUserContext", () => ({
  useCurrentUser: () => ({ user: { id: "user-1", email: "jane.doe@example.com" }, isLoading: false, isError: false }),
}));

import RoleMembersPage from "@features/csm-admin/pages/RoleMembersPage";
import GroupMembersPage from "@features/csm-admin/pages/GroupMembersPage";
import TeamMembersPage from "@features/csm-admin/pages/TeamMembersPage";

function jsonResponse(body: unknown): Response {
  return {
    ok: true,
    status: 200,
    statusText: "OK",
    json: async () => body,
    text: async () => JSON.stringify(body),
  } as unknown as Response;
}

function renderAt(
  path: string,
  routePath: string,
  element: ReactElement,
): ReturnType<typeof render> {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[path]}>
        <Routes>
          <Route path={routePath} element={element} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

/**
 * These exercise the actual routed page components end to end (URL -> page
 * -> DirectoryMemberPage -> DirectoryMembersList -> the request body), not
 * just DirectoryMembersList in isolation with hand-picked props. That
 * distinction matters here: DirectoryMembersList.test.tsx proves the shared
 * component sends whatever `filterKey` it's given, but a thin per-entity
 * page passing the *wrong* `filterKey` (e.g. TeamMembersPage wired to
 * `groupIds` instead of `teamIds`) would still pass that test — the bug is
 * only visible by rendering the real page component, as done below.
 */
describe("role/group/team member pages send the correct filter key", () => {
  beforeEach(() => {
    authFetchMock.mockReset();
    authFetchMock.mockResolvedValue(
      jsonResponse({ users: [], total: 0, limit: 20, offset: 0 }),
    );
  });

  it("RoleMembersPage filters users/search by roleIds", async () => {
    renderAt("/admin/roles/agent", "/admin/roles/:id", <RoleMembersPage />);
    await waitFor(() => expect(authFetchMock).toHaveBeenCalled());
    const [, requestInit] = authFetchMock.mock.calls[0];
    expect(JSON.parse(requestInit.body as string).filters).toEqual({ roleIds: ["agent"] });
  });

  it("GroupMembersPage filters users/search by groupIds", async () => {
    const groupId = "11111111-1111-1111-1111-111111111111";
    renderAt(`/admin/groups/${groupId}`, "/admin/groups/:id", <GroupMembersPage />);
    await waitFor(() => expect(authFetchMock).toHaveBeenCalled());
    const [, requestInit] = authFetchMock.mock.calls[0];
    expect(JSON.parse(requestInit.body as string).filters).toEqual({ groupIds: [groupId] });
  });

  it("TeamMembersPage filters users/search by teamIds, not groupIds or roleIds", async () => {
    renderAt("/admin/teams/alpha", "/admin/teams/:id", <TeamMembersPage />);
    await waitFor(() => expect(authFetchMock).toHaveBeenCalled());
    const [, requestInit] = authFetchMock.mock.calls[0];
    expect(JSON.parse(requestInit.body as string).filters).toEqual({ teamIds: ["alpha"] });
  });
});
