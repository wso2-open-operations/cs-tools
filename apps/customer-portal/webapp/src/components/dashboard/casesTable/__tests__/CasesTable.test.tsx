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
import CasesTable from "../CasesTable";
import useGetProjectCases from "@/api/useGetProjectCases";
import useGetCasesFilters from "@/api/useGetCasesFilters";

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

vi.mock("@wso2/oxygen-ui", () => ({
  ListingTable: {
    Container: ({ children }: any) => <div>{children}</div>,
  },
  colors: {
    orange: { 500: "#ff9800" },
    green: { 500: "#4caf50" },
    blue: { 500: "#2196f3" },
    grey: { 500: "#9e9e9e" },
    red: { 500: "#f44336" },
    yellow: { 600: "#fdd835" },
    purple: { 400: "#ab47bc" },
  },
}));

vi.mock("../CasesTableHeader", () => ({
  default: ({
    onFilterClick,
    onRemoveFilter,
    onUpdateFilter,
    onClearAll,
  }: any) => (
    <div data-testid="cases-table-header">
      <button onClick={onFilterClick}>Filter</button>
      <button onClick={() => onRemoveFilter("statusId")}>Remove status</button>
      <button onClick={() => onUpdateFilter("priority", "High")}>
        Update priority
      </button>
      <button onClick={onClearAll}>Clear All</button>
    </div>
  ),
}));

vi.mock("../CasesList", () => ({
  default: ({ onPageChange, onRowsPerPageChange }: any) => (
    <div data-testid="cases-list">
      <button onClick={() => onPageChange(null, 2)}>Change Page</button>
      <button onClick={() => onRowsPerPageChange({ target: { value: "25" } })}>
        Change Rows
      </button>
    </div>
  ),
}));

vi.mock("@/components/common/filterPanel/FilterPopover", () => ({
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
  const mockProjectId = "proj-123";
  const mockUseGetProjectCases = vi.mocked(useGetProjectCases);
  const mockUseGetCasesFilters = vi.mocked(useGetCasesFilters);

  beforeEach(() => {
    vi.clearAllMocks();
    mockUseGetProjectCases.mockReturnValue({
      data: { cases: [], totalRecords: 0, offset: 0, limit: 10 },
      isLoading: false,
    } as any);
    mockUseGetCasesFilters.mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: false,
    } as any);
  });

  it("should render correctly", () => {
    render(<CasesTable projectId={mockProjectId} />);

    expect(screen.getByTestId("cases-table-header")).toBeInTheDocument();
    expect(screen.getByTestId("cases-list")).toBeInTheDocument();
    expect(mockUseGetProjectCases).toHaveBeenCalledWith(
      mockProjectId,
      expect.objectContaining({
        pagination: { offset: 0, limit: 10 },
      }),
    );
  });

  it("should open and close filter popover", () => {
    render(<CasesTable projectId={mockProjectId} />);

    expect(screen.queryByTestId("filter-popover")).toBeNull();

    // Open filter
    fireEvent.click(screen.getByText("Filter"));
    expect(screen.getByTestId("filter-popover")).toBeInTheDocument();

    // Close filter
    fireEvent.click(screen.getByText("Close"));
    expect(screen.queryByTestId("filter-popover")).toBeNull();
  });

  it("should update filters and fetch data when searching", async () => {
    render(<CasesTable projectId={mockProjectId} />);

    // Open filter and search
    fireEvent.click(screen.getByText("Filter"));
    fireEvent.click(screen.getByText("Search"));

    await waitFor(() => {
      expect(mockUseGetProjectCases).toHaveBeenCalledWith(
        mockProjectId,
        expect.objectContaining({
          filters: expect.objectContaining({
            statusId: 1,
            severityId: 2,
          }),
        }),
      );
    });
  });

  it("should handle page changes", async () => {
    render(<CasesTable projectId={mockProjectId} />);

    fireEvent.click(screen.getByText("Change Page"));

    await waitFor(() => {
      expect(mockUseGetProjectCases).toHaveBeenCalledWith(
        mockProjectId,
        expect.objectContaining({
          pagination: { offset: 20, limit: 10 }, // Page 2 (index) * 10
        }),
      );
    });
  });

  it("should handle rows per page changes", async () => {
    render(<CasesTable projectId={mockProjectId} />);

    fireEvent.click(screen.getByText("Change Rows"));

    await waitFor(() => {
      expect(mockUseGetProjectCases).toHaveBeenCalledWith(
        mockProjectId,
        expect.objectContaining({
          pagination: { offset: 0, limit: 25 }, // Reset to page 0
        }),
      );
    });
  });

  it("should handle remove filter", async () => {
    render(<CasesTable projectId={mockProjectId} />);

    // Simulate applying a filter first (Search)
    fireEvent.click(screen.getByText("Filter"));
    fireEvent.click(screen.getByText("Search"));

    // Then remove
    fireEvent.click(screen.getByText("Remove status"));

    await waitFor(() => {
      // Expect statusId to be undefined or removed from filters
      expect(mockUseGetProjectCases).toHaveBeenCalledWith(
        mockProjectId,
        expect.objectContaining({
          filters: expect.not.objectContaining({ statusId: expect.anything() }),
        }),
      );
    });
  });

  it("should propagate loading and error states to FilterPopover", () => {
    mockUseGetCasesFilters.mockReturnValue({
      isLoading: true,
      isError: true,
    } as any);

    render(<CasesTable projectId={mockProjectId} />);

    // Open filter
    fireEvent.click(screen.getByText("Filter"));

    expect(screen.getByTestId("filters-loading")).toBeInTheDocument();
    expect(screen.getByTestId("filters-error")).toBeInTheDocument();
  });
});
