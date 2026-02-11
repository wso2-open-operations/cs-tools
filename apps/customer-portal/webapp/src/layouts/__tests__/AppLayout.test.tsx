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
import { MemoryRouter } from "react-router";
import AppLayout from "@layouts/AppLayout";
import { LoaderProvider } from "@context/linear-loader/LoaderContext";

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

vi.mock("@wso2/oxygen-ui", async (importOriginal) => {
  const actual = (await importOriginal()) as any;
  return {
    ...actual,
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
    NotificationBanner: ({ visible, title, message }: any) =>
      visible ? (
        <div data-testid="notification-banner">
          <h3>{title}</h3>
          <p>{message}</p>
        </div>
      ) : null,
    LinearProgress: () => <div data-testid="linear-progress" />,
  };
});

// Mock useLoader
const mockUseLoader = {
  isVisible: false,
  showLoader: vi.fn(),
  hideLoader: vi.fn(),
};

vi.mock("../../context/linear-loader/LoaderContext", async (importOriginal) => {
  const actual = (await importOriginal()) as any;
  return {
    ...actual,
    useLoader: () => mockUseLoader,
  };
});

// Mock react-router
const mockLocation = { pathname: "/dashboard" };
vi.mock("react-router", async (importOriginal) => {
  const actual = (await importOriginal()) as any;
  return {
    ...actual,
    useLocation: () => mockLocation,
    useParams: () => ({ projectId: "project-1" }),
    Outlet: () => <div data-testid="outlet" />,
  };
});

// Mock child components
vi.mock("@components/common/footer/Footer", async () => ({
  default: () => <div data-testid="footer" />,
}));

vi.mock("@components/common/header/Header", async () => ({
  default: ({ onToggleSidebar }: { onToggleSidebar: () => void }) => (
    <div data-testid="header">
      <button onClick={onToggleSidebar}>Toggle</button>
    </div>
  ),
}));

vi.mock(
  "@components/common/notification-banner/GlobalNotificationBanner",
  async () => ({
    default: () => <div data-testid="global-notification-banner" />,
  }),
);

vi.mock("@components/common/side-nav-bar/SideBar", async () => ({
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
    render(
      <LoaderProvider>
        <MemoryRouter>
          <AppLayout />
        </MemoryRouter>
      </LoaderProvider>,
    );

    expect(screen.getByTestId("header")).toBeInTheDocument();
    expect(screen.getByTestId("sidebar")).toBeInTheDocument();
    expect(screen.getByTestId("app-main")).toBeInTheDocument();
    expect(screen.getByTestId("outlet")).toBeInTheDocument();
    expect(screen.getByTestId("footer")).toBeInTheDocument();
  });

  it("should NOT render Sidebar on the project hub (landing page)", () => {
    mockLocation.pathname = "/";
    render(
      <LoaderProvider>
        <MemoryRouter>
          <AppLayout />
        </MemoryRouter>
      </LoaderProvider>,
    );

    expect(screen.getByTestId("header")).toBeInTheDocument();
    expect(screen.queryByTestId("sidebar")).toBeNull();
  });

  it("should render global loader when isVisible is true", () => {
    mockUseLoader.isVisible = true;
    render(
      <LoaderProvider>
        <MemoryRouter>
          <AppLayout />
        </MemoryRouter>
      </LoaderProvider>,
    );

    expect(screen.getByTestId("linear-progress")).toBeInTheDocument();
  });

  it("should NOT render global loader when isVisible is false", () => {
    mockUseLoader.isVisible = false;
    render(
      <LoaderProvider>
        <MemoryRouter>
          <AppLayout />
        </MemoryRouter>
      </LoaderProvider>,
    );

    expect(screen.queryByTestId("linear-progress")).toBeNull();
  });

  it("should call toggleSidebar when header toggle is clicked", () => {
    render(
      <LoaderProvider>
        <MemoryRouter>
          <AppLayout />
        </MemoryRouter>
      </LoaderProvider>,
    );

    const toggleButton = screen.getByText("Toggle");
    fireEvent.click(toggleButton);

    expect(mockShellActions.toggleSidebar).toHaveBeenCalled();
  });

  it("should pass shell actions to SideBar", () => {
    render(
      <LoaderProvider>
        <MemoryRouter>
          <AppLayout />
        </MemoryRouter>
      </LoaderProvider>,
    );

    fireEvent.click(screen.getByText("Select"));
    expect(mockShellActions.setActiveMenuItem).toHaveBeenCalledWith("item-1");

    fireEvent.click(screen.getByText("Toggle Menu"));
    expect(mockShellActions.toggleMenu).toHaveBeenCalledWith("menu-1");
  });

  it("should pass shell state to SideBar", () => {
    mockShellState.sidebarCollapsed = true;
    render(
      <LoaderProvider>
        <MemoryRouter>
          <AppLayout />
        </MemoryRouter>
      </LoaderProvider>,
    );

    expect(screen.getByText("Collapsed: true")).toBeInTheDocument();
  });
});
