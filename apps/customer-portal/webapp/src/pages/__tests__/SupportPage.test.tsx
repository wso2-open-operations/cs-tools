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

import type { ReactElement } from "react";
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

// Mock @asgardeo/react to avoid ESM buffer import errors in vitest
vi.mock("@asgardeo/react", () => ({
  useAsgardeo: () => ({
    isLoading: false,
    isSignedIn: true,
    state: { isAuthenticated: true },
  }),
}));

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

// Mock @wso2/oxygen-ui components
vi.mock("@wso2/oxygen-ui", () => ({
  Box: ({ children }: any) => <div data-testid="box">{children}</div>,
  Stack: ({ children, spacing }: any) => (
    <div data-testid="stack" data-spacing={spacing}>
      {children}
    </div>
  ),
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
    orange: { 600: "#FB8C00" },
  },
  alpha: (color: string, opacity: number) => `${color}-${opacity}`,
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
  alpha: (color: string, opacity: number) => `alpha(${color}, ${opacity})`,
  useTheme: () => ({
    palette: {
      primary: { main: "#0070F3" },
      secondary: { main: "#71717A" },
      error: { main: "#EF4444" },
      warning: { main: "#F59E0B" },
      info: { main: "#3B82F6" },
      success: { main: "#10B981" },
      text: { primary: "#000000", secondary: "#6B7280" },
    },
  }),
}));

// Mock icons
vi.mock("@wso2/oxygen-ui-icons-react", () => ({
  Bot: () => <svg data-testid="icon-bot" />,
  ArrowRight: () => <svg data-testid="icon-arrow-right" />,
  CircleAlert: () => <svg data-testid="icon-alert" />,
  CircleQuestionMark: () => <svg data-testid="icon-question-mark" />,
  CircleCheck: () => <svg data-testid="icon-check" />,
  Clock: () => <svg data-testid="icon-clock" />,
  FileText: () => <svg data-testid="icon-file-text" />,
  MessageSquare: () => <svg data-testid="icon-message" />,
  MessageSquareDiff: () => <svg data-testid="icon-message-diff" />,
  MessageSquareMore: () => <svg data-testid="icon-message-more" />,
  TrendingUp: () => <svg data-testid="icon-trending-up" />,
  Info: () => <svg data-testid="icon-info" />,
  Server: () => <svg data-testid="icon-server" />,
  MessageCircle: () => <svg data-testid="icon-message-circle" />,
  User: () => <svg data-testid="icon-user" />,
  Shield: () => <svg data-testid="icon-shield" />,
  Rocket: () => <svg data-testid="icon-rocket" />,
}));

// Mock useGetProjectSupportStats
const mockUseGetProjectSupportStats = vi.fn();
vi.mock("@api/useGetProjectSupportStats", () => ({
  useGetProjectSupportStats: (id: string) => mockUseGetProjectSupportStats(id),
}));

// Mock useGetProjectCases (avoids pulling in useAsgardeo)
vi.mock("@api/useGetProjectCases", () => ({
  __esModule: true,
  default: () => ({ data: { cases: [] }, isLoading: false }),
}));

// Mock useGetChatHistory
vi.mock("@api/useGetChatHistory", () => ({
  useGetChatHistory: () => ({ data: { chatHistory: [] } }),
}));

// Mock support overview card components so SupportPage renders without full Oxygen UI tree
vi.mock(
  "@components/support/support-overview-cards/SupportOverviewCard",
  () => ({
    __esModule: true,
    default: ({
      title,
      children,
    }: {
      title: string;
      children: ReactElement;
    }) => (
      <div data-testid="support-overview-card">
        <span>{title}</span>
        {children}
      </div>
    ),
  }),
);
vi.mock(
  "@components/support/support-overview-cards/OutstandingCasesList",
  () => ({
    __esModule: true,
    default: () => <div data-testid="outstanding-cases-list">Cases list</div>,
  }),
);
vi.mock("@components/support/support-overview-cards/ChatHistoryList", () => ({
  __esModule: true,
  default: () => <div data-testid="chat-history-list">Chat list</div>,
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
    expect(screen.getAllByTestId("icon-file-text")).toHaveLength(2);
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

  it("should render Outstanding Cases and Chat History overview cards when data is loaded", () => {
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

    expect(screen.getByText("Outstanding Cases")).toBeInTheDocument();
    expect(screen.getByText("Chat History")).toBeInTheDocument();
  });
});
