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
import AppLayout from "@/layouts/AppLayout";

// Mock @wso2/oxygen-ui
const mockShellActions = {
  toggleSidebar: vi.fn(),
  setActiveMenuItem: vi.fn(),
  toggleMenu: vi.fn(),
};

const mockShellState = {
  sidebarCollapsed: false,
  expandedMenus: {},
};

vi.mock("@wso2/oxygen-ui", () => ({
  AppShell: Object.assign(
    ({ children }: { children: any }) => (
      <div data-testid="app-shell">{children}</div>
    ),
    {
      Navbar: ({ children }: { children: any }) => (
        <nav data-testid="app-navbar">{children}</nav>
      ),
      Sidebar: ({ children }: { children: any }) => (
        <aside data-testid="app-sidebar">{children}</aside>
      ),
      Main: ({ children }: { children: any }) => (
        <main data-testid="app-main">{children}</main>
      ),
      Footer: ({ children }: { children: any }) => (
        <footer data-testid="app-footer">{children}</footer>
      ),
    },
  ),
  useAppShell: vi.fn(() => ({
    state: mockShellState,
    actions: mockShellActions,
  })),
  Box: ({ children }: { children: any }) => <div>{children}</div>,
  NotificationBanner: ({ visible, title, message }: any) =>
    visible ? (
      <div data-testid="notification-banner">
        <h3>{title}</h3>
        <p>{message}</p>
      </div>
    ) : null,
  LinearProgress: () => <div data-testid="linear-progress" />,
}));

// Mock useLoader
const mockUseLoader = {
  isVisible: false,
  showLoader: vi.fn(),
  hideLoader: vi.fn(),
};

vi.mock("@/context/linearLoader/LoaderContext", () => ({
  useLoader: () => mockUseLoader,
}));

// Mock react-router
const mockLocation = { pathname: "/dashboard" };
vi.mock("react-router", () => ({
  useLocation: () => mockLocation,
  Outlet: () => <div data-testid="outlet" />,
}));

// Mock child components
vi.mock("@/components/common/footer/Footer", () => ({
  default: () => <div data-testid="footer" />,
}));

vi.mock("@/components/common/header/Header", () => ({
  default: ({ onToggleSidebar }: { onToggleSidebar: () => void }) => (
    <div data-testid="header">
      <button onClick={onToggleSidebar}>Toggle</button>
    </div>
  ),
}));

vi.mock(
  "@/components/common/notificationBanner/GlobalNotificationBanner",
  () => ({
    default: () => <div data-testid="global-notification-banner" />,
  }),
);

vi.mock("@/components/common/sideNavBar/SideBar", () => ({
  default: ({ collapsed, onSelect, onToggleExpand }: any) => (
    <div data-testid="sidebar">
      <span>Collapsed: {collapsed.toString()}</span>
      <button onClick={() => onSelect("item-1")}>Select</button>
      <button onClick={() => onToggleExpand("menu-1")}>Toggle Menu</button>
    </div>
  ),
}));

describe("AppLayout", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockLocation.pathname = "/dashboard";
    mockShellState.sidebarCollapsed = false;
    mockShellState.expandedMenus = {};
    mockUseLoader.isVisible = false;
  });

  it("should render Header, Sidebar, Main, and Footer on a project page", () => {
    render(<AppLayout />);

    expect(screen.getByTestId("header")).toBeInTheDocument();
    expect(screen.getByTestId("sidebar")).toBeInTheDocument();
    expect(screen.getByTestId("app-main")).toBeInTheDocument();
    expect(screen.getByTestId("outlet")).toBeInTheDocument();
    expect(screen.getByTestId("footer")).toBeInTheDocument();
  });

  it("should NOT render Sidebar on the project hub (landing page)", () => {
    mockLocation.pathname = "/";
    render(<AppLayout />);

    expect(screen.getByTestId("header")).toBeInTheDocument();
    expect(screen.queryByTestId("sidebar")).toBeNull();
  });

  it("should render global loader when isVisible is true", () => {
    mockUseLoader.isVisible = true;
    render(<AppLayout />);

    expect(screen.getByTestId("linear-progress")).toBeInTheDocument();
  });

  it("should NOT render global loader when isVisible is false", () => {
    mockUseLoader.isVisible = false;
    render(<AppLayout />);

    expect(screen.queryByTestId("linear-progress")).toBeNull();
  });

  it("should call toggleSidebar when header toggle is clicked", () => {
    render(<AppLayout />);

    const toggleButton = screen.getByText("Toggle");
    toggleButton.click();

    expect(mockShellActions.toggleSidebar).toHaveBeenCalled();
  });

  it("should pass shell actions to SideBar", () => {
    render(<AppLayout />);

    screen.getByText("Select").click();
    expect(mockShellActions.setActiveMenuItem).toHaveBeenCalledWith("item-1");

    screen.getByText("Toggle Menu").click();
    expect(mockShellActions.toggleMenu).toHaveBeenCalledWith("menu-1");
  });

  it("should pass shell state to SideBar", () => {
    mockShellState.sidebarCollapsed = true;
    render(<AppLayout />);

    expect(screen.getByText("Collapsed: true")).toBeInTheDocument();
  });
});
