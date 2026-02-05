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
import UserProfile from "@/components/common/header/UserProfile";
import { mockUserDetails } from "@/models/mockData";

// Mock UserMenu and Skeleton from @wso2/oxygen-ui
vi.mock("@wso2/oxygen-ui", () => ({
  UserMenu: ({ user }: { user: any }) => (
    <div data-testid="user-menu">
      <span data-testid="user-name">{user.name}</span>
      <span data-testid="user-email">{user.email}</span>
    </div>
  ),
  Skeleton: () => <div data-testid="skeleton" />,
  colors: {
    blue: { 700: "#1d4ed8" },
    purple: { 400: "#a78bfa" },
  },
}));

// Mock icons
vi.mock("@wso2/oxygen-ui-icons-react", () => ({
  User: () => <svg data-testid="user-icon" />,
  Shield: () => <svg data-testid="shield-icon" />,
  Info: () => <svg data-testid="info-icon" />,
  Clock: () => <svg data-testid="clock-icon" />,
  Server: () => <svg data-testid="server-icon" />,
  CircleAlert: () => <svg data-testid="circle-alert-icon" />,
  Rocket: () => <svg data-testid="rocket-icon" />,
}));

// Mock ErrorIndicator
vi.mock("@/components/common/errorIndicator/ErrorIndicator", () => ({
  default: ({ entityName }: any) => (
    <div data-testid="error-indicator">Error: {entityName}</div>
  ),
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

// Mock useMockConfig
const mockUseMockConfig = vi.fn();
vi.mock("@/providers/MockConfigProvider", () => ({
  useMockConfig: () => mockUseMockConfig(),
}));

// Mock useGetUserDetails and generic logger
const mockUseGetUserDetails = vi.fn();
vi.mock("@/api/useGetUserDetails", () => ({
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
    // Default mocks
    mockUseAsgardeo.mockReturnValue({ isSignedIn: true, isLoading: false });
    mockUseMockConfig.mockReturnValue({ isMockEnabled: false });
    mockUseGetUserDetails.mockReturnValue({
      data: mockUserDetails,
      isLoading: false,
      isError: false,
    });
  });

  it("should render the user name and email via UserMenu when authenticated", () => {
    render(<UserProfile />);

    const expectedName = `${mockUserDetails.firstName} ${mockUserDetails.lastName}`;
    expect(screen.getByTestId("user-name")).toHaveTextContent(expectedName);
    expect(screen.getByTestId("user-email")).toHaveTextContent(
      mockUserDetails.email,
    );
  });

  it("should NOT render if not signed in, not loading, and mocks disabled", () => {
    mockUseAsgardeo.mockReturnValue({ isSignedIn: false, isLoading: false });
    mockUseMockConfig.mockReturnValue({ isMockEnabled: false });

    const { container } = render(<UserProfile />);
    expect(container).toBeEmptyDOMElement();
  });

  it("should render if mocks are enabled even if not signed in", () => {
    mockUseAsgardeo.mockReturnValue({ isSignedIn: false, isLoading: false });
    mockUseMockConfig.mockReturnValue({ isMockEnabled: true });
    render(<UserProfile />);
    expect(screen.getByTestId("user-menu")).toBeInTheDocument();
  });

  it("should render skeletons when loading", () => {
    mockUseAsgardeo.mockReturnValue({ isSignedIn: true, isLoading: true });
    render(<UserProfile />);

    // Two skeletons expected: name and email
    expect(screen.getAllByTestId("skeleton")).toHaveLength(2);
  });

  it("should render error indicator when error occurs", () => {
    mockUseGetUserDetails.mockReturnValue({
      data: null,
      isLoading: false,
      isError: true,
      error: new Error("Failed"),
    });

    render(<UserProfile />);
    expect(screen.getByTestId("error-indicator")).toHaveTextContent(
      "Error: user",
    );
  });
});
