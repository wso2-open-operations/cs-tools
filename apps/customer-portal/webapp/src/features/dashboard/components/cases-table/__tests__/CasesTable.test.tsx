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
import CasesTable from "@features/dashboard/components/cases-table/CasesTable";
import { useGetProjectCasesPage } from "@api/useGetProjectCasesPage";
import useGetProjectFilters from "@api/useGetProjectFilters";
import { ThemeProvider, createTheme } from "@wso2/oxygen-ui";

const mockNavigate = vi.fn();

// Mock dependencies
vi.mock("react-router", () => ({
  useNavigate: () => mockNavigate,
  useLocation: () => ({ pathname: "/projects/p1/dashboard", state: null }),
}));

vi.mock("@api/useGetProjectCasesPage");
vi.mock("@api/useGetProjectFilters");
vi.mock("@api/useGetProjectCases", () => ({
  default: () => ({
    data: undefined,
    isLoading: false,
    isError: false,
    hasNextPage: false,
    isFetchingNextPage: false,
    fetchNextPage: vi.fn(),
  }),
}));
vi.mock("@api/usePostProjectDeploymentsSearch", () => ({
  usePostProjectDeploymentsSearchInfinite: () => ({
    data: undefined,
    isLoading: false,
    hasNextPage: false,
    isFetchingNextPage: false,
    fetchNextPage: vi.fn(),
  }),
}));
vi.mock("@features/settings/api/useGetProjectContacts", () => ({
  default: () => ({ data: [], isLoading: false }),
}));
vi.mock("@api/usePostProjectDeploymentsSearch", () => ({
  usePostProjectDeploymentsSearchInfinite: () => ({
    data: undefined,
    isLoading: false,
    hasNextPage: false,
    isFetchingNextPage: false,
    fetchNextPage: vi.fn(),
  }),
}));
vi.mock("@features/settings/api/useGetProjectContacts", () => ({
  default: () => ({ data: [], isLoading: false }),
}));

vi.mock("@asgardeo/react", () => ({
  useAsgardeo: () => ({
    isLoading: false,
    isSignedIn: true,
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
  default: ({ activeFiltersCount, isFiltersOpen, onFilterToggle }: any) => (
    <div data-testid="cases-table-header">
      <span data-testid="active-filters-count">{activeFiltersCount}</span>
      <span data-testid="filters-open">{String(isFiltersOpen)}</span>
      <button onClick={onFilterToggle} data-testid="filter-button">
        Filter
      </button>
    </div>
  ),
}));

vi.mock("../CasesFilters", () => ({
  default: ({ filterFields, onFilterChange }: any) => (
    <div data-testid="cases-filters">
      <span data-testid="filter-field-ids">
        {filterFields.map((f: any) => f.id).join(",")}
      </span>
      <button onClick={() => onFilterChange("severityIds", ["2"])}>
        Set severity
      </button>
    </div>
  ),
}));

vi.mock("../CasesList", () => ({
  default: ({
    data,
    onPageChange,
    onRowsPerPageChange,
    onCaseClick,
  }: any) => (
    <div data-testid="cases-list">
      <span data-testid="case-count">{data?.cases.length || 0}</span>
      <span data-testid="total-records">{data?.totalRecords || 0}</span>
      <button onClick={() => onPageChange(null, 2)}>Change Page</button>
      <button onClick={() => onRowsPerPageChange({ target: { value: "25" } })}>
        Change Rows
      </button>
      {onCaseClick && (
        <button
          onClick={() => onCaseClick({ id: "case-1", title: "Case 1" })}
          data-testid="case-title-click"
        >
          Case 1
        </button>
      )}
    </div>
  ),
}));

describe("CasesTable", () => {
  const theme = createTheme();
  const mockProjectId = "proj-123";
  const mockUseGetProjectCasesPage = vi.mocked(useGetProjectCasesPage);
  const mockUseGetCasesFilters = vi.mocked(useGetProjectFilters);

  beforeEach(() => {
    vi.clearAllMocks();
    mockUseGetProjectCasesPage.mockReturnValue({
      data: { cases: [], totalRecords: 0, offset: 0, limit: 10 },
      isFetching: false,
      isError: false,
    } as unknown as ReturnType<typeof useGetProjectCasesPage>);
    mockUseGetCasesFilters.mockReturnValue({
      data: undefined,
      isFetching: false,
      isError: false,
    } as unknown as ReturnType<typeof useGetProjectFilters>);
  });

  it("should render correctly", () => {
    render(
      <ThemeProvider theme={theme}>
        <CasesTable projectId={mockProjectId} />
      </ThemeProvider>,
    );

    expect(screen.getByTestId("cases-table-header")).toBeInTheDocument();
    expect(screen.getByTestId("cases-list")).toBeInTheDocument();
    expect(mockUseGetProjectCasesPage).toHaveBeenCalledWith(
      mockProjectId,
      expect.objectContaining({
        sortBy: { field: "updatedOn", order: "desc" },
      }),
      0,
      5,
      { enabled: true },
    );
  });

  it("should open and close the inline filters when the filter toggle is pressed", () => {
    render(
      <ThemeProvider theme={theme}>
        <CasesTable projectId={mockProjectId} />
      </ThemeProvider>,
    );

    expect(screen.queryByTestId("cases-filters")).toBeNull();

    fireEvent.click(screen.getByTestId("filter-button"));
    expect(screen.getByTestId("cases-filters")).toBeInTheDocument();
    // The Created By filter sits alongside the metadata-driven filters.
    expect(screen.getByTestId("filter-field-ids")).toHaveTextContent(
      "createdBy",
    );

    fireEvent.click(screen.getByTestId("filter-button"));
    expect(screen.queryByTestId("cases-filters")).toBeNull();
  });

  it("should send the selected filter to the page query and reset to the first page", async () => {
    render(
      <ThemeProvider theme={theme}>
        <CasesTable projectId={mockProjectId} />
      </ThemeProvider>,
    );

    fireEvent.click(screen.getByText("Change Page"));
    fireEvent.click(screen.getByTestId("filter-button"));
    fireEvent.click(screen.getByText("Set severity"));

    await waitFor(() => {
      const calls = mockUseGetProjectCasesPage.mock.calls;
      const lastCall = calls.at(-1)!;
      expect(lastCall[1]?.filters?.severityIds).toEqual([2]);
      expect(lastCall[2]).toBe(0); // offset reset by the filter change
    });
    expect(screen.getByTestId("active-filters-count")).toHaveTextContent("1");
  });

  it("should clear active filters when the filter toggle is pressed with filters applied", async () => {
    render(
      <ThemeProvider theme={theme}>
        <CasesTable projectId={mockProjectId} />
      </ThemeProvider>,
    );

    fireEvent.click(screen.getByTestId("filter-button"));
    fireEvent.click(screen.getByText("Set severity"));
    await waitFor(() =>
      expect(screen.getByTestId("active-filters-count")).toHaveTextContent("1"),
    );

    fireEvent.click(screen.getByTestId("filter-button"));

    await waitFor(() => {
      expect(screen.getByTestId("active-filters-count")).toHaveTextContent("0");
      const calls = mockUseGetProjectCasesPage.mock.calls;
      expect(calls.at(-1)![1]?.filters?.severityIds).toBeUndefined();
    });
  });

  it("should show 0 total records when no cases match filter", async () => {
    mockUseGetProjectCasesPage.mockReturnValue({
      data: {
        cases: [],
        totalRecords: 0,
        offset: 0,
        limit: 10,
      },
      isFetching: false,
      isError: false,
    } as unknown as ReturnType<typeof useGetProjectCasesPage>);

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

    fireEvent.click(screen.getByText("Change Page"));

    await waitFor(() => {
      const calls = mockUseGetProjectCasesPage.mock.calls;
      const lastCall = calls[calls.length - 1];
      const rowsPerPage = 5;
      expect(lastCall[2]).toBe(2 * rowsPerPage); // offset for page 2
      expect(lastCall[3]).toBe(rowsPerPage); // limit
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
      const calls = mockUseGetProjectCasesPage.mock.calls;
      const lastCall = calls[calls.length - 1];
      expect(lastCall[2]).toBe(0); // offset (page reset to 0)
      expect(lastCall[3]).toBe(25); // new rows per page
    });
  });

  it.skip("should handle remove filter", async () => {
    mockUseGetProjectCasesPage.mockReturnValue({
      data: {
        cases: [{ id: "1", status: { id: "1" } }] as never[],
        totalRecords: 1,
        offset: 0,
        limit: 10,
      },
      isFetching: false,
      isError: false,
    } as unknown as ReturnType<typeof useGetProjectCasesPage>);

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

  it("should navigate to case details when onCaseClick is triggered from list", () => {
    render(
      <ThemeProvider theme={theme}>
        <CasesTable projectId={mockProjectId} />
      </ThemeProvider>,
    );

    const caseTitleButton = screen.getByTestId("case-title-click");
    fireEvent.click(caseTitleButton);

    // useModifierAwareNavigate forwards (path, options) to navigate.
    expect(mockNavigate).toHaveBeenCalledWith(
      `/projects/${mockProjectId}/support/cases/case-1`,
      undefined,
    );
  });
});
