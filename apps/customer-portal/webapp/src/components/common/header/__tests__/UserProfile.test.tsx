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
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import UserProfile from "@components/common/header/UserProfile";

const mockUserDetails = {
  id: "user-1",
  email: "john@example.com",
  lastName: "Doe",
  firstName: "John",
  timeZone: "UTC",
};

vi.mock("@components/common/header/UserProfileModal", () => ({
  default: () => null,
}));

// Mock UserMenu compound components from @wso2/oxygen-ui
vi.mock("@wso2/oxygen-ui", () => {
  function Trigger({ name }: { name: string }) {
    return (
      <button type="button" data-testid="user-menu-trigger" aria-label="Account">
        <span data-testid="trigger-name">{name.length === 0 ? "" : name}</span>
      </button>
    );
  }
  function Header({ name, email }: { name: string; email: string }) {
    return (
      <div data-testid="user-menu-header">
        <span data-testid="header-name">{name}</span>
        <span data-testid="header-email">{email}</span>
      </div>
    );
  }
  function UserMenuRoot({ children }: { children: ReactNode }) {
    return <div data-testid="user-menu">{children}</div>;
  }
  const UserMenu = Object.assign(UserMenuRoot, {
    Trigger,
    Header,
    Divider: () => <hr data-testid="user-menu-divider" />,
    Item: () => null,
    Logout: () => null,
  });
  return {
    UserMenu,
    colors: {
      blue: { 700: "#1D4ED8" },
      purple: { 400: "#A78BFA" },
    },
  };
});

vi.mock("@wso2/oxygen-ui-icons-react", () => ({
  User: () => <svg data-testid="user-icon" />,
  LogOut: () => <svg data-testid="logout-icon" />,
}));

// Mock react-router
vi.mock("react-router", () => ({
  useNavigate: () => vi.fn(),
}));

// Mock useAsgardeo
const mockUseAsgardeo = vi.fn();
vi.mock("@asgardeo/react", () => ({
  useAsgardeo: () => mockUseAsgardeo(),
}));

// Mock useGetUserDetails and generic logger
const mockUseGetUserDetails = vi.fn();
vi.mock("@api/useGetUserDetails", () => ({
  default: () => mockUseGetUserDetails(),
}));

vi.mock("@/hooks/useLogger", () => ({
  useLogger: () => ({
    debug: vi.fn(),
  }),
}));

describe("UserProfile", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseAsgardeo.mockReturnValue({ isSignedIn: true, isLoading: false });
    mockUseGetUserDetails.mockReturnValue({
      data: mockUserDetails,
      isLoading: false,
      isError: false,
    });
  });

  it("should render the user name and email via UserMenu when authenticated", () => {
    render(<UserProfile />);

    const expectedName = `${mockUserDetails.firstName} ${mockUserDetails.lastName}`;
    expect(screen.getByTestId("trigger-name")).toHaveTextContent(expectedName);
    expect(screen.getByTestId("header-name")).toHaveTextContent(expectedName);
    expect(screen.getByTestId("header-email")).toHaveTextContent(
      mockUserDetails.email,
    );
  });

  it("should NOT render if not signed in and not loading", () => {
    mockUseAsgardeo.mockReturnValue({ isSignedIn: false, isLoading: false });

    const { container } = render(<UserProfile />);
    expect(container).toBeEmptyDOMElement();
  });

  it("should keep the account menu trigger available while auth is loading", () => {
    mockUseAsgardeo.mockReturnValue({ isSignedIn: true, isLoading: true });
    render(<UserProfile />);

    expect(screen.getByTestId("user-menu-trigger")).toBeInTheDocument();
    expect(screen.getByTestId("header-name")).toHaveTextContent("Loading…");
  });

  it("should keep the account menu trigger available while profile (useGetUserDetails) is loading", () => {
    mockUseGetUserDetails.mockReturnValue({
      data: undefined,
      isLoading: true,
      isError: false,
    });
    render(<UserProfile />);

    expect(screen.getByTestId("user-menu-trigger")).toBeInTheDocument();
    expect(screen.getByTestId("header-name")).toHaveTextContent("Loading…");
  });

  it("should show cached name and email when isError but userDetails data exists", () => {
    mockUseGetUserDetails.mockReturnValue({
      data: mockUserDetails,
      isLoading: false,
      isError: true,
      error: new Error("stale"),
    });
    render(<UserProfile />);

    const expectedName = `${mockUserDetails.firstName} ${mockUserDetails.lastName}`;
    expect(screen.getByTestId("header-name")).toHaveTextContent(expectedName);
    expect(screen.getByTestId("header-email")).toHaveTextContent(
      mockUserDetails.email,
    );
  });

  it("should show Unknown User in the menu header when user details fail to load", () => {
    mockUseGetUserDetails.mockReturnValue({
      data: null,
      isLoading: false,
      isError: true,
      error: new Error("Failed"),
    });

    render(<UserProfile />);
    expect(screen.getByTestId("header-name")).toHaveTextContent("Unknown User");
    expect(screen.getByTestId("header-email")).toHaveTextContent("--");
  });
});
