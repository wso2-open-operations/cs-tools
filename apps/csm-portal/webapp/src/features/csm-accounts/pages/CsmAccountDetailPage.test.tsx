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

import { act, fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi, beforeEach } from "vitest";
import type { JSX, ReactElement } from "react";
import { MemoryRouter, Route, Routes, useLocation } from "react-router";
import "@testing-library/jest-dom/vitest";
import type { UseQueryResult } from "@tanstack/react-query";
import type { Account } from "@features/csm-accounts/types/csmAccounts";

const useGetAccountMock = vi.fn();
const patchMutateMock = vi.fn();
const patchResetMock = vi.fn();
const editAccountTeamsDialogMock = vi.fn();
const useGetUsersMeMock = vi.fn(() => ({ data: { roles: ["admin"] } }));
let patchIsPending = false;
let patchIsError = false;
let patchError: Error | null = null;

// `QueryErrorState` (imported by the page for its error state) pulls in
// `@api/backend/client` -> `useAuthApiClient` -> `@config/apiConfig`, which
// throws at module load when `window.config` isn't set — not present under
// vitest. Same stub other page tests use (e.g. `useAccountProjects.test.tsx`).
vi.mock("@config/apiConfig", () => ({
  apiConfig: { backendUrl: "https://example.test" },
}));

// The page imports `BackendApiError` directly to classify a failed save —
// stub it with a real class (so `instanceof` still works), same approach as
// `ProblemDetailPage.test.tsx`.
vi.mock("@api/backend/client", () => ({
  BackendApiError: class BackendApiError extends Error {
    status: number;
    constructor(status: number, message: string) {
      super(message);
      this.status = status;
    }
  },
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
vi.mock("@features/csm-accounts/api/usePatchAccountTeams", () => ({
  usePatchAccountTeams: () => ({
    mutate: patchMutateMock,
    reset: patchResetMock,
    isPending: patchIsPending,
    isError: patchIsError,
    error: patchError,
  }),
}));
// Exercised in isolation by its own test file; here we only assert this page
// opens it and wires the expected props/callbacks. Renders a real, findable
// element (rather than null) so a test can assert the dialog actually
// unmounts on close/success, not just that a callback fired.
vi.mock("@features/csm-accounts/components/EditAccountTeamsDialog", () => ({
  default: (props: unknown) => {
    editAccountTeamsDialogMock(props);
    return <div data-testid="edit-account-teams-dialog" />;
  },
}));
// Defaults to an admin caller so existing tests (written before the admin
// gate) keep exercising the edit affordance without every one needing to
// stub this out explicitly.
vi.mock("@features/settings/api/useGetUsersMe", () => ({
  useGetUsersMe: () => useGetUsersMeMock(),
}));

// Imported after the mocks above so the module picks them up.
import CsmAccountDetailPage from "@features/csm-accounts/pages/CsmAccountDetailPage";
import { BackendApiError } from "@api/backend/client";

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
  beforeEach(() => {
    patchMutateMock.mockReset();
    patchResetMock.mockReset();
    editAccountTeamsDialogMock.mockReset();
    useGetUsersMeMock.mockReset();
    useGetUsersMeMock.mockReturnValue({ data: { roles: ["admin"] } });
    patchIsPending = false;
    patchIsError = false;
    patchError = null;
  });

  it("shows a placeholder and an edit trigger when neither CRE nor SRE team is set", () => {
    mockAccount({ data: BASE_ACCOUNT });
    renderPage(<CsmAccountDetailPage />);
    expect(screen.getByText("CRE / SRE team")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Edit CRE / SRE team" })).toBeInTheDocument();
  });

  it("hides the edit trigger for a non-admin caller", () => {
    useGetUsersMeMock.mockReturnValue({ data: { roles: ["agent"] } });
    mockAccount({ data: BASE_ACCOUNT });
    renderPage(<CsmAccountDetailPage />);
    expect(
      screen.queryByRole("button", { name: "Edit CRE / SRE team" }),
    ).not.toBeInTheDocument();
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

  it("opens EditAccountTeamsDialog with the account's current teams when the edit trigger is clicked", () => {
    mockAccount({
      data: {
        ...BASE_ACCOUNT,
        creTeam: { id: "team-cre-1", name: "CRE Alpha" },
        sreTeam: null,
      },
    });
    renderPage(<CsmAccountDetailPage />);

    expect(editAccountTeamsDialogMock).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "Edit CRE / SRE team" }));
    expect(editAccountTeamsDialogMock).toHaveBeenCalledWith(
      expect.objectContaining({
        currentCreTeam: { id: "team-cre-1", name: "CRE Alpha" },
        currentSreTeam: null,
        isSaving: false,
        saveError: null,
      }),
    );
  });

  it("submits the dialog's patch via the mutation and closes the dialog on success", () => {
    mockAccount({ data: BASE_ACCOUNT });
    patchMutateMock.mockImplementation((_patch, opts) => {
      opts?.onSuccess?.();
    });
    renderPage(<CsmAccountDetailPage />);

    fireEvent.click(screen.getByRole("button", { name: "Edit CRE / SRE team" }));
    const { onSave } = editAccountTeamsDialogMock.mock.calls[0][0];
    act(() => onSave({ creTeamId: "team-cre-2" }));

    expect(patchMutateMock).toHaveBeenCalledWith(
      { creTeamId: "team-cre-2" },
      expect.objectContaining({ onSuccess: expect.any(Function) }),
    );
    // The dialog's onSuccess callback set `editTeamsOpen` back to false, so
    // it actually unmounts — assert the rendered element is gone, not just
    // that a call count reset to zero.
    expect(screen.queryByTestId("edit-account-teams-dialog")).not.toBeInTheDocument();
  });

  it("shows the upstream message inline for a client (<500) error", () => {
    mockAccount({ data: BASE_ACCOUNT });
    patchIsError = true;
    patchError = new BackendApiError(400, "Team not found");
    renderPage(<CsmAccountDetailPage />);

    fireEvent.click(screen.getByRole("button", { name: "Edit CRE / SRE team" }));
    expect(editAccountTeamsDialogMock).toHaveBeenCalledWith(
      expect.objectContaining({ saveError: "Team not found" }),
    );
  });

  it("shows a generic fallback inline for a server (5xx) error, not the raw message", () => {
    mockAccount({ data: BASE_ACCOUNT });
    patchIsError = true;
    patchError = new BackendApiError(500, "internal: nil pointer at x.go:42");
    renderPage(<CsmAccountDetailPage />);

    fireEvent.click(screen.getByRole("button", { name: "Edit CRE / SRE team" }));
    expect(editAccountTeamsDialogMock).toHaveBeenCalledWith(
      expect.objectContaining({
        saveError: "Could not update the account's teams. Please try again.",
      }),
    );
  });

  it("resets the mutation before reopening, so a stale error doesn't reappear", () => {
    mockAccount({ data: BASE_ACCOUNT });
    renderPage(<CsmAccountDetailPage />);

    fireEvent.click(screen.getByRole("button", { name: "Edit CRE / SRE team" }));
    expect(patchResetMock).toHaveBeenCalled();
  });
});
