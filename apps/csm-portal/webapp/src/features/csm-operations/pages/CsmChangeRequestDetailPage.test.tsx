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

import { fireEvent, render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import type { JSX } from "react";
import {
  MemoryRouter,
  Route,
  Routes,
  useLocation,
  useNavigate,
  type NavigateOptions,
  type To,
} from "react-router";
import type { UseQueryResult } from "@tanstack/react-query";
import type { BeChangeRequestDetail } from "@api/backend/types";
import { BackendApiError } from "@api/backend/client";

const navigateMock = vi.fn();
const useGetChangeRequestMock = vi.fn();
const patchMutateMock = vi.fn();
const patchResetMock = vi.fn();
const showErrorMock = vi.fn();
const editChangeRequestDialogMock = vi.fn();
let patchIsPending = false;
let patchIsError = false;
let patchError: Error | null = null;

// The backend client reads runtime config (`CSM_PORTAL_BACKEND_BASE_URL`) at
// module load, which isn't present under vitest. The page imports
// `BackendApiError` from it directly, so stub the module with a real class
// (so `instanceof` still works) — same approach as CsmIncidentDetailPage.test.tsx.
vi.mock("@api/backend/client", () => ({
  BackendApiError: class BackendApiError extends Error {
    status: number;
    constructor(status: number, message: string) {
      super(message);
      this.status = status;
    }
  },
}));

vi.mock("@hooks/useNavTransition", () => ({
  useNavTransition: () => navigateMock,
}));
vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: showErrorMock }),
}));
vi.mock("@features/csm-operations/api/useGetChangeRequest", () => ({
  useGetChangeRequest: () => useGetChangeRequestMock(),
}));
vi.mock("@features/csm-operations/api/usePatchChangeRequest", () => ({
  usePatchChangeRequest: () => ({
    mutate: patchMutateMock,
    reset: patchResetMock,
    isPending: patchIsPending,
    isError: patchIsError,
    error: patchError,
  }),
}));
vi.mock("@features/csm-operations/components/ChangeRequestApprovals", () => ({
  default: () => null,
}));
// Exercised in isolation by EditChangeRequestDialog.test.tsx; here we only
// assert this page wires `saveError` and resets the mutation before opening.
vi.mock("@features/csm-operations/components/EditChangeRequestDialog", () => ({
  default: (props: unknown) => {
    editChangeRequestDialogMock(props);
    return null;
  },
}));
vi.mock("@features/csm-operations/api/useCsmChangeRequestComments", () => ({
  useGetCsmChangeRequestComments: () => ({ data: [] }),
  usePostCsmChangeRequestComment: () => ({ isPending: false, mutate: vi.fn() }),
}));
vi.mock("@features/csm-cases/api/useCsmCaseAttachments", () => ({
  useGetCsmCaseAttachments: () => ({ data: [] }),
  usePostCsmCaseAttachment: () => ({ isPending: false, mutate: vi.fn() }),
  useDownloadCsmCaseAttachment: () => vi.fn(),
}));
vi.mock("@features/csm-cases/components/CaseActivitiesFeed", () => ({
  default: () => null,
}));
vi.mock("@features/csm-cases/components/CaseDetailWidgets", () => ({
  AttachmentsWidget: () => null,
}));

// Imported after the mocks above so the module picks them up.
import CsmChangeRequestDetailPage from "@features/csm-operations/pages/CsmChangeRequestDetailPage";

// `useNavTransition` is mocked module-wide (above); this page's tab strip is
// now a real URL path segment (`usePathTabs`), so the mock is bridged to the
// real `useNavigate` from this render tree — same pattern as
// CsmCaseDetailPage.test.tsx's `NavigateBridge` — so a simulated tab click
// actually drives the router instead of being a no-op.
function NavigateBridge(): null {
  const navigate = useNavigate();
  navigateMock.mockImplementation((to: To, options?: NavigateOptions) =>
    navigate(to, options),
  );
  return null;
}

function LocationProbe(): JSX.Element {
  const location = useLocation();
  return <div data-testid="location-probe">{location.pathname}</div>;
}

function renderPage(
  initialEntry = "/operations/change-requests/chg-1",
): ReturnType<typeof render> {
  return render(
    <MemoryRouter initialEntries={[initialEntry]}>
      <NavigateBridge />
      <LocationProbe />
      <Routes>
        <Route
          path="/operations/change-requests/:id/:tab?"
          element={<CsmChangeRequestDetailPage />}
        />
      </Routes>
    </MemoryRouter>,
  );
}

const BASE_CR: BeChangeRequestDetail = {
  id: "chg-1",
  number: "CHG0009988",
  subject: "Upgrade the gateway cluster",
  case: { id: "case-1", name: "CASE0001234" },
  createdOn: "2026-01-01T00:00:00Z",
  state: "new",
  type: "normal",
  assignedTeam: { id: "team-1", name: "Platform" },
};

function mockQueryResult(
  overrides: Partial<UseQueryResult<BeChangeRequestDetail | null, Error>>,
): void {
  useGetChangeRequestMock.mockReturnValue({
    data: null,
    isLoading: false,
    isError: false,
    error: null,
    ...overrides,
  });
}

beforeEach(() => {
  navigateMock.mockClear();
  patchMutateMock.mockClear();
  showErrorMock.mockClear();
  patchIsPending = false;
  patchIsError = false;
  patchError = null;
  patchResetMock.mockClear();
  editChangeRequestDialogMock.mockClear();
});

describe("CsmChangeRequestDetailPage", () => {
  it("renders the linked case as a clickable reference to the case route", () => {
    mockQueryResult({ data: BASE_CR });
    renderPage();

    screen
      .getByText("CASE0001234")
      .closest('[role="button"]')
      ?.dispatchEvent(new MouseEvent("click", { bubbles: true }));

    expect(navigateMock).toHaveBeenCalledWith("/cases/case-1");
  });

  it("renders a dash for the linked case when there is no case reference", () => {
    mockQueryResult({ data: { ...BASE_CR, case: null } });
    renderPage();
    const linkedCaseCell = screen.getByText("Linked case").parentElement!;
    expect(within(linkedCaseCell).getByText("—")).toBeInTheDocument();
  });
});

describe("CsmChangeRequestDetailPage — tab is a real URL path segment", () => {
  function goToTab(name: RegExp): void {
    fireEvent.click(screen.getByRole("tab", { name }));
  }

  it("defaults to the Approval tab when the URL carries no tab segment", () => {
    mockQueryResult({ data: BASE_CR });
    renderPage();
    expect(screen.getByRole("tab", { name: /approval/i })).toHaveAttribute(
      "aria-selected",
      "true",
    );
  });

  it("clicking a tab navigates to that tab's own URL", () => {
    mockQueryResult({ data: BASE_CR });
    renderPage();

    goToTab(/comments/i);

    expect(screen.getByTestId("location-probe")).toHaveTextContent(
      "/operations/change-requests/chg-1/comments",
    );
    expect(screen.getByRole("tab", { name: /comments/i })).toHaveAttribute(
      "aria-selected",
      "true",
    );
  });

  it("loading the page directly at a tab's URL opens on that tab", () => {
    mockQueryResult({ data: BASE_CR });
    renderPage("/operations/change-requests/chg-1/attachments");
    expect(screen.getByRole("tab", { name: /attachments/i })).toHaveAttribute(
      "aria-selected",
      "true",
    );
  });

  it("falls back to Approval for an unknown tab segment, without redirecting the URL", () => {
    mockQueryResult({ data: BASE_CR });
    renderPage("/operations/change-requests/chg-1/bogus-tab");

    expect(screen.getByRole("tab", { name: /approval/i })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    expect(screen.getByTestId("location-probe")).toHaveTextContent(
      "/operations/change-requests/chg-1/bogus-tab",
    );
  });
});

describe("CsmChangeRequestDetailPage — Clone", () => {
  it("navigates to the create form with router state built from this record", () => {
    mockQueryResult({
      data: {
        ...BASE_CR,
        description: "<p>Upgrade the gateway.</p>",
        impact: "high",
        assignedEngineer: { id: "user-1", name: "Jane Doe" },
      },
    });
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: /clone/i }));
    expect(navigateMock).toHaveBeenCalledWith(
      "/operations/change-requests/new",
      expect.objectContaining({
        state: expect.objectContaining({
          sourceNumber: "CHG0009988",
          subject: "Upgrade the gateway cluster",
          type: "normal",
          impact: "high",
          assignedEngineerId: "user-1",
          assignedEngineerLabel: "Jane Doe",
        }),
      }),
    );
  });

  it("never puts the deployment, state, or approval fields into the clone's router state", () => {
    mockQueryResult({
      data: {
        ...BASE_CR,
        deployment: { id: "dep-1", name: "prod" },
        state: "closed",
        hasCustomerApproved: true,
      },
    });
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: /clone/i }));
    const [, options] = navigateMock.mock.calls[0];
    const keys = Object.keys(options.state);
    expect(keys).not.toContain("deployment");
    expect(keys).not.toContain("state");
    expect(keys).not.toContain("hasCustomerApproved");
  });
});

describe("CsmChangeRequestDetailPage — Request approval (New -> Assess)", () => {
  it("shows the Request approval button when the backend flags 'assess' as a legal next state", () => {
    mockQueryResult({ data: { ...BASE_CR, legalNextStates: ["assess"] } });
    renderPage();
    expect(
      screen.getByRole("button", { name: /request approval/i }),
    ).toBeInTheDocument();
  });

  it("hides the button when legalNextStates is empty (no transition available)", () => {
    mockQueryResult({ data: { ...BASE_CR, legalNextStates: [] } });
    renderPage();
    expect(
      screen.queryByRole("button", { name: /request approval/i }),
    ).not.toBeInTheDocument();
  });

  it("hides the button when legalNextStates is absent — data-driven, no hardcoded state check", () => {
    mockQueryResult({ data: { ...BASE_CR, legalNextStates: undefined } });
    renderPage();
    expect(
      screen.queryByRole("button", { name: /request approval/i }),
    ).not.toBeInTheDocument();
  });

  it("shows a disabled Request approval button when the state allows it but there is no assigned team", () => {
    mockQueryResult({
      data: { ...BASE_CR, legalNextStates: ["assess"], assignedTeam: null },
    });
    renderPage();
    const button = screen.getByRole("button", { name: /request approval/i });
    expect(button).toBeDisabled();
    fireEvent.click(button);
    expect(patchMutateMock).not.toHaveBeenCalled();
  });

  it("exposes the blocked reason to keyboard users via a focusable, labeled wrapper", () => {
    mockQueryResult({
      data: { ...BASE_CR, legalNextStates: ["assess"], assignedTeam: null },
    });
    renderPage();
    const button = screen.getByRole("button", { name: /request approval/i });
    const focusTarget = button.closest('[tabindex="0"]');
    expect(focusTarget).not.toBeNull();
    expect(focusTarget).toHaveAttribute(
      "aria-label",
      "Request approval: Set an assigned team before requesting approval",
    );
  });

  it("leaves Request approval enabled when both the state and the assigned team allow it", () => {
    mockQueryResult({
      data: { ...BASE_CR, legalNextStates: ["assess"], assignedTeam: { id: "team-1", name: "Platform" } },
    });
    renderPage();
    expect(
      screen.getByRole("button", { name: /request approval/i }),
    ).toBeEnabled();
  });

  it("PATCHes { requestApproval: true } for this CR when clicked", () => {
    mockQueryResult({ data: { ...BASE_CR, legalNextStates: ["assess"] } });
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: /request approval/i }));
    expect(patchMutateMock).toHaveBeenCalledWith(
      { id: "chg-1", patch: { requestApproval: true } },
      expect.objectContaining({ onError: expect.any(Function) }),
    );
  });

  it("surfaces a mutation error via the shared error banner", () => {
    mockQueryResult({ data: { ...BASE_CR, legalNextStates: ["assess"] } });
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: /request approval/i }));
    const [, options] = patchMutateMock.mock.calls[0];
    const err = new Error("boom");
    options.onError(err);
    expect(showErrorMock).toHaveBeenCalledWith(
      "Could not request approval for this change request.",
      err,
    );
  });

  it("surfaces the backend's real rejection reason for a 4xx state-transition error", () => {
    mockQueryResult({ data: { ...BASE_CR, legalNextStates: ["assess"] } });
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: /request approval/i }));
    const [, options] = patchMutateMock.mock.calls[0];
    const err = new BackendApiError(409, "State transition rejected: approver required");
    options.onError(err);
    expect(showErrorMock).toHaveBeenCalledWith(
      "State transition rejected: approver required",
      err,
    );
  });

  it("falls back to the generic message for a 5xx error even with a body message", () => {
    mockQueryResult({ data: { ...BASE_CR, legalNextStates: ["assess"] } });
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: /request approval/i }));
    const [, options] = patchMutateMock.mock.calls[0];
    const err = new BackendApiError(500, "internal error detail");
    options.onError(err);
    expect(showErrorMock).toHaveBeenCalledWith(
      "Could not request approval for this change request.",
      err,
    );
  });
});

describe("CsmChangeRequestDetailPage — Edit dialog error wiring", () => {
  it("resets the shared mutation before opening the Edit dialog, so a stale error from elsewhere isn't shown as this save's error", () => {
    mockQueryResult({ data: { ...BASE_CR, legalNextStates: ["assess"] } });
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: /^edit$/i }));
    expect(patchResetMock).toHaveBeenCalled();
  });

  it("passes no saveError to the dialog when the mutation hasn't failed", () => {
    mockQueryResult({ data: { ...BASE_CR, legalNextStates: ["assess"] } });
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: /^edit$/i }));
    const [props] = editChangeRequestDialogMock.mock.calls.at(-1)!;
    expect(props.saveError).toBeNull();
  });

  it("passes the backend's rejection reason as saveError for a 4xx failure", () => {
    patchIsError = true;
    patchError = new BackendApiError(
      400,
      "isCustomerApproved, isCustomerReviewed, and requestApproval are mutually exclusive",
    );
    mockQueryResult({ data: { ...BASE_CR, legalNextStates: ["assess"] } });
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: /^edit$/i }));
    const [props] = editChangeRequestDialogMock.mock.calls.at(-1)!;
    expect(props.saveError).toBe(
      "isCustomerApproved, isCustomerReviewed, and requestApproval are mutually exclusive",
    );
  });

  it("falls back to a generic saveError for a 5xx failure", () => {
    patchIsError = true;
    patchError = new BackendApiError(500, "internal error detail");
    mockQueryResult({ data: { ...BASE_CR, legalNextStates: ["assess"] } });
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: /^edit$/i }));
    const [props] = editChangeRequestDialogMock.mock.calls.at(-1)!;
    expect(props.saveError).toBe("Could not update the change request.");
  });
});
