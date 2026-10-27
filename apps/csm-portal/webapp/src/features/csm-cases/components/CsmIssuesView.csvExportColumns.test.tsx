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

// End-to-end-in-jsdom check for the "Export CSV reflects the Customise
// columns picker" change: unlike CsmIssuesView.test.tsx (which stubs
// FilteredCsvExportButton out entirely), this file keeps the REAL export
// button/hook/caseToCsvRow chain wired up, mocks only the network layer
// (useBackendApi) and the final browser-download step (downloadCsv's Blob +
// anchor click), and drives the actual "Customise columns" checkboxes via
// Testing Library -- the same interaction a real user performs. This was
// written to get real confidence in the columnPrefs-driven CSV export
// without needing a live backend/Asgardeo session, both of which were
// unavailable in the dev environment this change was authored in.

import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import type { BeCaseSearchResponse, BeCaseSearchView } from "@api/backend/types";
import { formatBackendTimestampForDisplay } from "@utils/dateTime";
import { stateLabel } from "@features/csm-dashboard/utils/abtDashboard";

// Same format options CsmIssuesView.tsx uses for both Created and Updated --
// kept here instead of hardcoding expected date strings, since the actual
// rendered text depends on resolveDisplayTimeZone() (the test runner's own
// local timezone), not a fixed zone.
const DATE_FORMAT: Intl.DateTimeFormatOptions = {
  year: "numeric",
  month: "short",
  day: "numeric",
  hour: "2-digit",
  minute: "2-digit",
};

const mockPost = vi.fn();
vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: mockPost }),
}));
vi.mock("@config/apiConfig", () => ({
  apiConfig: { backendUrl: "https://example.test" },
}));
vi.mock("@context/current-user/CurrentUserContext", () => ({
  useCurrentUser: () => ({
    user: { id: "user-1", roles: undefined },
    isLoading: false,
    isError: false,
  }),
}));
vi.mock("@context/current-user/usePortalAccess", () => ({
  usePortalAccess: () => ({
    canWrite: true,
    canUseSecurityCenter: true,
    canUpdateDeleteAnyComment: true,
  }),
}));
vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: vi.fn() }),
}));
vi.mock("@context/success-banner/SuccessBannerContext", () => ({
  useSuccessBanner: () => ({ showSuccess: vi.fn() }),
}));
vi.mock("@hooks/useIdTokenClaims", () => ({
  useIdTokenClaims: () => ({ email: "user@example.test" }),
}));
vi.mock("@api/useDirectoryUsers", () => ({
  useDirectoryUsers: () => ({ data: [] }),
}));
vi.mock("@features/csm-cases/api/useGetCsmCases", () => ({
  // total must be non-zero: FilteredCsvExportButton disables itself on
  // `isError || total === 0`, reading this same on-screen-list total (not
  // the export's own separately-paged count) -- see CsmIssuesView.tsx.
  useGetCsmCases: () => ({
    data: { cases: [], total: 1, hasMore: false },
    isLoading: false,
    isFetching: false,
    isError: false,
    error: null,
    refetch: vi.fn(),
    dataUpdatedAt: 0,
  }),
}));
vi.mock("@features/csm-cases/components/CasesFilterBar", () => ({
  default: () => <div>FilterBar</div>,
}));
vi.mock("@features/csm-cases/components/CasesList", () => ({
  // Real CsmIssuesView renders this via a prop (not a child) — forward it so
  // the real ColumnCustomizerButton is reachable, same pattern as the sibling
  // CsmIssuesView.test.tsx.
  default: ({ columnCustomizer }: { columnCustomizer?: React.ReactNode }) => (
    <div>
      CasesList
      {columnCustomizer}
    </div>
  ),
}));
vi.mock("@components/RefreshButton", () => ({
  default: () => <div>RefreshButton</div>,
}));

// The one mock that makes this file different from CsmIssuesView.test.tsx:
// FilteredCsvExportButton is intentionally NOT mocked, so clicking "Export
// CSV" runs the real useFilteredCsvExport -> fetchAllPages -> fetchCasesExportPage
// -> caseToCsvRow chain. Only the final step (downloadCsv's Blob + anchor
// click) is intercepted -- jsdom's Blob has no .text(), and asserting on
// plain header/rows arrays is a cleaner surface than parsing CSV text back
// out of a Blob anyway. fetchAllPages/rowsToCsvText/csvField all stay real.
const mockDownloadCsv = vi.fn();
vi.mock("@utils/csvExport", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@utils/csvExport")>();
  return {
    ...actual,
    downloadCsv: (header: string[], rows: string[][], filename: string) =>
      mockDownloadCsv(header, rows, filename),
  };
});

import CsmIssuesView from "@features/csm-cases/components/CsmIssuesView";

beforeEach(() => {
  window.localStorage.clear();
  mockPost.mockReset();
  mockDownloadCsv.mockReset();
});

function renderCases() {
  return render(
    <MemoryRouter initialEntries={["/cases"]}>
      <CsmIssuesView title="Cases" enableColumnCustomization />
    </MemoryRouter>,
  );
}

function sampleCase(overrides: Partial<BeCaseSearchView>): BeCaseSearchView {
  return {
    id: "case-1",
    number: "CS0100001",
    internalId: "TESTSUB-1",
    subject: "Sample case for export test",
    severity: "Critical (P1)",
    type: "case",
    state: "work_in_progress",
    workState: null,
    assignedEngineer: { id: "eng-1", email: "eng@example.test", name: "Test Engineer" },
    createdOn: "2026-09-01 10:00:00",
    updatedOn: "2026-09-02 11:30:00",
    createdBy: { id: null, email: "reporter@example.test", name: "Test Reporter" },
    project: { id: "proj-1", name: "Test Project" },
    deployedProduct: { id: "prod-1", name: "WSO2 API Manager 4.5.0" },
    product: null,
    account: { id: "acct-1", name: "Test Account" },
    escalationLevel: "2",
    ...overrides,
  };
}

describe("Export CSV reflects the Customise columns picker", () => {
  it("includes Created and Escalation Level only once the user has toggled them on, with the right values", async () => {
    const theCase = sampleCase({});
    mockPost.mockImplementation(async (url: string) => {
      if (url !== "/cases/search") throw new Error(`unexpected endpoint: ${url}`);
      const body: BeCaseSearchResponse = {
        cases: [theCase], total: 1, limit: 50, offset: 0, hasMore: false,
      };
      return body;
    });

    renderCases();

    // Default export (Product/Type/Severity visible, Created/Escalation not):
    // confirm the baseline does NOT include them yet.
    fireEvent.click(screen.getByRole("button", { name: "Export CSV" }));
    await waitFor(() => expect(mockDownloadCsv).toHaveBeenCalledTimes(1));
    expect(mockDownloadCsv.mock.calls[0][0]).toEqual([
      "Case ID", "Subject", "Project", "Product", "Type", "Severity", "State", "Updated",
    ]);

    // Now toggle Created + Escalation on via the real "Customise columns" UI.
    fireEvent.click(screen.getByRole("button", { name: "Customise cases columns" }));
    const escalationOption = screen.getByText("Escalation");
    fireEvent.click(screen.getByText("Created"));
    fireEvent.click(escalationOption);
    // Close the popover: MUI's Modal hides the rest of the app from the
    // accessibility tree (aria-hidden) while it's open, so "Export CSV"
    // itself is unreachable until this closes. Escape must be dispatched on
    // a node INSIDE the popover's own portaled subtree -- Modal's Escape
    // handler lives on that subtree, and a native event fired on
    // document.body never crosses into it. The popover's content can also
    // stay mounted-but-hidden rather than unmounting, so retry the click
    // itself (via waitFor) instead of first asserting the popover text is gone.
    fireEvent.keyDown(escalationOption, { key: "Escape", code: "Escape" });
    await waitFor(() => fireEvent.click(screen.getByRole("button", { name: "Export CSV" })));
    await waitFor(() => expect(mockDownloadCsv).toHaveBeenCalledTimes(2));

    const [header, rows, filename] = mockDownloadCsv.mock.calls[1];
    // Column order follows CASE_OPTIONAL_COLUMNS (Product, Type, Severity,
    // Created, Escalation here), with the fixed State/Updated columns always
    // last -- not insertion/toggle order. Real label is "Escalation", not
    // "Escalation Level" -- this assertion is what caught that the first
    // time around.
    expect(header).toEqual([
      "Case ID", "Subject", "Project", "Product", "Type", "Severity", "Created", "Escalation", "State", "Updated",
    ]);
    expect(rows).toEqual([
      [
        "TESTSUB-1/CS0100001",
        "Sample case for export test",
        "Test Project",
        "WSO2 API Manager 4.5.0",
        "Case",
        "S1",
        formatBackendTimestampForDisplay(theCase.createdOn, DATE_FORMAT),
        "EL2",
        stateLabel(theCase.state!),
        formatBackendTimestampForDisplay(theCase.updatedOn, DATE_FORMAT),
      ],
    ]);
    expect(filename).toMatch(/^cases-export-.*\.csv$/);
  });

  it("omits Escalation Level's value (blank, not a dash) for a non-escalated case, and 'Not escalated' never leaks in", async () => {
    const theCase = sampleCase({ escalationLevel: "0" });
    mockPost.mockResolvedValue(
      { cases: [theCase], total: 1, limit: 50, offset: 0, hasMore: false } satisfies BeCaseSearchResponse,
    );

    renderCases();
    fireEvent.click(screen.getByRole("button", { name: "Customise cases columns" }));
    const escalationOption2 = screen.getByText("Escalation");
    fireEvent.click(escalationOption2);
    fireEvent.keyDown(escalationOption2, { key: "Escape", code: "Escape" });
    await waitFor(() => fireEvent.click(screen.getByRole("button", { name: "Export CSV" })));
    await waitFor(() => expect(mockDownloadCsv).toHaveBeenCalledTimes(1));

    const [header, rows] = mockDownloadCsv.mock.calls[0];
    const escalationIdx = header.indexOf("Escalation");
    expect(escalationIdx).toBeGreaterThan(-1);
    const escalationCell = rows[0][escalationIdx];
    expect(escalationCell).toBe("");
  });
});
