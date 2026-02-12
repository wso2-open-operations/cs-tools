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
import ProjectHub from "@pages/ProjectHub";
import { MemoryRouter } from "react-router";

const testProjects = [
  {
    id: "project-1",
    key: "PROJ1",
    name: "Project 1",
    description: "Description 1",
    createdOn: "2026-01-01 10:00:00",
  },
  {
    id: "project-2",
    key: "PROJ2",
    name: "Project 2",
    description: "Description 2",
    createdOn: "2026-01-02 11:00:00",
  },
];

// Mock @wso2/oxygen-ui
vi.mock("@wso2/oxygen-ui", () => ({
  Box: ({ children }: any) => <div data-testid="box">{children}</div>,
  Typography: ({ children, variant }: any) => (
    <div data-testid="typography" data-variant={variant}>
      {children}
    </div>
  ),
  IconButton: ({ children }: any) => (
    <button data-testid="icon-button">{children}</button>
  ),
  Tooltip: ({ children, title }: any) => (
    <div data-testid="tooltip" title={title}>
      {children}
    </div>
  ),
  colors: {
    blue: { 700: "#1D4ED8" },
    purple: { 400: "#A78BFA" },
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
  TriangleAlert: () => <div data-testid="triangle-alert-icon" />,
}));

vi.mock("@components/common/empty-state/EmptyIcon", () => ({
  default: () => <div data-testid="empty-icon" />,
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

// Mock useLoader
const mockUseLoader = vi.fn();
vi.mock("@context/linear-loader/LoaderContext", () => ({
  useLoader: () => mockUseLoader(),
}));

// Mock sub-components
vi.mock("@components/project-hub/project-card/ProjectCard", () => ({
  default: ({ title }: any) => <div data-testid="project-card">{title}</div>,
}));

vi.mock("@components/project-hub/project-card/ProjectCardSkeleton", () => ({
  default: () => <div data-testid="project-skeleton" />,
}));

// Mock hooks
const mockUseGetProjects = vi.fn();
vi.mock("@api/useGetProjects", () => ({
  default: (...args: any[]) => mockUseGetProjects(...args),
}));

const mockLogger = {
  debug: vi.fn(),
  info: vi.fn(),
  warn: vi.fn(),
  error: vi.fn(),
};
vi.mock("@hooks/useLogger", () => ({
  useLogger: () => mockLogger,
}));

describe("ProjectHub", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseAsgardeo.mockReturnValue({ isLoading: false });
    mockUseMockConfig.mockReturnValue({ isMockEnabled: true });
    mockUseLoader.mockReturnValue({
      showLoader: vi.fn(),
      hideLoader: vi.fn(),
    });
    mockUseGetProjects.mockReturnValue({
      isLoading: false,
      data: {
        projects: testProjects,
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

    render(
      <MemoryRouter>
        <ProjectHub />
      </MemoryRouter>,
    );

    expect(screen.getAllByTestId("project-skeleton")).toHaveLength(3);
    expect(screen.queryByText("No Projects Yet")).not.toBeInTheDocument();
    expect(screen.getByText(/Select Your Project/i)).toBeInTheDocument();
  });

  it("should render loading skeletons when isAuthLoading is true", () => {
    mockUseAsgardeo.mockReturnValue({ isLoading: true });
    mockUseGetProjects.mockReturnValue({
      isLoading: false,
      data: null,
      isError: false,
    });

    render(
      <MemoryRouter>
        <ProjectHub />
      </MemoryRouter>,
    );

    expect(screen.getAllByTestId("project-skeleton")).toHaveLength(3);
    expect(screen.queryByText("No Projects Yet")).not.toBeInTheDocument();
    expect(screen.getByText(/Select Your Project/i)).toBeInTheDocument();
  });

  it("should render project cards when data is loaded", () => {
    render(
      <MemoryRouter>
        <ProjectHub />
      </MemoryRouter>,
    );

    expect(screen.getAllByTestId("project-card")).toHaveLength(
      testProjects.length,
    );
    expect(screen.getByText(testProjects[0].name)).toBeInTheDocument();
    expect(screen.getByText(/Select Your Project/i)).toBeInTheDocument();
    expect(
      screen.getByText(/Choose a project to access your support cases/i),
    ).toBeInTheDocument();
  });

  it("should render error message when isError is true", async () => {
    mockUseGetProjects.mockReturnValue({
      isLoading: false,
      data: null,
      isError: true,
    });

    render(
      <MemoryRouter>
        <ProjectHub />
      </MemoryRouter>,
    );

    expect(screen.getByTestId("tooltip")).toBeInTheDocument();
    expect(screen.queryByText("No Projects Yet")).not.toBeInTheDocument();
    expect(screen.getByText(/Select Your Project/i)).toBeInTheDocument();
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

    render(
      <MemoryRouter>
        <ProjectHub />
      </MemoryRouter>,
    );

    expect(screen.getByTestId("empty-icon")).toBeInTheDocument();
    // Specific text matches to avoid duplicate regex matches
    expect(screen.getByText("No Projects Yet")).toBeInTheDocument();
    expect(
      screen.getByText(
        "Projects will appear here once they are created or assigned to you",
      ),
    ).toBeInTheDocument();
  });

  it("should log debug message when projects are loaded", async () => {
    render(
      <MemoryRouter>
        <ProjectHub />
      </MemoryRouter>,
    );
    await waitFor(() => {
      expect(mockLogger.debug).toHaveBeenCalledWith(
        expect.stringContaining("projects loaded"),
      );
    });
  });
});
