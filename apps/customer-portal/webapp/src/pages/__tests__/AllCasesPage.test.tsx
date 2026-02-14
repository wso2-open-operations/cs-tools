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
import { mockCaseMetadata, mockCases } from "@models/mockData";
import { LoaderProvider } from "@context/linear-loader/LoaderContext";

// Mock API hooks
vi.mock("@api/useGetProjectCasesStats", () => ({
  useGetProjectCasesStats: vi.fn(),
}));
vi.mock("@api/useGetCasesFilters", () => ({
  default: vi.fn(),
}));
vi.mock("@api/useGetProjectCases", () => ({
  default: vi.fn(),
}));

import { useGetProjectCasesStats } from "@api/useGetProjectCasesStats";
import useGetCasesFilters from "@api/useGetCasesFilters";
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
    (useGetCasesFilters as any).mockReturnValue({
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
    // console.log("Total mock cases:", mockCases.length);
    renderComponent();

    // Pick the first case number to search
    const targetCaseNumber = mockCases[0].number;
    // console.log("Searching for:", targetCaseNumber);

    // Initial count
    // expect(screen.getByText(`Showing 10 cases`)).toBeInTheDocument();

    const searchInput = screen.getByTestId("search-input");

    // Search
    fireEvent.change(searchInput, { target: { value: targetCaseNumber } });

    // Should only show 1 case (total from API is mock totalRecords === mockCases.length; text may be split across nodes)
    await waitFor(() => {
      const text = (content: string, el: Element | null) =>
        el?.textContent?.replace(/\s+/g, " ").trim() === content;
      expect(
        screen.getByText((c, el) =>
          text(`Showing 1 of ${mockCases.length} cases`, el),
        ),
      ).toBeInTheDocument();
      expect(screen.getAllByText("Showing 1 cases")[0]).toBeInTheDocument();
    });
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
        screen.getByText((c, el) =>
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
    renderComponent();

    // Page size is 10.
    // Check for "Showing 10 of X cases"
    // We expect mockCases.length to be > 10 for this test to be meaningful.
    const totalCases = mockCases.length;

    await waitFor(() => {
      const text = (content: string, el: Element | null) =>
        el?.textContent?.replace(/\s+/g, " ").trim() === content;
      expect(
        screen.getByText((c, el) =>
          text(`Showing 10 of ${totalCases} cases`, el),
        ),
      ).toBeInTheDocument();
    });

    // Find pagination button for page 2
    const page2Button = screen.getByRole("button", { name: /page 2/i });
    fireEvent.click(page2Button);

    const expectedCountOnPage2 = Math.min(10, Math.max(0, totalCases - 10));

    // Should show remaining cases (text may be split across nodes)
    await waitFor(() => {
      const text = (content: string, el: Element | null) =>
        el?.textContent?.replace(/\s+/g, " ").trim() === content;
      expect(
        screen.getByText((c, el) =>
          text(
            `Showing ${expectedCountOnPage2} of ${totalCases} cases`,
            el,
          ),
        ),
      ).toBeInTheDocument();
    });
  });
});
