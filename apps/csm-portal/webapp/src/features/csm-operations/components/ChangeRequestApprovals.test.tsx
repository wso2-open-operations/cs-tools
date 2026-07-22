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
import "@testing-library/jest-dom/vitest";
import type { UseQueryResult } from "@tanstack/react-query";
import type { BeChangeRequestApprovalsView } from "@api/backend/types";

const useGetChangeRequestApprovalsMock = vi.fn();

// The backend client reads runtime config (`CSM_PORTAL_BACKEND_BASE_URL`) at
// module load, which isn't present under vitest. QueryErrorState imports
// `BackendApiError` from it, so stub the module (same approach as
// CsmAnnouncementsPage.test.tsx).
vi.mock("@api/backend/client", () => ({
  BackendApiError: class BackendApiError extends Error {},
  useBackendApi: () => ({ get: vi.fn() }),
}));

vi.mock("@features/csm-operations/api/useGetChangeRequestApprovals", () => ({
  useGetChangeRequestApprovals: () => useGetChangeRequestApprovalsMock(),
}));

// Imported after the mock above so the module picks it up.
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

describe("ChangeRequestApprovals", () => {
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
});
