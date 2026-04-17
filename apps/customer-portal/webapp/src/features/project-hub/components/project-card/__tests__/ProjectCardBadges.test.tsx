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
import ProjectCardBadges from "@features/project-hub/components/project-card/ProjectCardBadges";

vi.mock("@features/project-details/utils/projectDetails", () => ({
  getSLAStatusColor: (): "primary" => "primary",
}));

// Mock @wso2/oxygen-ui
vi.mock("@wso2/oxygen-ui", () => ({
  Box: ({ children, display, justifyContent, alignItems }: any) => (
    <div data-testid="box" style={{ display, justifyContent, alignItems }}>
      {children}
    </div>
  ),
  Chip: ({ label }: any) => <div data-testid="chip">{label}</div>,
  Form: {
    CardContent: ({ children, sx }: any) => (
      <div data-testid="card-content" style={sx}>
        {children}
      </div>
    ),
  },
}));

describe("ProjectCardBadges", () => {
  it("should render project key chip", () => {
    const props = {
      projectKey: "PROJ-1",
      slaStatus: "Needs Attention",
    };

    render(<ProjectCardBadges {...props} />);

    expect(screen.getByText(props.projectKey)).toBeInTheDocument();
    expect(screen.getByText(props.slaStatus)).toBeInTheDocument();
  });

  it("should render both project key and SLA chips", () => {
    const props = {
      projectKey: "PROJ-123",
      slaStatus: "Needs Attention",
    };

    render(<ProjectCardBadges {...props} />);

    const chips = screen.getAllByTestId("chip");
    expect(chips).toHaveLength(2);
    expect(chips[0].textContent).toBe(props.projectKey);
    expect(chips[1].textContent).toBe(props.slaStatus);
  });
});
