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
import { MemoryRouter } from "react-router";
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

// Separate file from CsmIssuesView.test.tsx: needs its own CasesFilterBar
// mock that actually exposes an interactive type-selector (that file's
// mock is a static placeholder), and a useGetCsmCases spy that captures the
// query filters it was called with, to prove digiops-cs#2907's fix: the
// case-type control genuinely changes the query for the one unlocked,
// multi-type caller (Support/CsmCasesPage), while every other, still-locked
// caller keeps its lock enforced regardless of what the (hidden) control
// would have said.

vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: vi.fn() }),
}));
vi.mock("@config/apiConfig", () => ({
  apiConfig: { backendUrl: "https://example.test" },
}));
vi.mock("@context/current-user/CurrentUserContext", () => ({
  useCurrentUser: () => ({ user: { id: "user-1" }, isLoading: false, isError: false }),
}));
vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: vi.fn() }),
}));
vi.mock("@hooks/useIdTokenClaims", () => ({
  useIdTokenClaims: () => ({ email: "user@example.test" }),
}));
vi.mock("@api/useDirectoryUsers", () => ({
  useDirectoryUsers: () => ({ data: [] }),
}));

interface QueryFilters {
  caseTypes: string[];
}

const useGetCsmCasesSpy = vi.fn((_filters: QueryFilters) => {
  void _filters;
  return {
    data: { cases: [], total: 0, hasMore: false },
    isLoading: false,
    isFetching: false,
    isError: false,
    error: null,
    refetch: vi.fn(),
    dataUpdatedAt: 0,
  };
});
vi.mock("@features/csm-cases/api/useGetCsmCases", () => ({
  useGetCsmCases: (filters: QueryFilters) => useGetCsmCasesSpy(filters),
}));

// Stub filter bar exposing one button that fires the same shape of onChange
// the real case-type `MultiSelectField` would, so the test drives the
// component through its real onChange contract rather than poking state
// directly.
vi.mock("@features/csm-cases/components/CasesFilterBar", () => ({
  default: ({
    filters,
    onChange,
  }: {
    filters: { caseTypes: string[] };
    onChange: (next: unknown) => void;
  }) => (
    <button onClick={() => onChange({ ...filters, caseTypes: ["engagement"] })}>
      Pick engagement type
    </button>
  ),
}));
vi.mock("@features/csm-cases/components/CasesList", () => ({
  default: () => <div>CasesList</div>,
}));
vi.mock("@components/FilteredCsvExportButton", () => ({
  default: () => <div>ExportButton</div>,
}));
vi.mock("@components/RefreshButton", () => ({
  default: () => <div>RefreshButton</div>,
}));

import CsmIssuesView from "@features/csm-cases/components/CsmIssuesView";

function lastQueryFilters(): QueryFilters {
  const calls = useGetCsmCasesSpy.mock.calls;
  return calls.length > 0 ? calls[calls.length - 1][0] : { caseTypes: [] };
}

describe("CsmIssuesView case-type lock vs. unlocked control (digiops-cs#2907)", () => {
  it("an unlocked view (control visible) sends the user's own type selection to the query", () => {
    render(
      <MemoryRouter initialEntries={["/cases"]}>
        <CsmIssuesView title="Cases" lockedFilters={{ caseTypes: ["case"] }} />
      </MemoryRouter>,
    );

    fireEvent.click(screen.getByText("Pick engagement type"));

    expect(lastQueryFilters().caseTypes).toEqual(["engagement"]);
  });

  it("a locked view (hideTypeFilter set) keeps forcing the lock into the query, ignoring the control", () => {
    render(
      <MemoryRouter initialEntries={["/security-reports"]}>
        <CsmIssuesView
          title="Security Reports"
          hideTypeFilter
          lockedFilters={{ caseTypes: ["security_report_analysis"] }}
        />
      </MemoryRouter>,
    );

    // The stub always tries to switch to "engagement" -- on a still-locked
    // (hideTypeFilter) view that attempted change must have no effect on
    // the query, unlike the unlocked case above.
    fireEvent.click(screen.getByText("Pick engagement type"));

    expect(lastQueryFilters().caseTypes).toEqual(["security_report_analysis"]);
  });

  it("clearing the type selection on the unlocked view falls back to every case type, not the lock", () => {
    render(
      <MemoryRouter initialEntries={["/cases"]}>
        <CsmIssuesView title="Cases" lockedFilters={{ caseTypes: ["case"] }} />
      </MemoryRouter>,
    );

    // Initial render, before any interaction: caseTypes starts empty (no URL
    // params), which the "no type filter" fallback expands to every type.
    expect(lastQueryFilters().caseTypes.length).toBeGreaterThan(1);
    expect(lastQueryFilters().caseTypes).not.toEqual(["case"]);
  });
});
