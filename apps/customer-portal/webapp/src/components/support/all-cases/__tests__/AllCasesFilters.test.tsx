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
import AllCasesFilters from "@components/support/all-cases/AllCasesFilters";
import { mockCaseMetadata } from "@models/mockData";
import { ThemeProvider, createTheme } from "@wso2/oxygen-ui";

describe("AllCasesFilters", () => {
  const theme = createTheme();
  const mockOnFilterChange = vi.fn();
  const defaultFilters = {
    statusId: "",
    severityId: "",
    caseTypes: "",
    deploymentId: "",
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("should render all filter select components", () => {
    render(
      <ThemeProvider theme={theme}>
        <AllCasesFilters
          filters={defaultFilters}
          filterMetadata={mockCaseMetadata}
          onFilterChange={mockOnFilterChange}
        />
      </ThemeProvider>,
    );

    // Check for labels (Status, Severity, Category, Deployment)
    // MUI renders labels in multiple places (InputLabel, Legend), so we check getAllByText
    expect(screen.getAllByText("Status")[0]).toBeInTheDocument();
    expect(screen.getAllByText("Severity")[0]).toBeInTheDocument();
    expect(screen.getAllByText("Category")[0]).toBeInTheDocument();
    expect(screen.getAllByText("Deployment")[0]).toBeInTheDocument();
  });

  it("should call onFilterChange when a filter is changed", async () => {
    render(
      <ThemeProvider theme={theme}>
        <AllCasesFilters
          filters={defaultFilters}
          filterMetadata={mockCaseMetadata}
          onFilterChange={mockOnFilterChange}
        />
      </ThemeProvider>,
    );

    // Find the Status select trigger by role and name
    const statusSelect = screen.getByRole("combobox", { name: /Status/i });

    // MUI Select uses a hidden input but also a visible div for the value
    // We click the visible div to open the menu
    fireEvent.mouseDown(statusSelect);

    // MUI Select options are rendered in a Portal
    // We use findByText to find the option by its label
    const option = await screen.findByText("Open");
    fireEvent.click(option);

    // Filter key for Status is 'statusId'
    expect(mockOnFilterChange).toHaveBeenCalledWith("statusId", "1");
  });
});
