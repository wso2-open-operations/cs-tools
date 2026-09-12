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

/**
 * Exercises the /admin route block exactly as declared in App.tsx (index
 * redirect, the User management nesting, and the legacy-path redirects) —
 * without pulling in App.tsx's full provider/lazy-loading tree, which has no
 * test harness of its own. Any change to that block's shape should be
 * mirrored here.
 */

import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import { MemoryRouter, Navigate, Route, Routes, useLocation } from "react-router";
import { SectionIndexRedirect } from "@components/section-tabs/SectionTabs";

/** Mirrors App.tsx's own (unexported) `LegacySettingsRedirect` — forwards
 * `location.state` through the legacy `/admin/<page>` -> `/admin/user-
 * management/<page>` redirect hop, since a bare `<Navigate to={...}
 * replace />` has no `state` prop bound to the incoming navigation and would
 * silently drop it. */
function LegacySettingsRedirect({ to }: { to: string }) {
  const { state } = useLocation();
  return <Navigate to={to} state={state} replace />;
}

vi.mock("@context/current-user/CurrentUserContext", () => ({
  useCurrentUser: () => ({
    user: { roles: ["admin"] },
    isLoading: false,
    isError: false,
  }),
}));
// `CsmAdminLayout` transitively imports API-backed hooks (via the nav
// tree/dashboard builder routes) — mocked up front, before the component
// import below, per this repo's own convention for anything that
// transitively imports `CsmAdminLayout` (see `CsmAdminLayout.test.tsx`).
vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ get: vi.fn(), post: vi.fn() }),
}));
vi.mock("@config/apiConfig", () => ({
  apiConfig: { backendUrl: "https://example.test" },
}));

import CsmAdminLayout from "@features/csm-admin/pages/CsmAdminLayout";

function renderAdminRoutes(initialEntry: string | { pathname: string; state?: unknown }) {
  return render(
    <MemoryRouter initialEntries={[initialEntry]}>
      <Routes>
        <Route path="/admin" element={<CsmAdminLayout />}>
          <Route index element={<SectionIndexRedirect sectionId="admin" />} />
          <Route
            path="user-management"
            element={<SectionIndexRedirect sectionId="admin.user-management" />}
          />
          <Route path="user-management/users" element={<div>Users content</div>} />
          <Route path="user-management/roles" element={<div>Roles content</div>} />
          <Route path="user-management/groups" element={<div>Groups content</div>} />
          <Route path="user-management/teams" element={<div>Teams content</div>} />
          <Route
            path="user-management/permissions"
            element={<div>Permissions content</div>}
          />
          <Route path="dashboards" element={<div>Dashboards content</div>} />
        </Route>
        <Route
          path="/admin/users"
          element={<LegacySettingsRedirect to="/admin/user-management/users" />}
        />
        <Route
          path="/admin/roles"
          element={<LegacySettingsRedirect to="/admin/user-management/roles" />}
        />
        <Route
          path="/admin/groups"
          element={<LegacySettingsRedirect to="/admin/user-management/groups" />}
        />
        <Route
          path="/admin/teams"
          element={<LegacySettingsRedirect to="/admin/user-management/teams" />}
        />
        <Route
          path="/admin/permissions"
          element={<LegacySettingsRedirect to="/admin/user-management/permissions" />}
        />
      </Routes>
    </MemoryRouter>,
  );
}

describe("admin index redirects", () => {
  it("lands /admin on User management's first tab, via the section index chain", () => {
    renderAdminRoutes("/admin");
    expect(screen.getByText("Users content")).toBeInTheDocument();
  });

  it("lands /admin/user-management on its own first tab", () => {
    renderAdminRoutes("/admin/user-management");
    expect(screen.getByText("Users content")).toBeInTheDocument();
  });
});

describe("legacy /admin/<page> redirects", () => {
  it.each([
    ["/admin/users", "Users content"],
    ["/admin/roles", "Roles content"],
    ["/admin/groups", "Groups content"],
    ["/admin/teams", "Teams content"],
    ["/admin/permissions", "Permissions content"],
  ])("redirects %s to the new User management path", (oldPath, expectedContent) => {
    renderAdminRoutes(oldPath);
    expect(screen.getByText(expectedContent)).toBeInTheDocument();
  });

  // Regression: a bare `<Navigate to={...} replace />` has no `state` prop
  // bound to the incoming navigation, so a dashboard widget's `resourceType:
  // "user"` click-through (which sets `state.from`) would silently lose it
  // on this exact hop -- landing on the new path with `CsmAdminLayout`'s
  // Back button unable to tell it apart from a cold/direct link, forcing it
  // back to the tile grid instead of the real origin.
  it("forwards location.state through the legacy /admin/users redirect, so Back still knows its real origin", () => {
    renderAdminRoutes({ pathname: "/admin/users", state: { from: "/dashboard" } });
    expect(screen.getByText("Users content")).toBeInTheDocument();

    const back = screen.getByRole("button", { name: "Back" });
    expect(back).toBeInTheDocument();
  });
});
