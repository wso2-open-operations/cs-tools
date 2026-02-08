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
import { useAsgardeo } from "@asgardeo/react";
import { MemoryRouter, Route, Routes } from "react-router";
import AuthGuard from "@AuthGuard";
import { describe, it, expect, vi, beforeEach } from "vitest";
import React from "react";
import { useLoader } from "@context/linear-loader/LoaderContext";

// Mock dependencies
vi.mock("@asgardeo/react", () => ({
  useAsgardeo: vi.fn(),
  AsgardeoProvider: ({ children }: { children: React.ReactNode }) => (
    <>{children}</>
  ),
}));

vi.mock("@context/linear-loader/LoaderContext", () => ({
  useLoader: vi.fn(),
}));

describe("AuthGuard", () => {
  const mockShowLoader = vi.fn();
  const mockHideLoader = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    (useLoader as any).mockReturnValue({
      showLoader: mockShowLoader,
      hideLoader: mockHideLoader,
    });
  });

  it("shows loader and renders app layout when authentication is loading", () => {
    (useAsgardeo as any).mockReturnValue({
      isLoading: true,
      isSignedIn: false,
    });

    render(
      <MemoryRouter initialEntries={["/"]}>
        <Routes>
          <Route element={<AuthGuard />}>
            <Route path="/" element={<div data-testid="app-layout" />} />
          </Route>
        </Routes>
      </MemoryRouter>,
    );

    expect(screen.getByTestId("app-layout")).toBeInTheDocument();
    expect(mockShowLoader).toHaveBeenCalled();
  });

  it("hides loader and redirects to login when user is not signed in and not loading", () => {
    (useAsgardeo as any).mockReturnValue({
      isLoading: false,
      isSignedIn: false,
    });

    render(
      <MemoryRouter initialEntries={["/protected"]}>
        <Routes>
          <Route path="/login" element={<div>Login Page</div>} />
          <Route element={<AuthGuard />}>
            <Route path="/protected" element={<div>Protected Content</div>} />
          </Route>
        </Routes>
      </MemoryRouter>,
    );

    expect(mockHideLoader).toHaveBeenCalled();
    expect(screen.getByText("Login Page")).toBeInTheDocument();
  });

  it("hides loader and renders protected content when user is signed in", () => {
    (useAsgardeo as any).mockReturnValue({
      isLoading: false,
      isSignedIn: true,
    });

    render(
      <MemoryRouter initialEntries={["/protected"]}>
        <Routes>
          <Route element={<AuthGuard />}>
            <Route path="/protected" element={<div>Protected Content</div>} />
          </Route>
        </Routes>
      </MemoryRouter>,
    );

    expect(mockHideLoader).toHaveBeenCalled();
    expect(screen.getByText("Protected Content")).toBeInTheDocument();
  });
});
