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
import { beforeEach, describe, expect, it, vi } from "vitest";
import ProjectSwitcher from "@components/header/ProjectSwitcher";
import * as useGetProjectsModule from "@api/useGetProjects";

const mockProjects = [
  {
    id: "1",
    name: "Project A",
    key: "PA",
    createdOn: "2025-01-01",
    description: "Desc A",
    hasAgent: true,
    activeCasesCount: 3,
    activeChatsCount: 4,
    actionRequiredCount: 0,
  },
  {
    id: "2",
    name: "Project B",
    key: "PB",
    createdOn: "2025-01-02",
    description: "Desc B",
    hasAgent: true,
    activeCasesCount: 15,
    activeChatsCount: 5,
    actionRequiredCount: 0,
  },
];

const defaultHookReturn = {
  data: {},
  isLoading: false,
  isError: false,
  fetchNextPage: vi.fn(),
  hasNextPage: false,
  isFetchingNextPage: false,
};

vi.mock("@api/useGetProjects", () => ({
  default: vi.fn(),
  flattenProjectPages: vi.fn(),
  getTotalRecords: vi.fn(),
}));

vi.mock("@api/useGetProjectDetails", () => ({
  default: vi.fn(() => ({ data: undefined })),
}));

vi.mock("@hooks/useDebouncedValue", () => ({
  useDebouncedValue: (value: string) => value,
}));

vi.mock("@components/select-menu-load-more-row/SelectMenuLoadMoreRow", () => ({
  SelectMenuLoadMoreRow: () => null,
}));

vi.mock("@features/project-hub/constants/projectHubConstants", async (importOriginal) => ({
  ...(await importOriginal<
    typeof import("@features/project-hub/constants/projectHubConstants")
  >()),
  PROJECT_HUB_PROJECTS_PAGE_SIZE: 10,
  PROJECT_HUB_SEARCH_DEBOUNCE_MS: 300,
  PROJECT_HUB_SEARCH_PLACEHOLDER: "Search projects...",
}));

// Mock @wso2/oxygen-ui
vi.mock("@wso2/oxygen-ui", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@wso2/oxygen-ui")>()),
  Box: ({ children, onScroll, onKeyDown, onMouseDown, onClick }: any) => (
    <div onScroll={onScroll} onKeyDown={onKeyDown} onMouseDown={onMouseDown} onClick={onClick}>
      {children}
    </div>
  ),
  Skeleton: ({ variant, width, height }: any) => (
    <div
      data-testid="skeleton"
      data-variant={variant}
      style={{ width, height }}
    />
  ),
  Typography: ({ children }: any) => <span>{children}</span>,
  Tooltip: ({ children }: any) => <>{children}</>,
  TextField: ({ placeholder, value, onChange }: any) => (
    <input
      data-testid="search-input"
      placeholder={placeholder}
      value={value}
      onChange={onChange}
    />
  ),
  Header: {
    Switchers: ({ children }: any) => <div>{children}</div>,
  },
  colors: {
    blue: { 700: "#1D4ED8" },
    purple: { 400: "#A78BFA" },
  },
}));

// Mock icons
vi.mock("@wso2/oxygen-ui-icons-react", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@wso2/oxygen-ui-icons-react")>()),
  FolderOpen: () => <svg data-testid="icon-FolderOpen" />,
  Search: () => <svg data-testid="icon-Search" />,
  Info: () => <svg data-testid="icon-Info" />,
  TriangleAlert: () => <svg data-testid="icon-TriangleAlert" />,
}));

// Mock ErrorIndicator
vi.mock("@components/error-indicator/ErrorIndicator", () => ({
  default: ({ entityName }: any) => (
    <div data-testid="error-indicator">Error: {entityName}</div>
  ),
}));

describe("ProjectSwitcher", () => {
  const mockOnProjectChange = vi.fn();

  beforeEach(() => {
    mockOnProjectChange.mockClear();
    vi.mocked(useGetProjectsModule.default).mockReturnValue(defaultHookReturn as any);
    vi.mocked(useGetProjectsModule.flattenProjectPages).mockReturnValue(mockProjects as any);
    vi.mocked(useGetProjectsModule.getTotalRecords).mockReturnValue(2);
  });

  it("should render projects in the dropdown", () => {
    render(
      <ProjectSwitcher
        projectId="1"
        onProjectChange={mockOnProjectChange}
      />,
    );

    // The dropdown is a popover that opens from the trigger.
    expect(screen.queryByRole("menuitem")).not.toBeInTheDocument();
    fireEvent.click(screen.getByText("Project A", { exact: false }));

    const items = screen.getAllByRole("menuitem");
    expect(items).toHaveLength(mockProjects.length);
    mockProjects.forEach((project, index) => {
      expect(items[index]).toHaveTextContent(project.name);
      expect(items[index]).toHaveTextContent(project.key);
    });
  });

  it("should call onProjectChange when a different project is selected", () => {
    render(
      <ProjectSwitcher
        projectId="1"
        onProjectChange={mockOnProjectChange}
      />,
    );

    fireEvent.click(screen.getByText("Project A", { exact: false }));
    fireEvent.click(
      screen.getByRole("menuitem", { name: /Project B/ }),
    );

    expect(mockOnProjectChange).toHaveBeenCalledWith(mockProjects[1].id);
  });

  it("should render skeleton when isLoading is true", () => {
    vi.mocked(useGetProjectsModule.default).mockReturnValue({
      ...defaultHookReturn,
      isLoading: true,
    } as any);
    vi.mocked(useGetProjectsModule.flattenProjectPages).mockReturnValue([]);
    vi.mocked(useGetProjectsModule.getTotalRecords).mockReturnValue(0);

    render(
      <ProjectSwitcher
        onProjectChange={mockOnProjectChange}
      />,
    );

    expect(screen.getByTestId("skeleton")).toBeInTheDocument();
    expect(screen.getByTestId("icon-FolderOpen")).toBeInTheDocument();
    expect(screen.queryByTestId("project-select")).not.toBeInTheDocument();
  });

  it("should render error indicator when isError is true", () => {
    vi.mocked(useGetProjectsModule.default).mockReturnValue({
      ...defaultHookReturn,
      isError: true,
    } as any);
    vi.mocked(useGetProjectsModule.flattenProjectPages).mockReturnValue([]);
    vi.mocked(useGetProjectsModule.getTotalRecords).mockReturnValue(0);

    render(
      <ProjectSwitcher
        onProjectChange={mockOnProjectChange}
      />,
    );

    expect(screen.getByTestId("error-indicator")).toBeInTheDocument();
    expect(screen.getByText("Error: Projects")).toBeInTheDocument();
    expect(screen.queryByTestId("project-select")).not.toBeInTheDocument();
  });

  describe("single-project user", () => {
    beforeEach(() => {
      vi.mocked(useGetProjectsModule.flattenProjectPages).mockReturnValue([
        mockProjects[0],
      ] as never);
      vi.mocked(useGetProjectsModule.getTotalRecords).mockReturnValue(1);
    });

    it("shows the project name when the URL project is the user's project", () => {
      render(<ProjectSwitcher projectId="1" onProjectChange={mockOnProjectChange} />);

      expect(screen.getByText("Project A")).toBeInTheDocument();
      expect(screen.queryByText("Select Project")).not.toBeInTheDocument();
    });

    it("does not show the user's own project name for a project id they cannot access", () => {
      render(
        <ProjectSwitcher projectId="not-mine" onProjectChange={mockOnProjectChange} />,
      );

      expect(screen.getByText("Select Project")).toBeInTheDocument();
      expect(screen.queryByText("Project A")).not.toBeInTheDocument();
    });

    it("lets the chip navigate to the user's accessible project", () => {
      render(
        <ProjectSwitcher projectId="not-mine" onProjectChange={mockOnProjectChange} />,
      );

      fireEvent.click(screen.getByText("Select Project"));
      expect(mockOnProjectChange).toHaveBeenCalledWith("1");
    });

    it("keeps the chip inert when the URL has no project id", () => {
      render(<ProjectSwitcher onProjectChange={mockOnProjectChange} />);

      expect(screen.getByText("Project A")).toBeInTheDocument();
      fireEvent.click(screen.getByText("Project A"));
      expect(mockOnProjectChange).not.toHaveBeenCalled();
    });
  });
});
