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
import Actions from "@/components/header/Actions";

// Mock @wso2/oxygen-ui
vi.mock("@wso2/oxygen-ui", () => ({
  Button: ({ children, href }: { children: any; href: string }) => (
    <a href={href}>{children}</a>
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
    Users: () => <svg data-testid="icon-Users" />,
    Settings: mockIcon("Settings"),
  };
});

// Mock UserProfile
vi.mock("../UserProfile", () => ({
  default: () => <div data-testid="user-profile" />,
}));

describe("Actions", () => {
  it("should render join community button, theme toggle, and user profile", () => {
    render(<Actions />);

    expect(screen.getByText("Join our community")).toBeInTheDocument();
    expect(screen.getByTestId("theme-toggle")).toBeInTheDocument();
    expect(screen.getByTestId("user-profile")).toBeInTheDocument();
  });
});
