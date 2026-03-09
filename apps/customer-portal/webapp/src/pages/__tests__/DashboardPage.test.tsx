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
import type { ReactElement } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import DashboardPage from "@pages/DashboardPage";
import { DASHBOARD_STATS } from "@constants/dashboardConstants";
import { useGetProjectCasesStats } from "@api/useGetProjectCasesStats";
import { ErrorBannerProvider } from "@context/error-banner/ErrorBannerContext";
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

const mockFilters = {
  data: {
    caseTypes: [
      { id: "8d4b87bd1b18f010cb6898aebd4bcb59", label: "Incident" },
      { id: "0d5b8fbd1b18f010cb6898aebd4bcba5", label: "Query" },
      { id: "5aeff1201b74c210264c997a234bcb54", label: "Service Request" },
      { id: "ab36479047ccf510a0a29cd3846d43ee", label: "Security Report Analysis" },
    ],
  },
  isLoading: false,
  isError: false,
};

const mockCasesStats = {
  data: {
    totalCases: 150,
    averageResponseTime: 4.5,
    resolvedCases: { total: 125, currentMonth: 6 },
    stateCount: [
      { id: "1", label: "Open", count: 100 },
      { id: "3", label: "Closed", count: 25 },
      { id: "10", label: "Work In Progress", count: 15 },
      { id: "18", label: "Awaiting Info", count: 5 },
      { id: "1003", label: "Waiting On WSO2", count: 5 },
    ],
    severityCount: [],
    outstandingSeverityCount: [],
    caseTypeCount: [],
    casesTrend: [],
  },
  isLoading: false,
  isError: false,
};

vi.mock("@api/useGetProjectFilters", () => ({
  default: vi.fn(() => mockFilters),
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
  Alert: ({ children, onClose }: any) => (
    <div data-testid="error-banner" role="alert">
      {children}
      {onClose && <button onClick={onClose}>Close</button>}
    </div>
  ),
  Box: ({ children }: any) => <div data-testid="box">{children}</div>,
  Grid: ({ children, container }: any) => (
    <div data-testid={container ? "grid-container" : "grid-item"}>
      {children}
    </div>
  ),
  IconButton: ({ children, onClick, "aria-label": ariaLabel }: any) => (
    <button data-testid="icon-button" onClick={onClick} aria-label={ariaLabel}>
      {children}
    </button>
  ),
  LinearProgress: ({ value }: any) => (
    <div data-testid="linear-progress" data-value={value}></div>
  ),
  Paper: ({ children }: any) => (
    <div data-testid="error-banner" role="alert">
      {children}
    </div>
  ),
  Stack: ({ children }: any) => <div data-testid="stack">{children}</div>,
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
  colors: {
    common: { white: "#FFFFFF" },
    blue: { 500: "#3B82F6", 600: "#2563EB", 700: "#1D4ED8" },
    green: { 500: "#22C55E" },
    grey: { 300: "#D1D5DB", 500: "#9CA3AF" },
    orange: { 500: "#F97316" },
    purple: { 400: "#A78BFA", 500: "#A855F7" },
    red: { 500: "#EF4444", 600: "#DC2626" },
    yellow: { 600: "#EAB308" },
  },
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
  X: () => <span data-testid="icon-x" />,
}));

const renderWithProviders = (ui: ReactElement) =>
  render(
    <LoaderProvider>
      <ErrorBannerProvider>{ui}</ErrorBannerProvider>
    </LoaderProvider>,
  );

describe("DashboardPage", () => {
  it("should render correctly", () => {
    renderWithProviders(<DashboardPage />);

    expect(screen.getAllByTestId("stat-card")).toHaveLength(
      DASHBOARD_STATS.length,
    );
    expect(screen.getByTestId("chart-layout")).toBeInTheDocument();
  });

  it("should render error banner when cases statistics fail to load", () => {
    vi.mocked(useGetProjectCasesStats).mockReturnValueOnce({
      ...mockCasesStats,
      isError: true,
      data: undefined,
    } as any);

    renderWithProviders(<DashboardPage />);

    expect(screen.getByTestId("error-banner")).toBeInTheDocument();
    expect(
      screen.getByText("Could not load dashboard statistics."),
    ).toBeInTheDocument();
  });
});
