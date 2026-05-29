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
import { MemoryRouter } from "react-router";
import { describe, expect, it, vi } from "vitest";
import AuthGuard from "@layouts/AuthGuard";

vi.mock("@asgardeo/react-router", () => ({
  ProtectedRoute: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="protected-route">{children}</div>
  ),
}));

vi.mock("@layouts/AppLayout", () => ({
  default: () => <div data-testid="app-layout">App layout</div>,
}));

vi.mock("@features/settings/utils/settingsStorage", () => ({
  getLastSelectedProjectId: () => null,
}));

describe("AuthGuard", () => {
  it("renders protected app layout", () => {
    render(
      <MemoryRouter>
        <AuthGuard />
      </MemoryRouter>,
    );
    expect(screen.getByTestId("protected-route")).toBeInTheDocument();
    expect(screen.getByTestId("app-layout")).toBeInTheDocument();
  });
});
