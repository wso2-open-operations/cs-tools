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
import { MemoryRouter } from "react-router";
import { describe, expect, it, vi } from "vitest";
import { SortOrder } from "@/types/common";
import { EngagementsSortField } from "@features/engagements/types/engagements";
import EngagementsPage from "@features/engagements/pages/EngagementsPage";

const mockPageState = vi.hoisted(() => ({
  isStatFiltered: false,
  activeStatKey: undefined as string | undefined,
  isChartNavigation: false,
  chartNavEngagementLabel: undefined as string | undefined,
}));

vi.mock("react-router", async (importOriginal) => {
  const actual = await importOriginal<typeof import("react-router")>();
  return {
    ...actual,
    useNavigate: () => vi.fn(),
    useLocation: () => ({
      pathname: "/projects/proj-1/engagements",
      state: null,
    }),
  };
});

vi.mock("@features/engagements/hooks/useEngagementsPageState", () => ({
  useEngagementsPageState: () => ({
    projectId: "proj-1",
    projectName: "Acme",
    engagementSearchRequest: { filters: {}, sortBy: {} },
    loadedCasesForExport: [],
    hasCasesResponse: true,
    projectReady: true,
    filterMetadata: { caseStates: [] },
    stats: { totalCount: 0, activeCount: 0, stateCount: [] },
    isStatsLoading: false,
    isStatsError: false,
    searchTerm: "",
    isFiltersOpen: false,
    setIsFiltersOpen: vi.fn(),
    filters: {},
    sortField: EngagementsSortField.UpdatedOn,
    sortOrder: SortOrder.DESC,
    page: 1,
    rowsPerPage: 10,
    paginatedCases: [],
    isCasesAreaLoading: false,
    isCasesError: false,
    listHasRefinement: false,
    totalItems: 0,
    handlePageChange: vi.fn(),
    handleRowsPerPageChange: vi.fn(),
    handleFilterChange: vi.fn(),
    handleClearFilters: vi.fn(),
    handleSortChange: vi.fn(),
    handleSortFieldUiChange: vi.fn(),
    handleSearchChange: vi.fn(),
    handleStatCardClick: vi.fn(),
    isStatFiltered: mockPageState.isStatFiltered,
    activeStatKey: mockPageState.activeStatKey,
    clearStatFilter: vi.fn(),
    onCaseClick: vi.fn(),
    isChartNavigation: mockPageState.isChartNavigation,
    chartNavEngagementLabel: mockPageState.chartNavEngagementLabel,
    engagementTypeOptions: [],
  }),
}));

vi.mock("@features/engagements/components/EngagementsStatCards", () => ({
  default: () => <div data-testid="engagements-stat-cards" />,
}));

vi.mock("@features/engagements/components/EngagementsListSection", () => ({
  default: () => <div data-testid="engagements-list-section" />,
}));

vi.mock("@features/support/components/list-export/CaseListCsvExportButton", () => ({
  default: () => <button type="button">Export</button>,
}));

describe("EngagementsPage", () => {
  it("renders stat cards and list section in default view", () => {
    mockPageState.isStatFiltered = false;
    mockPageState.activeStatKey = undefined;
    mockPageState.isChartNavigation = false;

    render(
      <MemoryRouter>
        <EngagementsPage />
      </MemoryRouter>,
    );
    expect(screen.getByTestId("engagements-stat-cards")).toBeInTheDocument();
    expect(screen.getByTestId("engagements-list-section")).toBeInTheDocument();
  });

  it("renders completed engagements header when stat filter is active", () => {
    mockPageState.isStatFiltered = true;
    mockPageState.activeStatKey = "completed";
    mockPageState.isChartNavigation = false;

    render(
      <MemoryRouter>
        <EngagementsPage />
      </MemoryRouter>,
    );
    expect(screen.queryByTestId("engagements-stat-cards")).not.toBeInTheDocument();
    expect(screen.getByText("Completed Engagements")).toBeInTheDocument();
    expect(screen.getByText("Back")).toBeInTheDocument();
    expect(screen.getByTestId("engagements-list-section")).toBeInTheDocument();
  });
});
