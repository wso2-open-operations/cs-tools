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
import { MemoryRouter, Route, Routes } from "react-router";
import { describe, expect, it, vi, beforeEach } from "vitest";
import CustomerRoleGuard from "../CustomerRoleGuard";
import { useCustomerPermissions } from "@hooks/useCustomerPermissions";

vi.mock("@hooks/useCustomerPermissions");

describe("CustomerRoleGuard", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders loading state when permissions are loading", () => {
    vi.mocked(useCustomerPermissions).mockReturnValue({
      isLoading: true,
      isError: false,
      can: vi.fn(),
      hasAnyRole: vi.fn(),
    } as unknown as ReturnType<typeof useCustomerPermissions>);

    const { container } = render(
      <MemoryRouter>
        <CustomerRoleGuard module="security_admin" action="read">
          <div data-testid="protected-content">Secret Content</div>
        </CustomerRoleGuard>
      </MemoryRouter>,
    );

    expect(screen.queryByTestId("protected-content")).not.toBeInTheDocument();
    expect(container.querySelector(".MuiLinearProgress-root")).toBeInTheDocument();
  });

  it("renders error message when permission check errors", () => {
    vi.mocked(useCustomerPermissions).mockReturnValue({
      isLoading: false,
      isError: true,
      can: vi.fn(),
      hasAnyRole: vi.fn(),
    } as unknown as ReturnType<typeof useCustomerPermissions>);

    render(
      <MemoryRouter>
        <CustomerRoleGuard module="security_admin" action="read">
          <div data-testid="protected-content">Secret Content</div>
        </CustomerRoleGuard>
      </MemoryRouter>,
    );

    expect(screen.queryByTestId("protected-content")).not.toBeInTheDocument();
    expect(
      screen.getByText(/Unable to verify access permissions/i),
    ).toBeInTheDocument();
  });

  it("renders 403 Forbidden page when permission is denied", () => {
    vi.mocked(useCustomerPermissions).mockReturnValue({
      isLoading: false,
      isError: false,
      can: vi.fn().mockReturnValue(false),
      hasAnyRole: vi.fn().mockReturnValue(false),
    } as unknown as ReturnType<typeof useCustomerPermissions>);

    render(
      <MemoryRouter>
        <CustomerRoleGuard module="security_admin" action="read">
          <div data-testid="protected-content">Secret Content</div>
        </CustomerRoleGuard>
      </MemoryRouter>,
    );

    expect(screen.queryByTestId("protected-content")).not.toBeInTheDocument();
    expect(
      screen.getByText(/You don't have permission to access this page/i),
    ).toBeInTheDocument();
  });

  it("renders children when permission is granted", () => {
    vi.mocked(useCustomerPermissions).mockReturnValue({
      isLoading: false,
      isError: false,
      can: vi.fn().mockReturnValue(true),
      hasAnyRole: vi.fn().mockReturnValue(true),
    } as unknown as ReturnType<typeof useCustomerPermissions>);

    render(
      <MemoryRouter>
        <CustomerRoleGuard module="security_admin" action="read">
          <div data-testid="protected-content">Secret Content</div>
        </CustomerRoleGuard>
      </MemoryRouter>,
    );

    expect(screen.getByTestId("protected-content")).toBeInTheDocument();
  });

  it("blocks Operations index route when permission is denied while allowing unprotected child routes", () => {
    vi.mocked(useCustomerPermissions).mockReturnValue({
      isLoading: false,
      isError: false,
      can: vi.fn((module: string) => module !== "change_requests"),
      hasAnyRole: vi.fn().mockReturnValue(true),
    } as unknown as ReturnType<typeof useCustomerPermissions>);

    const { unmount } = render(
      <MemoryRouter initialEntries={["/projects/1/operations"]}>
        <Routes>
          <Route path="/projects/:projectId/operations">
            <Route
              index
              element={
                <CustomerRoleGuard module="change_requests" action="read">
                  <div data-testid="operations-page">Operations Page</div>
                </CustomerRoleGuard>
              }
            />
            <Route
              path="service-requests"
              element={
                <div data-testid="service-requests-page">
                  Service Requests Page
                </div>
              }
            />
          </Route>
        </Routes>
      </MemoryRouter>,
    );

    expect(screen.queryByTestId("operations-page")).not.toBeInTheDocument();
    expect(
      screen.getByText(/You don't have permission to access this page/i),
    ).toBeInTheDocument();

    unmount();

    render(
      <MemoryRouter initialEntries={["/projects/1/operations/service-requests"]}>
        <Routes>
          <Route path="/projects/:projectId/operations">
            <Route
              index
              element={
                <CustomerRoleGuard module="change_requests" action="read">
                  <div data-testid="operations-page">Operations Page</div>
                </CustomerRoleGuard>
              }
            />
            <Route
              path="service-requests"
              element={
                <div data-testid="service-requests-page">
                  Service Requests Page
                </div>
              }
            />
          </Route>
        </Routes>
      </MemoryRouter>,
    );

    expect(screen.getByTestId("service-requests-page")).toBeInTheDocument();
  });
});
