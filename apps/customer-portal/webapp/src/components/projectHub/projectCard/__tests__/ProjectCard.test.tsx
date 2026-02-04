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
import { describe, expect, it, vi, beforeEach } from "vitest";
import ProjectCard from "@/components/projectHub/projectCard/ProjectCard";

// Mock @wso2/oxygen-ui
vi.mock("@wso2/oxygen-ui", () => ({
  Form: {
    CardButton: ({ children, onClick }: any) => (
      <button data-testid="card-button" onClick={onClick}>
        {children}
      </button>
    ),
  },
}));

// Mock sub-components
vi.mock("../ProjectCardBadges", () => ({
  default: ({ projectKey, status, isError }: any) => (
    <div data-testid="badges">
      {projectKey} {status} {isError ? "Error" : "No Error"}
    </div>
  ),
}));

vi.mock("../ProjectCardInfo", () => ({
  default: ({ title, subtitle }: any) => (
    <div data-testid="info">
      {title} {subtitle}
    </div>
  ),
}));

vi.mock("../ProjectCardStats", () => ({
  default: ({ activeChats, date, openCases, isError }: any) => (
    <div data-testid="stats">
      {activeChats} {date} {openCases} {isError ? "Error" : "No Error"}
    </div>
  ),
}));

vi.mock("../ProjectCardActions", () => ({
  default: () => <div data-testid="actions" />,
}));

// Mock react-router
const mockNavigate = vi.fn();
vi.mock("react-router", () => ({
  useNavigate: () => mockNavigate,
}));

// Mock mockFunctions
vi.mock("@/models/mockFunctions", () => ({
  getMockActiveChats: vi.fn(() => 5),
  getMockOpenCases: vi.fn(() => 10),
  getMockStatus: vi.fn(() => "All Good"),
}));

describe("ProjectCard", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  const defaultProps = {
    id: "1",
    projectKey: "PROJ",
    title: "Project Title",
    subtitle: "Project Subtitle",
    date: "2026-01-17",
  };

  it("should render all sub-components with correct props", () => {
    render(<ProjectCard {...defaultProps} />);

    expect(screen.getByTestId("badges")).toBeInTheDocument();
    expect(screen.getByTestId("info")).toBeInTheDocument();
    expect(screen.getByTestId("stats")).toBeInTheDocument();
    expect(screen.getByTestId("actions")).toBeInTheDocument();

    expect(screen.getByTestId("badges")).toHaveTextContent("PROJ");
    expect(screen.getByTestId("info")).toHaveTextContent("Project Title");
    expect(screen.getByTestId("stats")).toHaveTextContent("2026-01-17");
  });

  it("should navigate to dashboard on click", () => {
    render(<ProjectCard {...defaultProps} />);

    fireEvent.click(screen.getByTestId("card-button"));

    expect(mockNavigate).toHaveBeenCalledWith("/1/dashboard");
  });

  it("should call onViewDashboard if provided", () => {
    const onViewDashboard = vi.fn();
    render(<ProjectCard {...defaultProps} onViewDashboard={onViewDashboard} />);

    fireEvent.click(screen.getByTestId("card-button"));

    expect(onViewDashboard).toHaveBeenCalled();
    expect(mockNavigate).not.toHaveBeenCalled();
  });

  it("should pass isError to sub-components when isStatsError is true", () => {
    render(<ProjectCard {...defaultProps} isStatsError={true} />);

    expect(screen.getByTestId("badges")).toHaveTextContent("Error");
    expect(screen.getByTestId("stats")).toHaveTextContent("Error");
  });
});
