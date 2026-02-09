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
import { MemoryRouter } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import SupportPage from "@pages/SupportPage";

// Mock react-router
vi.mock("react-router", async () => {
  const actual = await vi.importActual("react-router");
  return {
    ...actual,
    useParams: () => ({ projectId: "project-1" }),
    useNavigate: () => vi.fn(),
  };
});

// Mock useLogger
const mockLogger = {
  debug: vi.fn(),
  error: vi.fn(),
  info: vi.fn(),
  warn: vi.fn(),
};
vi.mock("@hooks/useLogger", () => ({
  useLogger: () => mockLogger,
}));

// Mock @asgardeo/react
vi.mock("@asgardeo/react", () => ({
  useAsgardeo: () => ({
    isLoading: false,
    state: { isAuthenticated: true },
  }),
}));

// Mock @wso2/oxygen-ui components
vi.mock("@wso2/oxygen-ui", () => ({
  Box: ({ children }: any) => <div data-testid="box">{children}</div>,
  Grid: ({ children, container, spacing, size, sx }: any) => (
    <div
      data-testid={container ? "grid-container" : "grid-item"}
      data-spacing={spacing}
      data-size={JSON.stringify(size)}
      style={sx}
    >
      {children}
    </div>
  ),
  Typography: ({ children, variant }: any) => (
    <div data-testid={`typography-${variant}`}>{children}</div>
  ),
  colors: {
    red: { 500: "#F44336" },
    blue: { 500: "#2196F3", 700: "#1D4ED8" },
    green: { 500: "#4CAF50" },
    purple: { 500: "#9C27B0", 400: "#A78BFA" },
  },
  Divider: () => <hr />,
  Button: ({ children }: any) => <button>{children}</button>,
  Card: ({ children, sx }: any) => <div style={sx}>{children}</div>,
  CardContent: ({ children, sx }: any) => <div style={sx}>{children}</div>,
  StatCard: ({ label, value, icon }: any) => {
    const ValueSkeleton =
      value && typeof value === "object" && "Skeleton" in value
        ? (value as any).Skeleton
        : null;

    return (
      <div data-testid="oxygen-stat-card">
        <div data-testid="stat-card-icon">{icon}</div>
        <span>{label}</span>
        <div data-testid="stat-card-value">
          {ValueSkeleton ? <ValueSkeleton variant="text" /> : value}
        </div>
      </div>
    );
  },
  Skeleton: ({ children, variant, width, height }: any) => (
    <div
      data-testid="skeleton"
      data-variant={variant}
      style={{ width, height }}
    >
      {children}
    </div>
  ),
  Paper: ({ children }: any) => <div data-testid="paper">{children}</div>,
}));

// Mock icons
vi.mock("@wso2/oxygen-ui-icons-react", () => ({
  Bot: () => <svg data-testid="icon-bot" />,
  CircleAlert: () => <svg data-testid="icon-alert" />,
  CircleCheck: () => <svg data-testid="icon-check" />,
  Clock: () => <svg data-testid="icon-clock" />,
  FileText: () => <svg data-testid="icon-file-text" />,
  MessageSquare: () => <svg data-testid="icon-message" />,
  MessageSquareDiff: () => <svg data-testid="icon-message-diff" />,
  MessageSquareMore: () => <svg data-testid="icon-message-more" />,
  TrendingUp: () => <svg data-testid="icon-trending-up" />,
  Info: () => <svg data-testid="icon-info" />,
  Server: () => <svg data-testid="icon-server" />,
  User: () => <svg data-testid="icon-user" />,
  Shield: () => <svg data-testid="icon-shield" />,
  Rocket: () => <svg data-testid="icon-rocket" />,
}));

// Mock useGetProjectSupportStats
const mockUseGetProjectSupportStats = vi.fn();
vi.mock("@api/useGetProjectSupportStats", () => ({
  useGetProjectSupportStats: (id: string) => mockUseGetProjectSupportStats(id),
}));

beforeEach(() => {
  vi.clearAllMocks();
});

describe("SupportPage", () => {
  it("should render loading state correctly", () => {
    mockUseGetProjectSupportStats.mockReturnValue({
      isLoading: true,
      data: null,
    });

    render(
      <MemoryRouter>
        <SupportPage />
      </MemoryRouter>,
    );

    const skeletons = screen.getAllByTestId("skeleton");
    expect(skeletons).toHaveLength(4);
    expect(screen.getByTestId("icon-file-text")).toBeInTheDocument();
    expect(screen.getAllByTestId("icon-bot")).toHaveLength(2);
    expect(mockLogger.debug).not.toHaveBeenCalled();
  });

  it("should render error state correctly and log the error", () => {
    mockUseGetProjectSupportStats.mockReturnValue({
      isLoading: false,
      isError: true,
      data: null,
    });

    render(
      <MemoryRouter>
        <SupportPage />
      </MemoryRouter>,
    );

    expect(
      screen.getByText(
        "Error loading support statistics. Please try again later.",
      ),
    ).toBeInTheDocument();
    expect(mockLogger.error).toHaveBeenCalledWith(
      "Failed to load support stats for project: project-1",
    );
  });

  it("should render statistics correctly when data is loaded", () => {
    mockUseGetProjectSupportStats.mockReturnValue({
      isLoading: false,
      data: {
        totalCases: 10,
        activeChats: 5,
        sessionChats: 15,
        resolvedChats: 20,
      },
    });

    render(
      <MemoryRouter>
        <SupportPage />
      </MemoryRouter>,
    );

    expect(screen.getByText("10")).toBeInTheDocument();
    expect(screen.getByText("5")).toBeInTheDocument();
    expect(screen.getByText("15")).toBeInTheDocument();
    expect(screen.getByText("20")).toBeInTheDocument();
    expect(screen.getByText("Ongoing Cases")).toBeInTheDocument();
    expect(screen.getByText("Active Chats")).toBeInTheDocument();
    expect(screen.getByText("Chat Sessions")).toBeInTheDocument();
    expect(screen.getByText("Resolved via Chat")).toBeInTheDocument();
    expect(mockLogger.debug).toHaveBeenCalledWith(
      "Support stats loaded for project: project-1",
    );
  });

  it("should render the 'Start New Chat' call-to-action card", () => {
    mockUseGetProjectSupportStats.mockReturnValue({
      isLoading: false,
      data: {
        totalCases: 10,
        activeChats: 5,
        sessionChats: 15,
        resolvedChats: 20,
      },
    });

    render(
      <MemoryRouter>
        <SupportPage />
      </MemoryRouter>,
    );

    expect(
      screen.getByText("Need help with something new?"),
    ).toBeInTheDocument();
    expect(screen.getByText("Start New Chat")).toBeInTheDocument();
    expect(screen.getAllByTestId("icon-bot")).toHaveLength(2);
  });
});
