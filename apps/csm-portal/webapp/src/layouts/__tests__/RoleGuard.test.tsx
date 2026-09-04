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

import "@testing-library/jest-dom/vitest";
import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import RoleGuard from "@layouts/RoleGuard";
import { usePermissions } from "@hooks/usePermissions";

vi.mock("@hooks/usePermissions", () => ({
  usePermissions: vi.fn(),
}));

vi.mock("@components/error/Error403Page", () => ({
  default: ({ message }: { message?: string }) => (
    <div data-testid="error-403">{message ?? "Forbidden"}</div>
  ),
}));

vi.mock("@components/route-fallback/RouteSuspenseFallback", () => ({
  default: () => <div data-testid="route-fallback">Loading...</div>,
}));

describe("RoleGuard", () => {
  it("renders loading fallback while permissions are loading", () => {
    vi.mocked(usePermissions).mockReturnValue({
      roles: [],
      isAdmin: false,
      isAgent: false,
      isTimecardApprover: false,
      isLoading: true,
      hasRole: vi.fn().mockReturnValue(false),
      hasAnyRole: vi.fn().mockReturnValue(false),
    });

    render(
      <RoleGuard allowedRoles={["admin"]}>
        <div data-testid="protected-content">Secret Content</div>
      </RoleGuard>,
    );

    expect(screen.getByTestId("route-fallback")).toBeInTheDocument();
    expect(screen.queryByTestId("protected-content")).not.toBeInTheDocument();
    expect(screen.queryByTestId("error-403")).not.toBeInTheDocument();
  });

  it("renders 403 page when user lacks the required role", () => {
    vi.mocked(usePermissions).mockReturnValue({
      roles: ["agent"],
      isAdmin: false,
      isAgent: true,
      isTimecardApprover: false,
      isLoading: false,
      hasRole: vi.fn((role) => role === "agent"),
      hasAnyRole: vi.fn((...roles) => roles.includes("agent")),
    });

    render(
      <RoleGuard allowedRoles={["admin"]}>
        <div data-testid="protected-content">Secret Content</div>
      </RoleGuard>,
    );

    expect(screen.getByTestId("error-403")).toBeInTheDocument();
    expect(screen.queryByTestId("protected-content")).not.toBeInTheDocument();
  });

  it("renders child content when user has the required role", () => {
    vi.mocked(usePermissions).mockReturnValue({
      roles: ["admin"],
      isAdmin: true,
      isAgent: false,
      isTimecardApprover: false,
      isLoading: false,
      hasRole: vi.fn((role) => role === "admin"),
      hasAnyRole: vi.fn((...roles) => roles.includes("admin")),
    });

    render(
      <RoleGuard allowedRoles={["admin"]}>
        <div data-testid="protected-content">Secret Content</div>
      </RoleGuard>,
    );

    expect(screen.getByTestId("protected-content")).toBeInTheDocument();
    expect(screen.queryByTestId("error-403")).not.toBeInTheDocument();
  });
});
