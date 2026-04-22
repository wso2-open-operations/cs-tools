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
import ProjectMetadata from "@features/project-details/components/project-overview/project-information/ProjectMetadata";

// Mock @wso2/oxygen-ui components
vi.mock("@wso2/oxygen-ui", () => ({
  Box: ({ children }: any) => <div>{children}</div>,
  Typography: ({ children }: any) => <span>{children}</span>,
  Grid: ({ children }: any) => <div>{children}</div>,
  Chip: ({ label, color }: any) => (
    <div data-testid="chip" data-color={color}>
      {label}
    </div>
  ),
  Skeleton: () => <div data-testid="skeleton" />,
}));

// Mock utils
vi.mock("@features/project-details/utils/projectDetails", () => ({
  getProjectTypeColor: vi.fn(() => "info"),
  getSupportTierColor: vi.fn(() => "warning"),
  getSLAStatusColor: vi.fn(() => "success"),
}));

describe("ProjectMetadata", () => {
  const defaultProps = {
    createdDate: "Jan 1, 2024",
    type: { id: "subscription", label: "Subscription" },
    supportTier: "Enterprise",
    slaStatus: "Good",
    goLivePlanDate: "Sep 12, 2025",
    onboardingStatus: "Not-Applicable",
    isLoading: false,
  };

  it("should render metadata fields correctly", () => {
    render(<ProjectMetadata {...defaultProps} />);

    expect(screen.getByText("Created Date")).toBeInTheDocument();
    expect(screen.getByText("Jan 1, 2024")).toBeInTheDocument();

    expect(screen.getByText("Project Type")).toBeInTheDocument();
    expect(screen.getByText("Subscription")).toBeInTheDocument();

    expect(screen.getByText("Support Tier")).toBeInTheDocument();
    expect(screen.getByText("Enterprise")).toBeInTheDocument();

    expect(screen.getByText("Overall Status")).toBeInTheDocument();
    expect(screen.getByText("Good")).toBeInTheDocument();

    expect(screen.getByText("Go Live Date")).toBeInTheDocument();
    expect(screen.getByText("Sep 12, 2025")).toBeInTheDocument();

    expect(screen.getByText("Onboarding Status")).toBeInTheDocument();
    expect(screen.getByText("Not-Applicable")).toBeInTheDocument();
  });

  it("should render skeletons when loading", () => {
    render(<ProjectMetadata {...defaultProps} isLoading={true} />);

    const skeletons = screen.getAllByTestId("skeleton");
    expect(skeletons.length).toBeGreaterThan(0);

    expect(screen.queryByText("Jan 1, 2024")).toBeNull();
    expect(screen.queryByText("Subscription")).toBeNull();
  });

  it("should assign correct colors to chips", () => {
    render(<ProjectMetadata {...defaultProps} />);

    expect(screen.getByText("Subscription")).toHaveAttribute(
      "data-color",
      "info",
    );
    expect(screen.getByText("Enterprise")).toHaveAttribute(
      "data-color",
      "warning",
    );
    expect(screen.getByText("Good")).toHaveAttribute("data-color", "success");
  });

  it("should hide onboarding status when hideOnboardingStatus is true", () => {
    render(<ProjectMetadata {...defaultProps} hideOnboardingStatus={true} />);
    expect(screen.queryByText("Onboarding Status")).not.toBeInTheDocument();
    expect(screen.queryByText("Not-Applicable")).not.toBeInTheDocument();
  });
});
