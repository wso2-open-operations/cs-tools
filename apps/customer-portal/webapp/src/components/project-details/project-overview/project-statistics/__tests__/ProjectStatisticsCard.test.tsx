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

import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import ProjectStatisticsCard from "@components/project-details/project-overview/project-statistics/ProjectStatisticsCard";
import type { ProjectStatsResponse } from "@models/responses";

// Mock @wso2/oxygen-ui components
vi.mock("@wso2/oxygen-ui", () => ({
  Card: ({ children }: any) => <div data-testid="card">{children}</div>,
  CardContent: ({ children }: any) => <div>{children}</div>,
  Typography: ({ children }: any) => <span>{children}</span>,
  Grid: ({ children, size, container }: any) => (
    <div
      data-testid={container ? "grid-container" : "grid-item"}
      data-size={container ? undefined : JSON.stringify(size)}
    >
      {children}
    </div>
  ),
  Box: ({ children }: any) => <div>{children}</div>,
  StatCard: ({ label, value }: any) => (
    <div data-testid="stat-card">
      <span data-testid="stat-label">{label}</span>
      <span data-testid="stat-value">{value}</span>
    </div>
  ),
  Divider: () => <hr />,
  Skeleton: () => <span data-testid="skeleton">Loading...</span>,
}));

// Mock icons
vi.mock("@wso2/oxygen-ui-icons-react", () => ({
  Activity: () => <svg data-testid="icon-activity" />,
}));

// Mock constants
vi.mock("@/constants/projectDetailsConstants", () => ({
  statItems: [
    {
      label: "Active Chats",
      key: "activeChats",
      icon: () => null,
      iconColor: "primary",
    },
    {
      label: "Open Cases",
      key: "openCases",
      icon: () => null,
      iconColor: "success",
    },
  ],
}));

describe("ProjectStatisticsCard", () => {
  const mockStats: ProjectStatsResponse["projectStats"] = {
    activeChats: 5,
    openCases: 10,
    deployments: 2,
    slaStatus: "Good",
  };

  it("should render card title and icon", () => {
    render(<ProjectStatisticsCard stats={mockStats} />);

    expect(screen.getByText("Project Statistics")).toBeInTheDocument();
    expect(screen.getByTestId("icon-activity")).toBeInTheDocument();
  });

  it("should render stat cards with correct data", () => {
    render(<ProjectStatisticsCard stats={mockStats} />);

    // Check for mocked labels from statItems
    expect(screen.getByText("Active Chats")).toBeInTheDocument();
    expect(screen.getByText("Open Cases")).toBeInTheDocument();

    // Check for values mapped from mockStats
    // We mocked StatCard to display value in a span with data-testid="stat-value"
    const values = screen.getAllByTestId("stat-value");
    expect(values[0]).toHaveTextContent("5");
    expect(values[1]).toHaveTextContent("10");
  });

  it("should render skeletons when loading", () => {
    render(<ProjectStatisticsCard stats={mockStats} isLoading={true} />);

    // Expect skeletons instead of values
    const skeletons = screen.getAllByTestId("skeleton");
    expect(skeletons.length).toBeGreaterThan(0);
    expect(screen.queryByText("5")).toBeNull();
  });

  it("should render -- for missing stats", () => {
    // Provide empty stats object or partially missing
    render(<ProjectStatisticsCard stats={{} as any} />);

    const values = screen.getAllByTestId("stat-value");
    expect(values[0]).toHaveTextContent("--");
  });

  it("should handle sidebar open/close for grid sizing", () => {
    const { rerender } = render(
      <ProjectStatisticsCard stats={mockStats} isSidebarOpen={false} />,
    );

    let gridItems = screen.getAllByTestId("grid-item");
    // Default (sidebar closed): { xs: 12, lg: 4 }
    expect(gridItems[0]).toHaveAttribute(
      "data-size",
      JSON.stringify({ xs: 12, lg: 4 }),
    );

    rerender(<ProjectStatisticsCard stats={mockStats} isSidebarOpen={true} />);
    gridItems = screen.getAllByTestId("grid-item");
    // Sidebar open: { xs: 12, xl: 4 }
    expect(gridItems[0]).toHaveAttribute(
      "data-size",
      JSON.stringify({ xs: 12, xl: 4 }),
    );
  });
});
