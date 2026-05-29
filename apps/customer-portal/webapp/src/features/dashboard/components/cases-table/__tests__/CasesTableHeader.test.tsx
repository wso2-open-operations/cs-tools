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
import { DASHBOARD_CASES_VIEW_TABS } from "@features/dashboard/constants/casesTable";
import { DashboardCasesViewMode } from "@features/dashboard/types/casesTable";

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

vi.mock("@features/dashboard/utils/dashboard", () => ({
  formatCasesTableClearFiltersLabel: (count: number) => `Clear Filters (${count})`,
}));

vi.mock("@components/tab-bar/TabBar", () => ({
  default: ({
    tabs,
    activeTab,
    onTabChange,
  }: {
    tabs: Array<{ id: string; label: string }>;
    activeTab: string;
    onTabChange: (tabId: string) => void;
  }) => (
    <div data-testid="cases-view-tabs">
      {tabs.map((tab) => (
        <button
          key={tab.id}
          type="button"
          aria-pressed={activeTab === tab.id}
          onClick={() => onTabChange(tab.id)}
        >
          {tab.label}
        </button>
      ))}
    </div>
  ),
}));

describe("CasesTableHeader", () => {
  const onViewModeChange = vi.fn();
  const mockProps = {
    activeFiltersCount: 0,
    isFiltersOpen: false,
    onFilterToggle: vi.fn(),
    viewTabs: DASHBOARD_CASES_VIEW_TABS,
    activeViewMode: DashboardCasesViewMode.AllCases,
    onViewModeChange,
  };

  it("should render title, view tabs, and filters button", () => {
    render(<CasesTableHeader {...mockProps} />);

    expect(screen.getByText("Outstanding Support Cases")).toBeInTheDocument();
    expect(screen.getByText("My Cases")).toBeInTheDocument();
    expect(screen.getByText("All Cases")).toBeInTheDocument();
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

  it("should call onViewModeChange when a view tab is clicked", () => {
    render(<CasesTableHeader {...mockProps} />);

    fireEvent.click(screen.getByText("My Cases"));
    expect(onViewModeChange).toHaveBeenCalledWith("my-cases");
  });
});
