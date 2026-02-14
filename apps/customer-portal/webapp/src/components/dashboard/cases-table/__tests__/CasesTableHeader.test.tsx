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
import { describe, expect, it, vi } from "vitest";
import CasesTableHeader from "@components/dashboard/cases-table/CasesTableHeader";

// Mock Oxygen UI components (include Menu for ActiveFilters)
vi.mock("@wso2/oxygen-ui", () => ({
  Box: ({ children }: any) => <div data-testid="box">{children}</div>,
  Typography: ({ children }: any) => <span>{children}</span>,
  Button: ({ children, onClick }: any) => (
    <button onClick={onClick}>{children}</button>
  ),
  Menu: ({ children, open }: any) =>
    open ? <div data-testid="menu">{children}</div> : null,
  MenuItem: ({ children }: any) => <div>{children}</div>,
}));

vi.mock("@wso2/oxygen-ui-icons-react", () => ({
  ListFilter: () => <span>FilterIcon</span>,
  Plus: () => <span>PlusIcon</span>,
}));

vi.mock("@components/common/filter-panel/ActiveFilters", () => ({
  default: ({ appliedFilters }: any) => (
    <div data-testid="active-filters">
      {Object.keys(appliedFilters).map((key) => (
        <span key={key}>{key}</span>
      ))}
    </div>
  ),
}));

describe("CasesTableHeader", () => {
  const mockProps = {
    activeFiltersCount: 0,
    appliedFilters: {},
    filterFields: [],
    onRemoveFilter: vi.fn(),
    onClearAll: vi.fn(),
    onUpdateFilter: vi.fn(),
    onFilterClick: vi.fn(),
    onCreateCase: vi.fn(),
  };

  it("should render title and buttons", () => {
    render(<CasesTableHeader {...mockProps} />);

    expect(screen.getByText("Outstanding cases")).toBeInTheDocument();
    expect(screen.getByText("Create case")).toBeInTheDocument();
    expect(screen.getByText("Filters")).toBeInTheDocument();
  });

  it("should call onCreateCase when button is clicked", () => {
    render(<CasesTableHeader {...mockProps} />);

    fireEvent.click(screen.getByText("Create case"));
    expect(mockProps.onCreateCase).toHaveBeenCalled();
  });

  it("should call onFilterClick when filter button is clicked", () => {
    render(<CasesTableHeader {...mockProps} />);

    fireEvent.click(screen.getByText("Filters"));
    expect(mockProps.onFilterClick).toHaveBeenCalled();
  });

  it("should pass active filters to ActiveFilters component", () => {
    render(
      <CasesTableHeader
        {...mockProps}
        activeFiltersCount={1}
        appliedFilters={{ status: "Open" }}
      />,
    );

    expect(screen.getByTestId("active-filters")).toBeInTheDocument();
    expect(screen.getByText("status")).toBeInTheDocument();
  });
});
