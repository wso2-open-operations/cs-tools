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
import ProjectCardBadges from "@/components/projectHub/projectCard/ProjectCardBadges";

// Mock @wso2/oxygen-ui
vi.mock("@wso2/oxygen-ui", () => ({
  Box: ({ children, display, justifyContent, alignItems }: any) => (
    <div data-testid="box" style={{ display, justifyContent, alignItems }}>
      {children}
    </div>
  ),
  Chip: ({ label, color }: any) => (
    <div data-testid="chip" data-color={color}>
      {label}
    </div>
  ),
  Form: {
    CardContent: ({ children, sx }: any) => (
      <div data-testid="card-content" style={sx}>
        {children}
      </div>
    ),
  },
}));

// Mock utils
vi.mock("@/utils/projectCard", () => ({
  getStatusColor: vi.fn((status) => {
    if (status === "All Good") return "success";
    if (status === "Need Attention") return "warning";
    return "default";
  }),
}));

// Mock ErrorIndicator
vi.mock("@/components/common/errorIndicator/ErrorIndicator", () => ({
  default: ({ entityName }: any) => (
    <div data-testid="error-indicator">Error: {entityName}</div>
  ),
}));

describe("ProjectCardBadges", () => {
  it("should render project key and status", () => {
    const props = {
      projectKey: "PROJ-1",
      status: "All Good",
    };

    render(<ProjectCardBadges {...props} />);

    expect(screen.getByText(props.projectKey)).toBeInTheDocument();
    expect(screen.getByText(props.status)).toBeInTheDocument();
  });

  it("should apply correct color to status chip", () => {
    const props = {
      projectKey: "PROJ-1",
      status: "Need Attention",
    };

    render(<ProjectCardBadges {...props} />);

    const chips = screen.getAllByTestId("chip");
    const statusChip = chips.find((chip) => chip.textContent === props.status);
    expect(statusChip?.getAttribute("data-color")).toBe("warning");
  });

  it("should render error indicator for status when isError is true", () => {
    const props = {
      projectKey: "PROJ-1",
      status: "All Good",
      isError: true,
    };

    render(<ProjectCardBadges {...props} />);

    expect(screen.getByText("Error: Status")).toBeInTheDocument();
    expect(screen.queryByText(props.status)).not.toBeInTheDocument();
  });
});
