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
import { describe, expect, it, vi, beforeEach } from "vitest";
import "@testing-library/jest-dom/vitest";
import type { CsmTimeCard } from "@features/csm-timecards/types/timeCards";

// CaseTimeCardsPanel pulls in useCaseTimeCards/useDecideTimeCard/
// useDeleteTimeCard (React Query hooks backed by useBackendApi) and
// useCurrentEngineer/useIsTeamLead (auth-derived). None of those are under
// test here -- the "View details" interaction is -- so every one of them is
// mocked directly rather than wiring a QueryClientProvider + backend mock,
// matching how other case-detail-tab tests in this feature folder isolate
// the component under test from its data layer.
const CARD_A: CsmTimeCard = {
  id: "tc-1",
  caseId: "case-1",
  caseNumber: "CS0000001",
  projectId: "proj-1",
  projectName: "Acme Project",
  workDate: "2026-07-01",
  userId: "user-1",
  userName: "Jane Doe",
  state: "submitted",
  billable: true,
  totalMinutes: 45,
  workLogComment: "<p>Investigated the reported latency issue.</p>",
};

const CARD_B: CsmTimeCard = {
  id: "tc-2",
  caseId: "case-1",
  caseNumber: "CS0000001",
  projectId: "proj-1",
  projectName: "Acme Project",
  workDate: "2026-07-02",
  userId: "user-2",
  userName: "John Smith",
  state: "approved",
  billable: false,
  totalMinutes: 20,
  approvedByName: "Lead Person",
};

// CaseTimeCardsPanel also imports BackendApiError directly from
// @api/backend/client (for its own error-message narrowing), which reads
// window.config at module load via @config/apiConfig -- mock it too so
// importing the panel doesn't trip that, same as TimeCardsTable.test.tsx.
vi.mock("@config/apiConfig", () => ({ apiConfig: { backendUrl: "https://example.test" } }));

const refetch = vi.fn();
vi.mock("@features/csm-timecards/api/useTimeCards", () => ({
  useCaseTimeCards: () => ({
    data: { cards: [CARD_A, CARD_B], truncated: false },
    isLoading: false,
    isError: false,
    refetch,
    isFetching: false,
    dataUpdatedAt: Date.now(),
  }),
  useDecideTimeCard: () => ({ mutate: vi.fn(), isPending: false }),
  useDeleteTimeCard: () => ({ mutate: vi.fn(), isPending: false }),
}));
vi.mock("@features/csm-timecards/api/useTimeSheets", () => ({
  useCurrentEngineer: () => ({ id: "user-1", name: "Jane Doe" }),
}));
vi.mock("@features/csm-timecards/hooks/useIsTeamLead", () => ({
  useIsTeamLead: () => false,
}));
vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: vi.fn() }),
}));

import CaseTimeCardsPanel from "@features/csm-timecards/components/CaseTimeCardsPanel";

describe("CaseTimeCardsPanel — view details", () => {
  beforeEach(() => {
    refetch.mockReset();
  });

  it("renders time cards as a grid table with a header row and one row per card", () => {
    render(<CaseTimeCardsPanel caseId="case-1" onLogTime={vi.fn()} onEditTimeCard={vi.fn()} />);

    // Header cells for the grid/subgrid table (matches the "Call requests"
    // tab's CallRequestsTable convention). "State" and "Billable" also occur
    // as row cell values (state chip label, billable label), so those two
    // just assert at least one match rather than exactly one.
    for (const label of ["Preview", "Engineer", "Minutes", "Logged", "Actions"]) {
      expect(screen.getByText(label)).toBeInTheDocument();
    }
    for (const label of ["State", "Billable"]) {
      expect(screen.getAllByText(label).length).toBeGreaterThan(0);
    }

    expect(screen.getByText("Jane Doe")).toBeInTheDocument();
    expect(screen.getByText("John Smith")).toBeInTheDocument();
    expect(screen.getByText("45 min")).toBeInTheDocument();
    expect(screen.getByText("20 min")).toBeInTheDocument();
  });

  it("shows a 'View details' action on every card row, regardless of state or ownership", () => {
    render(<CaseTimeCardsPanel caseId="case-1" onLogTime={vi.fn()} onEditTimeCard={vi.fn()} />);

    expect(screen.getByTestId("timecard-view-tc-1")).toBeInTheDocument();
    expect(screen.getByTestId("timecard-view-tc-2")).toBeInTheDocument();
  });

  it("opens the read-only details view with the row's own fields when clicked", () => {
    render(<CaseTimeCardsPanel caseId="case-1" onLogTime={vi.fn()} onEditTimeCard={vi.fn()} />);

    fireEvent.click(screen.getByTestId("timecard-view-tc-1"));

    const dialog = within(screen.getByRole("dialog"));
    expect(dialog.getByText("Engineer's comment")).toBeInTheDocument();
    expect(dialog.getByText("Investigated the reported latency issue.")).toBeInTheDocument();
  });

  it("opens the details view for an approved, other-engineer's card too", () => {
    render(<CaseTimeCardsPanel caseId="case-1" onLogTime={vi.fn()} onEditTimeCard={vi.fn()} />);

    fireEvent.click(screen.getByTestId("timecard-view-tc-2"));

    // The table row's own State column also shows the decision summary
    // ("Approved by: Lead Person"), so scope to the dialog specifically.
    expect(
      within(screen.getByRole("dialog")).getByText("Approved by: Lead Person"),
    ).toBeInTheDocument();
  });

  it("closes the details view when its row's action is clicked again", () => {
    render(<CaseTimeCardsPanel caseId="case-1" onLogTime={vi.fn()} onEditTimeCard={vi.fn()} />);

    const viewButtonA = screen.getByTestId("timecard-view-tc-1");
    fireEvent.click(viewButtonA);
    expect(screen.getByText("Engineer's comment")).toBeInTheDocument();

    fireEvent.click(viewButtonA);
    expect(screen.queryByText("Engineer's comment")).not.toBeInTheDocument();
  });

  it("closes the details view via its own Close button", () => {
    render(<CaseTimeCardsPanel caseId="case-1" onLogTime={vi.fn()} onEditTimeCard={vi.fn()} />);

    fireEvent.click(screen.getByTestId("timecard-view-tc-1"));
    fireEvent.click(screen.getByRole("button", { name: "Close" }));

    expect(screen.queryByText("Engineer's comment")).not.toBeInTheDocument();
  });

  it("shows Edit/Delete actions only for your own still-submitted card", () => {
    render(<CaseTimeCardsPanel caseId="case-1" onLogTime={vi.fn()} onEditTimeCard={vi.fn()} />);

    // CARD_A (tc-1) is owned by the signed-in engineer (user-1) and
    // "submitted" -- editable. CARD_B (tc-2) belongs to someone else and is
    // "approved" -- neither editable.
    expect(screen.getByRole("button", { name: "Edit time card" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Delete time card" })).toBeInTheDocument();
  });
});
