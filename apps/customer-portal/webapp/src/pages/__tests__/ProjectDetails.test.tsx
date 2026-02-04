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

import { render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi, beforeEach } from "vitest";
import ProjectDetails from "../ProjectDetails";

// Mock hooks
const mockUseLogger = {
  error: vi.fn(),
};

const mockShowLoader = vi.fn();
const mockHideLoader = vi.fn();

vi.mock("@/hooks/useLogger", () => ({
  useLogger: () => mockUseLogger,
}));

vi.mock("@/context/linearLoader/LoaderContext", () => ({
  useLoader: () => ({
    showLoader: mockShowLoader,
    hideLoader: mockHideLoader,
  }),
}));

vi.mock("react-router", () => ({
  useParams: () => ({ projectId: "123" }),
  useOutletContext: () => ({ sidebarCollapsed: false }),
}));

// Mock API hooks
const mockUseGetProjectDetails = vi.fn();
const mockUseGetProjectStat = vi.fn();

vi.mock("@/api/useGetProjectDetails", () => ({
  default: () => mockUseGetProjectDetails(),
}));

vi.mock("@/api/useGetProjectStat", () => ({
  useGetProjectStat: () => mockUseGetProjectStat(),
}));

// Mock Child Components
vi.mock("@/components/common/tabBar/TabBar", () => ({
  default: () => <div data-testid="tab-bar" />,
}));

vi.mock(
  "@/components/projectDetails/projectOverview/projectInformation/ProjectInformationCard",
  () => ({
    default: () => <div data-testid="project-info-card" />,
  }),
);

vi.mock(
  "@/components/projectDetails/projectOverview/projectStatistics/ProjectStatisticsCard",
  () => ({
    default: () => <div data-testid="project-stats-card" />,
  }),
);

vi.mock(
  "@/components/projectDetails/projectOverview/contactInfo/ContactInfoCard",
  () => ({
    default: () => <div data-testid="contact-info-card" />,
  }),
);

vi.mock(
  "@/components/projectDetails/projectOverview/recentActivity/RecentActivityCard",
  () => ({
    default: () => <div data-testid="recent-activity-card" />,
  }),
);

vi.mock("@wso2/oxygen-ui", () => ({
  Box: ({ children }: any) => <div>{children}</div>,
  Typography: ({ children }: any) => <span>{children}</span>,
  Grid: ({ children }: any) => <div>{children}</div>,
  colors: {
    blue: { 700: "#blue" },
    purple: { 700: "#purple" },
    teal: { 700: "#teal" },
    orange: { 700: "#orange" },
  },
}));

describe("ProjectDetails", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseGetProjectDetails.mockReturnValue({
      data: null,
      isLoading: false,
      error: null,
    });
    mockUseGetProjectStat.mockReturnValue({
      data: null,
      isLoading: false,
      error: null,
    });
  });

  it("should render project details content when data is loaded", () => {
    mockUseGetProjectDetails.mockReturnValue({
      data: { id: "123", name: "Test Project" },
      isLoading: false,
      error: null,
    });

    render(<ProjectDetails />);

    expect(screen.getByTestId("tab-bar")).toBeInTheDocument();
    expect(screen.getByTestId("project-info-card")).toBeInTheDocument();
  });

  it("should show loader when loading", () => {
    mockUseGetProjectDetails.mockReturnValue({
      data: null,
      isLoading: true,
      error: null,
    });

    render(<ProjectDetails />);

    expect(mockShowLoader).toHaveBeenCalled();
  });

  it("should hide loader when loading completes", () => {
    mockUseGetProjectDetails.mockReturnValue({
      data: null,
      isLoading: true,
      error: null,
    });
    mockUseGetProjectStat.mockReturnValue({
      data: null,
      isLoading: false,
      error: null,
    });

    const { rerender } = render(<ProjectDetails />);
    expect(mockShowLoader).toHaveBeenCalledTimes(1);
    expect(mockHideLoader).not.toHaveBeenCalled();

    mockUseGetProjectDetails.mockReturnValue({
      data: { id: "123" },
      isLoading: false,
      error: null,
    });

    rerender(<ProjectDetails />);
    expect(mockHideLoader).toHaveBeenCalledTimes(1);
  });

  it("should log error when project details fail to load", async () => {
    const error = new Error("Project Load Failed");
    mockUseGetProjectDetails.mockReturnValue({
      data: null,
      isLoading: false,
      error: error,
    });

    render(<ProjectDetails />);

    await waitFor(() => {
      expect(mockUseLogger.error).toHaveBeenCalledWith(
        "Error loading project details:",
        error,
      );
    });
  });

  it("should log error when project stats fail to load", async () => {
    const error = new Error("Stats Load Failed");
    mockUseGetProjectStat.mockReturnValue({
      data: null,
      isLoading: false,
      error: error,
    });

    render(<ProjectDetails />);

    await waitFor(() => {
      expect(mockUseLogger.error).toHaveBeenCalledWith(
        "Error loading project stats:",
        error,
      );
    });
  });
});
