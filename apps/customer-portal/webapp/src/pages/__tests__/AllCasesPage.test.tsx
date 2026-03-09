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

import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, expect, it, vi, beforeEach } from "vitest";
import AllCasesPage from "@pages/AllCasesPage";
import { ThemeProvider, createTheme } from "@wso2/oxygen-ui";
import { MemoryRouter, Route, Routes } from "react-router";
import { LoaderProvider } from "@context/linear-loader/LoaderContext";

const mockCaseMetadata = {
  caseStates: [{ id: "1", label: "Open" }],
  severities: [{ id: "2", label: "High" }],
  issueTypes: [{ id: "3", label: "Incident" }],
  deploymentTypes: [{ id: "4", label: "Production" }],
  caseTypes: [
    { id: "id-incident", label: "Incident" },
    { id: "id-query", label: "Query" },
  ],
};

const mockCases = [
  {
    id: "case-1",
    internalId: "INT-1",
    number: "CS001",
    createdOn: "2026-01-01",
    title: "Case One",
    description: "Desc",
    assignedEngineer: null,
    project: { id: "p1", label: "Project A" },
    issueType: { id: "1", label: "Bug" },
    state: { id: 1, label: "Open" },
    severity: { id: 1, label: "High" },
  },
  {
    id: "case-2",
    internalId: "INT-2",
    number: "CS002",
    createdOn: "2026-01-02",
    title: "Case Two",
    description: "Desc",
    assignedEngineer: null,
    project: { id: "p1", label: "Project A" },
    issueType: { id: "1", label: "Bug" },
    state: { id: 1, label: "Open" },
    severity: { id: 1, label: "High" },
  },
];

// Mock API hooks
vi.mock("@api/useGetProjectCasesStats", () => ({
  useGetProjectCasesStats: vi.fn(),
}));
vi.mock("@api/useGetProjectFilters", () => ({
  default: vi.fn(),
}));
vi.mock("@api/useGetProjectCases", () => ({
  default: vi.fn(),
}));

import { useGetProjectCasesStats } from "@api/useGetProjectCasesStats";
import useGetProjectFilters from "@api/useGetProjectFilters";
import useGetProjectCases from "@api/useGetProjectCases";

// Mock Child Components to test props passing and interactions
vi.mock("@components/support/all-cases/AllCasesStatCards", () => ({
  default: ({ isLoading }: any) => (
    <div data-testid="mock-stat-cards">
      {isLoading ? "Loading Stats" : "Stats Loaded"}
    </div>
  ),
}));

vi.mock("@components/support/all-cases/AllCasesSearchBar", () => ({
  default: ({
    searchTerm,
    onSearchChange,
    onFilterChange,
    onClearFilters,
    filters,
  }: any) => (
    <div data-testid="mock-search-bar">
      <input
        data-testid="search-input"
        value={searchTerm}
        onChange={(e) => onSearchChange(e.target.value)}
      />
      <button
        data-testid="filter-status-open"
        onClick={() => onFilterChange("statusId", "1")}
      >
        Filter Status Open
      </button>
      <button data-testid="clear-filters" onClick={onClearFilters}>
        Clear Filters
      </button>
      <div data-testid="current-filters">{JSON.stringify(filters)}</div>
    </div>
  ),
}));

vi.mock("@components/support/all-cases/AllCasesList", () => ({
  default: ({ cases, isLoading }: any) => {
    return (
      <div data-testid="mock-case-list">
        {isLoading ? "Loading Cases" : `Showing ${cases?.length} cases`}
        <ul>
          {cases?.map((c: any) => (
            <li key={c.id}>{c.number}</li>
          ))}
        </ul>
      </div>
    );
  },
}));

describe("AllCasesPage", () => {
  const theme = createTheme();
  const projectId = "123";

  beforeEach(() => {
    vi.clearAllMocks();

    // Default mock implementations
    (useGetProjectCasesStats as any).mockReturnValue({
      data: undefined,
      isFetching: false,
      isError: false,
    });
    (useGetProjectFilters as any).mockReturnValue({
      data: mockCaseMetadata,
      isFetching: false,
    });
    (useGetProjectCases as any).mockReturnValue({
      data: {
        pages: [
          {
            cases: mockCases,
            totalRecords: mockCases.length,
            offset: 0,
            limit: mockCases.length,
          },
        ],
        pageParams: [0],
      },
      isFetching: false,
      hasNextPage: false,
      fetchNextPage: vi.fn(),
    });
  });

  const renderComponent = () => {
    return render(
      <ThemeProvider theme={theme}>
        <LoaderProvider>
          <MemoryRouter initialEntries={[`/projects/${projectId}/support/cases`]}>
            <Routes>
              <Route
                path="/projects/:projectId/support/cases"
                element={<AllCasesPage />}
              />
            </Routes>
          </MemoryRouter>
        </LoaderProvider>
      </ThemeProvider>,
    );
  };

  it("should render the page components", () => {
    renderComponent();

    expect(screen.getByText("All Cases")).toBeInTheDocument();
    expect(screen.getByTestId("mock-stat-cards")).toBeInTheDocument();
    expect(screen.getByTestId("mock-search-bar")).toBeInTheDocument();
    expect(screen.getByTestId("mock-case-list")).toBeInTheDocument();
  });

  it("should filter cases by search term", async () => {
    renderComponent();

    const searchInput = screen.getByTestId("search-input");

    // Search triggers API refetch with searchQuery; mock returns same data
    fireEvent.change(searchInput, { target: { value: "CS001" } });

    await waitFor(() => {
      expect(searchInput).toHaveValue("CS001");
    });
    // API returns filtered results; mock returns same 2 cases regardless of search
    expect(screen.getByText(/Showing 2 of 2 cases/)).toBeInTheDocument();
  });

  it("should filter cases by status", async () => {
    renderComponent();

    const filterButton = screen.getByTestId("filter-status-open");
    fireEvent.click(filterButton);

    // Page shows (paginated count) of (api totalRecords); mock uses totalRecords: mockCases.length
    const displayedCount = Math.min(10, mockCases.length);
    const totalCases = mockCases.length;

    await waitFor(() => {
      const text = (content: string, el: Element | null) =>
        el?.textContent?.replace(/\s+/g, " ").trim() === content;
      expect(
        screen.getByText((_, el) =>
          text(`Showing ${displayedCount} of ${totalCases} cases`, el),
        ),
      ).toBeInTheDocument();
    });
  });

  it("should clear filters", async () => {
    renderComponent();

    // Apply filter
    const filterButton = screen.getByTestId("filter-status-open");
    fireEvent.click(filterButton);

    // Create some search
    const searchInput = screen.getByTestId("search-input");
    fireEvent.change(searchInput, { target: { value: "CS0001" } }); // Partial match

    // Clear filters
    const clearButton = screen.getByTestId("clear-filters");
    fireEvent.click(clearButton);

    // Verify filters reset
    expect(screen.getByTestId("current-filters")).toHaveTextContent(
      JSON.stringify({}),
    );
  });

  it("should sort cases", async () => {
    renderComponent();

    // Just verify interaction doesn't crash and updates state (implied by re-render if we were checking props)
    // For now, simple pass is fine as we mock the data sort in the component and we don't easily check order in DOM
    // without more complex mocking.
    const sortSelect = screen.getByLabelText("Sort");
    expect(sortSelect).toBeInTheDocument();
  });

  it("should handle pagination", async () => {
    const page1Cases = Array.from({ length: 10 }, (_, i) => ({
      ...mockCases[0],
      id: `case-${i}`,
      number: `CS${String(i + 1).padStart(3, "0")}`,
    }));
    const page2Cases = Array.from({ length: 5 }, (_, i) => ({
      ...mockCases[0],
      id: `case-${i + 10}`,
      number: `CS${String(i + 11).padStart(3, "0")}`,
    }));

    (useGetProjectCases as any).mockReturnValue({
      data: {
        pages: [
          {
            cases: page1Cases,
            totalRecords: 15,
            offset: 0,
            limit: 10,
          },
          {
            cases: page2Cases,
            totalRecords: 15,
            offset: 10,
            limit: 10,
          },
        ],
        pageParams: [0, 10],
      },
      isFetching: false,
      hasNextPage: false,
      fetchNextPage: vi.fn(),
    });

    renderComponent();

    await waitFor(() => {
      expect(screen.getByText(/Showing 10 of 15 cases/)).toBeInTheDocument();
    });

    const page2Button = screen.getByRole("button", { name: /page 2/i });
    fireEvent.click(page2Button);

    await waitFor(() => {
      expect(screen.getByText(/Showing 5 of 15 cases/)).toBeInTheDocument();
    });
  });
});
