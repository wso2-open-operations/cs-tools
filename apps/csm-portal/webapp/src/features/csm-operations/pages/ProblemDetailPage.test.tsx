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
import type { UseQueryResult } from "@tanstack/react-query";
import type { BeProblemDetail } from "@api/backend/types";

const navigateMock = vi.fn();
const useGetProblemMock = vi.fn();

vi.mock("react-router", () => ({
  useParams: () => ({ id: "prb-1" }),
}));
vi.mock("@hooks/useNavTransition", () => ({
  useNavTransition: () => navigateMock,
}));
vi.mock("@features/csm-operations/api/useGetProblem", () => ({
  useGetProblem: () => useGetProblemMock(),
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
  resolvedAt: "2026-01-01T00:00:00Z",
  resolvedBy: { id: "user-1", name: "Jane Doe" },
  openedAt: "2025-12-01T00:00:00Z",
  closedAt: "2026-01-02T00:00:00Z",
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
    // "Closed" also appears as the (unrelated) label of the closedAt
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

  it("navigates back to the problems tab from the back button", () => {
    mockQueryResult({ data: BASE_PROBLEM });
    render(<ProblemDetailPage />);
    screen.getByRole("button", { name: /back to problems/i }).click();
    expect(navigateMock).toHaveBeenCalledWith("/operations?tab=problems");
  });
});
