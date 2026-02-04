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
import TabBar, { type TabBarProps, type TabOption } from "../TabBar";

// Mock Icon Component
const MockIcon = ({ size }: { size?: number }) => (
  <span data-testid="mock-icon">{size}</span>
);

// Mock @wso2/oxygen-ui
vi.mock("@wso2/oxygen-ui", () => ({
  Card: ({ children, role, sx, ...props }: any) => (
    <div data-testid="card" role={role} style={sx} {...props}>
      {children}
    </div>
  ),
  Button: ({
    children,
    onClick,
    role,
    variant,
    startIcon,
    sx,
    ...props
  }: any) => (
    <button
      data-testid={`tab-button-${props["aria-selected"]}`}
      role={role}
      onClick={onClick}
      aria-selected={props["aria-selected"]}
      style={sx}
      {...props}
    >
      {startIcon && <span data-testid="button-start-icon">{startIcon}</span>}
      {children}
    </button>
  ),
  Box: ({ children, sx, component, ...props }: any) => (
    <span data-testid="badge" style={sx} {...props}>
      {children}
    </span>
  ),
}));

describe("TabBar", () => {
  const mockOnTabChange = vi.fn();

  const createMockTabs = (): TabOption[] => [
    { id: "tab1", label: "Overview" },
    { id: "tab2", label: "Details" },
    { id: "tab3", label: "Settings" },
  ];

  const defaultProps: TabBarProps = {
    tabs: createMockTabs(),
    activeTab: "tab1",
    onTabChange: mockOnTabChange,
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe("Rendering", () => {
    it("should render all tabs correctly", () => {
      render(<TabBar {...defaultProps} />);

      expect(screen.getByText("Overview")).toBeInTheDocument();
      expect(screen.getByText("Details")).toBeInTheDocument();
      expect(screen.getByText("Settings")).toBeInTheDocument();
    });

    it("should render with correct role attributes", () => {
      render(<TabBar {...defaultProps} />);

      const tablist = screen.getByRole("tablist");
      expect(tablist).toBeInTheDocument();

      const tabs = screen.getAllByRole("tab");
      expect(tabs).toHaveLength(3);
    });

    it("should set aria-selected=true for active tab", () => {
      render(<TabBar {...defaultProps} />);

      const tabs = screen.getAllByRole("tab");
      expect(tabs[0]).toHaveAttribute("aria-selected", "true");
      expect(tabs[1]).toHaveAttribute("aria-selected", "false");
      expect(tabs[2]).toHaveAttribute("aria-selected", "false");
    });

    it("should render with custom className when provided", () => {
      const props = { ...defaultProps, className: "custom-class" };
      render(<TabBar {...props} />);

      expect(screen.getByRole("tablist")).toBeInTheDocument();
    });
  });

  describe("Icons", () => {
    it("should render tabs with icons when provided", () => {
      const tabsWithIcons: TabOption[] = [
        { id: "tab1", label: "Home", icon: MockIcon },
        { id: "tab2", label: "Profile", icon: MockIcon },
      ];

      render(
        <TabBar
          tabs={tabsWithIcons}
          activeTab="tab1"
          onTabChange={mockOnTabChange}
        />,
      );

      const icons = screen.getAllByTestId("mock-icon");
      expect(icons).toHaveLength(2);
      expect(icons[0]).toHaveTextContent("16");
    });

    it("should render tabs without icons when not provided", () => {
      render(<TabBar {...defaultProps} />);

      const icons = screen.queryAllByTestId("mock-icon");
      expect(icons).toHaveLength(0);
    });
  });

  describe("Count Badges", () => {
    it("should render count badges when provided", () => {
      const tabsWithCounts: TabOption[] = [
        { id: "tab1", label: "Notifications", count: 5 },
        { id: "tab2", label: "Messages", count: 12 },
      ];

      render(
        <TabBar
          tabs={tabsWithCounts}
          activeTab="tab1"
          onTabChange={mockOnTabChange}
        />,
      );

      expect(screen.getByText("5")).toBeInTheDocument();
      expect(screen.getByText("12")).toBeInTheDocument();
    });

    it("should render string count values", () => {
      const tabsWithStringCount: TabOption[] = [
        { id: "tab1", label: "Items", count: "99+" },
      ];

      render(
        <TabBar
          tabs={tabsWithStringCount}
          activeTab="tab1"
          onTabChange={mockOnTabChange}
        />,
      );

      expect(screen.getByText("99+")).toBeInTheDocument();
    });

    it("should not render badge when count is undefined", () => {
      render(<TabBar {...defaultProps} />);

      const badges = screen.queryAllByTestId("badge");
      expect(badges).toHaveLength(0);
    });

    it("should render badge with custom color when provided", () => {
      const tabsWithBadgeColor: TabOption[] = [
        { id: "tab1", label: "Alerts", count: 3, badgeColor: "error.main" },
      ];

      render(
        <TabBar
          tabs={tabsWithBadgeColor}
          activeTab="tab1"
          onTabChange={mockOnTabChange}
        />,
      );

      const badge = screen.getByTestId("badge");
      expect(badge).toBeInTheDocument();
    });

    it("should render zero count", () => {
      const tabsWithZeroCount: TabOption[] = [
        { id: "tab1", label: "Empty", count: 0 },
      ];

      render(
        <TabBar
          tabs={tabsWithZeroCount}
          activeTab="tab1"
          onTabChange={mockOnTabChange}
        />,
      );

      expect(screen.getByText("0")).toBeInTheDocument();
    });
  });

  describe("Interaction", () => {
    it("should call onTabChange when a tab is clicked", () => {
      render(<TabBar {...defaultProps} />);

      const detailsTab = screen.getByText("Details");
      fireEvent.click(detailsTab);

      expect(mockOnTabChange).toHaveBeenCalledWith("tab2");
      expect(mockOnTabChange).toHaveBeenCalledTimes(1);
    });

    it("should call onTabChange with correct tab id for each tab", () => {
      render(<TabBar {...defaultProps} />);

      fireEvent.click(screen.getByText("Overview"));
      expect(mockOnTabChange).toHaveBeenCalledWith("tab1");

      vi.clearAllMocks();

      fireEvent.click(screen.getByText("Settings"));
      expect(mockOnTabChange).toHaveBeenCalledWith("tab3");
    });

    it("should allow clicking the active tab", () => {
      render(<TabBar {...defaultProps} />);

      const activeTab = screen.getByText("Overview");
      fireEvent.click(activeTab);

      expect(mockOnTabChange).toHaveBeenCalledWith("tab1");
    });
  });

  describe("Multiple Tabs with All Features", () => {
    it("should render tabs with icons, counts, and custom badge colors", () => {
      const complexTabs: TabOption[] = [
        { id: "tab1", label: "Overview", icon: MockIcon, count: 1 },
        { id: "tab2", label: "Deployments", icon: MockIcon, count: 5 },
        {
          id: "tab3",
          label: "Time Tracking",
          icon: MockIcon,
          count: 6,
          badgeColor: "success.main",
        },
      ];

      render(
        <TabBar
          tabs={complexTabs}
          activeTab="tab1"
          onTabChange={mockOnTabChange}
        />,
      );

      expect(screen.getByText("Overview")).toBeInTheDocument();
      expect(screen.getByText("Deployments")).toBeInTheDocument();
      expect(screen.getByText("Time Tracking")).toBeInTheDocument();

      expect(screen.getByText("1")).toBeInTheDocument();
      expect(screen.getByText("5")).toBeInTheDocument();
      expect(screen.getByText("6")).toBeInTheDocument();

      const icons = screen.getAllByTestId("mock-icon");
      expect(icons).toHaveLength(3);
    });
  });

  describe("Edge Cases", () => {
    it("should handle single tab", () => {
      const singleTab: TabOption[] = [{ id: "tab1", label: "Only Tab" }];

      render(
        <TabBar
          tabs={singleTab}
          activeTab="tab1"
          onTabChange={mockOnTabChange}
        />,
      );

      expect(screen.getByText("Only Tab")).toBeInTheDocument();
    });

    it("should handle empty tabs array gracefully", () => {
      render(<TabBar tabs={[]} activeTab="" onTabChange={mockOnTabChange} />);

      const tablist = screen.getByRole("tablist");
      expect(tablist).toBeInTheDocument();
      expect(screen.queryAllByRole("tab")).toHaveLength(0);
    });

    it("should handle very long tab labels", () => {
      const longLabelTabs: TabOption[] = [
        {
          id: "tab1",
          label: "This is a very long tab label that might wrap or truncate",
        },
      ];

      render(
        <TabBar
          tabs={longLabelTabs}
          activeTab="tab1"
          onTabChange={mockOnTabChange}
        />,
      );

      expect(
        screen.getByText(
          "This is a very long tab label that might wrap or truncate",
        ),
      ).toBeInTheDocument();
    });

    it("should handle activeTab that doesn't match any tab id", () => {
      render(<TabBar {...defaultProps} activeTab="nonexistent" />);

      const tabs = screen.getAllByRole("tab");
      tabs.forEach((tab) => {
        expect(tab).toHaveAttribute("aria-selected", "false");
      });
    });
  });
});
