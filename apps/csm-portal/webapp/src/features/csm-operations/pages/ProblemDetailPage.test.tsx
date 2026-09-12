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
import { describe, expect, it, vi, beforeEach } from "vitest";
import "@testing-library/jest-dom/vitest";
import type { UseQueryResult } from "@tanstack/react-query";
import type { BeProblemDetail } from "@api/backend/types";

const navigateMock = vi.fn();
const useGetProblemMock = vi.fn();
const useLocationMock = vi.fn<() => { state: { from?: string } | null }>(() => ({ state: null }));
const patchMutateMock = vi.fn();
const showErrorMock = vi.fn();
const editProblemDialogMock = vi.fn();
const problemFixNotesDialogMock = vi.fn();
let patchIsPending = false;

// The backend client reads runtime config at module load, which isn't
// present under vitest. The page imports `BackendApiError` from it directly,
// so stub the module with a real class (so `instanceof` still works) — same
// approach as CsmIncidentDetailPage.test.tsx / CsmChangeRequestDetailPage.test.tsx.
vi.mock("@api/backend/client", () => ({
  BackendApiError: class BackendApiError extends Error {
    status: number;
    constructor(status: number, message: string) {
      super(message);
      this.status = status;
    }
  },
}));

vi.mock("react-router", () => ({
  useParams: () => ({ id: "prb-1" }),
  useLocation: () => useLocationMock(),
}));
vi.mock("@hooks/useNavTransition", () => ({
  useNavTransition: () => navigateMock,
}));
vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: showErrorMock }),
}));
vi.mock("@features/csm-operations/api/useGetProblem", () => ({
  useGetProblem: () => useGetProblemMock(),
}));
vi.mock("@features/csm-operations/api/usePatchProblem", () => ({
  usePatchProblem: () => ({
    mutate: patchMutateMock,
    isPending: patchIsPending,
    isError: false,
    error: null,
  }),
}));
// Exercised in isolation by their own test files; here we only assert this
// page opens them and wires the expected props.
vi.mock("@features/csm-operations/components/EditProblemDialog", () => ({
  default: (props: unknown) => {
    editProblemDialogMock(props);
    return null;
  },
}));
vi.mock("@features/csm-operations/components/ProblemFixNotesDialog", () => ({
  default: (props: unknown) => {
    problemFixNotesDialogMock(props);
    return null;
  },
}));

// Imported after the mocks above so the module picks them up.
import ProblemDetailPage from "@features/csm-operations/pages/ProblemDetailPage";

const BASE_PROBLEM: BeProblemDetail = {
  id: "prb-1",
  number: "PRB0040157",
  subject: "Intermittent 502s on the gateway",
  state: "CLOSED",
  priority: "High",
  category: "",
  subcategory: null,
  originCase: { id: "inc-1", number: "INC0012345" },
  primaryIncident: { id: "inc-1", number: "INC0012345" },
  linkedIncidents: [
    { id: "inc-1", number: "INC0012345" },
    { id: "inc-2", number: "INC0012399" },
  ],
  linkedChangeRequest: { id: "chg-1", number: "CHG0009988" },
  assignedTo: { id: "user-1", name: "Jane Doe" },
  resolutionCode: "Fixed",
  causeNotes: "Root cause was a bad config push.",
  fixNotes: "Rolled back the config.",
  workaround: "Restart the pod.",
  resolvedOn: "2026-01-01T00:00:00Z",
  resolvedBy: { id: "user-1", name: "Jane Doe" },
  openedOn: "2025-12-01T00:00:00Z",
  closedOn: "2026-01-02T00:00:00Z",
};

function mockQueryResult(
  overrides: Partial<UseQueryResult<BeProblemDetail | null, Error>>,
): void {
  useGetProblemMock.mockReturnValue({
    data: null,
    isLoading: false,
    isError: false,
    error: null,
    ...overrides,
  });
}

describe("ProblemDetailPage", () => {
  beforeEach(() => {
    patchMutateMock.mockReset();
    showErrorMock.mockReset();
    editProblemDialogMock.mockReset();
    problemFixNotesDialogMock.mockReset();
    patchIsPending = false;
  });

  it("renders a loading skeleton while the query is pending", () => {
    mockQueryResult({ isLoading: true });
    const { container } = render(<ProblemDetailPage />);
    expect(container.querySelectorAll(".MuiSkeleton-root").length).toBeGreaterThan(0);
  });

  it("renders an error state when the query fails", () => {
    mockQueryResult({ isError: true, error: new Error("boom") });
    render(<ProblemDetailPage />);
    expect(screen.getByText(/Could not load problem/i)).toBeInTheDocument();
  });

  it("renders a not-found state when the problem is null", () => {
    mockQueryResult({ data: null });
    render(<ProblemDetailPage />);
    expect(screen.getByText(/Problem not found/i)).toBeInTheDocument();
  });

  it("renders subject, number, and state for a loaded problem", () => {
    mockQueryResult({ data: BASE_PROBLEM });
    render(<ProblemDetailPage />);
    expect(screen.getByText("Intermittent 502s on the gateway")).toBeInTheDocument();
    expect(screen.getByText("PRB0040157")).toBeInTheDocument();
    // "Closed" also appears as the (unrelated) label of the closedOn
    // MetaCell, so scope the assertion to the state Chip specifically.
    expect(screen.getByText("Closed", { selector: ".MuiChip-label" })).toBeInTheDocument();
  });

  it("renders an empty category/subcategory gracefully, without a stray blank chip", () => {
    mockQueryResult({ data: BASE_PROBLEM });
    render(<ProblemDetailPage />);
    // MetaCell renders "—" for both category and subcategory here (empty string / null).
    const dashes = screen.getAllByText("—");
    expect(dashes.length).toBeGreaterThanOrEqual(2);
  });

  it("renders every linkedIncidents entry as a separate reference, not just one", () => {
    mockQueryResult({ data: BASE_PROBLEM });
    render(<ProblemDetailPage />);
    // "INC0012345" appears 3 times: originCase (plain text), primaryIncident
    // (chip), and its entry within linkedIncidents (chip) — all distinct
    // renders, confirming linkedIncidents is treated as a real list.
    expect(screen.getAllByText("INC0012345")).toHaveLength(3);
    expect(screen.getByText("INC0012399")).toBeInTheDocument();
  });

  it("renders the linked change request as a clickable reference to the CR route", () => {
    mockQueryResult({ data: BASE_PROBLEM });
    render(<ProblemDetailPage />);
    screen.getByText("CHG0009988").closest('[role="button"]')?.dispatchEvent(
      new MouseEvent("click", { bubbles: true }),
    );
    expect(navigateMock).toHaveBeenCalledWith("/operations/change-requests/chg-1");
  });

  it("renders originCase as plain, non-navigable text (may not actually be a Case)", () => {
    mockQueryResult({ data: BASE_PROBLEM });
    render(<ProblemDetailPage />);
    const originLabels = screen.getAllByText("INC0012345");
    // None of the two "INC0012345" renders that are plain text (not a clickable chip)
    // should trigger navigation when clicked directly as text.
    const plainTextOccurrence = originLabels.find((el) => el.closest('[role="button"]') === null);
    expect(plainTextOccurrence).toBeDefined();
  });

  it("renders resolution notes when present", () => {
    mockQueryResult({ data: BASE_PROBLEM });
    render(<ProblemDetailPage />);
    expect(screen.getByText("Root cause was a bad config push.")).toBeInTheDocument();
    expect(screen.getByText("Rolled back the config.")).toBeInTheDocument();
    expect(screen.getByText("Restart the pod.")).toBeInTheDocument();
  });

  it("navigates back to the problems tab from the back button when no origin is known", () => {
    useLocationMock.mockReturnValue({ state: null });
    mockQueryResult({ data: BASE_PROBLEM });
    render(<ProblemDetailPage />);
    screen.getByRole("button", { name: "Back" }).click();
    expect(navigateMock).toHaveBeenCalledWith("/operations?tab=problems");
  });

  it("navigates back to the captured origin (e.g. a dashboard widget) when one is known", () => {
    useLocationMock.mockReturnValue({ state: { from: "/dashboard" } });
    mockQueryResult({ data: BASE_PROBLEM });
    render(<ProblemDetailPage />);
    screen.getByRole("button", { name: "Back" }).click();
    expect(navigateMock).toHaveBeenCalledWith("/dashboard");
  });

  it("navigates back to a captured origin's exact pathname + search (e.g. the Operations tab's own filters)", () => {
    useLocationMock.mockReturnValue({
      state: { from: "/operations?tab=problems&state=closed" },
    });
    mockQueryResult({ data: BASE_PROBLEM });
    render(<ProblemDetailPage />);
    screen.getByRole("button", { name: "Back" }).click();
    expect(navigateMock).toHaveBeenCalledWith("/operations?tab=problems&state=closed");
  });

  it("shows a transition action button for a New problem (assess) and PATCHes with { transition } on click", () => {
    mockQueryResult({ data: { ...BASE_PROBLEM, state: "NEW" } });
    render(<ProblemDetailPage />);
    const btn = screen.getByRole("button", { name: /Move to Assess/i });
    fireEvent.click(btn);
    expect(patchMutateMock).toHaveBeenCalledWith(
      { id: "prb-1", patch: { transition: "assess" } },
      expect.objectContaining({ onError: expect.any(Function) }),
    );
  });

  it("opens the optional fix-notes dialog instead of PATCHing directly for the fix transition", () => {
    mockQueryResult({ data: { ...BASE_PROBLEM, state: "ROOT_CAUSE_ANALYSIS" } });
    render(<ProblemDetailPage />);
    fireEvent.click(screen.getByRole("button", { name: /Move to Fix In Progress/i }));
    expect(patchMutateMock).not.toHaveBeenCalled();
    expect(problemFixNotesDialogMock).toHaveBeenCalled();
  });

  it("renders no transition action button for a Closed (terminal) problem", () => {
    mockQueryResult({ data: { ...BASE_PROBLEM, state: "CLOSED" } });
    render(<ProblemDetailPage />);
    expect(screen.queryByRole("button", { name: /Move to/i })).not.toBeInTheDocument();
  });

  it("opens EditProblemDialog with the current problem when Edit is clicked", () => {
    mockQueryResult({ data: BASE_PROBLEM });
    render(<ProblemDetailPage />);
    expect(editProblemDialogMock).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "Edit" }));
    expect(editProblemDialogMock).toHaveBeenCalledWith(
      expect.objectContaining({ problem: BASE_PROBLEM, isSaving: false }),
    );
  });

  it("shows an error banner message when a transition PATCH fails with a caller-actionable status", () => {
    mockQueryResult({ data: { ...BASE_PROBLEM, state: "NEW" } });
    patchMutateMock.mockImplementation((_input, opts) => {
      opts?.onError?.(new Error("State transition rejected"));
    });
    render(<ProblemDetailPage />);
    fireEvent.click(screen.getByRole("button", { name: /Move to Assess/i }));
    expect(showErrorMock).toHaveBeenCalled();
  });
});
