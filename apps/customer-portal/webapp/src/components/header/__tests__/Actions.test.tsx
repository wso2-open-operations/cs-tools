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
import { MemoryRouter } from "react-router";
import Actions from "@components/header/Actions";

// Mock @wso2/oxygen-ui
vi.mock("@wso2/oxygen-ui", () => ({
  Button: ({
    children,
    href,
    target,
    rel,
  }: {
    children: any;
    href: string;
    target?: string;
    rel?: string;
  }) => (
    <a href={href} target={target} rel={rel}>
      {children}
    </a>
  ),
  ColorSchemeToggle: () => (
    <button data-testid="theme-toggle">Toggle Theme</button>
  ),
  Divider: () => <hr />,
  Header: {
    Actions: ({ children }: { children: any }) => <div>{children}</div>,
  },
}));

// Mock icons
vi.mock("@wso2/oxygen-ui-icons-react", async (importOriginal) => {
  const actual =
    await importOriginal<typeof import("@wso2/oxygen-ui-icons-react")>();
  const mockIcon = (name: string) => () => <svg data-testid={`icon-${name}`} />;
  return {
    ...actual,
    Briefcase: mockIcon("Briefcase"),
    FileText: mockIcon("FileText"),
    FolderOpen: mockIcon("FolderOpen"),
    Headset: mockIcon("Headset"),
    Home: mockIcon("Home"),
    Megaphone: mockIcon("Megaphone"),
    RefreshCw: mockIcon("RefreshCw"),
    Shield: mockIcon("Shield"),
    Users: () => <svg data-testid="icon-Users" />,
    Settings: mockIcon("Settings"),
  };
});

// Actions reads the error page state; the provider is only mounted at the app root.
vi.mock("@context/error-page/ErrorPageContext", () => ({
  useErrorPageContext: () => ({ isProjectSuspended: false }),
}));

// GetHelpDropdown has its own tests (needs logger/query providers).
vi.mock("@components/header/GetHelpDropdown", () => ({
  default: () => <div data-testid="get-help" />,
}));

// Mock UserProfile
vi.mock("../UserProfile", () => ({
  default: () => <div data-testid="user-profile" />,
}));

describe("Actions", () => {
  it("should render theme toggle and user profile", () => {
    render(
      <MemoryRouter initialEntries={["/project-1/dashboard"]}>
        <Actions />
      </MemoryRouter>,
    );

    expect(screen.getByTestId("theme-toggle")).toBeInTheDocument();
    expect(screen.getByTestId("user-profile")).toBeInTheDocument();
    expect(screen.getByTestId("get-help")).toBeInTheDocument();
  });

  it("should hide Get Help on the project hub", () => {
    render(
      <MemoryRouter initialEntries={["/"]}>
        <Actions />
      </MemoryRouter>,
    );

    expect(screen.queryByTestId("get-help")).not.toBeInTheDocument();
    expect(screen.getByTestId("theme-toggle")).toBeInTheDocument();
  });
});
