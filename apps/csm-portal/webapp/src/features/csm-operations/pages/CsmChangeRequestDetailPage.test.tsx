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
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import type { JSX } from "react";
import { MemoryRouter, Route, Routes, useLocation } from "react-router";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { UseQueryResult } from "@tanstack/react-query";
import type { BeChangeRequestDetail } from "@api/backend/types";
import { BackendApiError } from "@api/backend/client";
import { CaseTabsProvider, useCaseTabsController } from "@context/case-tabs/CaseTabsContext";
import { CaseTabsBehaviorProvider } from "@context/case-tabs/CaseTabsBehaviorContext";
import { useCaseTabCloseConfirm } from "@features/case-tabs/hooks/useCaseTabCloseConfirm";
import LoggerProvider from "@context/logger/LoggerProvider";

const navigateMock = vi.fn();
const useGetChangeRequestMock = vi.fn();
const patchMutateMock = vi.fn();
const patchMutateAsyncMock = vi.fn<(input: unknown) => Promise<unknown>>();
const postCommentMutateAsyncMock = vi.fn<(input: unknown) => Promise<unknown>>();
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
const useGetChangeRequestApprovalsMock = vi.fn();
vi.mock("@features/csm-operations/api/useGetChangeRequestApprovals", () => ({
  useGetChangeRequestApprovals: () => useGetChangeRequestApprovalsMock(),
}));
vi.mock("@features/csm-operations/api/usePatchChangeRequest", () => ({
  usePatchChangeRequest: () => ({
    mutate: patchMutateMock,
    mutateAsync: patchMutateAsyncMock,
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
  usePostCsmChangeRequestComment: () => ({
    isPending: false,
    mutate: vi.fn(),
    mutateAsync: postCommentMutateAsyncMock,
  }),
}));
vi.mock("@features/csm-cases/api/useCsmCaseAttachments", () => ({
  useGetCsmCaseAttachments: () => ({ data: [] }),
  usePostCsmCaseAttachment: () => ({ isPending: false, mutate: vi.fn() }),
  useDownloadCsmCaseAttachment: () => vi.fn(),
  // Only reached by the reply composer's upload modal (`CsmUploadAttachmentModal`),
  // not exercised by this file's existing tests — the "reports its own draft
  // state" tests below are the first to actually mount the composer.
  MAX_ATTACHMENT_SIZE_BYTES: 10 * 1024 * 1024,
}));
vi.mock("@features/csm-cases/components/CaseActivitiesFeed", () => ({
  default: () => null,
}));
vi.mock("@features/csm-cases/components/CaseDetailWidgets", () => ({
  AttachmentsWidget: () => null,
}));

// Imported after the mocks above so the module picks them up.
import CsmChangeRequestDetailPage from "@features/csm-operations/pages/CsmChangeRequestDetailPage";

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
  patchMutateAsyncMock.mockReset();
  patchMutateAsyncMock.mockResolvedValue({ id: "chg-1" });
  postCommentMutateAsyncMock.mockReset();
  postCommentMutateAsyncMock.mockResolvedValue({ id: "comment-1" });
  useGetChangeRequestApprovalsMock.mockReturnValue({
    data: null,
    isLoading: false,
    isError: false,
    error: null,
  });
});

/** Surfaces the router's current search string, for the `?tab=` sync tests
 * below. */
function LocationSearchProbe(): JSX.Element {
  const location = useLocation();
  return <div data-testid="search-probe">{location.search}</div>;
}

/**
 * Real `<MemoryRouter>`/`<Routes>` (not a mocked `react-router`) — matches
 * this app's own convention for a hook/page that reads the router itself,
 * and `useQueryParamTabs` needs a real `useSearchParams` to actually
 * read/write the URL.
 */
function renderPage(
  initialEntry = "/operations/change-requests/chg-1",
): ReturnType<typeof render> {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[initialEntry]}>
        <Routes>
          <Route
            path="/operations/change-requests/:id"
            element={
              <>
                <CsmChangeRequestDetailPage />
                <LocationSearchProbe />
              </>
            }
          />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

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

  it("shows the Impact meta cell in the Overview grid, alongside the header chip", () => {
    mockQueryResult({ data: { ...BASE_CR, impact: "high" } });
    renderPage();
    const impactCell = screen.getByText("Impact").parentElement!;
    expect(within(impactCell).getByText("High")).toBeInTheDocument();
  });

  it("shows a dash for Impact in the Overview grid when unset", () => {
    mockQueryResult({ data: { ...BASE_CR, impact: undefined } });
    renderPage();
    const impactCell = screen.getByText("Impact").parentElement!;
    expect(within(impactCell).getByText("—")).toBeInTheDocument();
  });

  it("renders the lifecycle stepper for this CR's state", () => {
    mockQueryResult({ data: { ...BASE_CR, state: "implement" } });
    renderPage();
    expect(screen.getByRole("list", { name: /change request lifecycle/i })).toBeInTheDocument();
  });
});

describe("CsmChangeRequestDetailPage — blocking-reason header note", () => {
  it("shows 'Awaiting <stage> approval' when a stage is pending or requested", () => {
    mockQueryResult({ data: { ...BASE_CR, state: "assess" } });
    useGetChangeRequestApprovalsMock.mockReturnValue({
      data: {
        approvals: [
          {
            stage: "Assess",
            approverType: "STATIC_GROUP",
            approverName: null,
            status: "REQUESTED",
            approvers: [],
          },
        ],
      },
      isLoading: false,
      isError: false,
      error: null,
    });
    renderPage();
    expect(screen.getByText("Awaiting Assess approval")).toBeInTheDocument();
  });

  it("names the approver group when the stage carries one", () => {
    mockQueryResult({ data: { ...BASE_CR, state: "authorize" } });
    useGetChangeRequestApprovalsMock.mockReturnValue({
      data: {
        approvals: [
          {
            stage: "Authorize",
            approverType: "STATIC_GROUP",
            approverName: "Devops Approval",
            status: "PENDING",
            approvers: [],
          },
        ],
      },
      isLoading: false,
      isError: false,
      error: null,
    });
    renderPage();
    expect(screen.getByText("Awaiting Devops Approval")).toBeInTheDocument();
  });

  it("shows no blocking-reason note when no stage is pending/requested", () => {
    mockQueryResult({ data: { ...BASE_CR, state: "authorize" } });
    useGetChangeRequestApprovalsMock.mockReturnValue({
      data: {
        approvals: [
          {
            stage: "Assess",
            approverType: "STATIC_GROUP",
            approverName: null,
            status: "APPROVED",
            approvers: [],
          },
        ],
      },
      isLoading: false,
      isError: false,
      error: null,
    });
    renderPage();
    expect(screen.queryByText(/awaiting/i)).not.toBeInTheDocument();
  });

  it("suppresses the blocking-reason note once the CR is closed, even with stale pending approval data", () => {
    mockQueryResult({ data: { ...BASE_CR, state: "closed" } });
    useGetChangeRequestApprovalsMock.mockReturnValue({
      data: {
        approvals: [
          {
            stage: "Assess",
            approverType: "STATIC_GROUP",
            approverName: null,
            status: "REQUESTED",
            approvers: [],
          },
        ],
      },
      isLoading: false,
      isError: false,
      error: null,
    });
    renderPage();
    expect(screen.queryByText(/awaiting/i)).not.toBeInTheDocument();
  });
});

describe("CsmChangeRequestDetailPage — tab lives in the URL", () => {
  it("defaults to the Approval tab when ?tab= is absent", () => {
    mockQueryResult({ data: BASE_CR });
    renderPage();

    expect(screen.getByRole("tab", { name: /approval/i })).toHaveAttribute(
      "aria-selected",
      "true",
    );
  });

  it("writes the selected tab to ?tab= when switching tabs", () => {
    mockQueryResult({ data: BASE_CR });
    renderPage();

    fireEvent.click(screen.getByRole("tab", { name: /attachments/i }));

    expect(screen.getByTestId("search-probe")).toHaveTextContent("tab=attachments");
  });

  it("restores the tab named in the URL on a direct/cold load", () => {
    mockQueryResult({ data: BASE_CR });
    renderPage("/operations/change-requests/chg-1?tab=comments");

    expect(screen.getByRole("tab", { name: /comments/i })).toHaveAttribute(
      "aria-selected",
      "true",
    );
  });

  it("falls back to Approval for an unrecognised ?tab= value, without crashing", () => {
    mockQueryResult({ data: BASE_CR });
    renderPage("/operations/change-requests/chg-1?tab=not-a-real-tab");

    expect(screen.getByRole("tab", { name: /approval/i })).toHaveAttribute(
      "aria-selected",
      "true",
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

// ---------------------------------------------------------------------------
// Lifecycle transitions. `ChangeRequestActionBar` is exercised in isolation by
// its own test; these cover this page's half of the contract — which patch
// each target produces, and the comment-then-patch ordering the destructive
// ones go through.
// ---------------------------------------------------------------------------

/** Open the action bar's overflow menu. */
function openStateMenu(): void {
  fireEvent.click(screen.getByRole("button", { name: /change state/i }));
}

describe("CsmChangeRequestDetailPage — direct (non-destructive) transitions", () => {
  it("PATCHes { state: target } for a forward move that is not New -> Assess", () => {
    mockQueryResult({
      data: { ...BASE_CR, state: "scheduled", legalNextStates: ["implement"] },
    });
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: /start implementation/i }));
    expect(patchMutateMock).toHaveBeenCalledWith(
      { id: "chg-1", patch: { state: "implement" } },
      expect.objectContaining({ onError: expect.any(Function) }),
    );
  });

  it("keeps the requestApproval flag for New -> Assess rather than sending state", () => {
    mockQueryResult({ data: { ...BASE_CR, legalNextStates: ["assess"] } });
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: /request approval/i }));
    const [{ patch }] = patchMutateMock.mock.calls[0];
    expect(patch).toEqual({ requestApproval: true });
  });

  it("sends a state the backend added verbatim, with no frontend change", () => {
    mockQueryResult({
      data: { ...BASE_CR, state: "review", legalNextStates: ["awaiting_vendor"] },
    });
    renderPage();
    openStateMenu();
    fireEvent.click(screen.getByRole("menuitem", { name: /^awaiting vendor$/i }));
    expect(patchMutateMock).toHaveBeenCalledWith(
      { id: "chg-1", patch: { state: "awaiting_vendor" } },
      expect.anything(),
    );
  });

  it("surfaces the backend's real 4xx rejection reason for a transition", () => {
    mockQueryResult({
      data: { ...BASE_CR, state: "scheduled", legalNextStates: ["implement"] },
    });
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: /start implementation/i }));
    const [, options] = patchMutateMock.mock.calls[0];
    const err = new BackendApiError(409, "Change window has not opened yet");
    options.onError(err);
    expect(showErrorMock).toHaveBeenCalledWith("Change window has not opened yet", err);
  });

  it("falls back to a target-specific generic message for a 5xx", () => {
    mockQueryResult({
      data: { ...BASE_CR, state: "scheduled", legalNextStates: ["implement"] },
    });
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: /start implementation/i }));
    const [, options] = patchMutateMock.mock.calls[0];
    const err = new BackendApiError(500, "internal error detail");
    options.onError(err);
    expect(showErrorMock).toHaveBeenCalledWith(
      "Could not move this change request to Implement.",
      err,
    );
  });
});

describe("CsmChangeRequestDetailPage — destructive transitions need a reason first", () => {
  /** Render a CR that can be canceled, and open the confirmation dialog. */
  function openCancelDialog(): void {
    mockQueryResult({
      data: { ...BASE_CR, state: "implement", legalNextStates: ["review", "canceled"] },
    });
    renderPage();
    openStateMenu();
    fireEvent.click(screen.getByRole("menuitem", { name: /cancel change/i }));
  }

  function typeReason(text: string): void {
    fireEvent.change(screen.getByLabelText(/reason/i), { target: { value: text } });
  }

  it("opens the confirmation dialog instead of patching immediately", () => {
    openCancelDialog();
    expect(screen.getByRole("dialog")).toBeInTheDocument();
    expect(patchMutateMock).not.toHaveBeenCalled();
    expect(patchMutateAsyncMock).not.toHaveBeenCalled();
  });

  it("posts the reason as a comment BEFORE patching the state", async () => {
    openCancelDialog();
    typeReason("Latency regression in production.");
    fireEvent.click(screen.getByRole("button", { name: /^cancel change$/i }));

    await waitFor(() => expect(patchMutateAsyncMock).toHaveBeenCalled());
    expect(postCommentMutateAsyncMock).toHaveBeenCalledWith({
      changeRequestId: "chg-1",
      // Posted verbatim: the backing store for these notes is plain text.
      bodyHtml: "Latency regression in production.",
      internal: true,
    });
    expect(patchMutateAsyncMock).toHaveBeenCalledWith({
      id: "chg-1",
      patch: { state: "canceled" },
    });
    // Ordering, not just co-occurrence: an unexplained cancellation is worse
    // than a failed one, so the comment must land first.
    expect(
      postCommentMutateAsyncMock.mock.invocationCallOrder[0],
    ).toBeLessThan(patchMutateAsyncMock.mock.invocationCallOrder[0]);
  });

  it("closes the dialog once both halves succeed", async () => {
    openCancelDialog();
    typeReason("Latency regression in production.");
    fireEvent.click(screen.getByRole("button", { name: /^cancel change$/i }));
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  });

  it("does NOT patch the state when the reason comment fails", async () => {
    postCommentMutateAsyncMock.mockRejectedValueOnce(
      new BackendApiError(403, "Comments are disabled on this change request"),
    );
    openCancelDialog();
    typeReason("Latency regression in production.");
    fireEvent.click(screen.getByRole("button", { name: /^cancel change$/i }));

    await waitFor(() =>
      expect(screen.getByRole("alert")).toHaveTextContent(
        /comments are disabled on this change request/i,
      ),
    );
    expect(patchMutateAsyncMock).not.toHaveBeenCalled();
    expect(screen.getByRole("dialog")).toBeInTheDocument();
  });

  it("tells the engineer the reason was recorded when only the state change failed", async () => {
    patchMutateAsyncMock.mockRejectedValueOnce(
      new BackendApiError(409, "Cancellation is not permitted from this state"),
    );
    openCancelDialog();
    typeReason("Latency regression in production.");
    fireEvent.click(screen.getByRole("button", { name: /^cancel change$/i }));

    await waitFor(() =>
      expect(screen.getByRole("alert")).toHaveTextContent(
        /reason was recorded as a comment, but the state did not change/i,
      ),
    );
    expect(screen.getByRole("alert")).toHaveTextContent(
      /cancellation is not permitted from this state/i,
    );
    expect(screen.getByRole("alert")).toHaveTextContent(/don't need to retype it/i);
  });

  it("retrying after a failed patch re-sends only the state change, never the comment twice", async () => {
    patchMutateAsyncMock.mockRejectedValueOnce(new BackendApiError(409, "Rejected"));
    openCancelDialog();
    typeReason("Latency regression in production.");
    fireEvent.click(screen.getByRole("button", { name: /^cancel change$/i }));
    await waitFor(() => expect(screen.getByRole("alert")).toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: /^cancel change$/i }));
    await waitFor(() => expect(patchMutateAsyncMock).toHaveBeenCalledTimes(2));
    expect(postCommentMutateAsyncMock).toHaveBeenCalledTimes(1);
  });

  it("posts the reason verbatim as plain text, with no markup added around it", async () => {
    // The backing store for these notes is a plain-text field: production
    // entries carry raw newlines and no escaped entities, so anything the
    // portal wraps in markup shows up as literal tags at the source. What the
    // engineer typed is what gets written, character for character.
    const typed = "Latency < 50ms breached & customer impacted.\nCanceled by Jane Doe.";
    openCancelDialog();
    typeReason(typed);
    fireEvent.click(screen.getByRole("button", { name: /^cancel change$/i }));

    await waitFor(() => expect(postCommentMutateAsyncMock).toHaveBeenCalled());
    const posted = postCommentMutateAsyncMock.mock.calls[0][0] as {
      bodyHtml: string;
    };
    expect(posted.bodyHtml).toBe(typed);
  });

  it("routes the cancel transition through the same dialog", () => {
    mockQueryResult({
      data: { ...BASE_CR, state: "scheduled", legalNextStates: ["implement", "canceled"] },
    });
    renderPage();
    openStateMenu();
    fireEvent.click(screen.getByRole("menuitem", { name: /cancel change/i }));
    expect(
      screen.getByRole("heading", { name: /cancel this change request/i }),
    ).toBeInTheDocument();
    expect(patchMutateAsyncMock).not.toHaveBeenCalled();
  });
});

/**
 * Wraps the real page in a real open case-tab (`CaseTabsProvider` +
 * `useCaseTabCloseConfirm`), exposing an "open"/"close-this-tab" trigger —
 * for the `hasDraft`/close-confirm regression test below, which needs the
 * real `useReportCaseTabDraft` wiring inside the page to actually reach the
 * tab strip's own close-confirm dialog, not just a mocked stand-in for it.
 */
function CloseTabHarness({ caseId }: { caseId: string }): JSX.Element {
  const { openTab, tabs } = useCaseTabsController();
  const { requestClose, dialog } = useCaseTabCloseConfirm();
  return (
    <div>
      <button
        onClick={() =>
          openTab(caseId, "change_request", `/operations/change-requests/${caseId}`)
        }
      >
        open-tab
      </button>
      <button
        onClick={() => {
          const tab = tabs.find((t) => t.caseId === caseId);
          if (tab) requestClose(tab);
        }}
      >
        close-tab
      </button>
      {dialog}
    </div>
  );
}

function renderPageWithOpenTab(
  initialEntry = "/operations/change-requests/chg-1",
): ReturnType<typeof render> {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <LoggerProvider>
      <QueryClientProvider client={queryClient}>
        <MemoryRouter initialEntries={[initialEntry]}>
          <CaseTabsBehaviorProvider>
            <CaseTabsProvider>
              <CloseTabHarness caseId="chg-1" />
              <Routes>
                <Route
                  path="/operations/change-requests/:id"
                  element={<CsmChangeRequestDetailPage />}
                />
              </Routes>
            </CaseTabsProvider>
          </CaseTabsBehaviorProvider>
        </MemoryRouter>
      </QueryClientProvider>
    </LoggerProvider>,
  );
}

describe("CsmChangeRequestDetailPage — reports its own draft state to the tab strip", () => {
  // Regression test for bug: this page only called `useReportCaseTabMeta`,
  // not `useReportCaseTabDraft` (unlike `CsmCaseDetailPage`, which calls
  // both) — its tab's `hasDraft` never became `true`, so closing a change
  // request's tab with a reply half-written skipped the discard-confirm
  // dialog entirely, unlike a case tab in the same situation.
  it("closing this change request's tab with an open (unsent) reply asks for confirmation, same as a case tab does", () => {
    localStorage.setItem("csm.caseTabs.enabled", "1");
    mockQueryResult({ data: BASE_CR });
    renderPageWithOpenTab();

    fireEvent.click(screen.getByText("open-tab"));
    // The reply composer only renders on the Comments tab — "approval" is
    // this page's own default.
    fireEvent.click(screen.getByRole("tab", { name: /comments/i }));
    fireEvent.click(screen.getByText("Add a comment…"));

    fireEvent.click(screen.getByText("close-tab"));
    expect(screen.getByText("Close this case tab?")).toBeInTheDocument();
  });

  it("closing this change request's tab with no reply open closes it immediately, without confirming", () => {
    localStorage.setItem("csm.caseTabs.enabled", "1");
    mockQueryResult({ data: BASE_CR });
    renderPageWithOpenTab();

    fireEvent.click(screen.getByText("open-tab"));
    fireEvent.click(screen.getByText("close-tab"));
    expect(screen.queryByText("Close this case tab?")).not.toBeInTheDocument();
  });
});
