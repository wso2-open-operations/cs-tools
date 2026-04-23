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
import { describe, expect, it, vi, beforeEach } from "vitest";
import SideBar from "@components/side-nav-bar/SideBar";
import { APP_SHELL_NAV_ITEMS } from "@features/project-hub/constants/appLayoutConstants";
import type { AppShellNavItem } from "@features/project-hub/types/appLayout";
import { ProjectType as PROJECT_TYPE_LABELS } from "@/types/permission";

// Mock @wso2/oxygen-ui
vi.mock("@wso2/oxygen-ui", () => {
  const Sidebar = ({
    children,
    activeItem,
  }: {
    children: any;
    activeItem?: string;
  }) => (
    <aside data-testid="sidebar" data-active-item={activeItem}>
      {children}
    </aside>
  );

  Sidebar.Nav = ({ children }: { children: any }) => <nav>{children}</nav>;
  Sidebar.Category = ({ children }: { children: any }) => <ul>{children}</ul>;
  Sidebar.Item = ({ children, id }: { children: any; id: string }) => (
    <li data-testid={`sidebar-item-${id}`}>{children}</li>
  );
  Sidebar.ItemIcon = ({ children }: { children: any }) => <div>{children}</div>;
  Sidebar.ItemLabel = ({ children }: { children: any }) => (
    <span>{children}</span>
  );
  Sidebar.Footer = ({ children }: { children: any }) => (
    <footer>{children}</footer>
  );

  const Box = ({ children }: any) => <div>{children}</div>;
  const Typography = ({ children }: any) => <span>{children}</span>;
  const Paper = ({ children }: any) => <div>{children}</div>;
  const Button = ({ children }: any) => <button>{children}</button>;
  return {
    Sidebar,
    Box,
    Typography,
    Paper,
    Button,
    Link: ({
      children,
      to,
    }: {
      children: any;
      to: string;
      component?: any;
    }) => {
      // Avoid nesting buttons/links. Render a simple `a` tag directly enclosing children.
      return <a href={to}>{children}</a>;
    },
    colors: {
      blue: { 700: "#1D4ED8" },
      purple: { 400: "#A78BFA" },
    },
  };
});

// Mock icons - explicitly providing icons used in APP_SHELL_NAV_ITEMS and SideBar
vi.mock("@wso2/oxygen-ui-icons-react", () => {
  const mockIcon = (name: string) => () => <svg data-testid={`icon-${name}`} />;
  return {
    BarChart3: mockIcon("BarChart3"),
    Briefcase: mockIcon("Briefcase"),
    CircleAlert: mockIcon("CircleAlert"),
    Clock: mockIcon("Clock"),
    Cog: mockIcon("Cog"),
    FileText: mockIcon("FileText"),
    FolderOpen: mockIcon("FolderOpen"),
    Headset: mockIcon("Headset"),
    Home: mockIcon("Home"),
    Info: mockIcon("Info"),
    LayoutDashboard: mockIcon("LayoutDashboard"),
    Megaphone: mockIcon("Megaphone"),
    RefreshCw: mockIcon("RefreshCw"),
    Rocket: mockIcon("Rocket"),
    Server: mockIcon("Server"),
    Shield: mockIcon("Shield"),
    User: mockIcon("User"),
    Users: mockIcon("Users"),
    Settings: mockIcon("Settings"),
    Crown: mockIcon("Crown"),
  };
});

// Mock react-router
const mockLocation = { pathname: "/projects/project-1/dashboard" };
const mockParams: { projectId: string | undefined } = {
  projectId: "project-1",
};

let mockProjectTypeLabel: string = "Other";

const createMockProjectsResponse = () => [
  {
    id: "project-1",
    type: { label: mockProjectTypeLabel },
  },
];

vi.mock("react-router", () => ({
  useLocation: () => mockLocation,
  useParams: () => mockParams,
  Link: ({ children, to }: { children: any; to: string }) => (
    <a href={to}>{children}</a>
  ),
}));

vi.mock("@api/useGetProjects", () => {
  return {
    __esModule: true,
    default: () => ({
      data: {
        pages: [
          {
            projects: createMockProjectsResponse(),
            totalRecords: 1,
          },
        ],
      },
    }),
    flattenProjectPages: (data: any) =>
      data?.pages?.flatMap((page: any) => page.projects) ?? [],
  };
});

vi.mock("@api/useGetProjectDetails", () => ({
  __esModule: true,
  default: () => ({
    data: undefined,
    isLoading: false,
    isError: false,
  }),
}));

vi.mock("@components/side-nav-bar/SubscriptionWidget", () => {
  return {
    __esModule: true,
    default: () => <div data-testid="subscription-widget" />,
  };
});

let mockUsageMetricsEnabled = true;

vi.mock("@api/useGetMetadata", () => ({
  __esModule: true,
  default: () => ({
    data: {
      timeZones: [],
      featureFlags: { usageMetricsEnabled: mockUsageMetricsEnabled },
    },
    isLoading: false,
  }),
}));

describe("SideBar", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockLocation.pathname = "/projects/project-1/dashboard";
    mockParams.projectId = "project-1";
    mockProjectTypeLabel = "Other";
    mockUsageMetricsEnabled = true;
  });

  it("should render all navigation items except Operations and Engagements when the project type is not supported", () => {
    mockProjectTypeLabel = "Other";
    render(<SideBar collapsed={false} />);

    APP_SHELL_NAV_ITEMS.filter(
      (item: AppShellNavItem) =>
        item.id !== "operations" && item.id !== "engagements",
    ).forEach((item: AppShellNavItem) => {
      expect(screen.getByText(item.label)).toBeInTheDocument();
    });

    expect(screen.queryByText("Operations")).not.toBeInTheDocument();
    expect(screen.queryByText("Engagements")).not.toBeInTheDocument();
  });

  it("should render the Operations item for Managed Cloud Subscription", () => {
    mockProjectTypeLabel = PROJECT_TYPE_LABELS.MANAGED_CLOUD_SUBSCRIPTION;
    render(<SideBar collapsed={false} />);

    expect(screen.getByText("Operations")).toBeInTheDocument();
  });

  it("should render the Operations item for Cloud Support (SR-only)", () => {
    mockProjectTypeLabel = PROJECT_TYPE_LABELS.CLOUD_SUPPORT;
    render(<SideBar collapsed={false} />);

    expect(screen.getByText("Operations")).toBeInTheDocument();
  });

  it("should not render Operations for Cloud Evaluation Support", () => {
    mockProjectTypeLabel = PROJECT_TYPE_LABELS.CLOUD_EVALUATION_SUPPORT;
    render(<SideBar collapsed={false} />);

    expect(screen.queryByText("Operations")).not.toBeInTheDocument();
  });

  it("should render the settings item", () => {
    render(<SideBar collapsed={false} />);
    expect(screen.getByText("Settings")).toBeInTheDocument();
    expect(screen.getByTestId("icon-Settings")).toBeInTheDocument();
  });

  it("should hide Usage & Metrics when featureFlags.usageMetricsEnabled is false", () => {
    mockUsageMetricsEnabled = false;
    mockProjectTypeLabel = PROJECT_TYPE_LABELS.MANAGED_CLOUD_SUBSCRIPTION;
    render(<SideBar collapsed={false} />);

    expect(screen.queryByText("Usage & Metrics")).not.toBeInTheDocument();
  });

  it("should construct correct links with projectId", () => {
    render(<SideBar collapsed={false} />);

    const dashboardLink = screen.getByRole("link", { name: /dashboard/i });
    expect(dashboardLink).toHaveAttribute(
      "href",
      "/projects/project-1/dashboard",
    );
  });

  describe("activeItem computation", () => {
    it("should set activeItem to 'dashboard' when path is exactly /projects/{projectId}", () => {
      mockLocation.pathname = "/projects/project-1";
      render(<SideBar collapsed={false} />);
      const sidebar = screen.getByTestId("sidebar");
      expect(sidebar).toHaveAttribute("data-active-item", "dashboard");
    });

    it("should set activeItem to the segment after nested projectId", () => {
      mockLocation.pathname = "/projects/project-1/support/tickets";
      render(<SideBar collapsed={false} />);
      const sidebar = screen.getByTestId("sidebar");
      expect(sidebar).toHaveAttribute("data-active-item", "support");
    });

    it("should set activeItem to 'dashboard' when projectId is missing from path", () => {
      mockLocation.pathname = "/dashboard";
      mockParams.projectId = undefined;
      render(<SideBar collapsed={false} />);
      const sidebar = screen.getByTestId("sidebar");
      expect(sidebar).toHaveAttribute("data-active-item", "dashboard");
    });
  });
});
