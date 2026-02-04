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
import { beforeEach, describe, expect, it, vi } from "vitest";
import ActiveFilters from "../ActiveFilters";

// Mock Oxygen UI components
vi.mock("@wso2/oxygen-ui", () => ({
  Box: ({ children, onClick }: any) => (
    <div data-testid="box" onClick={onClick}>
      {children}
    </div>
  ),
  Chip: ({ label, onDelete, onClick, icon }: any) => (
    <div data-testid="chip">
      <span onClick={onClick}>
        {label}
        {icon}
      </span>
      {onDelete && (
        <button data-testid="chip-delete" onClick={onDelete}>
          x
        </button>
      )}
    </div>
  ),
  Button: ({ children, onClick }: any) => (
    <button onClick={onClick}>{children}</button>
  ),
  Menu: ({ children, open }: any) =>
    open ? <div data-testid="menu">{children}</div> : null,
  MenuItem: ({ children, onClick, selected }: any) => (
    <div data-testid="menu-item" data-selected={selected} onClick={onClick}>
      {children}
    </div>
  ),
}));

// Mock icons
vi.mock("@wso2/oxygen-ui-icons-react", () => ({
  ChevronDown: () => <span data-testid="chevron-down" />,
}));

describe("ActiveFilters", () => {
  const mockFilters = {
    status: "Open",
    priority: "High",
    search: "",
  };

  const mockFilterFields = [
    { id: "status", label: "Status", options: ["Open", "Closed"] },
    { id: "priority", label: "Priority", options: ["High", "Low"] },
    { id: "search", label: "Search" },
  ];

  const mockOnRemoveFilter = vi.fn();
  const mockOnClearAll = vi.fn();
  const mockOnUpdateFilter = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("should render active filters correctly", () => {
    render(
      <ActiveFilters
        appliedFilters={mockFilters}
        filterFields={mockFilterFields}
        onRemoveFilter={mockOnRemoveFilter}
        onClearAll={mockOnClearAll}
      />,
    );

    expect(screen.getByText("Open")).toBeInTheDocument();
    expect(screen.getByText("High")).toBeInTheDocument();
    expect(screen.getAllByTestId("chip")).toHaveLength(3); // All configured fields are rendered
    expect(screen.getByText("Clear filters")).toBeInTheDocument();
  });

  it("should return null if no filters are active", () => {
    const { container } = render(
      <ActiveFilters
        appliedFilters={{ status: "", priority: "" }}
        filterFields={mockFilterFields}
        onRemoveFilter={mockOnRemoveFilter}
        onClearAll={mockOnClearAll}
      />,
    );

    expect(container).toBeEmptyDOMElement();
  });

  it("should call onRemoveFilter when a filter chip is deleted", () => {
    render(
      <ActiveFilters
        appliedFilters={mockFilters}
        filterFields={mockFilterFields}
        onRemoveFilter={mockOnRemoveFilter}
        onClearAll={mockOnClearAll}
      />,
    );

    const deleteButtons = screen.getAllByTestId("chip-delete");
    fireEvent.click(deleteButtons[0]);

    expect(mockOnRemoveFilter).toHaveBeenCalledWith("status");
  });

  it("should call onClearAll when 'Clear filters' is clicked", () => {
    render(
      <ActiveFilters
        appliedFilters={mockFilters}
        filterFields={mockFilterFields}
        onRemoveFilter={mockOnRemoveFilter}
        onClearAll={mockOnClearAll}
      />,
    );

    fireEvent.click(screen.getByText("Clear filters"));

    expect(mockOnClearAll).toHaveBeenCalled();
  });

  it("should open menu on chip click and update filter on item select", () => {
    render(
      <ActiveFilters
        appliedFilters={mockFilters}
        filterFields={mockFilterFields}
        onRemoveFilter={mockOnRemoveFilter}
        onClearAll={mockOnClearAll}
        onUpdateFilter={mockOnUpdateFilter}
      />,
    );

    // Click on 'Open' status chip to open menu
    const statusChipLabel = screen.getByText("Open");
    fireEvent.click(statusChipLabel);

    expect(screen.getByTestId("menu")).toBeInTheDocument();

    // Select 'Closed' from menu
    const closedOption = screen.getByText("Closed");
    fireEvent.click(closedOption);

    expect(mockOnUpdateFilter).toHaveBeenCalledWith("status", "Closed");
  });

  it("should not open menu if field has no options", () => {
    render(
      <ActiveFilters
        appliedFilters={{ ...mockFilters, search: "test" }} // Add a value for search to make it render
        filterFields={mockFilterFields}
        onRemoveFilter={mockOnRemoveFilter}
        onClearAll={mockOnClearAll}
        onUpdateFilter={mockOnUpdateFilter}
      />,
    );
    // Search field has no options config
    const searchChip = screen.getByText("test");
    fireEvent.click(searchChip);

    expect(screen.queryByTestId("menu")).toBeNull();
  });
});
