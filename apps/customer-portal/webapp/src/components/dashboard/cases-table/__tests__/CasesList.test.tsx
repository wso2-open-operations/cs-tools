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
import CasesList from "@components/dashboard/cases-table/CasesList";

// Mock Oxygen UI components
vi.mock("@wso2/oxygen-ui", () => ({
  Box: ({ children }: any) => <div data-testid="box">{children}</div>,
  Table: ({ children }: any) => <table data-testid="table">{children}</table>,
  TableBody: ({ children }: any) => (
    <tbody data-testid="table-body">{children}</tbody>
  ),
  TableCell: ({ children }: any) => (
    <td data-testid="table-cell">{children}</td>
  ),
  TableContainer: ({ children }: any) => (
    <div data-testid="table-container">{children}</div>
  ),
  TableHead: ({ children }: any) => (
    <thead data-testid="table-head">{children}</thead>
  ),
  TableRow: ({ children }: any) => <tr data-testid="table-row">{children}</tr>,
  Typography: ({ children, onClick, ...rest }: any) => (
    <span onClick={onClick} {...rest}>
      {children}
    </span>
  ),
  Chip: ({ label }: any) => <span data-testid="chip">{label}</span>,
  IconButton: ({ children }: any) => <button>{children}</button>,
  Paper: ({ children }: any) => <div>{children}</div>,
  Avatar: ({ children }: any) => <div>{children}</div>,
  TablePagination: ({
    count,
    page,
    onPageChange,
    rowsPerPage,
    onRowsPerPageChange,
  }: any) => (
    <div data-testid="table-pagination">
      <span>Count: {count}</span>
      <span>Page: {page}</span>
      <button onClick={() => onPageChange(null, page + 1)}>Next Page</button>
      <input
        data-testid="rows-per-page-input"
        value={rowsPerPage}
        onChange={onRowsPerPageChange}
      />
    </div>
  ),
}));

vi.mock("@wso2/oxygen-ui-icons-react", () => ({
  ExternalLink: () => <span />,
  MoreVertical: () => <span />,
}));

vi.mock("../CasesTableSkeleton", () => ({
  default: () => (
    <tr data-testid="cases-skeleton">
      <td>Loading...</td>
    </tr>
  ),
}));

vi.mock("@utils/casesTable", () => ({
  getSeverityColor: () => "error.main",
  getStatusColor: () => "#000",
}));

describe("CasesList", () => {
  const mockData = {
    cases: [
      {
        id: "1",
        createdOn: "2024-01-01 10:00:00",
        title: "Test Case 1",
        number: "CS-001",
        assignedEngineer: "John Doe",
        severity: { id: 1, label: "High" },
        status: { id: 1, label: "Open" },
        project: { id: "p1", name: "Project 1" },
      },
      {
        id: "2",
        createdOn: "2024-01-02 11:00:00",
        title: "Test Case 2",
        number: "CS-002",
        assignedEngineer: null,
        severity: { id: 2, label: "Low" },
        status: { id: 2, label: "Closed" },
        project: { id: "p1", name: "Project 1" },
      },
    ],
    totalRecords: 2,
    offset: 0,
    limit: 10,
  } as any;

  const mockOnPageChange = vi.fn();
  const mockOnRowsPerPageChange = vi.fn();

  it("should render skeleton when loading", () => {
    render(
      <CasesList
        isLoading={true}
        data={undefined}
        page={0}
        rowsPerPage={10}
        onPageChange={mockOnPageChange}
        onRowsPerPageChange={mockOnRowsPerPageChange}
      />,
    );

    expect(screen.getByTestId("cases-skeleton")).toBeInTheDocument();
  });

  it("should render empty state when no cases", () => {
    render(
      <CasesList
        isLoading={false}
        data={{ cases: [], totalRecords: 0, offset: 0, limit: 10 } as any}
        page={0}
        rowsPerPage={10}
        onPageChange={mockOnPageChange}
        onRowsPerPageChange={mockOnRowsPerPageChange}
      />,
    );

    expect(screen.getByText("No cases found.")).toBeInTheDocument();
  });

  it("should render list of cases", () => {
    render(
      <CasesList
        isLoading={false}
        data={mockData}
        page={0}
        rowsPerPage={10}
        onPageChange={mockOnPageChange}
        onRowsPerPageChange={mockOnRowsPerPageChange}
      />,
    );

    expect(screen.getByText("Test Case 1")).toBeInTheDocument();
    expect(screen.getByText("ID: CS-001")).toBeInTheDocument();
    expect(screen.getByText("Test Case 2")).toBeInTheDocument();
    expect(screen.getByText("ID: CS-002")).toBeInTheDocument();
    expect(screen.getAllByTestId("table-row")).toHaveLength(3); // 1 header + 2 data rows
  });

  it("should handle page change interaction", () => {
    render(
      <CasesList
        isLoading={false}
        data={mockData}
        page={0}
        rowsPerPage={10}
        onPageChange={mockOnPageChange}
        onRowsPerPageChange={mockOnRowsPerPageChange}
      />,
    );

    fireEvent.click(screen.getByText("Next Page"));
    expect(mockOnPageChange).toHaveBeenCalledWith(null, 1);
  });

  it("should handle rows per page change interaction", () => {
    render(
      <CasesList
        isLoading={false}
        data={mockData}
        page={0}
        rowsPerPage={10}
        onPageChange={mockOnPageChange}
        onRowsPerPageChange={mockOnRowsPerPageChange}
      />,
    );

    fireEvent.change(screen.getByTestId("rows-per-page-input"), {
      target: { value: "25" },
    });
    expect(mockOnRowsPerPageChange).toHaveBeenCalled();
  });

  it("should call onCaseClick with case when case title is clicked", () => {
    const mockOnCaseClick = vi.fn();
    render(
      <CasesList
        isLoading={false}
        data={mockData}
        page={0}
        rowsPerPage={10}
        onPageChange={mockOnPageChange}
        onRowsPerPageChange={mockOnRowsPerPageChange}
        onCaseClick={mockOnCaseClick}
      />,
    );

    fireEvent.click(screen.getByText("Test Case 1"));
    expect(mockOnCaseClick).toHaveBeenCalledTimes(1);
    expect(mockOnCaseClick).toHaveBeenCalledWith(
      expect.objectContaining({ id: "1", title: "Test Case 1", number: "CS-001" }),
    );

    fireEvent.click(screen.getByText("Test Case 2"));
    expect(mockOnCaseClick).toHaveBeenCalledTimes(2);
    expect(mockOnCaseClick).toHaveBeenLastCalledWith(
      expect.objectContaining({ id: "2", title: "Test Case 2", number: "CS-002" }),
    );
  });

  it("should not call onCaseClick when not provided (title still rendered)", () => {
    render(
      <CasesList
        isLoading={false}
        data={mockData}
        page={0}
        rowsPerPage={10}
        onPageChange={mockOnPageChange}
        onRowsPerPageChange={mockOnRowsPerPageChange}
      />,
    );

    fireEvent.click(screen.getByText("Test Case 1"));
    expect(screen.getByText("Test Case 1")).toBeInTheDocument();
  });

  it("should render '--' for null or undefined values", () => {
    const dataWithNulls = {
      cases: [
        {
          id: "3",
          createdOn: null,
          title: null,
          number: "CS-003",
          assignedEngineer: null,
          severity: null,
          status: null,
          project: { id: "p1", name: "Project 1" },
        },
      ],
      totalRecords: 1,
      offset: 0,
      limit: 10,
    } as any;

    render(
      <CasesList
        isLoading={false}
        data={dataWithNulls}
        page={0}
        rowsPerPage={10}
        onPageChange={mockOnPageChange}
        onRowsPerPageChange={mockOnRowsPerPageChange}
      />,
    );

    // Check for '--' in place of title, severity, status, assignedEngineer, and date
    // Note: title is rendered inside a Typography, date has two display parts
    const dashes = screen.getAllByText("--");
    expect(dashes.length).toBeGreaterThanOrEqual(4); // title, assignedEngineer, severity, status parts
  });
});
