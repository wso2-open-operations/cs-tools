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
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { describe, expect, it, vi } from "vitest";
import type { JSX, ReactElement } from "react";
import { useLocation, MemoryRouter, Route, Routes } from "react-router";
import "@testing-library/jest-dom/vitest";

// The real client reads runtime config at module load, which isn't present
// under vitest (same approach as CaseActivitiesFeed.test.tsx). CasesList
// renders a CasePreviewDrawer alongside every row (closed by default here,
// since no test opens it), which calls useGetCsmCaseComments — the mock just
// keeps that hook's useBackendApi() call from throwing on missing runtime
// config; its query stays disabled (no caseId) so it's never actually invoked.
vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: vi.fn().mockResolvedValue({ comments: [] }) }),
}));

import CasesList from "@features/csm-cases/components/CasesList";
import type { CsmCaseRow } from "@features/csm-cases/types/csmCases";

function renderWithProviders(ui: ReactElement, initialEntries: string[]): ReturnType<typeof render> {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={initialEntries}>{ui}</MemoryRouter>
    </QueryClientProvider>,
  );
}

const CASE: CsmCaseRow = {
  id: "case-1",
  caseNumber: "CS-1007",
  subject: "Cluster fails to start",
  customer: "Acme Corp",
  accountId: "acct-1",
  projectId: "proj-1",
  projectName: "Acme Project",
  product: "WSO2 Identity Server",
  severity: "S2",
  state: "work_in_progress",
  assignee: "Jane Doe",
  assigneeIsMe: false,
  slaClockType: "first_response",
  minutesToBreach: 120,
  createdAt: "2026-01-01T00:00:00Z",
  updatedAt: "2026-01-02T00:00:00Z",
};

// Stand-in for the detail page: reads back whatever `state` the row link
// carried, so the test can assert the filtered list URL survived the
// navigation without depending on the real detail page's implementation.
function DetailStub(): JSX.Element {
  const location = useLocation();
  const from = (location.state as { from?: string } | undefined)?.from;
  return <div data-testid="from-state">{from ?? "(none)"}</div>;
}

describe("CasesList row navigation", () => {
  it("carries the current (filtered) list URL forward as router state", () => {
    renderWithProviders(
      <Routes>
        <Route
          path="/cases"
          element={<CasesList cases={[CASE]} isLoading={false} />}
        />
        <Route path="/cases/:id" element={<DetailStub />} />
      </Routes>,
      ["/cases?state=work_in_progress&severity=S2"],
    );

    fireEvent.click(screen.getByText("Cluster fails to start"));

    expect(screen.getByTestId("from-state")).toHaveTextContent(
      "/cases?state=work_in_progress&severity=S2",
    );
  });

  it("carries a bare list URL forward when no filters are active", () => {
    renderWithProviders(
      <Routes>
        <Route
          path="/cases"
          element={<CasesList cases={[CASE]} isLoading={false} />}
        />
        <Route path="/cases/:id" element={<DetailStub />} />
      </Routes>,
      ["/cases"],
    );

    fireEvent.click(screen.getByText("Cluster fails to start"));

    expect(screen.getByTestId("from-state")).toHaveTextContent("/cases");
  });
});

// Regression: reported live — a single case with an unusually long subject
// line forced the whole grid to scroll horizontally even with zero optional
// columns turned on. `noWrap`'s ellipsis only clips paint; it never affects a
// CSS grid item's max-content sizing contribution, which is what actually
// drove the grid wider. A `maxWidth` on the Subject cell's wrapping Box is
// what fixes it — this only guards that the style stays in place, since
// jsdom doesn't perform real CSS Grid intrinsic-size layout.
describe("CasesList — Subject cell has a capped width", () => {
  it("does not let the wrapping cell grow unbounded for a long subject", () => {
    const longSubject =
      "A very long case subject line that goes on and on and would otherwise force this column, and the whole table, to grow far past a normal viewport width";
    renderWithProviders(
      <Routes>
        <Route
          path="/cases"
          element={<CasesList cases={[{ ...CASE, subject: longSubject }]} isLoading={false} />}
        />
      </Routes>,
      ["/cases"],
    );

    const subjectCell = screen.getByTitle(longSubject).parentElement;
    expect(subjectCell).not.toBeNull();
    expect(getComputedStyle(subjectCell!).maxWidth).toBe("360px");
  });

  // Regression: reported live for Security Reports' Product column, but the
  // same shared CasesList renders Cases/Service Requests/Engagements too — an
  // optional column's track is mechanically identical to Subject's
  // (minmax(140px, 1fr) vs. Subject's minmax(280px, 3fr)), so it needed the
  // same maxWidth treatment, not just Subject.
  it("also caps an optional column's wrapping cell for a long value", () => {
    const longCustomer =
      "A very long customer account name that goes on and on and would otherwise force this optional column, and the whole table, to grow far past a normal viewport width";
    renderWithProviders(
      <Routes>
        <Route
          path="/cases"
          element={
            <CasesList
              cases={[{ ...CASE, customer: longCustomer }]}
              isLoading={false}
              optionalColumns={["customer"]}
            />
          }
        />
      </Routes>,
      ["/cases"],
    );

    const customerCell = screen.getByTitle(longCustomer).parentElement;
    expect(customerCell).not.toBeNull();
    expect(getComputedStyle(customerCell!).maxWidth).toBe("260px");
  });
});

describe("CasesList quick preview", () => {
  it("opens the preview drawer instead of navigating when the preview action is clicked", () => {
    renderWithProviders(
      <Routes>
        <Route
          path="/cases"
          element={<CasesList cases={[CASE]} isLoading={false} />}
        />
        <Route path="/cases/:id" element={<DetailStub />} />
      </Routes>,
      ["/cases"],
    );

    fireEvent.click(screen.getByRole("button", { name: "Quick preview CS-1007" }));

    expect(screen.getByText("View full details")).toBeInTheDocument();
    expect(screen.queryByTestId("from-state")).not.toBeInTheDocument();
  });

  it("closes the preview when the same row's eye is clicked again", () => {
    renderWithProviders(
      <Routes>
        <Route
          path="/cases"
          element={<CasesList cases={[CASE]} isLoading={false} />}
        />
        <Route path="/cases/:id" element={<DetailStub />} />
      </Routes>,
      ["/cases"],
    );

    const eye = screen.getByRole("button", { name: "Quick preview CS-1007" });
    fireEvent.click(eye);
    expect(screen.getByText("View full details")).toBeInTheDocument();

    fireEvent.click(eye);
    expect(screen.queryByText("View full details")).not.toBeInTheDocument();
  });

  it("switches the preview to a different row without requiring a close first", () => {
    const otherCase: CsmCaseRow = {
      ...CASE,
      id: "case-2",
      caseNumber: "CS-1008",
      subject: "Login fails intermittently",
    };
    renderWithProviders(
      <Routes>
        <Route
          path="/cases"
          element={<CasesList cases={[CASE, otherCase]} isLoading={false} />}
        />
        <Route path="/cases/:id" element={<DetailStub />} />
      </Routes>,
      ["/cases"],
    );

    fireEvent.click(screen.getByRole("button", { name: "Quick preview CS-1007" }));
    expect(screen.getByRole("link", { name: "View full details" })).toHaveAttribute(
      "href",
      expect.stringContaining("/case-1"),
    );

    // The open preview is a non-modal `FloatingSlidePanel`, not a `Drawer` --
    // it has no backdrop and doesn't mark the rest of the page `aria-hidden`,
    // so the other row's eye button is a normal, reachable element (no
    // `hidden: true` needed to find it, unlike a `Modal`-backed `Drawer`).
    //
    // A real click fires `mousedown` then `click` -- firing both (not just
    // `click`) exercises `useCloseOnOutsideClick`'s own `mousedown` listener
    // too, proving it excludes this eye button rather than racing its click
    // handler and undoing the switch.
    const otherEye = screen.getByRole("button", { name: "Quick preview CS-1008" });
    fireEvent.mouseDown(otherEye);
    fireEvent.click(otherEye);
    expect(screen.getByRole("link", { name: "View full details" })).toHaveAttribute(
      "href",
      expect.stringContaining("/case-2"),
    );
  });

  it("does not isolate the rest of the page from assistive tech while the preview is open (no focus trap, no aria-hidden)", () => {
    renderWithProviders(
      <Routes>
        <Route
          path="/cases"
          element={<CasesList cases={[CASE]} isLoading={false} />}
        />
        <Route path="/cases/:id" element={<DetailStub />} />
      </Routes>,
      ["/cases"],
    );

    // A `Modal`-backed `Drawer` would mark this sibling row's own subject
    // text `aria-hidden` (and remove it from the accessibility tree, so
    // `getByText` without `hidden: true` would throw) for as long as it's
    // open, regardless of any pointer-events workaround -- exactly the
    // CodeRabbit-flagged accessibility bug `FloatingSlidePanel` fixes.
    fireEvent.click(screen.getByRole("button", { name: "Quick preview CS-1007" }));
    expect(screen.getByText("View full details")).toBeInTheDocument();

    // The case's subject renders twice once the preview is open (once in
    // the row itself, once inside the preview content) -- assert on the
    // row's own copy specifically.
    const [rowSubject] = screen.getAllByText("Cluster fails to start");
    expect(rowSubject.closest('[aria-hidden="true"]')).not.toBeInTheDocument();
    expect(document.body).not.toHaveAttribute("aria-hidden");

    // Still keyboard-reachable, not stranded behind a focus trap.
    const quickPreviewButton = screen.getByRole("button", { name: "Quick preview CS-1007" });
    quickPreviewButton.focus();
    expect(quickPreviewButton).toHaveFocus();
  });

  it("closes the preview when clicking outside it, without needing the close button or the eye again", () => {
    renderWithProviders(
      <Routes>
        <Route
          path="/cases"
          element={<CasesList cases={[CASE]} isLoading={false} />}
        />
        <Route path="/cases/:id" element={<DetailStub />} />
      </Routes>,
      ["/cases"],
    );

    fireEvent.click(screen.getByRole("button", { name: "Quick preview CS-1007" }));
    expect(screen.getByText("View full details")).toBeInTheDocument();

    // Any outside element -- here, the page body itself, well outside the
    // drawer's own content -- should close it via the mousedown-level
    // click-away listener, matching a real click's actual first event.
    fireEvent.mouseDown(document.body);

    expect(screen.queryByText("View full details")).not.toBeInTheDocument();
  });

  it("does not close the preview when clicking inside its own content", () => {
    renderWithProviders(
      <Routes>
        <Route
          path="/cases"
          element={<CasesList cases={[CASE]} isLoading={false} />}
        />
        <Route path="/cases/:id" element={<DetailStub />} />
      </Routes>,
      ["/cases"],
    );

    fireEvent.click(screen.getByRole("button", { name: "Quick preview CS-1007" }));
    const detailsLink = screen.getByRole("link", { name: "View full details" });
    fireEvent.mouseDown(detailsLink);

    expect(screen.getByRole("link", { name: "View full details" })).toBeInTheDocument();
  });
});

describe("CasesList optional columns", () => {
  it("renders the widened optional column set (customer, created) when passed explicitly", () => {
    renderWithProviders(
      <Routes>
        <Route
          path="/cases"
          element={
            <CasesList
              cases={[CASE]}
              isLoading={false}
              optionalColumns={["customer", "createdAt"]}
            />
          }
        />
        <Route path="/cases/:id" element={<DetailStub />} />
      </Routes>,
      ["/cases"],
    );

    expect(screen.getByText("Customer")).toBeInTheDocument();
    expect(screen.getByText("Created")).toBeInTheDocument();
    expect(screen.getByText("Acme Corp")).toBeInTheDocument();
    // Neither of the legacy fixed-set columns is asked for here.
    expect(screen.queryByText("Product")).not.toBeInTheDocument();
    expect(screen.queryByText("Assignee")).not.toBeInTheDocument();
  });

  it("renders Issue type and Reporter (createdBy) when passed explicitly", () => {
    renderWithProviders(
      <Routes>
        <Route
          path="/cases"
          element={
            <CasesList
              cases={[{ ...CASE, issueType: "total_outage", createdBy: "John Reporter" }]}
              isLoading={false}
              optionalColumns={["issueType", "createdBy"]}
            />
          }
        />
        <Route path="/cases/:id" element={<DetailStub />} />
      </Routes>,
      ["/cases"],
    );

    expect(screen.getByText("Issue type")).toBeInTheDocument();
    expect(screen.getByText("Total outage")).toBeInTheDocument();
    expect(screen.getByText("Reporter")).toBeInTheDocument();
    expect(screen.getByText("John Reporter")).toBeInTheDocument();
  });

  it("shows an em dash for Issue type/Reporter when the row carries neither", () => {
    renderWithProviders(
      <Routes>
        <Route
          path="/cases"
          element={
            <CasesList
              cases={[CASE]}
              isLoading={false}
              optionalColumns={["issueType", "createdBy"]}
            />
          }
        />
        <Route path="/cases/:id" element={<DetailStub />} />
      </Routes>,
      ["/cases"],
    );

    // `CASE` has no `issueType`/`createdBy` set.
    expect(screen.getAllByText("—").length).toBeGreaterThan(0);
  });

  it("renders the escalation level chip for an escalated row", () => {
    renderWithProviders(
      <Routes>
        <Route
          path="/cases"
          element={
            <CasesList
              cases={[{ ...CASE, escalationLevel: "2" }]}
              isLoading={false}
              optionalColumns={["escalationLevel"]}
            />
          }
        />
        <Route path="/cases/:id" element={<DetailStub />} />
      </Routes>,
      ["/cases"],
    );

    expect(screen.getByText("Escalation")).toBeInTheDocument();
    expect(screen.getByText("EL2")).toBeInTheDocument();
  });

  it("renders nothing (not a placeholder chip) in the escalation column for a non-escalated row", () => {
    renderWithProviders(
      <Routes>
        <Route
          path="/cases"
          element={
            <CasesList
              cases={[{ ...CASE, escalationLevel: "0" }]}
              isLoading={false}
              optionalColumns={["escalationLevel"]}
            />
          }
        />
        <Route path="/cases/:id" element={<DetailStub />} />
      </Routes>,
      ["/cases"],
    );

    expect(screen.getByText("Escalation")).toBeInTheDocument();
    expect(screen.queryByText(/^EL/)).not.toBeInTheDocument();
    expect(screen.queryByText("Not escalated")).not.toBeInTheDocument();
  });

  it("keeps rendering the legacy fixed optional set when optionalColumns is omitted", () => {
    renderWithProviders(
      <Routes>
        <Route
          path="/cases"
          element={<CasesList cases={[CASE]} isLoading={false} />}
        />
        <Route path="/cases/:id" element={<DetailStub />} />
      </Routes>,
      ["/cases"],
    );

    expect(screen.getByText("Product")).toBeInTheDocument();
    expect(screen.queryByText("Customer")).not.toBeInTheDocument();
    expect(screen.queryByText("Created")).not.toBeInTheDocument();
  });
});

describe("CasesList sortable headers", () => {
  function renderSortable(initialField: "createdOn" | "updatedOn" | "severity" | "state") {
    const onSortFieldChange = vi.fn();
    const onSortOrderChange = vi.fn();
    renderWithProviders(
      <Routes>
        <Route
          path="/cases"
          element={
            <CasesList
              cases={[CASE]}
              isLoading={false}
              optionalColumns={["severity", "createdAt"]}
              sortField={initialField}
              sortOrder="desc"
              onSortFieldChange={onSortFieldChange}
              onSortOrderChange={onSortOrderChange}
            />
          }
        />
        <Route path="/cases/:id" element={<DetailStub />} />
      </Routes>,
      ["/cases"],
    );
    return { onSortFieldChange, onSortOrderChange };
  }

  it("activates a non-active header at desc order when clicked", () => {
    const { onSortFieldChange, onSortOrderChange } = renderSortable("updatedOn");

    fireEvent.click(screen.getByText("Created"));

    expect(onSortFieldChange).toHaveBeenCalledWith("createdOn");
    expect(onSortOrderChange).toHaveBeenCalledWith("desc");
  });

  it("toggles order when the already-active header is clicked again", () => {
    const { onSortFieldChange, onSortOrderChange } = renderSortable("severity");

    fireEvent.click(screen.getByText("Severity"));

    expect(onSortFieldChange).not.toHaveBeenCalled();
    expect(onSortOrderChange).toHaveBeenCalledWith("asc");
  });

  it("marks only the currently active column as sorted", () => {
    renderSortable("state");

    // MUI's `TableSortLabel` reflects the active column via `aria-sort` on
    // its containing header text (rendered as a nested `<span>` here rather
    // than a real `<th>`, so assert on the label's own active/inactive
    // class instead of `aria-sort`).
    const activeLabel = screen.getByText("State").closest(".MuiTableSortLabel-root");
    const inactiveLabel = screen.getByText("Updated").closest(".MuiTableSortLabel-root");

    expect(activeLabel).toHaveClass("Mui-active");
    expect(inactiveLabel).not.toHaveClass("Mui-active");
  });

  it("renders every clickable header when sort props are wired up", () => {
    renderSortable("updatedOn");

    ["Created", "Severity", "State", "Updated"].forEach((label) => {
      expect(screen.getByText(label).closest(".MuiTableSortLabel-root")).toBeInTheDocument();
    });
  });

  it("keeps headers as plain text when no sort props are passed", () => {
    renderWithProviders(
      <Routes>
        <Route
          path="/cases"
          element={
            <CasesList cases={[CASE]} isLoading={false} optionalColumns={["severity", "createdAt"]} />
          }
        />
        <Route path="/cases/:id" element={<DetailStub />} />
      </Routes>,
      ["/cases"],
    );

    ["Created", "Severity", "State", "Updated"].forEach((label) => {
      expect(screen.getByText(label).closest(".MuiTableSortLabel-root")).not.toBeInTheDocument();
    });
  });
});
