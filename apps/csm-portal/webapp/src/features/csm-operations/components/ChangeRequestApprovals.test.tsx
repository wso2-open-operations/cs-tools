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
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import type { UseQueryResult } from "@tanstack/react-query";
import type { BeChangeRequestApprovalsView } from "@api/backend/types";

const useGetChangeRequestApprovalsMock = vi.fn();
const useCurrentUserMock = vi.fn();
const useDecideChangeRequestApprovalMock = vi.fn();
const showErrorMock = vi.fn();
const decideMutateMock = vi.fn();

// The backend client reads runtime config (`CSM_PORTAL_BACKEND_BASE_URL`) at
// module load, which isn't present under vitest. QueryErrorState imports
// `BackendApiError` from it, so stub the module (same approach as
// CsmAnnouncementsPage.test.tsx).
vi.mock("@api/backend/client", () => ({
  BackendApiError: class BackendApiError extends Error {},
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

// Imported after the mocks above so the modules pick them up.
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

  it("collapses NOT_REQUIRED approvers behind a default-collapsed disclosure, sorted after notable ones", () => {
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
              { id: "a3", name: "Not Needed Two", status: "NOT_REQUIRED" },
            ],
          },
        ],
      },
    });
    render(<ChangeRequestApprovals id="chg-1" />);

    // Open the stage accordion.
    fireEvent.click(screen.getByText("Authorize"));

    // The one notable approver is visible immediately.
    expect(screen.getByText("Approved Alice")).toBeInTheDocument();
    // The two NOT_REQUIRED approvers are collapsed by default.
    expect(screen.queryByText("Not Needed One")).not.toBeInTheDocument();
    expect(screen.queryByText("Not Needed Two")).not.toBeInTheDocument();
    expect(screen.getByText("2 not required")).toBeInTheDocument();

    // Expanding reveals them.
    fireEvent.click(screen.getByText("2 not required"));
    expect(screen.getByText("Not Needed One")).toBeInTheDocument();
    expect(screen.getByText("Not Needed Two")).toBeInTheDocument();
  });

  it("suffixes duplicate stage labels with '(N of M)', leaving single-occurrence stages untouched", () => {
    mockQueryResult({
      data: {
        approvals: [
          {
            stage: "Authorize",
            approverType: "STATIC_GROUP",
            approverName: "First group",
            status: "REQUESTED",
            approvers: [],
          },
          {
            stage: "Authorize",
            approverType: "STATIC_GROUP",
            approverName: "Second group",
            status: "APPROVED",
            approvers: [],
          },
          {
            stage: "Assess",
            approverType: "STATIC_GROUP",
            approverName: "Assess group",
            status: "APPROVED",
            approvers: [],
          },
        ],
      },
    });
    render(<ChangeRequestApprovals id="chg-1" />);

    expect(screen.getByText("Authorize (1 of 2)")).toBeInTheDocument();
    expect(screen.getByText("Authorize (2 of 2)")).toBeInTheDocument();
    // Single-occurrence stage keeps its plain label.
    expect(screen.getByText("Assess")).toBeInTheDocument();
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
    fireEvent.click(screen.getByText("Authorize"));

    expect(screen.getByText("Unnamed approver")).toBeInTheDocument();
    expect(screen.queryByText("Unknown approver")).not.toBeInTheDocument();
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

      fireEvent.click(screen.getByText("Authorize"));

      expect(screen.getByText("Current User")).toBeInTheDocument();
      expect(screen.getByText("Other Approver")).toBeInTheDocument();
      expect(screen.getAllByText("Approve")).toHaveLength(1);
      expect(screen.getAllByText("Reject")).toHaveLength(1);
    });

    it("hides Approve/Reject when no user is signed in", () => {
      mockQueryResult({ data: approvalsWithMyPending });
      mockCurrentUser(undefined);
      render(<ChangeRequestApprovals id="chg-1" />);

      fireEvent.click(screen.getByText("Authorize"));

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

      fireEvent.click(screen.getByText("Authorize"));

      expect(screen.queryByText("Approve")).not.toBeInTheDocument();
      expect(screen.queryByText("Reject")).not.toBeInTheDocument();
    });

    it("submits the decision with the CR id when Approve is clicked", () => {
      mockQueryResult({ data: approvalsWithMyPending });
      mockCurrentUser("me-id");
      render(<ChangeRequestApprovals id="chg-1" />);

      fireEvent.click(screen.getByText("Authorize"));
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

      fireEvent.click(screen.getByText("Authorize"));
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

      fireEvent.click(screen.getByText("Authorize"));

      expect(screen.getByText("Approve").closest("button")).toBeDisabled();
      expect(screen.getByText("Reject").closest("button")).toBeDisabled();
    });
  });
});
