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

import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import AppShellLayout from "@layouts/AppShellLayout";

describe("AppShellLayout", () => {
  it("should render shell regions and main content", () => {
    render(
      <AppShellLayout
        header={<div data-testid="header-region">Header</div>}
        sidebar={<aside data-testid="sidebar-region">Sidebar</aside>}
        footer={<footer data-testid="footer-region">Footer</footer>}
      >
        <div data-testid="page-content">Page</div>
      </AppShellLayout>,
    );

    expect(screen.getByTestId("app-shell")).toBeInTheDocument();
    expect(screen.getByTestId("app-navbar")).toBeInTheDocument();
    expect(screen.getByTestId("app-sidebar")).toBeInTheDocument();
    expect(screen.getByTestId("app-main")).toBeInTheDocument();
    expect(screen.getByTestId("app-footer")).toBeInTheDocument();
    expect(screen.getByTestId("page-content")).toBeInTheDocument();
  });

  it("should render overlay drawer instead of inline sidebar when sidebarOverlay is true", () => {
    const onSidebarClose = vi.fn();

    render(
      <AppShellLayout
        header={<div>Header</div>}
        sidebar={<aside data-testid="sidebar-region">Sidebar</aside>}
        footer={<footer>Footer</footer>}
        sidebarOverlay
        sidebarOpen
        onSidebarClose={onSidebarClose}
      >
        <div>Page</div>
      </AppShellLayout>,
    );

    expect(screen.getByTestId("app-sidebar-drawer")).toBeInTheDocument();
    expect(screen.getByTestId("app-sidebar-backdrop")).toBeInTheDocument();
    expect(screen.getByTestId("sidebar-region")).toBeInTheDocument();

    fireEvent.click(screen.getByTestId("app-sidebar-backdrop"));
    expect(onSidebarClose).toHaveBeenCalledTimes(1);

    fireEvent.keyDown(window, { key: "Escape" });
    expect(onSidebarClose).toHaveBeenCalledTimes(2);
  });

  it("should omit sidebar slot when not provided", () => {
    render(
      <AppShellLayout header={<div>Header</div>} footer={<div>Footer</div>}>
        <div>Page</div>
      </AppShellLayout>,
    );

    expect(screen.queryByTestId("app-sidebar")).toBeNull();
    expect(screen.getByTestId("app-main")).toBeInTheDocument();
  });
});
