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

import { render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi, beforeEach } from "vitest";
import ProjectHub from "@/pages/ProjectHub";
import { mockProjects } from "@/models/mockData";

// Mock @wso2/oxygen-ui
vi.mock("@wso2/oxygen-ui", () => ({
  Box: ({ children }: any) => <div data-testid="box">{children}</div>,
  Typography: ({ children, variant }: any) => (
    <div data-testid="typography" data-variant={variant}>
      {children}
    </div>
  ),
  colors: {
    blue: { 700: "#1d4ed8" },
    purple: { 400: "#a78bfa" },
  },
}));

// Mock icons
vi.mock("@wso2/oxygen-ui-icons-react", () => ({
  FolderOpen: () => <svg data-testid="folder-icon" />,
  Info: () => <div data-testid="info-icon" />,
  Server: () => <div data-testid="server-icon" />,
  Clock: () => <div data-testid="clock-icon" />,
  User: () => <div data-testid="user-icon" />,
  Shield: () => <div data-testid="shield-icon" />,
  Rocket: () => <div data-testid="rocket-icon" />,
  CircleAlert: () => <div data-testid="alert-icon" />,
}));

// Mock useAsgardeo
const mockUseAsgardeo = vi.fn();
vi.mock("@asgardeo/react", () => ({
  useAsgardeo: () => mockUseAsgardeo(),
}));

// Mock useMockConfig
const mockUseMockConfig = vi.fn();
vi.mock("@/providers/MockConfigProvider", () => ({
  useMockConfig: () => mockUseMockConfig(),
}));

// Mock sub-components
vi.mock("@/components/projectHub/projectCard/ProjectCard", () => ({
  default: ({ title, isStatsError }: any) => (
    <div
      data-testid="project-card"
      data-stats-error={isStatsError ? "true" : "false"}
    >
      {title}
    </div>
  ),
}));

vi.mock("@/components/projectHub/projectCard/ProjectCardSkeleton", () => ({
  default: () => <div data-testid="project-skeleton" />,
}));

// Mock hooks
const mockUseGetProjects = vi.fn();
vi.mock("@/api/useGetProjects", () => ({
  default: (...args: any[]) => mockUseGetProjects(...args),
}));

const mockLogger = {
  debug: vi.fn(),
  info: vi.fn(),
  warn: vi.fn(),
  error: vi.fn(),
};
vi.mock("@/hooks/useLogger", () => ({
  useLogger: () => mockLogger,
}));

describe("ProjectHub", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseAsgardeo.mockReturnValue({ isLoading: false });
    mockUseMockConfig.mockReturnValue({ isMockEnabled: true });
    mockUseGetProjects.mockReturnValue({
      isLoading: false,
      data: {
        projects: mockProjects,
      },
      isError: false,
    });
  });

  it("should render loading skeletons when isLoading is true", () => {
    mockUseGetProjects.mockReturnValue({
      isLoading: true,
      data: null,
      isError: false,
    });

    render(<ProjectHub />);

    expect(screen.getAllByTestId("project-skeleton")).toHaveLength(3);
  });

  it("should render loading skeletons when isAuthLoading is true", () => {
    mockUseAsgardeo.mockReturnValue({ isLoading: true });
    mockUseGetProjects.mockReturnValue({
      isLoading: false,
      data: null, // Data might not be available yet
      isError: false,
    });

    render(<ProjectHub />);

    expect(screen.getAllByTestId("project-skeleton")).toHaveLength(3);
  });

  it("should render project cards when data is loaded", () => {
    render(<ProjectHub />);

    expect(screen.getAllByTestId("project-card")).toHaveLength(
      mockProjects.length,
    );
    expect(screen.getByText(mockProjects[0].name)).toBeInTheDocument();
  });

  it("should pass isStatsError=true to ProjectCard when mocks are disabled", () => {
    mockUseMockConfig.mockReturnValue({ isMockEnabled: false });

    render(<ProjectHub />);

    const cards = screen.getAllByTestId("project-card");
    expect(cards[0]).toHaveAttribute("data-stats-error", "true");
  });

  it("should pass isStatsError=false to ProjectCard when mocks are enabled", () => {
    mockUseMockConfig.mockReturnValue({ isMockEnabled: true });

    render(<ProjectHub />);

    const cards = screen.getAllByTestId("project-card");
    expect(cards[0]).toHaveAttribute("data-stats-error", "false");
  });

  it("should render error message when isError is true", async () => {
    mockUseGetProjects.mockReturnValue({
      isLoading: false,
      data: null,
      isError: true,
    });

    render(<ProjectHub />);

    expect(screen.getByText(/Error loading projects/i)).toBeInTheDocument();
    await waitFor(() => {
      expect(mockLogger.error).toHaveBeenCalledWith(
        expect.stringContaining("Failed to load projects"),
      );
    });
  });

  it("should render empty state when no projects are returned", () => {
    mockUseGetProjects.mockReturnValue({
      isLoading: false,
      data: {
        projects: [],
      },
      isError: false,
    });

    render(<ProjectHub />);

    expect(screen.getByText(/No projects available/i)).toBeInTheDocument();
  });

  it("should log debug message when projects are loaded", async () => {
    render(<ProjectHub />);
    await waitFor(() => {
      expect(mockLogger.debug).toHaveBeenCalledWith(
        expect.stringContaining("projects loaded"),
      );
    });
  });
});
