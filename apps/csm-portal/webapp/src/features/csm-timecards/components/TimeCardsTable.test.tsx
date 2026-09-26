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

import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import type { ReactElement } from "react";
import { MemoryRouter, Route, Routes, useLocation } from "react-router";
import TimeCardsTable from "@features/csm-timecards/components/TimeCardsTable";
import type { CsmTimeCard } from "@features/csm-timecards/types/timeCards";

// TimeCardsTable's "View details" action now renders
// TimeCardCasePreviewDrawer, which pulls in useGetCsmCaseDetail /
// useGetCsmCaseComments -- both read window.config at module load (via
// @config/apiConfig) and call useBackendApi() unconditionally, even while
// their own query stays disabled outside the View-details test below. Mock
// both so importing this component doesn't trip either; get/postMock are
// shared, named instances (reset per test) so that test can configure real
// resolved values instead of the default `undefined`.
const getMock = vi.fn();
const postMock = vi.fn();
vi.mock("@config/apiConfig", () => ({ apiConfig: { backendUrl: "https://example.test" } }));
vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({
    get: getMock,
    post: postMock,
    patch: vi.fn(),
    put: vi.fn(),
    postEmpty: vi.fn(),
    del: vi.fn(),
    getBlob: vi.fn(),
  }),
}));

beforeEach(() => {
  getMock.mockReset();
  postMock.mockReset();
});

// CasePreviewContent (rendered inside TimeCardCasePreviewDrawer once its case
// query resolves) links out via react-router's Link, so every render needs
// Router context in the tree even on tests that never open the drawer.
function renderWithClient(ui: ReactElement): ReturnType<typeof render> {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>{ui}</MemoryRouter>
    </QueryClientProvider>,
  );
}

function LocationProbe(): ReactElement {
  const location = useLocation();
  return <div data-testid="location-probe">{location.pathname}</div>;
}

/** Like renderWithClient, but with a real destination route mounted so a
 * click that navigates (CasePreviewContent's "View full details" link) can
 * be asserted against where it actually lands, not just that some handler
 * fired -- see DashboardWidgetTile.test.tsx for the same pattern. */
function renderWithRoutes(ui: ReactElement, destinationPath: string): ReturnType<typeof render> {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={["/"]}>
        <Routes>
          <Route path="/" element={ui} />
          <Route path={destinationPath} element={<LocationProbe />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

const CARD: CsmTimeCard = {
  id: "tc-1",
  caseId: "case-1",
  caseNumber: "CS0352584",
  projectId: "proj-1",
  projectName: "Acme Project",
  workDate: "2026-07-01",
  userId: "user-1",
  userName: "Jane Doe",
  state: "submitted",
  billable: true,
  totalMinutes: 30,
};

const APPROVED_CARD: CsmTimeCard = {
  ...CARD,
  id: "tc-2",
  caseNumber: "CS0352585",
  userName: "John Roe",
  state: "approved",
};

const ROLE_CTX = { isOwner: false, isApprover: false, isAdmin: false };
const APPROVER_ROLE_CTX = { isOwner: false, isApprover: true, isAdmin: false };

describe("TimeCardsTable column visibility", () => {
  it("shows the Case column but not the Engineer column on the personal view (My time sheets)", () => {
    renderWithClient(
      <TimeCardsTable
        cards={[CARD]}
        isLoading={false}
        emptyText="No cards"
        groupBy="case"
        roleFor={() => ROLE_CTX}
        onCardAction={vi.fn()}
      />,
    );

    expect(screen.getByRole("columnheader", { name: "Case" })).toBeInTheDocument();
    expect(
      screen.queryByRole("columnheader", { name: "Engineer" }),
    ).not.toBeInTheDocument();

    expect(screen.getByText("CS0352584")).toBeInTheDocument();
    expect(screen.queryByText("Jane Doe")).not.toBeInTheDocument();
  });

  it("shows both the Case and Engineer columns when showEngineerColumn is set (All / Approvals)", () => {
    renderWithClient(
      <TimeCardsTable
        cards={[CARD]}
        isLoading={false}
        emptyText="No cards"
        groupBy="case"
        showEngineerColumn
        roleFor={() => ROLE_CTX}
        onCardAction={vi.fn()}
      />,
    );

    expect(screen.getByRole("columnheader", { name: "Case" })).toBeInTheDocument();
    expect(screen.getByRole("columnheader", { name: "Engineer" })).toBeInTheDocument();

    expect(screen.getByText("CS0352584")).toBeInTheDocument();
    expect(screen.getByText("Jane Doe")).toBeInTheDocument();
  });
});

describe("TimeCardsTable bulk-select (Approvals tab)", () => {
  it("renders no checkboxes at all when selectable is omitted (default, every other tab)", () => {
    renderWithClient(
      <TimeCardsTable
        cards={[CARD]}
        isLoading={false}
        emptyText="No cards"
        groupBy="case"
        roleFor={() => APPROVER_ROLE_CTX}
        onCardAction={vi.fn()}
      />,
    );

    expect(screen.queryAllByRole("checkbox")).toHaveLength(0);
  });

  it("only shows a row checkbox for cards the viewer can actually approve, not e.g. an already-approved one", () => {
    renderWithClient(
      <TimeCardsTable
        cards={[CARD, APPROVED_CARD]}
        isLoading={false}
        emptyText="No cards"
        groupBy="case"
        selectable
        selectedIds={new Set()}
        onToggleSelect={vi.fn()}
        onToggleSelectAll={vi.fn()}
        roleFor={() => APPROVER_ROLE_CTX}
        onCardAction={vi.fn()}
      />,
    );

    // One header select-all checkbox + one row checkbox (the submitted card
    // only) — the approved card gets no checkbox at all.
    expect(screen.getAllByRole("checkbox")).toHaveLength(2);
    expect(
      screen.getByRole("checkbox", { name: /select time card cs0352584/i }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("checkbox", { name: /select time card cs0352585/i }),
    ).not.toBeInTheDocument();
  });

  it("calls onToggleSelect with the card when its row checkbox is clicked", () => {
    const onToggleSelect = vi.fn();
    renderWithClient(
      <TimeCardsTable
        cards={[CARD]}
        isLoading={false}
        emptyText="No cards"
        groupBy="case"
        selectable
        selectedIds={new Set()}
        onToggleSelect={onToggleSelect}
        onToggleSelectAll={vi.fn()}
        roleFor={() => APPROVER_ROLE_CTX}
        onCardAction={vi.fn()}
      />,
    );

    fireEvent.click(screen.getByRole("checkbox", { name: /select time card cs0352584/i }));
    expect(onToggleSelect).toHaveBeenCalledWith(CARD);
  });

  it("reflects selectedIds on the row checkbox and shows the header checkbox checked once every selectable row is selected", () => {
    renderWithClient(
      <TimeCardsTable
        cards={[CARD]}
        isLoading={false}
        emptyText="No cards"
        groupBy="case"
        selectable
        selectedIds={new Set([CARD.id])}
        onToggleSelect={vi.fn()}
        onToggleSelectAll={vi.fn()}
        roleFor={() => APPROVER_ROLE_CTX}
        onCardAction={vi.fn()}
      />,
    );

    const rowCheckbox = screen.getByRole("checkbox", { name: /select time card cs0352584/i });
    const headerCheckbox = screen.getByRole("checkbox", { name: /select all approvable/i });
    expect(rowCheckbox).toBeChecked();
    expect(headerCheckbox).toBeChecked();
  });

  it("calls onToggleSelectAll with only the selectable cards on the page when the header checkbox is clicked", () => {
    const onToggleSelectAll = vi.fn();
    renderWithClient(
      <TimeCardsTable
        cards={[CARD, APPROVED_CARD]}
        isLoading={false}
        emptyText="No cards"
        groupBy="case"
        selectable
        selectedIds={new Set()}
        onToggleSelect={vi.fn()}
        onToggleSelectAll={onToggleSelectAll}
        roleFor={() => APPROVER_ROLE_CTX}
        onCardAction={vi.fn()}
      />,
    );

    fireEvent.click(screen.getByRole("checkbox", { name: /select all approvable/i }));
    expect(onToggleSelectAll).toHaveBeenCalledWith([CARD]);
  });

  it("disables the header select-all checkbox when no row on the page is approvable", () => {
    renderWithClient(
      <TimeCardsTable
        cards={[APPROVED_CARD]}
        isLoading={false}
        emptyText="No cards"
        groupBy="case"
        selectable
        selectedIds={new Set()}
        onToggleSelect={vi.fn()}
        onToggleSelectAll={vi.fn()}
        roleFor={() => APPROVER_ROLE_CTX}
        onCardAction={vi.fn()}
      />,
    );

    expect(screen.getByRole("checkbox", { name: /select all approvable/i })).toBeDisabled();
  });
});

describe("TimeCardsTable row action buttons (Approvals tab)", () => {
  it("renders Approve/Reject as icon-only buttons, not visible text, while still exposing an accessible name", () => {
    renderWithClient(
      <TimeCardsTable
        cards={[CARD]}
        isLoading={false}
        emptyText="No cards"
        groupBy="case"
        showActionsColumn
        roleFor={() => APPROVER_ROLE_CTX}
        onCardAction={vi.fn()}
      />,
    );

    expect(screen.getByRole("button", { name: "Approve" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Reject" })).toBeInTheDocument();
    expect(screen.queryByText("Approve")).not.toBeInTheDocument();
    expect(screen.queryByText("Reject")).not.toBeInTheDocument();
  });

  it("calls onCardAction when an enabled row action button is clicked", () => {
    const onCardAction = vi.fn();
    renderWithClient(
      <TimeCardsTable
        cards={[CARD]}
        isLoading={false}
        emptyText="No cards"
        groupBy="case"
        showActionsColumn
        roleFor={() => APPROVER_ROLE_CTX}
        onCardAction={onCardAction}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Approve" }));
    expect(onCardAction).toHaveBeenCalledWith(CARD, "approve");
  });

  // Even a single checked row disables the row-level buttons -- mixing
  // "acting directly on this row" with "this row is part of a selection" at
  // the same time reads as confusing regardless of count (explicit product
  // follow-up on this feature: "must be implemented for single selection as
  // well... otherwise user gets confused").
  it("disables row action buttons once even a single row is selected", () => {
    renderWithClient(
      <TimeCardsTable
        cards={[CARD]}
        isLoading={false}
        emptyText="No cards"
        groupBy="case"
        showActionsColumn
        selectable
        selectedIds={new Set([CARD.id])}
        onToggleSelect={vi.fn()}
        onToggleSelectAll={vi.fn()}
        roleFor={() => APPROVER_ROLE_CTX}
        onCardAction={vi.fn()}
      />,
    );

    expect(screen.getByRole("button", { name: "Approve" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Reject" })).toBeDisabled();
  });

  // Clicking one row's own Approve/Reject while several rows are checked is
  // ambiguous (act on just this row, or the whole selection?) -- disabled
  // for every row once any are selected, per review feedback on this feature.
  it("disables every row's Approve/Reject once multiple rows are selected, across the whole page not just the selected rows", () => {
    renderWithClient(
      <TimeCardsTable
        cards={[CARD, { ...CARD, id: "tc-3", caseNumber: "CS0352586" }]}
        isLoading={false}
        emptyText="No cards"
        groupBy="case"
        showActionsColumn
        selectable
        selectedIds={new Set([CARD.id, "tc-3"])}
        onToggleSelect={vi.fn()}
        onToggleSelectAll={vi.fn()}
        roleFor={() => APPROVER_ROLE_CTX}
        onCardAction={vi.fn()}
      />,
    );

    for (const button of screen.getAllByRole("button", { name: "Approve" })) {
      expect(button).toBeDisabled();
    }
    for (const button of screen.getAllByRole("button", { name: "Reject" })) {
      expect(button).toBeDisabled();
    }
  });
});

describe("TimeCardsTable edit action", () => {
  it("shows the edit icon for the card's own owner on a submitted card, and calls onCardAction with \"edit\"", () => {
    const onCardAction = vi.fn();
    renderWithClient(
      <TimeCardsTable
        cards={[CARD]}
        isLoading={false}
        emptyText="No cards"
        groupBy="case"
        roleFor={() => ({ isOwner: true, isApprover: false, isAdmin: false })}
        onCardAction={onCardAction}
      />,
    );

    const editButton = screen.getByTestId(`timecard-edit-${CARD.id}`);
    expect(editButton).toBeInTheDocument();
    fireEvent.click(editButton);
    expect(onCardAction).toHaveBeenCalledWith(CARD, "edit");
  });

  it("shows the edit icon regardless of showActionsColumn (unlike approve/reject)", () => {
    renderWithClient(
      <TimeCardsTable
        cards={[CARD]}
        isLoading={false}
        emptyText="No cards"
        groupBy="case"
        showActionsColumn={false}
        roleFor={() => ({ isOwner: true, isApprover: false, isAdmin: false })}
        onCardAction={vi.fn()}
      />,
    );

    expect(screen.getByTestId(`timecard-edit-${CARD.id}`)).toBeInTheDocument();
  });

  it("hides the edit icon for a non-owner (e.g. an approver viewing someone else's card)", () => {
    renderWithClient(
      <TimeCardsTable
        cards={[CARD]}
        isLoading={false}
        emptyText="No cards"
        groupBy="case"
        showActionsColumn
        roleFor={() => ({ isOwner: false, isApprover: true, isAdmin: false })}
        onCardAction={vi.fn()}
      />,
    );

    expect(screen.queryByTestId(`timecard-edit-${CARD.id}`)).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Approve" })).toBeInTheDocument();
  });

  it("hides the edit icon once the card is no longer submitted, even for the owner", () => {
    renderWithClient(
      <TimeCardsTable
        cards={[{ ...CARD, state: "approved" }]}
        isLoading={false}
        emptyText="No cards"
        groupBy="case"
        roleFor={() => ({ isOwner: true, isApprover: false, isAdmin: false })}
        onCardAction={vi.fn()}
      />,
    );

    expect(screen.queryByTestId(`timecard-edit-${CARD.id}`)).not.toBeInTheDocument();
  });
});

describe("TimeCardsTable View details drawer", () => {
  it("opens the preview drawer and navigates to the case detail page via 'View full details'", async () => {
    getMock.mockResolvedValue({
      id: "case-1",
      number: "CS0352584",
      subject: "Cluster fails to start",
    });
    postMock.mockResolvedValue({ comments: [] });

    renderWithRoutes(
      <TimeCardsTable
        cards={[CARD]}
        isLoading={false}
        emptyText="No cards"
        groupBy="case"
        roleFor={() => ROLE_CTX}
        onCardAction={vi.fn()}
      />,
      "/cases/case-1",
    );

    fireEvent.click(screen.getByRole("button", { name: "View details" }));
    await waitFor(() => expect(screen.getByText("Cluster fails to start")).toBeInTheDocument());

    fireEvent.click(screen.getByRole("link", { name: "View full details" }));
    await waitFor(() => expect(screen.getByTestId("location-probe")).toBeInTheDocument());
  });

  it("closes the preview when the same row's eye is clicked again", () => {
    getMock.mockReturnValue(new Promise(() => {}));
    renderWithClient(
      <TimeCardsTable
        cards={[CARD]}
        isLoading={false}
        emptyText="No cards"
        groupBy="case"
        roleFor={() => ROLE_CTX}
        onCardAction={vi.fn()}
      />,
    );

    const eye = screen.getByTestId("timecard-view-tc-1");
    fireEvent.click(eye);
    expect(screen.getByText("Time card")).toBeInTheDocument();

    fireEvent.click(eye);
    expect(screen.queryByText("Time card")).not.toBeInTheDocument();
  });

  it("switches the preview to a different row without requiring a close first", () => {
    getMock.mockReturnValue(new Promise(() => {}));
    renderWithClient(
      <TimeCardsTable
        cards={[CARD, APPROVED_CARD]}
        isLoading={false}
        emptyText="No cards"
        groupBy="case"
        roleFor={() => ROLE_CTX}
        onCardAction={vi.fn()}
      />,
    );

    // Assert the drawer's own "Engineer" field, not the case number --
    // CS0352584/CS0352585 are already rendered in the table's Case column,
    // so asserting those alone could pass even if the drawer never actually
    // switched (or never opened at all).
    fireEvent.click(screen.getByTestId("timecard-view-tc-1"));
    expect(screen.getByText("Jane Doe")).toBeInTheDocument();

    // A real click fires `mousedown` then `click` -- firing both (not just
    // `click`) exercises `useCloseOnOutsideClick`'s own `mousedown` listener
    // too, proving it excludes this eye button rather than racing its click
    // handler and undoing the switch.
    const otherEye = screen.getByTestId("timecard-view-tc-2");
    fireEvent.mouseDown(otherEye);
    fireEvent.click(otherEye);
    expect(screen.getByText("John Roe")).toBeInTheDocument();
    expect(screen.queryByText("Jane Doe")).not.toBeInTheDocument();
  });

  it("closes the preview when clicking outside it, without needing the close button or the eye again", () => {
    getMock.mockReturnValue(new Promise(() => {}));
    renderWithClient(
      <TimeCardsTable
        cards={[CARD]}
        isLoading={false}
        emptyText="No cards"
        groupBy="case"
        roleFor={() => ROLE_CTX}
        onCardAction={vi.fn()}
      />,
    );

    fireEvent.click(screen.getByTestId("timecard-view-tc-1"));
    expect(screen.getByText("Time card")).toBeInTheDocument();

    fireEvent.mouseDown(document.body);

    expect(screen.queryByText("Time card")).not.toBeInTheDocument();
  });
});

describe("TimeCardsTable — preview drawer respects bulk-selection gating", () => {
  // Regression test: the row-level Approve/Reject buttons disable while a
  // bulk selection is active (selectionActive), but the preview drawer used
  // to always receive the card's full, unfiltered action list regardless --
  // letting a user bypass that restriction by approving/rejecting from the
  // drawer instead of the row.
  it("does not offer Approve/Reject in the preview drawer while a bulk selection is active", () => {
    getMock.mockReturnValue(new Promise(() => {}));
    const submittedCard: CsmTimeCard = { ...CARD, id: "tc-3", state: "submitted" };
    renderWithClient(
      <TimeCardsTable
        cards={[submittedCard]}
        isLoading={false}
        emptyText="No cards"
        groupBy="case"
        roleFor={() => APPROVER_ROLE_CTX}
        onCardAction={vi.fn()}
        selectable
        selectedIds={new Set(["tc-3"])}
        onToggleSelect={vi.fn()}
      />,
    );

    fireEvent.click(screen.getByTestId("timecard-view-tc-3"));

    expect(screen.getByText("Time card")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Approve" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Reject" })).not.toBeInTheDocument();
  });
});
