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
import { describe, expect, it, vi } from "vitest";
import ProjectName from "../ProjectName";

// Mock @wso2/oxygen-ui components
vi.mock("@wso2/oxygen-ui", () => ({
  Box: ({ children }: any) => <div>{children}</div>,
  Typography: ({ children }: any) => <span>{children}</span>,
  Chip: ({ label }: any) => <div data-testid="chip">{label}</div>,
  Skeleton: () => <div data-testid="skeleton" />,
}));

describe("ProjectName", () => {
  it("should render project name and key when not loading", () => {
    const name = "WSO2 Super App";
    const projectKey = "WSA";

    render(
      <ProjectName name={name} projectKey={projectKey} isLoading={false} />,
    );

    expect(screen.getByText("Project Name")).toBeInTheDocument();
    expect(screen.getByText(name)).toBeInTheDocument();
    expect(screen.getByText(projectKey)).toBeInTheDocument();
  });

  it("should render skeletons when loading", () => {
    render(<ProjectName name="test" projectKey="key" isLoading={true} />);

    expect(screen.getByText("Project Name")).toBeInTheDocument();
    const skeletons = screen.getAllByTestId("skeleton");
    expect(skeletons.length).toBeGreaterThan(0);
    expect(screen.queryByText("test")).toBeNull();
    expect(screen.queryByText("key")).toBeNull();
  });
});
