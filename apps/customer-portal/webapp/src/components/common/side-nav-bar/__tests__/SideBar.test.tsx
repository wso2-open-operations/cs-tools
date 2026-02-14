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
import SideBar from "@components/common/side-nav-bar/SideBar";
import { APP_SHELL_NAV_ITEMS } from "@constants/appLayoutConstants";

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
    Link: ({ children, to }: { children: any; to: string }) => (
      <a href={to}>{children}</a>
    ),
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
    Briefcase: mockIcon("Briefcase"),
    FileText: mockIcon("FileText"),
    FolderOpen: mockIcon("FolderOpen"),
    Headset: mockIcon("Headset"),
    Home: mockIcon("Home"),
    Megaphone: mockIcon("Megaphone"),
    RefreshCw: mockIcon("RefreshCw"),
    Shield: mockIcon("Shield"),
    Users: mockIcon("Users"),
    Settings: mockIcon("Settings"),
    Crown: mockIcon("Crown"),
    Info: mockIcon("Info"),
    Server: mockIcon("Server"),
    Clock: mockIcon("Clock"),
  };
});

// Mock react-router
const mockLocation = { pathname: "/project-1/dashboard" };
const mockParams: { projectId: string | undefined } = {
  projectId: "project-1",
};

vi.mock("react-router", () => ({
  useLocation: () => mockLocation,
  useParams: () => mockParams,
  Link: ({ children, to }: { children: any; to: string }) => (
    <a href={to}>{children}</a>
  ),
}));

// Mock SubscriptionWidget (path must match: @components/common/side-nav-bar/SubscriptionWidget)
vi.mock("@components/common/side-nav-bar/SubscriptionWidget", () => ({
  default: () => <div data-testid="subscription-widget" />,
}));

describe("SideBar", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockLocation.pathname = "/project-1/dashboard";
    mockParams.projectId = "project-1";
  });

  it("should render all navigation items from APP_SHELL_NAV_ITEMS", () => {
    render(<SideBar collapsed={false} />);

    APP_SHELL_NAV_ITEMS.forEach((item) => {
      expect(screen.getByText(item.label)).toBeInTheDocument();
    });
  });

  it("should render the subscription widget", () => {
    render(<SideBar collapsed={false} />);
    expect(screen.getByTestId("subscription-widget")).toBeInTheDocument();
  });

  it("should render the settings item", () => {
    render(<SideBar collapsed={false} />);
    expect(screen.getByText("Settings")).toBeInTheDocument();
    expect(screen.getByTestId("icon-Settings")).toBeInTheDocument();
  });

  it("should construct correct links with projectId", () => {
    render(<SideBar collapsed={false} />);

    const dashboardLink = screen.getByRole("link", { name: /dashboard/i });
    expect(dashboardLink).toHaveAttribute("href", "/project-1/dashboard");
  });

  describe("activeItem computation", () => {
    it("should set activeItem to 'dashboard' when path is exactly /{projectId}", () => {
      mockLocation.pathname = "/project-1";
      render(<SideBar collapsed={false} />);
      const sidebar = screen.getByTestId("sidebar");
      expect(sidebar).toHaveAttribute("data-active-item", "dashboard");
    });

    it("should set activeItem to the segment after nested projectId", () => {
      mockLocation.pathname = "/project-1/support/tickets";
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
