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

import type { ReactElement } from "react";
import { fireEvent, render as rtlRender, screen } from "@testing-library/react";
import { describe, expect, it, vi, beforeEach } from "vitest";
import { MemoryRouter } from "react-router";
import "@testing-library/jest-dom/vitest";
import ChangeRequestsTab from "@features/csm-operations/components/ChangeRequestsTab";
import { useSearchChangeRequests } from "@features/csm-operations/api/useSearchChangeRequests";
import type { BeChangeRequestSearchView } from "@api/backend/types";

// The backend client reads runtime config at module load, which isn't present
// under vitest (same approach as CsmAnnouncementsPage.test.tsx).
vi.mock("@api/backend/client", () => ({
  BackendApiError: class BackendApiError extends Error {},
  useBackendApi: () => ({ post: vi.fn() }),
}));

vi.mock("@features/csm-operations/api/useSearchChangeRequests", () => ({
  useSearchChangeRequests: vi.fn(),
}));

// The filter bar's SRE Team control goes through the shared team registry
// query — stub it out so this test doesn't need a QueryClientProvider,
// same rationale as the other mocks below.
vi.mock("@features/csm-dashboard/api/useTeams", () => ({
  useTeams: () => ({ data: [], isLoading: false }),
}));

// Only the column picker's storage key derives from the signed-in user.
vi.mock("@context/current-user/CurrentUserContext", () => ({
  useCurrentUser: () => ({ user: { id: "user-1" }, isLoading: false, isError: false }),
}));
vi.mock("@hooks/useIdTokenClaims", () => ({
  useIdTokenClaims: () => ({ email: "user@example.test" }),
}));

// The real export button needs an `ErrorBannerProvider` ancestor this test
// doesn't set up (same approach as CsmIssuesView.test.tsx) — it's unrelated
// to the column-customization behavior under test here.
vi.mock("@components/FilteredCsvExportButton", () => ({
  default: () => <div>ExportButton</div>,
}));

const mockedUseSearch = vi.mocked(useSearchChangeRequests);
// The picker only lists the 10 optional columns (Number/Subject/State/Updated
// are fixed and never appear in it).
const CR_OPTIONAL_COLUMN_COUNT = 10;

function render(ui: ReactElement): ReturnType<typeof rtlRender> {
  return rtlRender(<MemoryRouter>{ui}</MemoryRouter>);
}

const CR: BeChangeRequestSearchView = {
  id: "cr-1",
  number: "CHG-1001",
  subject: "Upgrade cluster to WSO2 IS 7.1",
  project: { id: "proj-1", name: "Acme Project" },
  impact: "low",
  state: "new",
  plannedStartOn: "2026-08-01T00:00:00Z",
  plannedEndOn: "2026-08-02T00:00:00Z",
  product: { id: "prod-1", name: "WSO2 Identity Server" },
  assignedEngineer: { id: "user-2", name: "Jane Doe" },
  assignedTeam: { id: "team-1", name: "Platform CRE" },
  type: "Standard",
  case: { id: "case-1", name: "CS-2001" },
  createdOn: "2026-07-01T00:00:00Z",
  updatedOn: "2026-07-05T00:00:00Z",
};

function mockResult(
  overrides: Partial<ReturnType<typeof useSearchChangeRequests>>,
): void {
  mockedUseSearch.mockReturnValue({
    data: undefined,
    isLoading: false,
    isFetching: false,
    isError: false,
    error: null,
    refetch: vi.fn(),
    dataUpdatedAt: 0,
    ...overrides,
  } as unknown as ReturnType<typeof useSearchChangeRequests>);
}

beforeEach(() => {
  mockedUseSearch.mockReset();
  window.localStorage.clear();
});

describe("ChangeRequestsTab — list states", () => {
  it("renders a row from the search result with its default-visible columns", () => {
    mockResult({ data: { changeRequests: [CR], total: 1, limit: 20, offset: 0 } });
    render(<ChangeRequestsTab />);

    expect(screen.getByText("CHG-1001")).toBeInTheDocument();
    expect(screen.getByText("Upgrade cluster to WSO2 IS 7.1")).toBeInTheDocument();
    // Default-visible optional columns: Project, Impact, Planned start.
    expect(screen.getByText("Acme Project")).toBeInTheDocument();
    expect(screen.getByRole("columnheader", { name: "Planned start" })).toBeInTheDocument();
    // Not default-visible.
    expect(screen.queryByRole("columnheader", { name: "Product" })).not.toBeInTheDocument();
    expect(screen.queryByRole("columnheader", { name: "Assigned engineer" })).not.toBeInTheDocument();
  });

  it("shows the empty state when there are no change requests", () => {
    mockResult({ data: { changeRequests: [], total: 0, limit: 20, offset: 0 } });
    render(<ChangeRequestsTab />);
    expect(screen.getByText(/no change requests found/i)).toBeInTheDocument();
  });
});

describe("ChangeRequestsTab — customise columns", () => {
  it("offers every widened optional column in the picker", () => {
    mockResult({ data: { changeRequests: [CR], total: 1, limit: 20, offset: 0 } });
    render(<ChangeRequestsTab />);

    fireEvent.click(
      screen.getByRole("button", { name: "Customise change request columns" }),
    );

    // Project/Impact/Planned start are already default-visible, so they
    // render twice (once as a column header, once in the picker) — assert
    // those via their column header instead of `getByText` to avoid an
    // ambiguous match.
    for (const label of ["Project", "Impact", "Planned start"]) {
      expect(screen.getByRole("columnheader", { name: label, hidden: true })).toBeInTheDocument();
    }
    // The rest are only offered, not shown yet — unambiguous with `getByText`.
    for (const label of [
      "Planned end",
      "Product",
      "Assigned engineer",
      "Assigned team",
      "Type",
      "Case",
      "Created",
    ]) {
      expect(screen.getByText(label)).toBeInTheDocument();
    }
  });

  it("adds the Product column when checked, and it renders the row's product name", () => {
    mockResult({ data: { changeRequests: [CR], total: 1, limit: 20, offset: 0 } });
    render(<ChangeRequestsTab />);

    fireEvent.click(
      screen.getByRole("button", { name: "Customise change request columns" }),
    );
    // Column order matches `CHANGE_REQUEST_OPTIONAL_COLUMNS`: Project(0),
    // Impact(1), Planned start(2), Planned end(3), Product(4), ...
    const checkboxes = screen.getAllByRole("checkbox");
    fireEvent.click(checkboxes[4]);

    expect(
      screen.getByRole("columnheader", { name: "Product", hidden: true }),
    ).toBeInTheDocument();
    expect(screen.getByText("WSO2 Identity Server")).toBeInTheDocument();
  });

  it("never lets every column be unchecked", () => {
    mockResult({ data: { changeRequests: [CR], total: 1, limit: 20, offset: 0 } });
    render(<ChangeRequestsTab />);

    fireEvent.click(
      screen.getByRole("button", { name: "Customise change request columns" }),
    );
    for (let i = 0; i < CR_OPTIONAL_COLUMN_COUNT; i++) {
      const checkbox = screen.getAllByRole("checkbox")[i];
      if (checkbox && !checkbox.hasAttribute("disabled") && (checkbox as HTMLInputElement).checked) {
        fireEvent.click(checkbox);
      }
    }

    // The invariant under test: the hook refuses to let the last visible
    // optional column be unchecked. Fixed columns (Number/Subject/State/
    // Updated) always render a header regardless, so assert the checkbox
    // state directly rather than counting headers.
    const remainingChecked = screen
      .getAllByRole("checkbox")
      .filter((checkbox) => (checkbox as HTMLInputElement).checked);
    expect(remainingChecked.length).toBe(1);
    expect(remainingChecked[0]).toBeDisabled();
  });
});
