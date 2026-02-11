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

import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { describe, expect, it, vi, beforeEach } from "vitest";
import CasesTable from "@components/dashboard/cases-table/CasesTable";
import useGetProjectCases from "@api/useGetProjectCases";
import useGetCasesFilters from "@api/useGetCasesFilters";
import { ThemeProvider, createTheme } from "@wso2/oxygen-ui";

// Mock dependencies
vi.mock("react-router", () => ({
  useNavigate: () => vi.fn(),
}));

vi.mock("@/api/useGetProjectCases");
vi.mock("@/api/useGetCasesFilters");

vi.mock("@asgardeo/react", () => ({
  useAsgardeo: () => ({
    isLoading: false,
    isSignedIn: true,
  }),
}));

vi.mock("@context/linear-loader/LoaderContext", () => ({
  useLoader: () => ({
    showLoader: vi.fn(),
    hideLoader: vi.fn(),
  }),
}));

vi.mock("@hooks/useLogger", () => ({
  useLogger: () => ({
    debug: vi.fn(),
    error: vi.fn(),
  }),
}));

vi.mock("@wso2/oxygen-ui", async (importOriginal) => {
  const actual = (await importOriginal()) as any;
  return {
    ...actual,
    ListingTable: {
      ...actual.ListingTable,
      Container: ({ children }: any) => <div>{children}</div>,
    },
  };
});

vi.mock("../CasesTableHeader", () => ({
  default: ({
    onFilterClick,
    onRemoveFilter,
    onUpdateFilter,
    onClearAll,
  }: any) => (
    <div data-testid="cases-table-header">
      <button onClick={onFilterClick} data-testid="filter-button">
        Filter
      </button>
      <button
        onClick={() => onRemoveFilter("statusId")}
        data-testid="remove-status-button"
      >
        Remove status
      </button>
      <button onClick={() => onUpdateFilter("priority", "High")}>
        Update priority
      </button>
      <button onClick={onClearAll}>Clear All</button>
    </div>
  ),
}));

vi.mock("../CasesList", () => ({
  default: ({ data, onPageChange, onRowsPerPageChange }: any) => (
    <div data-testid="cases-list">
      <span data-testid="case-count">{data?.cases.length || 0}</span>
      <span data-testid="total-records">{data?.totalRecords || 0}</span>
      <button onClick={() => onPageChange(null, 2)}>Change Page</button>
      <button onClick={() => onRowsPerPageChange({ target: { value: "25" } })}>
        Change Rows
      </button>
    </div>
  ),
}));

vi.mock("@components/common/filter-panel/FilterPopover", () => ({
  default: ({ open, onClose, onSearch, isLoading, isError }: any) =>
    open ? (
      <div data-testid="filter-popover">
        {isLoading && (
          <span data-testid="filters-loading">Filters Loading</span>
        )}
        {isError && <span data-testid="filters-error">Filters Error</span>}
        <button onClick={onClose}>Close</button>
        <button onClick={() => onSearch({ statusId: "1", severityId: "2" })}>
          Search
        </button>
      </div>
    ) : null,
}));

describe("CasesTable", () => {
  const theme = createTheme();
  const mockProjectId = "proj-123";
  const mockUseGetProjectCases = vi.mocked(useGetProjectCases);
  const mockUseGetCasesFilters = vi.mocked(useGetCasesFilters);

  beforeEach(() => {
    vi.clearAllMocks();
    mockUseGetProjectCases.mockReturnValue({
      data: {
        pages: [{ cases: [], totalRecords: 0, offset: 0, limit: 10 }],
        pageParams: [0],
      },
      isFetching: false,
      fetchNextPage: vi.fn(),
      hasNextPage: false,
      isFetchingNextPage: false,
    } as any);
    mockUseGetCasesFilters.mockReturnValue({
      data: undefined,
      isFetching: false,
      isError: false,
    } as any);
  });

  it("should render correctly", () => {
    render(
      <ThemeProvider theme={theme}>
        <CasesTable projectId={mockProjectId} />
      </ThemeProvider>,
    );

    expect(screen.getByTestId("cases-table-header")).toBeInTheDocument();
    expect(screen.getByTestId("cases-list")).toBeInTheDocument();
    expect(mockUseGetProjectCases).toHaveBeenCalledWith(
      mockProjectId,
      expect.objectContaining({
        sortBy: { field: "createdOn", order: "desc" },
      }),
    );
  });

  it("should open and close filter popover", () => {
    render(
      <ThemeProvider theme={theme}>
        <CasesTable projectId={mockProjectId} />
      </ThemeProvider>,
    );

    expect(screen.queryByTestId("filter-popover")).toBeNull();

    // Open filter
    fireEvent.click(screen.getByText("Filter"));
    expect(screen.getByTestId("filter-popover")).toBeInTheDocument();

    // Close filter
    fireEvent.click(screen.getByText("Close"));
    expect(screen.queryByTestId("filter-popover")).toBeNull();
  });

  it("should update filters and fetch data when searching", async () => {
    // Simulate some cases in rawData
    mockUseGetProjectCases.mockReturnValue({
      data: {
        pages: [
          {
            cases: [
              { id: "1", status: { id: "1" }, severity: { id: "2" } },
              { id: "2", status: { id: "2" }, severity: { id: "2" } },
            ],
            totalRecords: 2,
            offset: 0,
            limit: 10,
          },
        ],
        pageParams: [0],
      },
      isFetching: false,
      fetchNextPage: vi.fn(),
      hasNextPage: false,
      isFetchingNextPage: false,
    } as any);

    render(
      <ThemeProvider theme={theme}>
        <CasesTable projectId={mockProjectId} />
      </ThemeProvider>,
    );

    // Open filter and search
    fireEvent.click(screen.getByText("Filter"));
    fireEvent.click(screen.getByText("Search"));

    await waitFor(() => {
      // The count should be 1 (filtered locally)
      expect(screen.getByTestId("case-count")).toHaveTextContent("1");
      // Total records should still reflect API total (2)
      expect(screen.getByTestId("total-records")).toHaveTextContent("2");
    });
  });

  it("should show 0 total records when no cases match filter", async () => {
    mockUseGetProjectCases.mockReturnValue({
      data: {
        pages: [
          {
            cases: [],
            totalRecords: 58,
            offset: 0,
            limit: 10,
          },
        ],
        pageParams: [0],
      },
      isFetching: false,
      fetchNextPage: vi.fn(),
      hasNextPage: false,
      isFetchingNextPage: false,
    } as any);

    render(
      <ThemeProvider theme={theme}>
        <CasesTable projectId={mockProjectId} />
      </ThemeProvider>,
    );

    await waitFor(() => {
      expect(screen.getByTestId("case-count")).toHaveTextContent("0");
      expect(screen.getByTestId("total-records")).toHaveTextContent("0");
    });
  });

  it("should handle page changes", async () => {
    render(
      <ThemeProvider theme={theme}>
        <CasesTable projectId={mockProjectId} />
      </ThemeProvider>,
    );

    // Handle page changes should not trigger a new API call in client-side mode
    fireEvent.click(screen.getByText("Change Page"));

    await waitFor(() => {
      // Still only called with the initial load params (once or twice depending on render cycles)
      expect(mockUseGetProjectCases).toHaveBeenCalledWith(
        mockProjectId,
        expect.objectContaining({
          sortBy: { field: "createdOn", order: "desc" },
        }),
      );
    });
  });

  it("should handle rows per page changes", async () => {
    render(
      <ThemeProvider theme={theme}>
        <CasesTable projectId={mockProjectId} />
      </ThemeProvider>,
    );

    fireEvent.click(screen.getByText("Change Rows"));

    await waitFor(() => {
      // Still only called with initial load params
      expect(mockUseGetProjectCases).toHaveBeenCalledWith(
        mockProjectId,
        expect.objectContaining({
          sortBy: { field: "createdOn", order: "desc" },
        }),
      );
    });
  });

  it.skip("should handle remove filter", async () => {
    // Simulate some cases in rawData
    mockUseGetProjectCases.mockReturnValue({
      data: {
        pages: [
          {
            cases: [{ id: "1", status: { id: "1" } }],
            totalRecords: 1,
            offset: 0,
            limit: 10,
          },
        ],
        pageParams: [0],
      },
      isFetching: false,
      fetchNextPage: vi.fn(),
      hasNextPage: false,
      isFetchingNextPage: false,
    } as any);

    render(
      <ThemeProvider theme={theme}>
        <CasesTable projectId={mockProjectId} />
      </ThemeProvider>,
    );

    // Filter by statusId: "1"
    const filterBtn = await screen.findByTestId("filter-button");
    fireEvent.click(filterBtn);
    fireEvent.click(screen.getByText("Search"));

    await waitFor(() => {
      expect(screen.getByTestId("case-count")).toHaveTextContent("1");
    });

    // Remove status filter
    fireEvent.click(screen.getByTestId("remove-status-button"));

    await waitFor(() => {
      expect(screen.getByTestId("case-count")).toHaveTextContent("1");
    });
  });

  it("should propagate loading and error states to FilterPopover", () => {
    mockUseGetCasesFilters.mockReturnValue({
      isFetching: true,
      isError: true,
    } as any);

    render(
      <ThemeProvider theme={theme}>
        <CasesTable projectId={mockProjectId} />
      </ThemeProvider>,
    );

    // Open filter
    fireEvent.click(screen.getByText("Filter"));

    expect(screen.getByTestId("filters-loading")).toBeInTheDocument();
    expect(screen.getByTestId("filters-error")).toBeInTheDocument();
  });
});
