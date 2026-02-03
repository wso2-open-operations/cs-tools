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
import UserProfile from "@/components/header/UserProfile";
import { mockUser } from "@/models/mockData";

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

// Mock useLogger
vi.mock("@/hooks/useLogger", () => ({
  useLogger: () => ({
    debug: vi.fn(),
    info: vi.fn(),
    warn: vi.fn(),
    error: vi.fn(),
  }),
}));

describe("UserProfile", () => {
  it("should render the user name and email via UserMenu", () => {
    render(<UserProfile />);

    expect(screen.getByText(mockUser.name)).toBeInTheDocument();
    expect(screen.getByText(mockUser.email)).toBeInTheDocument();
  });

  it("should have the user menu container", () => {
    render(<UserProfile />);
    expect(screen.getByTestId("user-menu")).toBeInTheDocument();
  });
});
