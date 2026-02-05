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
import ProjectSwitcher from "@/components/common/header/ProjectSwitcher";
import { mockProjects } from "@/models/mockData";

// Mock @wso2/oxygen-ui
vi.mock("@wso2/oxygen-ui", () => ({
  Box: ({ children }: { children: any }) => <div>{children}</div>,
  Skeleton: ({ variant, width, height }: any) => (
    <div
      data-testid="skeleton"
      data-variant={variant}
      style={{ width, height }}
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
    blue: { 700: "#1d4ed8" },
    purple: { 400: "#a78bfa" },
  },
}));

// Mock icons
vi.mock("@wso2/oxygen-ui-icons-react", () => ({
  FolderOpen: () => <svg data-testid="icon-FolderOpen" />,
  Info: () => <svg data-testid="icon-Info" />,
  Server: () => <svg data-testid="icon-Server" />,
  Clock: () => <svg data-testid="icon-Clock" />,
  User: () => <svg data-testid="icon-User" />,
  Shield: () => <svg data-testid="icon-Shield" />,
  Rocket: () => <svg data-testid="icon-Rocket" />,
  CircleAlert: () => <svg data-testid="icon-CircleAlert" />,
}));

// Mock ErrorIndicator
vi.mock("@/components/common/errorIndicator/ErrorIndicator", () => ({
  default: ({ entityName }: any) => (
    <div data-testid="error-indicator">Error: {entityName}</div>
  ),
}));

describe("ProjectSwitcher", () => {
  const mockOnProjectChange = vi.fn();

  beforeEach(() => {
    mockOnProjectChange.mockClear();
  });

  it("should render projects in the dropdown", () => {
    render(
      <ProjectSwitcher
        projects={mockProjects}
        selectedProject={mockProjects[0]}
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
        projects={mockProjects}
        selectedProject={mockProjects[0]}
        onProjectChange={mockOnProjectChange}
      />,
    );

    const select = screen.getByTestId("project-select");
    fireEvent.change(select, { target: { value: mockProjects[1].id } });

    expect(mockOnProjectChange).toHaveBeenCalledWith(mockProjects[1].id);
  });

  it("should render skeleton when isLoading is true", () => {
    render(
      <ProjectSwitcher
        projects={mockProjects}
        onProjectChange={mockOnProjectChange}
        isLoading={true}
      />,
    );

    expect(screen.getByTestId("skeleton")).toBeInTheDocument();
    expect(screen.getByTestId("icon-FolderOpen")).toBeInTheDocument();
    expect(screen.queryByTestId("project-select")).not.toBeInTheDocument();
  });

  it("should render error indicator when isError is true", () => {
    render(
      <ProjectSwitcher
        projects={mockProjects}
        onProjectChange={mockOnProjectChange}
        isError={true}
      />,
    );

    expect(screen.getByTestId("error-indicator")).toBeInTheDocument();
    expect(screen.getByText("Error: Projects")).toBeInTheDocument();
    expect(screen.queryByTestId("project-select")).not.toBeInTheDocument();
  });
});
