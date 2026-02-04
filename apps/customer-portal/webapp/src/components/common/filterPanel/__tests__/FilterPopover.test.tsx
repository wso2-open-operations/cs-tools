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
import FilterPopover, { type FilterField } from "../FilterPopover";

// Mock Oxygen UI components
vi.mock("@wso2/oxygen-ui", () => ({
  Dialog: ({ children, open }: any) =>
    open ? <div data-testid="dialog">{children}</div> : null,
  DialogTitle: ({ children }: any) => (
    <div data-testid="dialog-title">{children}</div>
  ),
  DialogContent: ({ children }: any) => (
    <div data-testid="dialog-content">{children}</div>
  ),
  DialogActions: ({ children }: any) => (
    <div data-testid="dialog-actions">{children}</div>
  ),

  InputLabel: ({ children, id }: any) => <label id={id}>{children}</label>,
  Select: ({ children, value, onChange, labelId }: any) => (
    <select
      data-testid="select"
      data-label-id={labelId}
      value={value}
      onChange={(e) => onChange({ target: { value: e.target.value } } as any)}
    >
      {children}
    </select>
  ),
  MenuItem: ({ children, value }: any) => (
    <option value={value}>{children}</option>
  ),
  Button: ({ children, onClick }: any) => (
    <button onClick={onClick}>{children}</button>
  ),
  Typography: ({ children }: any) => <>{children}</>,
  Box: ({ children }: any) => <div data-testid="box">{children}</div>,
  IconButton: ({ children, onClick }: any) => (
    <button data-testid="icon-button" onClick={onClick}>
      {children}
    </button>
  ),
  FormControl: ({ children }: any) => (
    <div data-testid="form-control">{children}</div>
  ),
  TextField: ({ value, onChange, placeholder, label }: any) => (
    <div>
      <label>{label}</label>
      <input
        data-testid="text-field"
        value={value}
        onChange={onChange}
        placeholder={placeholder}
      />
    </div>
  ),
}));

// Mock icons
vi.mock("@wso2/oxygen-ui-icons-react", () => ({
  X: () => <span data-testid="icon-x" />,
}));

describe("FilterPopover", () => {
  const mockFilters: Record<string, any> = {
    status: "",
    priority: "",
    search: "",
  };

  const mockFields: FilterField[] = [
    {
      id: "status",
      label: "Status",
      type: "select",
      options: ["Open", "Closed"],
    },
    {
      id: "priority",
      label: "Priority",
      type: "select",
      options: [
        { value: "high", label: "High" },
        { value: "low", label: "Low" },
      ],
    },
    { id: "search", label: "Search", type: "text" },
  ];

  const mockOnClose = vi.fn();
  const mockOnSearch = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("should render correctly when open", () => {
    render(
      <FilterPopover
        open={true}
        onClose={mockOnClose}
        onSearch={mockOnSearch}
        initialFilters={mockFilters}
        fields={mockFields}
      />,
    );

    expect(screen.getByTestId("dialog")).toBeInTheDocument();
    expect(screen.getByText("Advanced Search")).toBeInTheDocument();
    expect(screen.getByText("Status")).toBeInTheDocument();
    expect(screen.getByText("Priority")).toBeInTheDocument();
    expect(screen.getAllByText("Search").length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByTestId("select")).toHaveLength(2);
    expect(screen.getByTestId("text-field")).toBeInTheDocument();
  });

  it("should render correct accessibility attributes", () => {
    render(
      <FilterPopover
        open={true}
        onClose={mockOnClose}
        onSearch={mockOnSearch}
        initialFilters={mockFilters}
        fields={mockFields}
      />,
    );

    // Check Select label association
    const statusSelect = screen.getAllByTestId("select")[0];
    expect(statusSelect).toHaveAttribute("data-label-id", "status-label");
    // Verify label exists with correct ID
    // Note: In our mock, InputLabel renders <label id={id}>
    // We can find it by ID directly using document query or ensure it exists via text search which we did in render test
    // But let's be specific about ID:
    // Note: testing-library suggests getByLabelText but since we are mocking structure differently,
    // let's just verify the ID presence on the label element if possible or rely on the Fact that text "Status" is inside a label with that ID.
    // Since our mock renders <label id="status-label">Status</label>, we can do:
    const statusLabel = screen.getByText("Status");
    expect(statusLabel.tagName).toBe("LABEL");
    expect(statusLabel).toHaveAttribute("id", "status-label");

    // Check TextField label
    const searchInputContainer = screen.getByTestId("text-field").parentElement;
    expect(searchInputContainer).toHaveTextContent("Search");
  });

  it("should not render when closed", () => {
    render(
      <FilterPopover
        open={false}
        onClose={mockOnClose}
        onSearch={mockOnSearch}
        initialFilters={mockFilters}
        fields={mockFields}
      />,
    );

    expect(screen.queryByTestId("dialog")).toBeNull();
  });

  it("should update filter state on select change", () => {
    render(
      <FilterPopover
        open={true}
        onClose={mockOnClose}
        onSearch={mockOnSearch}
        initialFilters={mockFilters}
        fields={mockFields}
      />,
    );

    const statusSelect = screen.getAllByTestId("select")[0];
    fireEvent.change(statusSelect, { target: { value: "Closed" } });

    expect(statusSelect).toHaveValue("Closed");
  });

  it("should update filter state on text input change", () => {
    render(
      <FilterPopover
        open={true}
        onClose={mockOnClose}
        onSearch={mockOnSearch}
        initialFilters={mockFilters}
        fields={mockFields}
      />,
    );

    const searchInput = screen.getByTestId("text-field");
    fireEvent.change(searchInput, { target: { value: "test query" } });

    expect(searchInput).toHaveValue("test query");
  });

  it("should call onSearch with new filters when 'Search' is clicked", () => {
    render(
      <FilterPopover
        open={true}
        onClose={mockOnClose}
        onSearch={mockOnSearch}
        initialFilters={mockFilters}
        fields={mockFields}
      />,
    );

    // Change status
    const statusSelect = screen.getAllByTestId("select")[0];
    fireEvent.change(statusSelect, { target: { value: "Open" } });

    // Change search
    const searchInput = screen.getByTestId("text-field");
    fireEvent.change(searchInput, { target: { value: "bug" } });

    // Click Search
    const searchButton = screen.getByRole("button", { name: "Search" });
    fireEvent.click(searchButton);

    expect(mockOnSearch).toHaveBeenCalledWith({
      status: "Open",
      priority: "",
      search: "bug",
    });
    expect(mockOnClose).toHaveBeenCalled();
  });

  it("should call onClose when close button is clicked", () => {
    render(
      <FilterPopover
        open={true}
        onClose={mockOnClose}
        onSearch={mockOnSearch}
        initialFilters={mockFilters}
        fields={mockFields}
      />,
    );

    fireEvent.click(screen.getByTestId("icon-button"));

    expect(mockOnClose).toHaveBeenCalled();
  });

  it("should reset filters when 'Reset' is clicked", () => {
    render(
      <FilterPopover
        open={true}
        onClose={mockOnClose}
        onSearch={mockOnSearch}
        initialFilters={{ ...mockFilters, status: "Open" }}
        fields={mockFields}
      />,
    );

    const statusSelect = screen.getAllByTestId("select")[0];
    expect(statusSelect).toHaveValue("Open");

    fireEvent.click(screen.getByText("Reset"));

    expect(statusSelect).toHaveValue("");
  });
});
