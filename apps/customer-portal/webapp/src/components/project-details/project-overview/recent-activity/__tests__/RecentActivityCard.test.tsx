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
import RecentActivityCard from "../RecentActivityCard";
import { getRecentActivityItems } from "@/constants/projectDetailsConstants";

// Mock @wso2/oxygen-ui components
vi.mock("@wso2/oxygen-ui", () => ({
  Card: ({ children }: any) => <div data-testid="card">{children}</div>,
  CardContent: ({ children }: any) => <div>{children}</div>,
  Typography: ({ children }: any) => <span>{children}</span>,
  Box: ({ children }: any) => <div>{children}</div>,
  Divider: () => <hr />,
  Chip: ({ label, color }: any) => (
    <span data-testid="chip" data-color={color}>
      {label}
    </span>
  ),
  Skeleton: () => <div data-testid="skeleton">Loading...</div>,
}));

// Mock icons
vi.mock("@wso2/oxygen-ui-icons-react", () => ({
  Zap: () => <svg data-testid="icon-zap" />,
}));

// Mock constants/utils
vi.mock("@/constants/projectDetailsConstants", () => ({
  getRecentActivityItems: vi.fn(),
}));

describe("RecentActivityCard", () => {
  const mockActivityData = {
    // This data structure depends on ProjectStatsResponse["recentActivity"]
    // but we are mocking getRecentActivityItems so we can pass anything here
    // or pass null, as long as getRecentActivityItems handles it or we mock the return.
    lastDeploymentOn: "2026-01-01",
    totalTimeLogged: 10,
    billableHours: 5,
    systemHealth: "Good",
  };

  it("should render card title and icon", () => {
    (getRecentActivityItems as any).mockReturnValue([]);
    render(<RecentActivityCard />);

    expect(screen.getByText("Recent Activity")).toBeInTheDocument();
    expect(screen.getByTestId("icon-zap")).toBeInTheDocument();
  });

  it("should render activity items correctly", () => {
    const mockItems = [
      { label: "Last Deployment", value: "2 days ago", type: "text" },
      {
        label: "System Health",
        value: "Operational",
        type: "chip",
        chipColor: "success",
      },
    ];
    (getRecentActivityItems as any).mockReturnValue(mockItems);

    render(<RecentActivityCard activity={mockActivityData} />);

    // Check labels
    expect(screen.getByText("Last Deployment")).toBeInTheDocument();
    expect(screen.getByText("System Health")).toBeInTheDocument();

    // Check text value
    expect(screen.getByText("2 days ago")).toBeInTheDocument();

    // Check chip
    const chip = screen.getByTestId("chip");
    expect(chip).toHaveTextContent("Operational");
    expect(chip).toHaveAttribute("data-color", "success");
  });

  it("should render skeletons when loading", () => {
    // Even when loading, getRecentActivityItems is called to get structure
    const mockItems = [
      { label: "Item 1", value: "Val 1", type: "text" },
      { label: "Item 2", value: "Val 2", type: "text" },
      { label: "Item 3", value: "Val 3", type: "text" },
      { label: "Item 4", value: "Val 4", type: "text" },
    ];
    (getRecentActivityItems as any).mockReturnValue(mockItems);

    render(<RecentActivityCard isLoading={true} />);

    // Expect skeletons equal to number of items
    const skeletons = screen.getAllByTestId("skeleton");
    expect(skeletons).toHaveLength(4);

    // Values should not be visible
    expect(screen.queryByText("Val 1")).toBeNull();
  });

  it("should handle empty or missing data gracefully", () => {
    (getRecentActivityItems as any).mockReturnValue([
      { label: "Last Deployment", value: "N/A", type: "text" },
    ]);

    render(<RecentActivityCard activity={undefined} />);

    expect(screen.getByText("Last Deployment")).toBeInTheDocument();
    expect(screen.getByText("N/A")).toBeInTheDocument();
  });
});
