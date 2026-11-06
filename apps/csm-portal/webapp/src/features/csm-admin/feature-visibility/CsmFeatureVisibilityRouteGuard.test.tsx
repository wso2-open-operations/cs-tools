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
import { MemoryRouter, Route, Routes } from "react-router";
import "@testing-library/jest-dom/vitest";
import CsmFeatureVisibilityRouteGuard from "@features/csm-admin/feature-visibility/CsmFeatureVisibilityRouteGuard";

const user = vi.hoisted(() => ({ roles: ["admin"] as string[], isLoading: false }));
vi.mock("@context/current-user/CurrentUserContext", () => ({
  useCurrentUser: () => ({
    user: user.isLoading ? undefined : { id: "user-1", roles: user.roles },
    isLoading: user.isLoading,
    isError: false,
    error: null,
  }),
}));
vi.mock("@components/error/Error403Page", () => ({ default: () => <div>forbidden page</div> }));

function renderGuard() {
  return render(
    <MemoryRouter initialEntries={["/admin/feature-visibility"]}>
      <Routes>
        <Route path="/admin/feature-visibility" element={<CsmFeatureVisibilityRouteGuard />}>
          <Route index element={<div>feature visibility page</div>} />
        </Route>
      </Routes>
    </MemoryRouter>,
  );
}

beforeEach(() => {
  user.roles = ["admin"];
  user.isLoading = false;
});

describe("CsmFeatureVisibilityRouteGuard", () => {
  it("lets an admin in", () => {
    renderGuard();
    expect(screen.getByText("feature visibility page")).toBeInTheDocument();
  });

  it("turns away everyone who is not an admin", () => {
    for (const roles of [["cs_engineer"], ["viewer"], ["dashboard_designer"]]) {
      user.roles = roles;
      const { unmount } = renderGuard();
      expect(screen.getByText("forbidden page")).toBeInTheDocument();
      expect(screen.queryByText("feature visibility page")).not.toBeInTheDocument();
      unmount();
    }
  });

  it("shows neither page while the user is still loading", () => {
    user.isLoading = true;
    renderGuard();
    expect(screen.queryByText("forbidden page")).not.toBeInTheDocument();
    expect(screen.queryByText("feature visibility page")).not.toBeInTheDocument();
  });
});
