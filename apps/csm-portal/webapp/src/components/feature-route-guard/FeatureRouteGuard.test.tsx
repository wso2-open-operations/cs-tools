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

import { act, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { MemoryRouter, Route, Routes, useLocation } from "react-router";
import type { JSX } from "react";
import "@testing-library/jest-dom/vitest";
import { resetFeatureStatesForTests } from "@config/featureFlags";
import {
  resetSharedFeatureOverridesForTests,
  setSharedFeatureOverrides,
} from "@config/featureOverridesStore";

vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ get: vi.fn(), post: vi.fn() }),
}));
vi.mock("@config/apiConfig", () => ({
  apiConfig: { backendUrl: "https://example.test" },
}));
const queryState = vi.hoisted(() => ({ isPending: false }));
vi.mock("@hooks/useFeatureOverrides", () => ({
  useFeatureOverridesQuery: () => ({ isPending: queryState.isPending }),
}));
const currentRoles = vi.hoisted(() => ({ value: ["cs_engineer"] as string[] }));
vi.mock("@context/current-user/CurrentUserContext", () => ({
  useCurrentUser: () => ({
    user: { id: "user-1", roles: currentRoles.value },
    isLoading: false,
    isError: false,
    error: null,
  }),
}));

import FeatureRouteGuard from "@components/feature-route-guard/FeatureRouteGuard";

function Probe(): JSX.Element {
  const location = useLocation();
  return <div data-testid="location-probe">{location.pathname}</div>;
}

function renderAt(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route element={<FeatureRouteGuard />}>
          <Route path="/operations/problems" element={<div>problems page</div>} />
          <Route path="/operations/incidents" element={<div>incidents page</div>} />
          <Route path="/updates" element={<div>updates page</div>} />
          <Route path="/dashboard" element={<div>dashboard page</div>} />
          <Route
            path="/admin/user-management/permissions"
            element={<div>permissions own placeholder</div>}
          />
        </Route>
      </Routes>
      <Probe />
    </MemoryRouter>,
  );
}

function setConfig(value: unknown): void {
  window.config = {
    ...window.config,
    CSM_PORTAL_FEATURE_OVERRIDES: value,
  } as Window["config"];
  resetFeatureStatesForTests();
}

beforeEach(() => {
  queryState.isPending = false;
  vi.spyOn(console, "warn").mockImplementation(() => undefined);
  currentRoles.value = ["cs_engineer"];
  resetSharedFeatureOverridesForTests();
  setConfig(undefined);
  setSharedFeatureOverrides({
    rows: { operations: "preview", "operations.incidents": "enabled" },
    savedOnce: true,
  });
});

afterEach(() => {
  vi.restoreAllMocks();
  resetSharedFeatureOverridesForTests();
  setConfig(undefined);
});

describe("FeatureRouteGuard and a page in Preview", () => {
  it("shows everyone but an admin the coming-soon page, with the URL kept", () => {
    renderAt("/operations/problems");
    expect(screen.getByText(/is in preview and isn't open to everyone yet/)).toBeInTheDocument();
    expect(screen.queryByText("problems page")).not.toBeInTheDocument();
    expect(screen.getByTestId("location-probe")).toHaveTextContent("/operations/problems");
  });

  it("opens the page for an admin, under a Preview note", () => {
    currentRoles.value = ["admin"];
    renderAt("/operations/problems");
    expect(screen.getByText("problems page")).toBeInTheDocument();
    expect(screen.getByText(/This page isn't open to everyone yet/)).toBeInTheDocument();
  });

  it("opens an Available page for everyone with no note", () => {
    renderAt("/operations/incidents");
    expect(screen.getByText("incidents page")).toBeInTheDocument();
    expect(screen.queryByText(/isn't open to everyone yet/)).not.toBeInTheDocument();
  });

  it("adds no note to a page that was never restricted", () => {
    currentRoles.value = ["admin"];
    renderAt("/dashboard");
    expect(screen.getByText("dashboard page")).toBeInTheDocument();
    expect(screen.queryByText(/isn't open to everyone yet/)).not.toBeInTheDocument();
  });

  it("lets a page through that renders its own, more specific message", () => {
    setSharedFeatureOverrides({
      rows: { "admin.user-management.permissions": "preview" },
      savedOnce: true,
    });
    renderAt("/admin/user-management/permissions");
    expect(screen.getByText("permissions own placeholder")).toBeInTheDocument();
    expect(screen.queryByText(/is in preview and isn't open to everyone yet/)).not.toBeInTheDocument();
  });

  it("repaints when an admin's save reaches the shell", () => {
    renderAt("/operations/incidents");
    expect(screen.getByText("incidents page")).toBeInTheDocument();
    // Someone sets Incidents to Preview in Settings; this open page follows.
    act(() => {
      setSharedFeatureOverrides({
        rows: { operations: "preview", "operations.incidents": "preview" },
        savedOnce: true,
      });
    });
    expect(screen.getByText(/is in preview and isn't open to everyone yet/)).toBeInTheDocument();
  });
});

describe("FeatureRouteGuard and a page hidden by an older config", () => {
  it("still sends the link to the first open destination until someone saves", () => {
    resetSharedFeatureOverridesForTests();
    setConfig({ updates: "hidden" });
    renderAt("/updates");
    expect(screen.queryByText("updates page")).not.toBeInTheDocument();
    expect(screen.getByTestId("location-probe")).not.toHaveTextContent("/updates");
  });
});

describe("FeatureRouteGuard on the very first load", () => {
  afterEach(() => vi.useRealTimers());

  it("holds the page back until the saved state is known", () => {
    queryState.isPending = true;
    renderAt("/operations/problems");
    expect(screen.queryByText("problems page")).not.toBeInTheDocument();
    expect(screen.queryByText(/is in preview and isn't open to everyone yet/)).not.toBeInTheDocument();
  });

  it("goes on from the config if the answer takes too long", () => {
    vi.useFakeTimers();
    queryState.isPending = true;
    renderAt("/operations/incidents");
    expect(screen.queryByText("incidents page")).not.toBeInTheDocument();
    act(() => {
      vi.advanceTimersByTime(3000);
    });
    expect(screen.getByText("incidents page")).toBeInTheDocument();
  });
});
