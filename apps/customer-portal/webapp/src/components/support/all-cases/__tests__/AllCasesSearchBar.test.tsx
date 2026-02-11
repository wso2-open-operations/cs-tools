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

import { render, screen, fireEvent } from "@testing-library/react";
import { describe, expect, it, vi, beforeEach } from "vitest";
import AllCasesSearchBar from "@components/support/all-cases/AllCasesSearchBar";
import { ThemeProvider, createTheme } from "@wso2/oxygen-ui";

// Mock AllCasesFilters to simplify
vi.mock("../AllCasesFilters", () => ({
  default: () => <div data-testid="all-cases-filters" />,
}));

describe("AllCasesSearchBar", () => {
  const theme = createTheme();
  const mockOnSearchChange = vi.fn();
  const mockOnFilterChange = vi.fn();
  const mockOnClearFilters = vi.fn();
  const defaultFilters = {
    statusId: "",
    severityId: "",
    issueTypes: "",
    deploymentId: "",
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("should render the search input", () => {
    render(
      <ThemeProvider theme={theme}>
        <AllCasesSearchBar
          searchTerm=""
          onSearchChange={mockOnSearchChange}
          isFiltersOpen={false}
          onFiltersToggle={vi.fn()}
          filters={defaultFilters}
          filterMetadata={undefined}
          onFilterChange={mockOnFilterChange}
          onClearFilters={mockOnClearFilters}
        />
      </ThemeProvider>,
    );

    expect(
      screen.getByPlaceholderText(/Search cases by ID/i),
    ).toBeInTheDocument();
  });

  it("should call onSearchChange when typing", () => {
    render(
      <ThemeProvider theme={theme}>
        <AllCasesSearchBar
          searchTerm=""
          onSearchChange={mockOnSearchChange}
          isFiltersOpen={false}
          onFiltersToggle={vi.fn()}
          filters={defaultFilters}
          filterMetadata={undefined}
          onFilterChange={mockOnFilterChange}
          onClearFilters={mockOnClearFilters}
        />
      </ThemeProvider>,
    );

    const input = screen.getByPlaceholderText(/Search cases by ID/i);
    fireEvent.change(input, { target: { value: "test" } });

    expect(mockOnSearchChange).toHaveBeenCalledWith("test");
  });

  it("should call onFiltersToggle when clicking the filter button", () => {
    const mockOnFiltersToggle = vi.fn();
    render(
      <ThemeProvider theme={theme}>
        <AllCasesSearchBar
          searchTerm=""
          onSearchChange={mockOnSearchChange}
          isFiltersOpen={false}
          onFiltersToggle={mockOnFiltersToggle}
          filters={defaultFilters}
          filterMetadata={undefined}
          onFilterChange={mockOnFilterChange}
          onClearFilters={mockOnClearFilters}
        />
      </ThemeProvider>,
    );

    const filterButton = screen.getByText("Filters");
    fireEvent.click(filterButton);

    expect(mockOnFiltersToggle).toHaveBeenCalled();
  });

  it("should call onClearFilters when clicking clear button if filters are active", () => {
    const activeFilters = { ...defaultFilters, statusId: "Open" };

    render(
      <ThemeProvider theme={theme}>
        <AllCasesSearchBar
          searchTerm=""
          onSearchChange={mockOnSearchChange}
          isFiltersOpen={false}
          onFiltersToggle={vi.fn()}
          filters={activeFilters}
          filterMetadata={undefined}
          onFilterChange={mockOnFilterChange}
          onClearFilters={mockOnClearFilters}
        />
      </ThemeProvider>,
    );

    const clearButton = screen.getByText(/Clear Filters/i);
    fireEvent.click(clearButton);

    expect(mockOnClearFilters).toHaveBeenCalled();
  });
});
