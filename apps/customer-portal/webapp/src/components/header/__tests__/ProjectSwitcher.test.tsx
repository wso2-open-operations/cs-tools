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
import { describe, expect, it, vi } from "vitest";
import ProjectSwitcher from "@/components/header/ProjectSwitcher";
import { mockProjects } from "@/models/mockData";

// Mock @wso2/oxygen-ui
vi.mock("@wso2/oxygen-ui", () => ({
  Box: ({ children }: { children: any }) => <div>{children}</div>,
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
          Text: ({ primary }: any) => primary,
        },
      ),
    },
  ),
  Header: {
    Switchers: ({ children }: any) => <div>{children}</div>,
  },
}));

// Mock icons
vi.mock("@wso2/oxygen-ui-icons-react", () => ({
  FolderOpen: () => <svg data-testid="icon-FolderOpen" />,
}));

describe("ProjectSwitcher", () => {
  const mockOnProjectChange = vi.fn();

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
      expect(screen.getByText(project.name)).toBeInTheDocument();
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
    fireEvent.change(select, { target: { value: mockProjects[1].key } });

    expect(mockOnProjectChange).toHaveBeenCalledWith(mockProjects[1].key);
  });
});
