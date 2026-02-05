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
import ProjectInformationCard from "../ProjectInformationCard";

// Mock @wso2/oxygen-ui components
vi.mock("@wso2/oxygen-ui", () => ({
  Box: ({ children }: any) => <div>{children}</div>,
  Card: ({ children }: any) => <div>{children}</div>,
  CardContent: ({ children }: any) => <div>{children}</div>,
}));

// Mock child components
vi.mock("../ProjectHeader", () => ({
  default: () => <div data-testid="project-header" />,
}));
vi.mock("../ProjectName", () => ({
  default: ({ name, projectKey }: any) => (
    <div data-testid="project-name">
      {name} - {projectKey}
    </div>
  ),
}));
vi.mock("../ProjectDescription", () => ({
  default: ({ description }: any) => (
    <div data-testid="project-description">{description}</div>
  ),
}));
vi.mock("../ProjectMetadata", () => ({
  default: ({ slaStatus }: any) => (
    <div data-testid="project-metadata">SLA: {slaStatus}</div>
  ),
}));
vi.mock("../SubscriptionDetails", () => ({
  default: ({ startDate, endDate }: any) => (
    <div data-testid="subscription-details">
      {startDate} - {endDate}
    </div>
  ),
}));

// Mock utils
vi.mock("@/utils/projectStats", () => ({
  formatProjectDate: vi.fn((date) => `Formatted ${date}`),
}));

describe("ProjectInformationCard", () => {
  const mockProject: any = {
    key: "TEST-KEY",
    name: "Test Project",
    description: "Test Desc",
    createdOn: "2023-01-01",
    type: "Subscription",
    subscription: {
      supportTier: "Gold",
      startDate: "2023-01-01",
      endDate: "2024-01-01",
    },
  };

  it("should render all sub-components with correct data", () => {
    render(
      <ProjectInformationCard
        project={mockProject}
        slaStatus="Met"
        isLoading={false}
      />,
    );

    expect(screen.getByTestId("project-header")).toBeInTheDocument();

    expect(screen.getByTestId("project-name")).toHaveTextContent(
      "Test Project - TEST-KEY",
    );

    expect(screen.getByTestId("project-description")).toHaveTextContent(
      "Test Desc",
    );

    expect(screen.getByTestId("project-metadata")).toHaveTextContent(
      "SLA: Met",
    );

    // Dates should be formatted
    expect(screen.getByTestId("subscription-details")).toHaveTextContent(
      "Formatted 2023-01-01 - Formatted 2024-01-01",
    );
  });

  it("should handle missing project data gracefully (-- defaults)", () => {
    render(
      <ProjectInformationCard
        project={undefined}
        slaStatus="Unknown"
        isLoading={false}
      />,
    );

    expect(screen.getByTestId("project-name")).toHaveTextContent("-- - --");
    expect(screen.getByTestId("project-description")).toHaveTextContent("--");
    // Ensure it doesn't crash
    expect(screen.getByTestId("project-metadata")).toBeInTheDocument();
  });
});
