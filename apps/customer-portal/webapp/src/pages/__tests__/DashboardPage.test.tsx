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
import DashboardPage from "@pages/DashboardPage";
import { DASHBOARD_STATS } from "@constants/dashboardConstants";
import { useGetDashboardMockStats } from "@api/useGetDashboardMockStats";
import { useGetProjectCasesStats } from "@api/useGetProjectCasesStats";
import { LoaderProvider } from "@context/linear-loader/LoaderContext";

const mockNavigate = vi.fn();

beforeEach(() => {
  mockNavigate.mockClear();
});

// Mock react-router
vi.mock("react-router", () => ({
  useParams: () => ({ projectId: "project-1" }),
  useNavigate: () => mockNavigate,
}));

// Mock @asgardeo/react
vi.mock("@asgardeo/react", () => ({
  useAsgardeo: () => ({
    isLoading: false,
    state: { isAuthenticated: true },
  }),
}));

// Mock useLogger
vi.mock("@hooks/useLogger", () => ({
  useLogger: () => ({
    debug: vi.fn(),
    info: vi.fn(),
    warn: vi.fn(),
    error: vi.fn(),
  }),
}));

// Mock useLoader
vi.mock("@context/linear-loader/LoaderContext", async (importOriginal) => {
  const actual = (await importOriginal()) as any;
  return {
    ...actual,
    useLoader: () => ({
      showLoader: vi.fn(),
      hideLoader: vi.fn(),
    }),
  };
});

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
    casesTrend: {
      "Type A": 10,
      "Type B": 20,
      "Type C": 30,
      "Type D": 40,
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

vi.mock("@api/useGetDashboardMockStats", () => ({
  useGetDashboardMockStats: vi.fn(() => mockMockStats),
}));

vi.mock("@api/useGetProjectCasesStats", () => ({
  useGetProjectCasesStats: vi.fn(() => mockCasesStats),
}));

// Mock ChartLayout
vi.mock("@components/dashboard/charts/ChartLayout", () => ({
  default: () => <div data-testid="chart-layout" />,
}));

// Mock StatCard
vi.mock("@components/dashboard/stats/StatCard", () => ({
  StatCard: ({ label, value, isError }: any) => (
    <div data-testid="stat-card">
      <span>{label}</span>
      <span>{value}</span>
      {isError && <span>Error</span>}
    </div>
  ),
}));

// Mock CasesTable
vi.mock("@components/dashboard/cases-table/CasesTable", () => ({
  default: () => <div data-testid="cases-table" />,
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
  colors: {
    common: { white: "#FFFFFF" },
    blue: { 500: "#3B82F6", 700: "#1D4ED8" },
    green: { 500: "#22C55E" },
    orange: { 500: "#F97316" },
    red: { 500: "#EF4444" },
    yellow: { 600: "#EAB308" },
    purple: { 400: "#A78BFA" },
  },
  NotificationBanner: ({ visible, title, message }: any) =>
    visible ? (
      <div data-testid="notification-banner">
        {title && <span>{title}</span>}
        {message && <span>{message}</span>}
      </div>
    ) : null,
}));

// Mock icons
vi.mock("@wso2/oxygen-ui-icons-react", () => ({
  Activity: () => <svg data-testid="icon-activity" />,
  AlertCircle: () => <svg data-testid="icon-alert-circle" />,
  ArrowRight: () => <svg data-testid="icon-arrow-right" />,
  CheckCircle: () => <svg data-testid="icon-check-circle" />,
  CircleAlert: () => <svg data-testid="icon-circle-alert" />,
  Clock: () => <svg data-testid="icon-clock" />,
  Info: () => <svg data-testid="icon-info" />,
  MessageSquare: () => <svg data-testid="icon-message-square" />,
  TrendingDown: () => <svg data-testid="icon-trending-down" />,
  TrendingUp: () => <svg data-testid="icon-trending-up" />,
  Server: () => <svg data-testid="icon-server" />,
  Shield: () => <svg data-testid="icon-shield" />,
  User: () => <svg data-testid="icon-user" />,
  Rocket: () => <svg data-testid="icon-rocket" />,
}));

describe("DashboardPage", () => {
  it("should render correctly", () => {
    render(
      <LoaderProvider>
        <DashboardPage />
      </LoaderProvider>,
    );

    expect(screen.getAllByTestId("stat-card")).toHaveLength(
      DASHBOARD_STATS.length,
    );
    expect(screen.getByTestId("chart-layout")).toBeInTheDocument();
    expect(screen.getByText("Get Support")).toBeInTheDocument();
  });

  it("should navigate to support chat on button click", () => {
    render(
      <LoaderProvider>
        <DashboardPage />
      </LoaderProvider>,
    );
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

    render(
      <LoaderProvider>
        <DashboardPage />
      </LoaderProvider>,
    );

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

    render(
      <LoaderProvider>
        <DashboardPage />
      </LoaderProvider>,
    );

    expect(
      screen.getByText(
        "Error loading dashboard statistics. Please try again later.",
      ),
    ).toBeInTheDocument();
  });
});
