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

import type { ReactElement } from "react";
import { fireEvent, render as rtlRender, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { MemoryRouter } from "react-router";
import "@testing-library/jest-dom/vitest";
import CsmAccountsPage from "@features/csm-accounts/pages/CsmAccountsPage";
import { useSearchAccounts } from "@features/csm-accounts/api/useSearchAccounts";
import type { Account } from "@features/csm-accounts/types/csmAccounts";

// The backend client reads runtime config at module load, which isn't
// present under vitest — same approach as CsmAnnouncementsPage.test.tsx.
vi.mock("@api/backend/client", () => ({
  BackendApiError: class BackendApiError extends Error {},
  useBackendApi: () => ({ post: vi.fn() }),
}));

vi.mock("@features/csm-accounts/api/useSearchAccounts", () => ({
  useSearchAccounts: vi.fn(),
}));

vi.mock("@context/current-user/CurrentUserContext", () => ({
  useCurrentUser: () => ({ user: { id: "user-1" }, isLoading: false, isError: false }),
}));
vi.mock("@hooks/useIdTokenClaims", () => ({
  useIdTokenClaims: () => ({ email: "user@example.test" }),
}));

const mockedUseSearch = vi.mocked(useSearchAccounts);
// Name, SF ID, Tier, Region, Activated, Deactivated, Account manager,
// Technical owner, CRE team.
const ACCOUNT_COLUMN_COUNT = 9;

function render(ui: ReactElement): ReturnType<typeof rtlRender> {
  return rtlRender(<MemoryRouter>{ui}</MemoryRouter>);
}

const ACCOUNT: Account = {
  id: "acct-1",
  sfId: "SF-1001",
  name: "Acme Corp",
  tier: "enterprise",
  region: "APAC",
  activationDate: "2024-01-01T00:00:00Z",
  deactivationDate: null,
  ownerId: "owner-1",
  accountManager: { id: "u-1", name: "Jane Doe" },
  technicalOwner: { id: "u-2", name: "John Roe" },
  creTeam: { id: "team-1", name: "APAC CRE" },
  hasAgent: true,
  hasKbReferences: true,
  createdOn: "2024-01-01T00:00:00Z",
  updatedOn: "2024-01-01T00:00:00Z",
};

function mockResult(overrides: Partial<ReturnType<typeof useSearchAccounts>>): void {
  mockedUseSearch.mockReturnValue({
    data: undefined,
    isLoading: false,
    isFetching: false,
    isError: false,
    error: null,
    refetch: vi.fn(),
    dataUpdatedAt: 0,
    ...overrides,
  } as unknown as ReturnType<typeof useSearchAccounts>);
}

beforeEach(() => {
  mockedUseSearch.mockReset();
  window.localStorage.clear();
});

describe("CsmAccountsPage — list states", () => {
  it("renders a row from the search result", () => {
    mockResult({ data: { accounts: [ACCOUNT], total: 1, limit: 20, offset: 0, hasMore: false } });
    render(<CsmAccountsPage />);
    expect(screen.getByText("Acme Corp")).toBeInTheDocument();
    expect(screen.getByText("SF-1001")).toBeInTheDocument();
  });

  it("shows the empty state when there are no accounts", () => {
    mockResult({ data: { accounts: [], total: 0, limit: 20, offset: 0, hasMore: false } });
    render(<CsmAccountsPage />);
    expect(screen.getByText(/no accounts found/i)).toBeInTheDocument();
  });
});

describe("CsmAccountsPage — customise columns", () => {
  it("shows the default columns and hides Account manager until added", () => {
    mockResult({ data: { accounts: [ACCOUNT], total: 1, limit: 20, offset: 0, hasMore: false } });
    render(<CsmAccountsPage />);

    expect(screen.getByRole("columnheader", { name: "Name" })).toBeInTheDocument();
    expect(screen.getByRole("columnheader", { name: "Deactivated" })).toBeInTheDocument();
    expect(
      screen.queryByRole("columnheader", { name: "Account manager" }),
    ).not.toBeInTheDocument();
  });

  it("adds the Account manager column when checked, rendering the row's owner name", () => {
    mockResult({ data: { accounts: [ACCOUNT], total: 1, limit: 20, offset: 0, hasMore: false } });
    render(<CsmAccountsPage />);

    fireEvent.click(screen.getByRole("button", { name: "Customise accounts columns" }));
    // Column order: Name, SF ID, Tier, Region, Activated, Deactivated,
    // Account manager (7th), Technical owner, CRE team.
    const checkboxes = screen.getAllByRole("checkbox");
    fireEvent.click(checkboxes[6]);

    expect(
      screen.getByRole("columnheader", { name: "Account manager", hidden: true }),
    ).toBeInTheDocument();
    expect(screen.getByText("Jane Doe")).toBeInTheDocument();
  });

  it("never lets every column be unchecked", () => {
    mockResult({ data: { accounts: [ACCOUNT], total: 1, limit: 20, offset: 0, hasMore: false } });
    render(<CsmAccountsPage />);

    fireEvent.click(screen.getByRole("button", { name: "Customise accounts columns" }));
    for (let i = 0; i < ACCOUNT_COLUMN_COUNT; i++) {
      const checkbox = screen.getAllByRole("checkbox")[i];
      if (!checkbox.hasAttribute("disabled") && (checkbox as HTMLInputElement).checked) {
        fireEvent.click(checkbox);
      }
    }

    expect(screen.getAllByRole("columnheader", { hidden: true }).length).toBeGreaterThan(0);
  });

  it("persists a toggled column across a remount for the same user", () => {
    mockResult({ data: { accounts: [ACCOUNT], total: 1, limit: 20, offset: 0, hasMore: false } });
    const { unmount } = render(<CsmAccountsPage />);

    fireEvent.click(screen.getByRole("button", { name: "Customise accounts columns" }));
    fireEvent.click(screen.getAllByRole("checkbox")[6]); // Account manager
    unmount();

    render(<CsmAccountsPage />);
    expect(screen.getByRole("columnheader", { name: "Account manager" })).toBeInTheDocument();
  });
});
