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
import type { UseQueryResult } from "@tanstack/react-query";
import type { BeChangeRequestApprovalsView, BeCustomerContact, BeGroupDetail } from "@api/backend/types";

const useGetChangeRequestApprovalsMock = vi.fn();
const useCurrentUserMock = vi.fn();
const useDecideChangeRequestApprovalMock = vi.fn();
const useGroupDetailMock = vi.fn();
const showErrorMock = vi.fn();
const decideMutateMock = vi.fn();

// The backend client reads runtime config (`CSM_PORTAL_BACKEND_BASE_URL`) at
// module load, which isn't present under vitest. QueryErrorState imports
// `BackendApiError` from it, so stub the module (same approach as
// CsmAnnouncementsPage.test.tsx).
vi.mock("@api/backend/client", () => ({
  BackendApiError: class BackendApiError extends Error {
    status: number;
    constructor(status = 500, message = "") {
      super(message);
      this.status = status;
    }
  },
  useBackendApi: () => ({ get: vi.fn(), post: vi.fn() }),
}));

vi.mock("@features/csm-operations/api/useGetChangeRequestApprovals", () => ({
  useGetChangeRequestApprovals: () => useGetChangeRequestApprovalsMock(),
}));

vi.mock("@context/current-user/CurrentUserContext", () => ({
  useCurrentUser: () => useCurrentUserMock(),
}));

vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: showErrorMock }),
}));

vi.mock("@features/csm-operations/api/useDecideChangeRequestApproval", () => ({
  useDecideChangeRequestApproval: () => useDecideChangeRequestApprovalMock(),
}));

vi.mock("@features/csm-operations/api/useGroupDetail", () => ({
  useGroupDetail: (id: string | undefined) => useGroupDetailMock(id),
}));

// Imported after the mocks above so the modules pick them up.
import { BackendApiError } from "@api/backend/client";
import ChangeRequestApprovals from "@features/csm-operations/components/ChangeRequestApprovals";

function mockQueryResult(
  overrides: Partial<UseQueryResult<BeChangeRequestApprovalsView | null, Error>>,
): void {
  useGetChangeRequestApprovalsMock.mockReturnValue({
    data: null,
    isLoading: false,
    isError: false,
    error: null,
    ...overrides,
  });
}

function mockCurrentUser(id?: string): void {
  useCurrentUserMock.mockReturnValue({
    user: id ? { id } : undefined,
    isLoading: false,
    isError: false,
  });
}

function mockDecideMutation(overrides: { isPending?: boolean } = {}): void {
  useDecideChangeRequestApprovalMock.mockReturnValue({
    mutate: decideMutateMock,
    isPending: overrides.isPending ?? false,
  });
}

describe("ChangeRequestApprovals", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockCurrentUser(undefined);
    mockDecideMutation();
  });

  it("renders every approver as its own flat row -- no per-stage grouping or expand/collapse", () => {
    // Real ServiceNow's own Approvers list has no stage concept at all: every
    // approver record for the change request is one row in one flat table.
    // This test locks that in -- both approvers must be visible immediately,
    // with no accordion/disclosure to open first.
    mockQueryResult({
      data: {
        approvals: [
          {
            stage: "Authorize",
            approverType: "STATIC_GROUP",
            approverName: "Devops Approval",
            status: "REQUESTED",
            approvers: [
              { id: "a1", name: "Approver One", status: "REQUESTED" },
              { id: "a2", name: "Approver Two", status: "REQUESTED" },
            ],
          },
        ],
      },
    });
    render(<ChangeRequestApprovals id="chg-1" />);

    expect(screen.getByText("Approver One")).toBeInTheDocument();
    expect(screen.getByText("Approver Two")).toBeInTheDocument();
    // The raw backend stage name is shown as its label ("CAB Approval"), once
    // per row, next to the assignment group.
    expect(screen.queryByText("Authorize")).not.toBeInTheDocument();
    expect(screen.getAllByText("CAB Approval")).toHaveLength(2);
    expect(screen.getAllByText("Devops Approval")).toHaveLength(2);
  });

  it("shows NOT_REQUIRED approvers inline, flat, like every other row", () => {
    mockQueryResult({
      data: {
        approvals: [
          {
            stage: "Authorize",
            approverType: "STATIC_GROUP",
            approverName: "Devops Approval",
            status: "REQUESTED",
            approvers: [
              { id: "a1", name: "Not Needed One", status: "NOT_REQUIRED" },
              { id: "a2", name: "Approved Alice", status: "APPROVED" },
            ],
          },
        ],
      },
    });
    render(<ChangeRequestApprovals id="chg-1" />);

    expect(screen.getByText("Approved Alice")).toBeInTheDocument();
    expect(screen.getByText("Not Needed One")).toBeInTheDocument();
  });

  it("flattens approvers from multiple approval stages into one table, each carrying its own assignment group", () => {
    mockQueryResult({
      data: {
        approvals: [
          {
            stage: "Assess",
            approverType: "STATIC_GROUP",
            approverName: "SRE Team",
            status: "APPROVED",
            approvers: [{ id: "a1", name: "Assess Approver", status: "APPROVED" }],
          },
          {
            stage: "Authorize",
            approverType: "STATIC_GROUP",
            approverName: "Devops Approval",
            status: "REQUESTED",
            approvers: [{ id: "a2", name: "Authorize Approver", status: "REQUESTED" }],
          },
        ],
      },
    });
    render(<ChangeRequestApprovals id="chg-1" />);

    expect(screen.getByText("Assess Approver")).toBeInTheDocument();
    expect(screen.getByText("SRE Team")).toBeInTheDocument();
    expect(screen.getByText("Authorize Approver")).toBeInTheDocument();
    expect(screen.getByText("Devops Approval")).toBeInTheDocument();
  });

  it("renders a friendly fallback for an approver with no name, without an alarming 'unknown' label", () => {
    mockQueryResult({
      data: {
        approvals: [
          {
            stage: "Authorize",
            approverType: "STATIC_GROUP",
            approverName: "Devops Approval",
            status: "REQUESTED",
            approvers: [{ id: "no-name-id", name: null, status: "REQUESTED" }],
          },
        ],
      },
    });
    render(<ChangeRequestApprovals id="chg-1" />);

    expect(screen.getByText("Unnamed approver")).toBeInTheDocument();
    expect(screen.queryByText("Unknown approver")).not.toBeInTheDocument();
  });

  it("shows a dash for comments when the approver has none", () => {
    mockQueryResult({
      data: {
        approvals: [
          {
            stage: "Authorize",
            approverType: "STATIC_GROUP",
            approverName: "Devops Approval",
            status: "REQUESTED",
            approvers: [{ id: "a1", name: "No Comment", status: "REQUESTED", comments: null }],
          },
        ],
      },
    });
    render(<ChangeRequestApprovals id="chg-1" />);

    const row = screen.getByText("No Comment").closest("tr");
    expect(row).not.toBeNull();
    expect(row?.textContent).toContain("—");
  });

  it("shows the approver's own comment text when present", () => {
    mockQueryResult({
      data: {
        approvals: [
          {
            stage: "Authorize",
            approverType: "STATIC_GROUP",
            approverName: "Devops Approval",
            status: "REJECTED",
            approvers: [
              { id: "a1", name: "Commenter", status: "REJECTED", comments: "Needs a rollback plan first" },
            ],
          },
        ],
      },
    });
    render(<ChangeRequestApprovals id="chg-1" />);

    expect(screen.getByText("Needs a rollback plan first")).toBeInTheDocument();
  });

  describe("approval decision action", () => {
    const approvalsWithMyPending = {
      approvals: [
        {
          stage: "Authorize",
          approverType: "STATIC_GROUP" as const,
          approverName: "Devops Approval",
          status: "REQUESTED",
          approvers: [
            { id: "me-id", name: "Current User", status: "REQUESTED" },
            { id: "other-id", name: "Other Approver", status: "REQUESTED" },
          ],
        },
      ],
    };

    it("shows Approve/Reject only on the current user's own pending approval row", () => {
      mockQueryResult({ data: approvalsWithMyPending });
      mockCurrentUser("me-id");
      render(<ChangeRequestApprovals id="chg-1" />);

      expect(screen.getByText("Current User")).toBeInTheDocument();
      expect(screen.getByText("Other Approver")).toBeInTheDocument();
      expect(screen.getAllByText("Approve")).toHaveLength(1);
      expect(screen.getAllByText("Reject")).toHaveLength(1);
    });

    it("hides Approve/Reject when no user is signed in", () => {
      mockQueryResult({ data: approvalsWithMyPending });
      mockCurrentUser(undefined);
      render(<ChangeRequestApprovals id="chg-1" />);

      expect(screen.queryByText("Approve")).not.toBeInTheDocument();
      expect(screen.queryByText("Reject")).not.toBeInTheDocument();
    });

    it("hides Approve/Reject when the current user has no pending approval on this CR", () => {
      mockQueryResult({
        data: {
          approvals: [
            {
              stage: "Authorize",
              approverType: "STATIC_GROUP",
              approverName: "Devops Approval",
              status: "APPROVED",
              approvers: [{ id: "me-id", name: "Current User", status: "APPROVED" }],
            },
          ],
        },
      });
      mockCurrentUser("me-id");
      render(<ChangeRequestApprovals id="chg-1" />);

      expect(screen.queryByText("Approve")).not.toBeInTheDocument();
      expect(screen.queryByText("Reject")).not.toBeInTheDocument();
    });

    it("submits the decision with the CR id when Approve is clicked", () => {
      mockQueryResult({ data: approvalsWithMyPending });
      mockCurrentUser("me-id");
      render(<ChangeRequestApprovals id="chg-1" />);

      fireEvent.click(screen.getByText("Approve"));

      expect(decideMutateMock).toHaveBeenCalledWith(
        { id: "chg-1", decision: "approved" },
        expect.objectContaining({ onError: expect.any(Function) }),
      );
    });

    it("submits the decision with the CR id when Reject is clicked", () => {
      mockQueryResult({ data: approvalsWithMyPending });
      mockCurrentUser("me-id");
      render(<ChangeRequestApprovals id="chg-1" />);

      fireEvent.click(screen.getByText("Reject"));

      expect(decideMutateMock).toHaveBeenCalledWith(
        { id: "chg-1", decision: "rejected" },
        expect.objectContaining({ onError: expect.any(Function) }),
      );
    });

    it("disables Approve/Reject while a decision is in flight", () => {
      mockQueryResult({ data: approvalsWithMyPending });
      mockCurrentUser("me-id");
      mockDecideMutation({ isPending: true });
      render(<ChangeRequestApprovals id="chg-1" />);

      expect(screen.getByText("Approve").closest("button")).toBeDisabled();
      expect(screen.getByText("Reject").closest("button")).toBeDisabled();
    });
  });
});

describe("ChangeRequestApprovals — Peer / CAB / ECAB stages", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockCurrentUser(undefined);
    mockDecideMutation();
  });

  it("labels each stage Peer Approval, CAB Approval, ECAB Approval, whichever name the backend uses", () => {
    mockQueryResult({
      data: {
        approvals: [
          { stage: "Assess", approverType: "STATIC_GROUP", approverName: "SRE Team", status: "APPROVED", approvers: [{ id: "p", name: "Peer One", status: "APPROVED" }] },
          { stage: "CAB Approval", approverType: "STATIC_GROUP", approverName: "CAB", status: "REQUESTED", approvers: [{ id: "c", name: "Cab One", status: "REQUESTED" }] },
          { stage: "Emergency CAB", approverType: "STATIC_GROUP", approverName: "ECAB", status: "REQUESTED", approvers: [{ id: "e", name: "Ecab One", status: "REQUESTED" }] },
        ],
      },
    });
    render(<ChangeRequestApprovals id="chg-1" />);

    expect(screen.getByText("Stage", { selector: "th" })).toBeInTheDocument();
    expect(screen.getByText("Peer One").closest("tr")).toHaveTextContent("Peer Approval");
    expect(screen.getByText("Cab One").closest("tr")).toHaveTextContent("CAB Approval");
    expect(screen.getByText("Ecab One").closest("tr")).toHaveTextContent("ECAB Approval");
  });

  it("renders only what the backend returns: a Standard change with no stages shows the empty state", () => {
    mockQueryResult({ data: { approvals: [] } });
    render(<ChangeRequestApprovals id="chg-1" />);
    expect(screen.getByText(/no approval stages recorded/i)).toBeInTheDocument();
  });

  it("renders an Emergency change as a lone ECAB stage with no Peer or CAB rows", () => {
    mockQueryResult({
      data: {
        approvals: [
          { stage: "ECAB Approval", approverType: "STATIC_GROUP", approverName: "ECAB", status: "REQUESTED", approvers: [{ id: "e", name: "Ecab One", status: "REQUESTED" }] },
        ],
      },
    });
    render(<ChangeRequestApprovals id="chg-1" />);
    expect(screen.getAllByText("ECAB Approval")).toHaveLength(1);
    expect(screen.queryByText("Peer Approval")).not.toBeInTheDocument();
    expect(screen.queryByText("CAB Approval")).not.toBeInTheDocument();
  });
});

describe("ChangeRequestApprovals — internal approvals while the CR waits for the customer", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockCurrentUser("c");
    mockDecideMutation();
  });

  it("keeps showing the settled Peer and CAB stages, with no Approve/Reject, once the CR sits in customer_approval", () => {
    // Customer approval is a state of the CR, not an internal approval stage:
    // the panel only ever reflects what the backend returns, all settled here.
    mockQueryResult({
      data: {
        approvals: [
          { stage: "Peer Approval", approverType: "STATIC_GROUP", approverName: "Peers", status: "APPROVED", approvers: [{ id: "p", name: "Peer One", status: "APPROVED" }] },
          { stage: "CAB Approval", approverType: "STATIC_GROUP", approverName: "CAB", status: "APPROVED", approvers: [{ id: "c", name: "Cab One", status: "APPROVED" }] },
        ],
      },
    });
    render(<ChangeRequestApprovals id="chg-1" />);
    expect(screen.getByText("Peer One").closest("tr")).toHaveTextContent("Approved");
    expect(screen.getByText("Cab One").closest("tr")).toHaveTextContent("Approved");
    expect(screen.queryByRole("button", { name: /^approve$/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /^reject$/i })).not.toBeInTheDocument();
  });

  it("renders a backend-provided 'Customer Approval' stage under its own name rather than as Peer/CAB", () => {
    mockQueryResult({
      data: {
        approvals: [
          { stage: "Customer Approval", approverType: "DYNAMIC_CONTACT", approverName: null, status: "REQUESTED", approvers: [{ id: "x", name: "Acme Contact", status: "REQUESTED" }] },
        ],
      },
    });
    render(<ChangeRequestApprovals id="chg-1" />);
    expect(screen.getByText("Acme Contact").closest("tr")).toHaveTextContent("Customer Approval");
    expect(screen.queryByText("Peer Approval")).not.toBeInTheDocument();
    expect(screen.queryByText("CAB Approval")).not.toBeInTheDocument();
  });
});

describe("ChangeRequestApprovals — the creator cannot approve", () => {
  const stages = (approverId: string) => ({
    approvals: [
      {
        stage: "Authorize",
        approverType: "STATIC_GROUP" as const,
        approverName: "CAB",
        status: "REQUESTED",
        approvers: [
          { id: approverId, name: "Me", status: "REQUESTED" },
          { id: "someone-else", name: "Other Approver", status: "REQUESTED" },
        ],
      },
    ],
  });

  beforeEach(() => {
    vi.clearAllMocks();
    mockDecideMutation();
  });

  it("disables Approve and Reject on the creator's own pending row and explains why", () => {
    mockQueryResult({ data: stages("me-id") });
    mockCurrentUser("me-id");
    render(<ChangeRequestApprovals id="chg-1" isCreator />);

    expect(screen.getByText("Approve").closest("button")).toBeDisabled();
    expect(screen.getByText("Reject").closest("button")).toBeDisabled();
    expect(screen.getByLabelText(/you created this change request/i)).toBeInTheDocument();
    // Explanatory text that also tells them Cancel is still available.
    expect(screen.getByRole("alert")).toHaveTextContent(/can't approve or reject/i);
    expect(screen.getByRole("alert")).toHaveTextContent(/still cancel/i);
  });

  it("never submits a decision when the creator clicks the disabled controls", () => {
    mockQueryResult({ data: stages("me-id") });
    mockCurrentUser("me-id");
    render(<ChangeRequestApprovals id="chg-1" isCreator />);

    fireEvent.click(screen.getByText("Approve"));
    fireEvent.click(screen.getByText("Reject"));
    expect(decideMutateMock).not.toHaveBeenCalled();
  });

  it("renders no controls at all for a creator who has no pending row (e.g. ECAB excludes them)", () => {
    mockQueryResult({ data: stages("someone-else-entirely") });
    mockCurrentUser("me-id");
    render(<ChangeRequestApprovals id="chg-1" isCreator />);

    expect(screen.queryByText("Approve")).not.toBeInTheDocument();
    expect(screen.queryByText("Reject")).not.toBeInTheDocument();
  });

  it("lets a non-creator approver approve and reject, with no creator notice", () => {
    mockQueryResult({ data: stages("me-id") });
    mockCurrentUser("me-id");
    render(<ChangeRequestApprovals id="chg-1" isCreator={false} />);

    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    expect(screen.getByText("Approve").closest("button")).toBeEnabled();
    fireEvent.click(screen.getByText("Approve"));
    expect(decideMutateMock).toHaveBeenCalledWith(
      { id: "chg-1", decision: "approved" },
      expect.anything(),
    );
  });

  it("honours an explicit canDecide=false on one approver row only", () => {
    const data = stages("me-id");
    (data.approvals[0]!.approvers[0] as { canDecide?: boolean }).canDecide = false;
    mockQueryResult({ data });
    mockCurrentUser("me-id");
    render(<ChangeRequestApprovals id="chg-1" />);
    expect(screen.getByText("Approve").closest("button")).toBeDisabled();
  });

  it("enables Approve/Reject on the caller's own pending row when the backend says canDecide=true", () => {
    const data = stages("me-id");
    (data.approvals[0]!.approvers[0] as { canDecide?: boolean }).canDecide = true;
    mockQueryResult({ data });
    mockCurrentUser("me-id");
    render(<ChangeRequestApprovals id="chg-1" />);
    expect(screen.getByText("Approve").closest("button")).toBeEnabled();
  });

  it("surfaces the backend's readable 403 message when a decision is refused", () => {
    mockQueryResult({ data: stages("me-id") });
    mockCurrentUser("me-id");
    render(<ChangeRequestApprovals id="chg-1" />);
    fireEvent.click(screen.getByText("Approve"));
    const onError = decideMutateMock.mock.calls[0]![1].onError as (e: unknown) => void;
    const err = new (BackendApiError as unknown as new (s: number, m: string) => Error)(
      403,
      "The creator of a change request cannot approve it.",
    );
    onError(err);
    expect(showErrorMock).toHaveBeenCalledWith("The creator of a change request cannot approve it.", err);
  });

  it("falls back to a generic message for a non-backend or 5xx failure", () => {
    mockQueryResult({ data: stages("me-id") });
    mockCurrentUser("me-id");
    render(<ChangeRequestApprovals id="chg-1" />);
    fireEvent.click(screen.getByText("Reject"));
    const onError = decideMutateMock.mock.calls[0]![1].onError as (e: unknown) => void;
    onError(new Error("boom"));
    expect(showErrorMock).toHaveBeenCalledWith("Could not reject the change request.", expect.any(Error));
  });

  it("treats an absent canDecide as unknown and leaves the controls enabled", () => {
    mockQueryResult({ data: stages("me-id") });
    mockCurrentUser("me-id");
    render(<ChangeRequestApprovals id="chg-1" />);
    expect(screen.getByText("Approve").closest("button")).toBeEnabled();
  });
});

// A Review approver's controls follow what the backend sends and nothing else:
// the Review rows are REQUESTED (canDecide true) while the change is in Review,
// CANCELLED once it moved on (Customer Review / Closed / Rollback / Canceled), and
// a legacy row left REQUESTED after that arrives with canDecide=false. The panel
// is never told the change's state, so it cannot enable anything from it.
describe("ChangeRequestApprovals — a Review approver across the change request's lifecycle", () => {
  type Status = "REQUESTED" | "CANCELLED" | "APPROVED";
  const reviewData = (me: Status, canDecide?: boolean, extra: BeChangeRequestApprovalsView["approvals"] = []): BeChangeRequestApprovalsView => ({
    approvals: [
      { stage: "Peer Approval", approverType: "STATIC_GROUP", approverName: "Peers", status: "APPROVED", approvers: [{ id: "reviewer", name: "Rita Reviewer", status: "APPROVED" }] },
      { stage: "CAB Approval", approverType: "STATIC_GROUP", approverName: "CAB", status: "APPROVED", approvers: [{ id: "cab", name: "Cam Cab", status: "APPROVED" }] },
      {
        stage: "Review",
        approverType: "STATIC_GROUP",
        approverName: "Peers",
        status: me === "APPROVED" ? "APPROVED" : "PENDING",
        approvers: [
          { id: "reviewer", name: "Rita Reviewer", status: me, ...(canDecide === undefined ? {} : { canDecide }) },
          { id: "colleague", name: "Cole Colleague", status: me === "REQUESTED" ? "REQUESTED" : "CANCELLED" },
        ],
      },
      ...extra,
    ],
  });
  const reviewRow = (name: string): HTMLElement => {
    const row = screen
      .getAllByText(name)
      .map((el) => el.closest("tr"))
      .find((tr): tr is HTMLTableRowElement => tr !== null && within(tr).queryByText("Review") !== null);
    if (!row) throw new Error(`no Review row for ${name}`);
    return row;
  };

  beforeEach(() => {
    vi.clearAllMocks();
    mockCurrentUser("reviewer");
    mockDecideMutation();
  });

  it("is offered Approve / Reject while the change is in Review, and not once it moved to Customer Review, Closed, Rollback or Canceled", () => {
    // The same Review row, as the backend serves it at each step.
    mockQueryResult({ data: reviewData("REQUESTED", true) });
    const { rerender } = render(<ChangeRequestApprovals id="chg-1" />);
    expect(within(reviewRow("Rita Reviewer")).getByRole("button", { name: /^approve$/i })).toBeEnabled();
    expect(within(reviewRow("Rita Reviewer")).getByRole("button", { name: /^reject$/i })).toBeEnabled();

    const customerReview = {
      stage: "Customer Review",
      approverType: "STATIC_GROUP" as const,
      approverName: "Customer Group",
      status: "REQUESTED",
      approvers: [{ id: "contact", name: "Mia Member", status: "REQUESTED" }],
    };
    for (const [moved, extra] of [
      ["Customer Review", [customerReview]],
      ["Closed", []],
      ["Rollback", []],
      ["Canceled", []],
    ] as const) {
      // The change moved on: the reviewers' rows were cancelled, and the panel shows them as such.
      mockQueryResult({ data: reviewData("CANCELLED", false, [...extra]) });
      rerender(<ChangeRequestApprovals id="chg-1" />);
      expect(within(reviewRow("Rita Reviewer")).getByText("Cancelled"), moved).toBeInTheDocument();
      expect(screen.queryByRole("button", { name: /^approve$/i }), moved).not.toBeInTheDocument();
      expect(screen.queryByRole("button", { name: /^reject$/i }), moved).not.toBeInTheDocument();
    }
    expect(decideMutateMock).not.toHaveBeenCalled();
  });

  it("keeps a legacy REQUESTED Review row the backend flags canDecide=false disabled, and submits nothing from it", () => {
    mockQueryResult({ data: reviewData("REQUESTED", false) });
    render(<ChangeRequestApprovals id="chg-1" />);
    const row = reviewRow("Rita Reviewer");
    expect(within(row).getByText("Requested")).toBeInTheDocument();
    expect(within(row).getByRole("button", { name: /^approve$/i })).toBeDisabled();
    expect(within(row).getByRole("button", { name: /^reject$/i })).toBeDisabled();
    fireEvent.click(within(row).getByRole("button", { name: /^approve$/i }));
    fireEvent.click(within(row).getByRole("button", { name: /^reject$/i }));
    expect(decideMutateMock).not.toHaveBeenCalled();
  });

  it("shows the backend's readable 409 when the change moved on while the page was open", () => {
    mockQueryResult({ data: reviewData("REQUESTED", true) });
    render(<ChangeRequestApprovals id="chg-1" />);
    fireEvent.click(within(reviewRow("Rita Reviewer")).getByRole("button", { name: /^approve$/i }));
    const onError = decideMutateMock.mock.calls[0]![1].onError as (e: unknown) => void;
    const message =
      "this approval is no longer pending: the change request is in Closed, but the Review stage can only be decided while it is in Review";
    const err = new (BackendApiError as unknown as new (s: number, m: string) => Error)(409, message);
    onError(err);
    expect(showErrorMock).toHaveBeenCalledWith(message, err);
  });
});

describe("ChangeRequestApprovals — customer group stages (Customer Approval / Customer Review)", () => {
  const customerStage = (
    stage: string,
    approvers: Array<{ id: string; name: string; status: string; canDecide?: boolean }>,
  ) => ({
    approvals: [
      {
        stage,
        approverType: "STATIC_GROUP" as const,
        approverName: "Acme Reviewers",
        status: "REQUESTED",
        approvers,
      },
    ],
  });

  beforeEach(() => {
    vi.clearAllMocks();
    mockDecideMutation();
  });

  it.each([
    ["Customer Approval", "Customer Approval"],
    ["customer_approval", "Customer Approval"],
    ["CUSTOMER REVIEW", "Customer Review"],
    ["customer-review", "Customer Review"],
  ])("labels the stage %s as %s and shows the customer group in the Assignment group column", (raw, label) => {
    mockQueryResult({
      data: customerStage(raw, [{ id: "m1", name: "Member One", status: "REQUESTED" }]),
    });
    mockCurrentUser("someone-else");
    render(<ChangeRequestApprovals id="chg-1" />);
    const row = screen.getByText("Member One").closest("tr")!;
    expect(row).toHaveTextContent(label);
    expect(row).toHaveTextContent("Acme Reviewers");
    expect(row).toHaveTextContent("Requested");
  });

  it("falls back to 'Customer group' when a customer stage carries no group name", () => {
    mockQueryResult({
      data: {
        approvals: [
          { stage: "Customer Review", approverType: "STATIC_GROUP", approverName: null, status: "REQUESTED", approvers: [{ id: "m1", name: "Member One", status: "REQUESTED" }] },
        ],
      },
    });
    mockCurrentUser("someone-else");
    render(<ChangeRequestApprovals id="chg-1" />);
    expect(screen.getByText("Member One").closest("tr")).toHaveTextContent("Customer group");
  });

  it("shows internal and customer stages side by side, each row under its own stage label", () => {
    mockQueryResult({
      data: {
        approvals: [
          { stage: "Peer Approval", approverType: "STATIC_GROUP", approverName: "Peers", status: "APPROVED", approvers: [{ id: "p", name: "Peer One", status: "APPROVED" }] },
          { stage: "Customer Approval", approverType: "STATIC_GROUP", approverName: "Acme Reviewers", status: "REQUESTED", approvers: [{ id: "m1", name: "Member One", status: "REQUESTED" }] },
        ],
      },
    });
    mockCurrentUser("other");
    render(<ChangeRequestApprovals id="chg-1" />);
    expect(screen.getByText("Peer One").closest("tr")).toHaveTextContent("Peer Approval");
    expect(screen.getByText("Member One").closest("tr")).toHaveTextContent("Customer Approval");
  });

  // The customer answers in the customer portal; the CSM Approvals tab only shows the
  // stage. Nobody who signs in here (an internal user) holds a row of it.
  it("shows a CSM user (no row of their own) no Approve/Reject on a live customer stage, whatever its rows say", () => {
    mockQueryResult({
      data: customerStage("Customer Approval", [
        { id: "m1", name: "Member One", status: "REQUESTED", canDecide: false },
        { id: "m2", name: "Member Two", status: "REQUESTED", canDecide: false },
      ]),
    });
    mockCurrentUser("outsider");
    render(<ChangeRequestApprovals id="chg-1" />);
    expect(screen.getByText("Member One").closest("tr")).toHaveTextContent("Requested");
    expect(screen.getByText("Member Two").closest("tr")).toHaveTextContent("Requested");
    expect(screen.queryByRole("button", { name: /approve/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /reject/i })).not.toBeInTheDocument();
  });

  it.each([
    ["Customer Approval", "APPROVED", "Approved"],
    ["Customer Approval", "REJECTED", "Rejected"],
    ["Customer Review", "APPROVED", "Approved"],
    ["Customer Review", "REJECTED", "Rejected"],
  ])(
    "shows the answer the customer gave to the %s stage (%s): the deciding contact's row decided, the co-contacts' Cancelled, no controls",
    (stage, status, shown) => {
      mockQueryResult({
        data: {
          approvals: [
            {
              stage,
              approverType: "STATIC_GROUP",
              approverName: "Customer Group",
              status,
              approvers: [
                { id: "m1", name: "Member One", status, canDecide: false },
                { id: "m2", name: "Member Two", status: "CANCELLED", canDecide: false },
              ],
            },
          ],
        },
      });
      mockCurrentUser("outsider");
      render(<ChangeRequestApprovals id="chg-1" />);
      expect(screen.getByText("Member One").closest("tr")).toHaveTextContent(stage);
      expect(screen.getByText("Member One").closest("tr")).toHaveTextContent(shown);
      expect(screen.getByText("Member Two").closest("tr")).toHaveTextContent("Cancelled");
      expect(screen.queryByRole("button", { name: /^(approve|reject)$/i })).not.toBeInTheDocument();
    },
  );

  it("disables the creator's own customer-stage row (creator is never allowed to decide)", () => {
    mockQueryResult({
      data: customerStage("Customer Approval", [{ id: "me", name: "Me Member", status: "REQUESTED" }]),
    });
    mockCurrentUser("me");
    render(<ChangeRequestApprovals id="chg-1" isCreator />);
    const buttons = within(screen.getByText("Me Member").closest("tr")!).getAllByRole("button", {
      name: /^(Approve|Reject)$/,
    });
    expect(buttons).toHaveLength(2);
    buttons.forEach((b) => expect(b).toBeDisabled());
  });

  it("shows the creator's own row (listed Cancelled by the backend when they are a group member) with no controls", () => {
    mockQueryResult({
      data: customerStage("Customer Approval", [
        { id: "me", name: "Me Member", status: "CANCELLED" },
        { id: "m2", name: "Other Member", status: "REQUESTED", canDecide: false },
      ]),
    });
    mockCurrentUser("me");
    render(<ChangeRequestApprovals id="chg-1" isCreator />);
    expect(screen.getByText("Me Member").closest("tr")).toHaveTextContent("Cancelled");
    expect(screen.queryByRole("button", { name: /^approve$/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /^reject$/i })).not.toBeInTheDocument();
  });
});

describe("ChangeRequestApprovals — opening an Assignment group", () => {
  const PEER_GROUP = "11111111-1111-4111-8111-111111111111";
  const CAB_GROUP = "22222222-2222-4222-8222-222222222222";

  const GROUPS: Record<string, BeGroupDetail> = {
    [PEER_GROUP]: {
      id: PEER_GROUP,
      name: "Example Corp ABT",
      description: "The account's build team",
      email: "abt@example.com",
      manager: { id: "m1", name: "Mia Manager" },
      members: [
        { id: "p1", name: "Pat Peer", email: "pat.peer@example.com", userType: "INTERNAL", role: "lead" },
        { id: "p2", name: "Quinn Peer", email: "quinn.peer@example.com", userType: "INTERNAL", role: "member" },
      ],
      total: 2,
    },
    [CAB_GROUP]: {
      id: CAB_GROUP,
      name: "CAB Approval",
      description: null,
      email: null,
      manager: null,
      members: [
        { id: "c1", name: "Cam Cab", email: "cam.cab@example.com", role: "member" },
        { id: "c2", name: "Cleo Cab", email: "cleo.cab@example.com", role: "member" },
        { id: "c3", name: "Cyd Cab", email: null, role: "member" },
      ],
      total: 3,
    },
  };

  const CONTACTS: BeCustomerContact[] = [
    { id: "k1", name: "Mia Member", email: "mia.member@acme.example" },
    { id: "k2", name: "Max Member", email: "max.member@acme.example" },
  ];

  const approvalsData = (): BeChangeRequestApprovalsView => ({
    approvals: [
      {
        stage: "Peer Approval",
        approverType: "STATIC_GROUP",
        approverName: "Example Corp ABT",
        assignmentGroup: { id: PEER_GROUP, name: "Example Corp ABT" },
        status: "REQUESTED",
        approvers: [{ id: "me", name: "Pat Peer", status: "REQUESTED", canDecide: true }],
      },
      {
        stage: "CAB Approval",
        approverType: "STATIC_GROUP",
        approverName: "CAB Approval",
        assignmentGroup: { id: CAB_GROUP, name: "CAB Approval" },
        status: "REQUESTED",
        approvers: [{ id: "c1", name: "Cam Cab", status: "REQUESTED" }],
      },
      {
        stage: "Customer Approval",
        approverType: "STATIC_GROUP",
        approverName: "Customer Group",
        assignmentGroup: null,
        status: "REQUESTED",
        approvers: [
          { id: "k1-user", name: "Mia Member", status: "REQUESTED" },
          { id: "k2-user", name: "Max Member", status: "REQUESTED" },
        ],
      },
    ],
  });

  const groupResult = (overrides: Record<string, unknown> = {}) => ({
    data: undefined,
    isLoading: false,
    isError: false,
    error: null,
    refetch: vi.fn(),
    ...overrides,
  });

  beforeEach(() => {
    vi.clearAllMocks();
    mockCurrentUser("someone-else");
    mockDecideMutation();
    mockQueryResult({ data: approvalsData() });
    useGroupDetailMock.mockImplementation((id: string | undefined) => groupResult({ data: id ? GROUPS[id] : undefined }));
  });

  const openRow = (approverName: string, groupLabel: string): void => {
    const row = screen.getByText(approverName).closest("tr")!;
    fireEvent.click(within(row).getByRole("button", { name: `View members of ${groupLabel}` }));
  };

  it("renders the Assignment group as a real, keyboard-focusable button that announces a dialog -- and loads nothing until it is clicked", () => {
    render(<ChangeRequestApprovals id="chg-1" customerContacts={CONTACTS} />);

    const link = within(screen.getByText("Pat Peer").closest("tr")!).getByRole("button", {
      name: "View members of Example Corp ABT",
    });
    expect(link.tagName).toBe("BUTTON");
    expect(link).toHaveAttribute("type", "button");
    expect(link).toHaveAttribute("aria-haspopup", "dialog");
    expect(link).toHaveTextContent("Example Corp ABT");
    link.focus();
    expect(link).toHaveFocus();

    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(useGroupDetailMock).not.toHaveBeenCalled();
  });

  it("opens the Peer row's group: titled with its name, with Manager / Group email / Description and its members", () => {
    render(<ChangeRequestApprovals id="chg-1" customerContacts={CONTACTS} />);
    openRow("Pat Peer", "Example Corp ABT");

    const dialog = screen.getByRole("dialog", { name: "Example Corp ABT" });
    expect(useGroupDetailMock).toHaveBeenCalledWith(PEER_GROUP);
    expect(within(dialog).getByText("Manager")).toBeInTheDocument();
    expect(within(dialog).getByText("Mia Manager")).toBeInTheDocument();
    expect(within(dialog).getByText("Group email")).toBeInTheDocument();
    expect(within(dialog).getByRole("link", { name: "abt@example.com" })).toHaveAttribute("href", "mailto:abt@example.com");
    expect(within(dialog).getByText("Description")).toBeInTheDocument();
    expect(within(dialog).getByText("The account's build team")).toBeInTheDocument();

    expect(within(dialog).getByRole("heading", { name: "Group Members (2)" })).toBeInTheDocument();
    const list = within(dialog).getByRole("list", { name: "Group Members (2)" });
    const items = within(list).getAllByRole("listitem");
    expect(items).toHaveLength(2);
    expect(items[0]).toHaveTextContent("Pat Peer");
    expect(items[0]).toHaveTextContent("pat.peer@example.com");
    expect(items[0]).toHaveTextContent("Lead");
    expect(items[1]).toHaveTextContent("Quinn Peer");
    expect(items[1]).not.toHaveTextContent("Lead");
  });

  it("opens the CAB row's own group (not the peer one) and lists the CAB members; absent details are left out", () => {
    render(<ChangeRequestApprovals id="chg-1" customerContacts={CONTACTS} />);
    openRow("Cam Cab", "CAB Approval");

    const dialog = screen.getByRole("dialog", { name: "CAB Approval" });
    expect(useGroupDetailMock).toHaveBeenCalledWith(CAB_GROUP);
    expect(useGroupDetailMock).not.toHaveBeenCalledWith(PEER_GROUP);
    expect(within(dialog).getByRole("heading", { name: "Group Members (3)" })).toBeInTheDocument();
    expect(within(dialog).getAllByRole("listitem").map((li) => li.textContent)).toEqual([
      expect.stringContaining("Cam Cab"),
      expect.stringContaining("Cleo Cab"),
      expect.stringContaining("Cyd Cab"),
    ]);
    expect(within(dialog).queryByText("Manager")).not.toBeInTheDocument();
    expect(within(dialog).queryByText("Group email")).not.toBeInTheDocument();
    expect(within(dialog).queryByText("Description")).not.toBeInTheDocument();
  });

  it("opens the Customer Group's registered contacts from data already on the page -- no request", () => {
    render(<ChangeRequestApprovals id="chg-1" customerContacts={CONTACTS} />);
    openRow("Mia Member", "Customer Group");

    const dialog = screen.getByRole("dialog", { name: "Customer Group" });
    expect(within(dialog).getByRole("heading", { name: "Group Members (2)" })).toBeInTheDocument();
    const items = within(dialog).getAllByRole("listitem");
    expect(items[0]).toHaveTextContent("Mia Member");
    expect(items[0]).toHaveTextContent("mia.member@acme.example");
    expect(items[1]).toHaveTextContent("Max Member");
    expect(items[1]).toHaveTextContent("max.member@acme.example");
    expect(useGroupDetailMock).not.toHaveBeenCalled();
  });

  it("lists the customer stage's own approvers when the change request carries no customerContacts", () => {
    render(<ChangeRequestApprovals id="chg-1" />);
    openRow("Max Member", "Customer Group");

    const dialog = screen.getByRole("dialog", { name: "Customer Group" });
    expect(within(dialog).getByRole("heading", { name: "Group Members (2)" })).toBeInTheDocument();
    expect(within(dialog).getAllByRole("listitem").map((li) => li.textContent)).toEqual([
      "Mia Member",
      "Max Member",
    ]);
    expect(useGroupDetailMock).not.toHaveBeenCalled();
  });

  it("falls back to the stage's own approver row when the project has no registered contacts on the page", () => {
    mockQueryResult({
      data: {
        approvals: [
          {
            stage: "Customer Review",
            approverType: "STATIC_GROUP",
            approverName: "Customer Group",
            assignmentGroup: null,
            status: "PENDING",
            approvers: [{ id: "x", name: "Cancelled Contact", status: "CANCELLED" }],
          },
        ],
      },
    });
    render(<ChangeRequestApprovals id="chg-1" customerContacts={[]} />);
    fireEvent.click(screen.getByRole("button", { name: "View members of Customer Group" }));

    const dialog = screen.getByRole("dialog", { name: "Customer Group" });
    expect(within(dialog).getByRole("heading", { name: "Group Members (1)" })).toBeInTheDocument();
    expect(within(dialog).getByRole("listitem")).toHaveTextContent("Cancelled Contact");
    expect(useGroupDetailMock).not.toHaveBeenCalled();
  });

  it("shows a loading state while the group loads", () => {
    useGroupDetailMock.mockReturnValue(groupResult({ isLoading: true }));
    render(<ChangeRequestApprovals id="chg-1" customerContacts={CONTACTS} />);
    openRow("Pat Peer", "Example Corp ABT");

    const dialog = screen.getByRole("dialog", { name: "Example Corp ABT" });
    expect(within(dialog).getByRole("status", { name: "Loading group members" })).toBeInTheDocument();
    expect(within(dialog).queryByRole("list")).not.toBeInTheDocument();
  });

  it("shows an error state with Try again when the group cannot be loaded", () => {
    const refetch = vi.fn();
    useGroupDetailMock.mockReturnValue(groupResult({ isError: true, error: new Error("boom"), refetch }));
    render(<ChangeRequestApprovals id="chg-1" customerContacts={CONTACTS} />);
    openRow("Pat Peer", "Example Corp ABT");

    const dialog = screen.getByRole("dialog", { name: "Example Corp ABT" });
    expect(within(dialog).getByText("Could not load this group's members.")).toBeInTheDocument();
    fireEvent.click(within(dialog).getByRole("button", { name: "Try again" }));
    expect(refetch).toHaveBeenCalledTimes(1);
    // The approvals table itself is untouched by the failure.
    expect(screen.getByText("Pat Peer")).toBeInTheDocument();
  });

  it("shows an empty state for a group nobody is in", () => {
    useGroupDetailMock.mockReturnValue(
      groupResult({ data: { id: PEER_GROUP, name: "Example Corp ABT", members: [], total: 0 } satisfies BeGroupDetail }),
    );
    render(<ChangeRequestApprovals id="chg-1" customerContacts={CONTACTS} />);
    openRow("Pat Peer", "Example Corp ABT");

    const dialog = screen.getByRole("dialog", { name: "Example Corp ABT" });
    expect(within(dialog).getByRole("heading", { name: "Group Members (0)" })).toBeInTheDocument();
    expect(within(dialog).getByText("This group has no members.")).toBeInTheDocument();
    expect(within(dialog).queryByRole("list")).not.toBeInTheDocument();
  });

  it("says the group could not be found when the backend has no such group (404)", () => {
    useGroupDetailMock.mockReturnValue(groupResult({ data: null }));
    render(<ChangeRequestApprovals id="chg-1" customerContacts={CONTACTS} />);
    openRow("Pat Peer", "Example Corp ABT");

    expect(
      within(screen.getByRole("dialog", { name: "Example Corp ABT" })).getByText(/This group could not be found/),
    ).toBeInTheDocument();
  });

  it("closes with Escape and returns focus to the link that opened it", () => {
    render(<ChangeRequestApprovals id="chg-1" customerContacts={CONTACTS} />);
    const link = within(screen.getByText("Pat Peer").closest("tr")!).getByRole("button", {
      name: "View members of Example Corp ABT",
    });
    link.focus();
    fireEvent.click(link);

    const dialog = screen.getByRole("dialog", { name: "Example Corp ABT" });
    fireEvent.keyDown(dialog, { key: "Escape", code: "Escape" });

    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(link).toHaveFocus();
  });

  it("closes with the Close button, and can then open a different group", () => {
    render(<ChangeRequestApprovals id="chg-1" customerContacts={CONTACTS} />);
    openRow("Pat Peer", "Example Corp ABT");
    fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Close" }));
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();

    openRow("Cam Cab", "CAB Approval");
    expect(screen.getByRole("dialog", { name: "CAB Approval" })).toBeInTheDocument();
  });

  it("leaves a stage that carries no group (ServiceNow source, legacy row) as plain text", () => {
    mockQueryResult({
      data: {
        approvals: [
          {
            stage: "Authorize",
            approverType: "STATIC_GROUP",
            approverName: "Devops Approval",
            status: "REQUESTED",
            approvers: [{ id: "a1", name: "Approver One", status: "REQUESTED" }],
          },
        ],
      },
    });
    render(<ChangeRequestApprovals id="chg-1" />);
    const row = screen.getByText("Approver One").closest("tr")!;
    expect(row).toHaveTextContent("Devops Approval");
    expect(within(row).queryByRole("button", { name: /view members/i })).not.toBeInTheDocument();
  });

  it("does not break Approve / Reject: the controls still decide, with the dialog opened and closed around them", () => {
    mockCurrentUser("me");
    render(<ChangeRequestApprovals id="chg-1" customerContacts={CONTACTS} />);
    const row = screen.getByText("Pat Peer").closest("tr")!;

    fireEvent.click(within(row).getByRole("button", { name: "Approve" }));
    expect(decideMutateMock).toHaveBeenCalledWith({ id: "chg-1", decision: "approved" }, expect.anything());

    openRow("Pat Peer", "Example Corp ABT");
    fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Close" }));

    fireEvent.click(within(row).getByRole("button", { name: "Reject" }));
    expect(decideMutateMock).toHaveBeenLastCalledWith({ id: "chg-1", decision: "rejected" }, expect.anything());
  });
});
