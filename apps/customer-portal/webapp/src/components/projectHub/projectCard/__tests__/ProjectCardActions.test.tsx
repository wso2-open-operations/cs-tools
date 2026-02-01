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
import ProjectCardActions from "@/components/projectHub/projectCard/ProjectCardActions";

// Mock @wso2/oxygen-ui
vi.mock("@wso2/oxygen-ui", () => ({
  Button: ({ children, endIcon, onClick }: any) => (
    <button onClick={onClick}>
      {children}
      {endIcon}
    </button>
  ),
  Form: {
    CardActions: ({ children }: any) => <div>{children}</div>,
  },
  Stack: ({ children }: any) => <div>{children}</div>,
}));

// Mock icons
vi.mock("@wso2/oxygen-ui-icons-react", () => ({
  ArrowRight: () => <svg data-testid="arrow-right-icon" />,
}));

describe("ProjectCardActions", () => {
  it("should render 'View Dashboard' button", () => {
    render(<ProjectCardActions />);

    expect(screen.getByText("View Dashboard")).toBeInTheDocument();
    expect(screen.getByTestId("arrow-right-icon")).toBeInTheDocument();
  });
  it("should call onViewDashboard and stop propagation when 'View Dashboard' button is clicked", () => {
    const onViewDashboardMock = vi.fn();
    const onParentClick = vi.fn();

    render(
      <div onClick={onParentClick}>
        <ProjectCardActions onViewDashboard={onViewDashboardMock} />
      </div>,
    );

    const button = screen.getByText("View Dashboard");
    fireEvent.click(button);

    expect(onViewDashboardMock).toHaveBeenCalledTimes(1);
    expect(onParentClick).not.toHaveBeenCalled();
  });
});
