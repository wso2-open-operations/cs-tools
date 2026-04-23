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
import CasesTableHeader from "@features/dashboard/components/cases-table/CasesTableHeader";

const { mockNavigate, mockUseParams } = vi.hoisted(() => {
  const navigate = vi.fn();
  const useParams = vi.fn(() => ({ projectId: "project-1" }));
  return { mockNavigate: navigate, mockUseParams: useParams };
});

// Mock react-router hooks
vi.mock("react-router", () => ({
  useNavigate: () => mockNavigate,
  useParams: mockUseParams,
}));

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
  X: () => <span>XIcon</span>,
  ChevronDown: () => <span>ChevronDown</span>,
  ChevronUp: () => <span>ChevronUp</span>,
}));

vi.mock("@components/filter-panel/ActiveFilters", () => ({
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
    isFiltersOpen: false,
    onFilterToggle: vi.fn(),
  };

  it("should render title and buttons", () => {
    render(<CasesTableHeader {...mockProps} />);

    expect(screen.getByText("Outstanding Support Cases")).toBeInTheDocument();
    expect(screen.getByText("Create")).toBeInTheDocument();
    expect(screen.getByText("Filters")).toBeInTheDocument();
  });

  it("should call onFilterToggle when filter button is clicked", () => {
    render(<CasesTableHeader {...mockProps} />);

    fireEvent.click(screen.getByText("Filters"));
    expect(mockProps.onFilterToggle).toHaveBeenCalled();
  });

  it("should show Clear Filters when filters are active", () => {
    render(<CasesTableHeader {...mockProps} activeFiltersCount={1} />);

    expect(screen.getByText("Clear Filters (1)")).toBeInTheDocument();
  });
});
