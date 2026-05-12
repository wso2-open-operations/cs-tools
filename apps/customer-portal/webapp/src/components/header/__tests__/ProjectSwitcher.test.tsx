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

vi.mock("@features/project-hub/constants/projectHubConstants", () => ({
  PROJECT_HUB_PROJECTS_PAGE_SIZE: 10,
  PROJECT_HUB_SEARCH_DEBOUNCE_MS: 300,
  PROJECT_HUB_SEARCH_PLACEHOLDER: "Search projects...",
}));

// Mock @wso2/oxygen-ui
vi.mock("@wso2/oxygen-ui", () => ({
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
  ComplexSelect: Object.assign(
    ({ value, onChange, children }: any) => (
      <select
        data-testid="project-select"
        value={value}
        onChange={(e) => onChange(e)}
      >
        {children}
      </select>
    ),
    {
      ListHeader: ({ children }: any) => <option disabled>{children}</option>,
      MenuItem: Object.assign(
        ({ value, children }: any) => <option value={value}>{children}</option>,
        {
          Text: ({ primary, secondary }: any) => (
            <>
              {primary} {secondary}
            </>
          ),
        },
      ),
    },
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
vi.mock("@wso2/oxygen-ui-icons-react", () => ({
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

    expect(screen.getByTestId("project-select")).toBeInTheDocument();
    mockProjects.forEach((project) => {
      expect(
        screen.getByText(project.name, { exact: false }),
      ).toBeInTheDocument();
      expect(
        screen.getByText(project.key, { exact: false }),
      ).toBeInTheDocument();
    });
  });

  it("should call onProjectChange when a different project is selected", () => {
    render(
      <ProjectSwitcher
        projectId="1"
        onProjectChange={mockOnProjectChange}
      />,
    );

    const select = screen.getByTestId("project-select");
    fireEvent.change(select, { target: { value: mockProjects[1].id } });

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
});
