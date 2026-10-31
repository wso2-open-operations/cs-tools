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

import { render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

vi.mock("@config/apiConfig", () => ({ apiConfig: { backendUrl: "https://example.test" } }));

const access = { isAdmin: false };
vi.mock("@context/current-user/usePortalAccess", () => ({ usePortalAccess: () => access }));
vi.mock("@context/current-user/CurrentUserContext", () => ({ useCurrentUser: () => ({ user: undefined }) }));
vi.mock("@features/csm-admin/hooks/useTeamEntityId", () => ({
  useTeamEntityId: () => ({ teamEntityId: undefined, isLoading: false }),
}));
vi.mock("@features/csm-admin/api/useTeamMembers", () => ({ useGetTeamMembers: () => ({ data: undefined }) }));
vi.mock("@features/csm-admin/components/DirectoryMemberPage", () => ({ default: () => <div>member list</div> }));
vi.mock("@features/csm-admin/components/TeamMembersManageCard", () => ({
  default: (p: { teamKey: string; isAdmin: boolean }) => <div>manage {p.teamKey}</div>,
}));

import TeamMembersPage from "@features/csm-admin/pages/TeamMembersPage";

function renderPage() {
  render(
    <MemoryRouter initialEntries={["/admin/teams/vega"]}>
      <Routes>
        <Route path="/admin/teams/:id" element={<TeamMembersPage />} />
      </Routes>
    </MemoryRouter>,
  );
}

beforeEach(() => {
  access.isAdmin = false;
});

describe("TeamMembersPage Manage members", () => {
  it("is not rendered for a non-admin, who still sees the member list", () => {
    renderPage();
    expect(screen.getByText("member list")).toBeInTheDocument();
    expect(screen.queryByText(/^manage /)).not.toBeInTheDocument();
  });

  it("is rendered for an admin with the team's registry key from the route", () => {
    access.isAdmin = true;
    renderPage();
    expect(screen.getByText("manage vega")).toBeInTheDocument();
    expect(screen.getByText("member list")).toBeInTheDocument();
  });
});
