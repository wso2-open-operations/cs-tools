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
import { describe, expect, it, vi } from "vitest";
import type { JSX, ReactElement } from "react";
import { MemoryRouter, Route, Routes, useLocation } from "react-router";
import "@testing-library/jest-dom/vitest";
import type { UseQueryResult } from "@tanstack/react-query";
import type { Account } from "@features/csm-accounts/types/csmAccounts";

const useGetAccountMock = vi.fn();

// `QueryErrorState` (imported by the page for its error state) pulls in
// `@api/backend/client` -> `useAuthApiClient` -> `@config/apiConfig`, which
// throws at module load when `window.config` isn't set — not present under
// vitest. Same stub other page tests use (e.g. `useAccountProjects.test.tsx`).
vi.mock("@config/apiConfig", () => ({
  apiConfig: { backendUrl: "https://example.test" },
}));

vi.mock("@features/csm-accounts/api/useGetAccount", () => ({
  useGetAccount: () => useGetAccountMock(),
}));
vi.mock("@features/csm-accounts/api/useAccountProjects", () => ({
  useAccountProjects: () => ({
    data: { projects: [] },
    isLoading: false,
    isError: false,
    error: null,
  }),
}));

// Imported after the mocks above so the module picks them up.
import CsmAccountDetailPage from "@features/csm-accounts/pages/CsmAccountDetailPage";

const BASE_ACCOUNT: Account = {
  id: "acct-1",
  sfId: "sf-1",
  name: "Acme Corp",
  tier: "enterprise",
  region: "US",
  activationDate: "2026-01-01T00:00:00Z",
  ownerId: "owner-1",
  hasAgent: true,
  hasKbReferences: false,
  createdOn: "2026-01-01T00:00:00Z",
  updatedOn: "2026-01-01T00:00:00Z",
};

function mockAccount(overrides: Partial<UseQueryResult<Account | null, Error>>): void {
  useGetAccountMock.mockReturnValue({
    data: null,
    isLoading: false,
    isError: false,
    error: null,
    ...overrides,
  });
}

// Renders `path`'s current location as plain text, so a test can assert a
// link actually navigated (not just that a href/route prop is present)
// without mocking `useNavigate`/`useNavTransition` — same convention as
// `CaseDetailWidgets.test.tsx`'s `renderWithRoutes`.
function LocationProbe(): JSX.Element {
  const location = useLocation();
  return <div data-testid="location-probe">{location.pathname}</div>;
}

function renderPage(ui: ReactElement, extraRoutes: string[] = []): ReturnType<typeof render> {
  return render(
    <MemoryRouter initialEntries={["/customers/accounts/acct-1"]}>
      <Routes>
        <Route path="/customers/accounts/:id" element={ui} />
        {extraRoutes.map((path) => (
          <Route key={path} path={path} element={<LocationProbe />} />
        ))}
      </Routes>
    </MemoryRouter>,
  );
}

describe("CsmAccountDetailPage", () => {
  it("renders no CRE/SRE team cell when neither is set", () => {
    mockAccount({ data: BASE_ACCOUNT });
    renderPage(<CsmAccountDetailPage />);
    expect(screen.queryByText("CRE / SRE team")).not.toBeInTheDocument();
  });

  it("renders CRE and SRE team chips linking to the team directory page", () => {
    mockAccount({
      data: {
        ...BASE_ACCOUNT,
        creTeam: { id: "team-cre-1", name: "CRE Alpha" },
        sreTeam: { id: "team-sre-1", name: "SRE Beta" },
      },
    });
    renderPage(<CsmAccountDetailPage />, ["/admin/teams/:id"]);

    expect(screen.getByText("CRE / SRE team")).toBeInTheDocument();
    expect(screen.getByText("CRE Alpha")).toBeInTheDocument();
    expect(screen.getByText("SRE Beta")).toBeInTheDocument();

    fireEvent.click(screen.getByText("CRE Alpha"));
    expect(screen.getByTestId("location-probe")).toHaveTextContent(
      "/admin/teams/team-cre-1",
    );
  });

  it("renders only the SRE chip when only the SRE team is set", () => {
    mockAccount({
      data: { ...BASE_ACCOUNT, sreTeam: { id: "team-sre-1", name: "SRE Beta" } },
    });
    renderPage(<CsmAccountDetailPage />);

    expect(screen.getByText("CRE / SRE team")).toBeInTheDocument();
    expect(screen.getByText("SRE Beta")).toBeInTheDocument();
    expect(screen.queryByText("CRE Alpha")).not.toBeInTheDocument();
  });
});
