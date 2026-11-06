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
import type { ReactNode } from "react";
import { MemoryRouter, Route, Routes } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import type { CsmCaseRow } from "@features/csm-cases/types/csmCases";

// Unlike the sibling CsmIssuesView tests, this one has to drive the view's own
// selection state and its "Export selected" handler, so CasesList is stubbed
// with a minimal row list wired to the real selection callbacks (the real
// CasesList's checkbox rendering is covered in CasesList.test.tsx), and the
// download itself is replaced so the rows handed to it can be asserted.

vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: vi.fn() }),
}));
vi.mock("@config/apiConfig", () => ({
  apiConfig: { backendUrl: "https://example.test" },
}));
vi.mock("@context/current-user/CurrentUserContext", () => ({
  // `cs_engineer` holds canWrite, which gates row selection and the action.
  useCurrentUser: () => ({
    user: { id: "user-1", roles: ["cs_engineer"] },
    isLoading: false,
    isError: false,
  }),
}));
vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: vi.fn() }),
}));
const showSuccessMock = vi.fn();
vi.mock("@context/success-banner/SuccessBannerContext", () => ({
  useSuccessBanner: () => ({ showSuccess: showSuccessMock }),
}));
vi.mock("@hooks/useIdTokenClaims", () => ({
  useIdTokenClaims: () => ({ email: "user@example.test" }),
}));
vi.mock("@api/useDirectoryUsers", () => ({
  useDirectoryUsers: () => ({ data: [] }),
}));
vi.mock("@utils/csvExport", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@utils/csvExport")>()),
  downloadCsv: vi.fn(),
}));

function makeRow(id: string, caseNumber: string, subject: string): CsmCaseRow {
  return {
    id,
    caseNumber,
    wso2CaseId: `ACME-${caseNumber.slice(-3)}`,
    subject,
    customer: "Acme Corp",
    accountId: "acct-1",
    projectId: "proj-1",
    projectName: "Acme Project",
    product: "Example Gateway",
    severity: "S2",
    state: "open",
    assignee: "Jane Doe",
    assigneeIsMe: false,
    slaClockType: "first_response",
    minutesToBreach: 120,
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-02T00:00:00Z",
  };
}

const PAGE_ONE = [
  makeRow("case-1", "CS-1001", "First page, first row"),
  {
    ...makeRow("case-2", "CS-1002", "First page, second row"),
    resolvedAt: "2026-01-03T00:00:00Z",
    closedAt: "2026-01-04T00:00:00Z",
  },
];
const PAGE_TWO = [
  makeRow("case-3", "CS-1003", "=Second page, first row"),
  makeRow("case-4", "CS-1004", "Second page, second row"),
];

// Pages are 20 rows wide, so a total of 40 makes page 1 a real last page (the
// view clamps a page past the last one back down) while only two rows per
// page need building.
const useGetCsmCasesMock = vi.fn((_filters: unknown, page: number) => ({
  data: {
    cases: page === 0 ? PAGE_ONE : PAGE_TWO,
    total: 40,
    hasMore: page === 0,
  },
  isLoading: false,
  isFetching: false,
  isError: false,
  error: null,
  refetch: vi.fn(),
  dataUpdatedAt: 0,
}));
vi.mock("@features/csm-cases/api/useGetCsmCases", () => ({
  useGetCsmCases: (filters: unknown, page: number) => useGetCsmCasesMock(filters, page),
}));
vi.mock("@features/csm-cases/components/CasesFilterBar", () => ({
  default: () => <div>FilterBar</div>,
}));
vi.mock("@features/csm-cases/components/CasesList", () => ({
  default: ({
    cases,
    onToggleRow,
    selectedIds,
  }: {
    cases: CsmCaseRow[];
    onToggleRow?: (row: CsmCaseRow) => void;
    selectedIds?: ReadonlySet<string>;
  }): ReactNode => (
    <ul>
      {cases.map((c) => (
        <li key={c.id}>
          <input
            type="checkbox"
            aria-label={`Select ${c.caseNumber}`}
            checked={selectedIds?.has(c.id) ?? false}
            onChange={() => onToggleRow?.(c)}
          />
        </li>
      ))}
    </ul>
  ),
}));
vi.mock("@components/FilteredCsvExportButton", () => ({
  default: () => <div>ExportButton</div>,
}));
vi.mock("@components/RefreshButton", () => ({
  default: () => <div>RefreshButton</div>,
}));

import CsmIssuesView from "@features/csm-cases/components/CsmIssuesView";
import { downloadCsv } from "@utils/csvExport";
import { formatBackendTimestampForDisplay } from "@utils/dateTime";

// Node 26 exposes `window.localStorage` as undefined unless the process was
// started with `--localstorage-file`, which hides jsdom's own Storage.
function installLocalStorage(): void {
  if (typeof window.localStorage?.clear === "function") return;
  const store = new Map<string, string>();
  Object.defineProperty(window, "localStorage", {
    configurable: true,
    value: {
      getItem: (key: string) => store.get(key) ?? null,
      setItem: (key: string, value: string) => {
        store.set(key, String(value));
      },
      removeItem: (key: string) => {
        store.delete(key);
      },
      clear: () => {
        store.clear();
      },
      key: (index: number) => [...store.keys()][index] ?? null,
      get length() {
        return store.size;
      },
    },
  });
}

beforeEach(() => {
  installLocalStorage();
  window.localStorage.clear();
  vi.mocked(downloadCsv).mockClear();
  showSuccessMock.mockClear();
});

describe("CsmIssuesView Export selected", () => {
  it("exports only the rows ticked across two pages, with the expected CSV fields", () => {
    render(
      <MemoryRouter initialEntries={["/cases"]}>
        <Routes>
          <Route path="/cases" element={<CsmIssuesView title="Cases" />} />
        </Routes>
      </MemoryRouter>,
    );

    // The action only appears once something is selected.
    expect(screen.queryByRole("button", { name: /Export selected/ })).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("checkbox", { name: "Select CS-1002" }));
    fireEvent.click(screen.getByRole("button", { name: "Go to next page" }));
    // Page two is showing, and page one's pick is still held.
    expect(screen.getByRole("checkbox", { name: "Select CS-1003" })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("checkbox", { name: "Select CS-1003" }));

    fireEvent.click(screen.getByRole("button", { name: "Export selected (2)" }));

    const fmt = (ts: string) =>
      formatBackendTimestampForDisplay(ts, {
        year: "numeric",
        month: "short",
        day: "numeric",
        hour: "2-digit",
        minute: "2-digit",
      });
    const updated = fmt("2026-01-02T00:00:00Z");
    const created = fmt("2026-01-01T00:00:00Z");
    expect(downloadCsv).toHaveBeenCalledTimes(1);
    const [header, rows, filename] = vi.mocked(downloadCsv).mock.calls[0];
    expect(header).toEqual([
      "Case ID",
      "Subject",
      "Project",
      "Product",
      "Severity",
      "State",
      "Updated",
      "Created",
      "Resolved",
      "Closed",
    ]);
    expect(rows).toEqual([
      [
        "ACME-002/CS-1002",
        "First page, second row",
        "Acme Project",
        "Example Gateway",
        "S2",
        "Open",
        updated,
        created,
        fmt("2026-01-03T00:00:00Z"),
        fmt("2026-01-04T00:00:00Z"),
      ],
      [
        "ACME-003/CS-1003",
        "=Second page, first row",
        "Acme Project",
        "Example Gateway",
        "S2",
        "Open",
        updated,
        created,
        "",
        "",
      ],
    ]);
    expect(filename).toMatch(/^cases-selected-export-.*\.csv$/);
    expect(showSuccessMock).toHaveBeenCalledWith("Exported 2 selected cases to CSV.");
  });
});
