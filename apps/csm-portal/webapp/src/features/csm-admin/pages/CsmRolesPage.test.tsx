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
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import { MemoryRouter } from "react-router";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";

const postMock = vi.fn();

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

import CsmRolesPage from "@features/csm-admin/pages/CsmRolesPage";

function renderPage(): ReturnType<typeof render> {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>
        <CsmRolesPage />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("CsmRolesPage", () => {
  it("renders the role catalogue and links each row to its member page", async () => {
    postMock.mockResolvedValue({
      roles: [
        { id: "agent", name: "Agent" },
        { id: "admin", name: "Admin" },
      ],
      total: 2,
      limit: 20,
      offset: 0,
    });
    renderPage();

    // DirectoryEntityTable renders each row as a full-cell "stretched click"
    // link (an absolutely-positioned, visually empty anchor over the row) so
    // the whole row is click-through; its accessible name is the deliberately
    // descriptive `aria-label="View members of {name}"`, not the bare row
    // name, since a screen-reader user tabbing through the row-level anchors
    // needs to know what activating one does.
    const agentLink = await screen.findByRole("link", {
      name: "View members of Agent",
    });
    expect(agentLink).toHaveAttribute("href", "/admin/roles/agent");
    const adminLink = await screen.findByRole("link", {
      name: "View members of Admin",
    });
    expect(adminLink).toHaveAttribute("href", "/admin/roles/admin");
  });

  it("renders an empty state rather than an empty table when there are no matches", async () => {
    postMock.mockResolvedValue({ roles: [], total: 0, limit: 20, offset: 0 });
    renderPage();

    expect(await screen.findByText(/No roles found/i)).toBeInTheDocument();
  });
});
