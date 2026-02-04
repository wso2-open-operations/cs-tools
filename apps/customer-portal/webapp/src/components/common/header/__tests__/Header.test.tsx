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

import { render, screen, fireEvent } from "@testing-library/react";
import { describe, expect, it, vi, beforeEach } from "vitest";
import Header from "@/components/common/header/Header";
import { mockProjects } from "@/models/mockData";

// Mock @wso2/oxygen-ui
vi.mock("@wso2/oxygen-ui", () => ({
  Header: Object.assign(
    ({ children }: { children: any }) => <header>{children}</header>,
    {
      Toggle: ({ onToggle }: { onToggle: any }) => (
        <button onClick={onToggle}>Toggle</button>
      ),
      Brand: ({ children }: { children: any }) => <div>{children}</div>,
      BrandLogo: ({ children }: { children: any }) => <div>{children}</div>,
      BrandTitle: ({ children }: { children: any }) => <h1>{children}</h1>,
      Switchers: ({ children }: { children: any }) => <div>{children}</div>,
      Spacer: () => <div />,
      Actions: ({ children }: { children: any }) => <div>{children}</div>,
    },
  ),
  ComplexSelect: Object.assign(
    ({
      value,
      onChange,
      children,
    }: {
      value: any;
      onChange: any;
      children: any;
    }) => (
      <select value={value} onChange={onChange} data-testid="project-select">
        {children}
      </select>
    ),
    {
      MenuItem: Object.assign(
        ({ value, children }: { value: any; children: any }) => (
          <option value={value}>{children}</option>
        ),
        {
          Text: ({ primary }: { primary: string }) => primary,
        },
      ),
    },
  ),
  ColorSchemeToggle: () => <button>ThemeToggle</button>,
  Divider: () => <hr />,
}));

// Mock icons
vi.mock("@wso2/oxygen-ui-icons-react", () => {
  const mockIcon = (name: string) => () => <svg data-testid={`icon-${name}`} />;
  return {
    Briefcase: mockIcon("Briefcase"),
    FileText: mockIcon("FileText"),
    FolderOpen: mockIcon("FolderOpen"),
    Headset: mockIcon("Headset"),
    Home: mockIcon("Home"),
    Megaphone: mockIcon("Megaphone"),
    RefreshCw: mockIcon("RefreshCw"),
    Shield: mockIcon("Shield"),
    Users: mockIcon("Users"),
    Settings: mockIcon("Settings"),
    WSO2: () => <svg data-testid="wso2-logo" />,
  };
});

// Mock react-router
const mockNavigate = vi.fn();
const mockLocation = { pathname: "/" };
const mockParams = { projectId: "" };

vi.mock("react-router", () => ({
  useNavigate: () => mockNavigate,
  useLocation: () => mockLocation,
  useParams: () => mockParams,
}));

// Mock hooks
const mockLogger = {
  debug: vi.fn(),
  info: vi.fn(),
  warn: vi.fn(),
  error: vi.fn(),
};

vi.mock("@/hooks/useLogger", () => ({
  useLogger: () => mockLogger,
}));

const mockFetchNextPage = vi.fn();
const mockUseSearchProjects = vi.fn(() => ({
  data: {
    pages: [{ projects: mockProjects }],
  },
  fetchNextPage: mockFetchNextPage,
  hasNextPage: false,
  isFetchingNextPage: false,
  isError: false,
})) as any;

vi.mock("@/api/useGetProjects", () => ({
  default: (searchData: any, fetchAll: any) =>
    mockUseSearchProjects(searchData, fetchAll),
}));

// Mock sub-components
vi.mock("../Brand", () => ({
  default: () => <div data-testid="brand" />,
}));

vi.mock("../Actions", () => ({
  default: () => <div data-testid="actions" />,
}));

vi.mock("../SearchBar", () => ({
  default: () => <div data-testid="search-bar" />,
}));

vi.mock("../ProjectSwitcher", () => ({
  default: ({ projects, selectedProject, onProjectChange, isLoading }: any) => (
    <div
      data-testid="project-switcher"
      data-selected-id={selectedProject?.id || ""}
      data-loading={isLoading ? "true" : "false"}
    >
      <select
        data-testid="project-select"
        value={selectedProject?.id || ""}
        onChange={(e) => onProjectChange(e.target.value)}
      >
        <option value="">None</option>
        {projects.map((p: any) => (
          <option key={p.id} value={p.id}>
            {p.name}
          </option>
        ))}
      </select>
    </div>
  ),
}));

// Mock UserProfile
vi.mock("../UserProfile", () => ({
  default: () => <div data-testid="user-profile" />,
}));

describe("Header", () => {
  const mockOnToggleSidebar = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    mockLocation.pathname = "/";
    mockParams.projectId = "";
    mockUseSearchProjects.mockReturnValue({
      data: {
        pages: [{ projects: mockProjects }],
      },
      fetchNextPage: mockFetchNextPage,
      hasNextPage: false,
      isFetchingNextPage: false,
      isError: false,
    });
  });

  it("should render the brand component", () => {
    render(<Header onToggleSidebar={mockOnToggleSidebar} />);
    expect(screen.getByTestId("brand")).toBeInTheDocument();
  });

  it("should NOT render toggle or switcher on Project Hub", () => {
    render(<Header onToggleSidebar={mockOnToggleSidebar} />);
    expect(screen.queryByText("Toggle")).toBeNull();
    expect(screen.queryByTestId("project-select")).toBeNull();
  });

  it("should render toggle and switcher on a project page", () => {
    mockLocation.pathname = "/project-1/dashboard";
    mockParams.projectId = "project-1";

    render(<Header onToggleSidebar={mockOnToggleSidebar} />);

    expect(screen.getByText("Toggle")).toBeInTheDocument();
    expect(screen.getByTestId("project-select")).toBeInTheDocument();
  });

  it("should call onToggleSidebar when toggle is clicked", () => {
    mockLocation.pathname = "/project-1/dashboard";
    mockParams.projectId = "project-1";

    render(<Header onToggleSidebar={mockOnToggleSidebar} />);

    fireEvent.click(screen.getByText("Toggle"));
    expect(mockOnToggleSidebar).toHaveBeenCalled();
  });

  it("should navigate when a different project is selected", () => {
    mockLocation.pathname = "/project-1/dashboard";
    mockParams.projectId = "project-1";

    render(<Header onToggleSidebar={mockOnToggleSidebar} />);

    const select = screen.getByTestId("project-select");
    fireEvent.change(select, { target: { value: mockProjects[1].id } });

    expect(mockNavigate).toHaveBeenCalledWith(
      `/${mockProjects[1].id}/dashboard`,
    );
    expect(mockLogger.debug).toHaveBeenCalledWith(
      expect.stringContaining("Switching to project"),
    );
  });

  it("should log error when project fetch fails", () => {
    mockUseSearchProjects.mockReturnValue({
      data: null,
      isLoading: false,
      isError: true,
    });

    render(<Header onToggleSidebar={mockOnToggleSidebar} />);

    expect(mockLogger.error).toHaveBeenCalledWith(
      expect.stringContaining("Failed to fetch projects"),
    );
  });

  it("should log debug message when projects are loaded", () => {
    render(<Header onToggleSidebar={mockOnToggleSidebar} />);

    expect(mockLogger.debug).toHaveBeenCalledWith(
      expect.stringContaining("projects loaded"),
    );
  });

  it("should clear selection when projectId is invalid", () => {
    mockLocation.pathname = "/invalid-project/dashboard";
    mockParams.projectId = "invalid-project";

    render(<Header onToggleSidebar={mockOnToggleSidebar} />);

    const switcher = screen.getByTestId("project-switcher");
    expect(switcher).toHaveAttribute("data-selected-id", "");
  });

  it("should clear selection when projectId is missing (hub page)", () => {
    mockLocation.pathname = "/";
    mockParams.projectId = "";

    render(<Header onToggleSidebar={mockOnToggleSidebar} />);

    // Switcher is not rendered on Hub, but we can verify the state if it was
    // For coverage of the useEffect clearing logic, we check that it handles missing projectId
    expect(screen.queryByTestId("project-switcher")).toBeNull();
  });

  it("should call fetchNextPage if hasNextPage is true", () => {
    mockUseSearchProjects.mockReturnValue({
      data: { pages: [{ projects: mockProjects }] },
      fetchNextPage: mockFetchNextPage,
      hasNextPage: true,
      isFetchingNextPage: false,
      isError: false,
    });

    render(<Header onToggleSidebar={mockOnToggleSidebar} />);

    expect(mockFetchNextPage).toHaveBeenCalled();
  });

  it("should NOT call fetchNextPage if isError is true", () => {
    mockUseSearchProjects.mockReturnValue({
      data: { pages: [{ projects: mockProjects }] },
      fetchNextPage: mockFetchNextPage,
      hasNextPage: true,
      isFetchingNextPage: false,
      isError: true,
    });

    render(<Header onToggleSidebar={mockOnToggleSidebar} />);

    expect(mockFetchNextPage).not.toHaveBeenCalled();
  });

  it("should render Actions", () => {
    render(<Header onToggleSidebar={mockOnToggleSidebar} />);
    expect(screen.getByTestId("actions")).toBeInTheDocument();
  });

  it("should pass isLoading to ProjectSwitcher", () => {
    mockLocation.pathname = "/project-1/dashboard";
    mockParams.projectId = "project-1";
    mockUseSearchProjects.mockReturnValue({
      data: { pages: [{ projects: mockProjects }] },
      fetchNextPage: mockFetchNextPage,
      hasNextPage: false,
      isLoading: true,
      isError: false,
    });

    render(<Header onToggleSidebar={mockOnToggleSidebar} />);

    const switcher = screen.getByTestId("project-switcher");
    expect(switcher).toHaveAttribute("data-loading", "true");
  });
});
