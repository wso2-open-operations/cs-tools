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

import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import DashboardPage from "@/pages/DashboardPage";
import { DASHBOARD_STATS } from "@/constants/dashboardConstants";
import { useGetDashboardMockStats } from "@/api/useGetDashboardMockStats";
import { useGetProjectCasesStats } from "@/api/useGetProjectCasesStats";

const mockNavigate = vi.fn();

beforeEach(() => {
  mockNavigate.mockClear();
});

// Mock react-router
vi.mock("react-router", () => ({
  useParams: () => ({ projectId: "project-1" }),
  useNavigate: () => mockNavigate,
}));

// Mock useLogger
vi.mock("@/hooks/useLogger", () => ({
  useLogger: () => ({
    debug: vi.fn(),
    info: vi.fn(),
    warn: vi.fn(),
    error: vi.fn(),
  }),
}));

const mockMockStats = {
  data: {
    totalCases: { trend: { value: "12%", direction: "up", color: "success" } },
    openCases: { trend: { value: "5%", direction: "down", color: "error" } },
    resolvedCases: {
      trend: { value: "8%", direction: "up", color: "success" },
    },
    avgResponseTime: {
      trend: { value: "0.5h", direction: "down", color: "error" },
    },
  },
  isLoading: false,
  isError: false,
};

const mockCasesStats = {
  data: {
    totalCases: 150,
    openCases: 25,
    averageResponseTime: 4.5,
    resolvedCases: { total: 125 },
  },
  isLoading: false,
  isError: false,
};

vi.mock("@/api/useGetDashboardMockStats", () => ({
  useGetDashboardMockStats: vi.fn(() => mockMockStats),
}));

vi.mock("@/api/useGetProjectCasesStats", () => ({
  useGetProjectCasesStats: vi.fn(() => mockCasesStats),
}));

// Mock StatCard
vi.mock("@/components/dashboard/stats/StatCard", () => ({
  StatCard: ({ label, value }: any) => (
    <div data-testid="stat-card">
      <span>{label}</span>
      <span>{value}</span>
    </div>
  ),
}));

// Mock @wso2/oxygen-ui
vi.mock("@wso2/oxygen-ui", () => ({
  Box: ({ children }: any) => <div data-testid="box">{children}</div>,
  Grid: ({ children, container }: any) => (
    <div data-testid={container ? "grid-container" : "grid-item"}>
      {children}
    </div>
  ),
  Typography: ({ children, variant }: any) => (
    <div data-testid={`typography-${variant}`}>{children}</div>
  ),
  Button: ({ children, onClick, startIcon, endIcon }: any) => (
    <button data-testid="button" onClick={onClick}>
      {startIcon}
      {children}
      {endIcon}
    </button>
  ),
  LinearProgress: ({ color }: any) => (
    <div data-testid="linear-progress" data-color={color}></div>
  ),
}));

// Mock icons
vi.mock("@wso2/oxygen-ui-icons-react", () => ({
  MessageSquare: () => <svg data-testid="icon-message-square" />,
  ArrowRight: () => <svg data-testid="icon-arrow-right" />,
  Clock: () => <svg data-testid="icon-clock" />,
  AlertCircle: () => <svg data-testid="icon-alert-circle" />,
  CheckCircle: () => <svg data-testid="icon-check-circle" />,
  Activity: () => <svg data-testid="icon-activity" />,
  TrendingUp: () => <svg data-testid="icon-trending-up" />,
  TrendingDown: () => <svg data-testid="icon-trending-down" />,
  Info: () => <svg data-testid="icon-info" />,
}));

describe("DashboardPage", () => {
  it("should render correctly", () => {
    render(<DashboardPage />);

    expect(screen.getAllByTestId("stat-card")).toHaveLength(
      DASHBOARD_STATS.length,
    );
    expect(screen.getByText("Get Support")).toBeInTheDocument();
  });

  it("should navigate to support chat on button click", () => {
    render(<DashboardPage />);
    const button = screen.getByText("Get Support");
    fireEvent.click(button);

    expect(mockNavigate).toHaveBeenCalledWith("/project-1/support/chat");
  });

  it("should render error message when statistics fail to load", () => {
    vi.mocked(useGetDashboardMockStats).mockReturnValueOnce({
      ...mockMockStats,
      isError: true,
      data: undefined,
    } as any);

    render(<DashboardPage />);

    expect(
      screen.getByText(
        "Error loading dashboard statistics. Please try again later.",
      ),
    ).toBeInTheDocument();
  });

  it("should render error message when cases statistics fail to load", () => {
    vi.mocked(useGetProjectCasesStats).mockReturnValueOnce({
      ...mockCasesStats,
      isError: true,
      data: undefined,
    } as any);

    render(<DashboardPage />);

    expect(
      screen.getByText(
        "Error loading dashboard statistics. Please try again later.",
      ),
    ).toBeInTheDocument();
  });
});
