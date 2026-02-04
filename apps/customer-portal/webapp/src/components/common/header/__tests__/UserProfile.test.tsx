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

// Mock @wso2/oxygen-ui
vi.mock("@wso2/oxygen-ui", () => ({
  UserMenu: ({ user }: { user: any }) => (
    <div data-testid="user-menu">
      <span>{user.name}</span>
      <span>{user.email}</span>
    </div>
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

// Mock useGetUserDetails
const mockUseGetUserDetails = vi.fn();
vi.mock("@/api/useGetUserDetails", () => ({
  default: () => mockUseGetUserDetails(),
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

    expect(screen.getByText(mockUserDetails.firstName)).toBeInTheDocument();
    expect(screen.getByText(mockUserDetails.lastName)).toBeInTheDocument();
    expect(screen.getByText(mockUserDetails.email)).toBeInTheDocument();
    expect(screen.getByText(mockUserDetails.timeZone)).toBeInTheDocument();
    expect(screen.getByText(mockUserDetails.id)).toBeInTheDocument();
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

  it("should render error user if fetching fails", () => {
    mockUseGetUserDetails.mockReturnValue({
      data: null,
      isLoading: false,
      isError: true,
      error: new Error("Failed"),
    });

    render(<UserProfile />);
    expect(screen.getByTestId("user-menu")).toBeInTheDocument();
  });
});
