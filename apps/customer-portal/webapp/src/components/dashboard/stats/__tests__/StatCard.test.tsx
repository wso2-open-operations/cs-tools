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
import { beforeEach, describe, expect, it, vi } from "vitest";
import { StatCard } from "../StatCard";

// Mock useParams
vi.mock("react-router", () => ({
  useParams: () => ({ projectId: "project-1" }),
}));

// Mock useGetDashboardMockStats
const mockUseGetDashboardMockStats = vi.fn();
vi.mock("@/api/useGetDashboardMockStats", () => ({
  useGetDashboardMockStats: (id: string) => mockUseGetDashboardMockStats(id),
}));

// Mock @wso2/oxygen-ui
vi.mock("@wso2/oxygen-ui", () => ({
  Grid: ({ children, container, sx }: any) => (
    <div data-testid={container ? "grid-container" : "grid-item"} style={sx}>
      {children}
    </div>
  ),
  Box: ({ children, sx }: any) => <div style={sx}>{children}</div>,
  Typography: ({ children, variant }: any) => (
    <div data-testid={`typography-${variant}`}>{children}</div>
  ),
  Card: ({ children, sx }: any) => (
    <div data-testid="card" style={sx}>
      {children}
    </div>
  ),
  Tooltip: ({ children, title }: any) => <div title={title}>{children}</div>,
  Skeleton: ({ variant, width }: any) => (
    <div data-testid="skeleton" data-variant={variant} style={{ width }}></div>
  ),
  useTheme: () => ({
    palette: {
      primary: { light: "#000" },
      secondary: { light: "#000" },
      error: { light: "#000" },
      warning: { light: "#000" },
      info: { light: "#000" },
      success: { light: "#000" },
    },
  }),
  alpha: (color: string, opacity: number) => `rgba(${color}, ${opacity})`,
}));

// Mock icons
vi.mock("@wso2/oxygen-ui-icons-react", () => ({
  TrendingUp: () => <svg data-testid="icon-trending-up" />,
  TrendingDown: () => <svg data-testid="icon-trending-down" />,
  Info: () => <svg data-testid="icon-info" />,
  FileText: () => <svg data-testid="icon-file-text" />,
  Clock: () => <svg data-testid="icon-clock" />,
  CheckCircle: () => <svg data-testid="icon-check-circle" />,
  AlertCircle: () => <svg data-testid="icon-alert-circle" />,
  Activity: () => <svg data-testid="icon-activity" />,
}));

describe("StatCard", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  const defaultProps = {
    label: "Total Cases",
    value: 156,
    icon: <svg data-testid="icon-clock" />,
    iconColor: "primary" as const,
    tooltipText: "Total number of cases",
  };

  it("should render skeleton when loading", () => {
    render(<StatCard {...defaultProps} isLoading={true} />);

    const skeletons = screen.getAllByTestId("skeleton");
    expect(skeletons.length).toBeGreaterThan(0);
  });

  it("should render stat when data is loaded", () => {
    render(<StatCard {...defaultProps} />);

    expect(screen.getByText("156")).toBeInTheDocument();
    expect(screen.getByText("Total Cases")).toBeInTheDocument();
    expect(screen.getByTestId("icon-clock")).toBeInTheDocument();
    // Verify typography variant is h4
    expect(screen.getByTestId("typography-h4")).toBeInTheDocument();
  });

  it("should render trend when provided", () => {
    const trend = {
      value: "12%",
      direction: "up" as const,
      color: "success" as const,
    };
    render(<StatCard {...defaultProps} trend={trend} />);

    expect(screen.getByText("12%")).toBeInTheDocument();
    expect(screen.getByTestId("icon-trending-up")).toBeInTheDocument();
    expect(screen.getByText("vs last month")).toBeInTheDocument();
  });
});
