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
 * Exercises `usePathSectionTabs` directly against the real nav tree
 * (`operations`) and real `featureFlags` resolution, the same way
 * `usePathTabs.test.tsx` exercises its hook — proving the URL actually
 * changes on `select()`, not just that some navigate function was called.
 * Page-level rendering (SectionTabs' WIP chip / hidden omission, the legacy
 * `?tab=` redirect) is covered in OperationsPage.test.tsx and
 * CsmSecurityCenterPage.test.tsx.
 */

import { fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import type { JSX } from "react";
import { MemoryRouter, Route, Routes, useLocation } from "react-router";
import "@testing-library/jest-dom/vitest";
import { usePathSectionTabs } from "@hooks/useSectionTabs";
import { resetFeatureStatesForTests } from "@config/featureFlags";

function setOverrides(value: unknown): void {
  window.config = {
    ...window.config,
    CSM_PORTAL_FEATURE_OVERRIDES: value,
  } as Window["config"];
  resetFeatureStatesForTests();
}

function LocationProbe(): JSX.Element {
  const location = useLocation();
  return <div data-testid="location-probe">{location.pathname}</div>;
}

function Probe(): JSX.Element {
  const { tabs, activeKey, select } = usePathSectionTabs("operations", "/operations");
  return (
    <div>
      <div data-testid="active-key">{activeKey}</div>
      <div data-testid="tab-keys">{tabs.map((tab) => tab.key).join(",")}</div>
      <button type="button" onClick={() => select("change_requests")}>
        Go to change requests
      </button>
    </div>
  );
}

function renderAt(initialEntry: string): ReturnType<typeof render> {
  return render(
    <MemoryRouter initialEntries={[initialEntry]}>
      <LocationProbe />
      <Routes>
        <Route path="/operations" element={<Probe />} />
        <Route path="/operations/:tab" element={<Probe />} />
      </Routes>
    </MemoryRouter>,
  );
}

describe("usePathSectionTabs", () => {
  beforeEach(() => {
    setOverrides(undefined);
  });

  afterEach(() => {
    setOverrides(undefined);
  });

  it("resolves the requested tab straight off a known, enabled URL segment", () => {
    renderAt("/operations/change_requests");
    expect(screen.getByTestId("active-key")).toHaveTextContent("change_requests");
  });

  it("falls back to the first enabled tab for an unknown segment, without redirecting the URL", () => {
    renderAt("/operations/not-a-real-tab");

    expect(screen.getByTestId("active-key")).toHaveTextContent("service_requests");
    expect(screen.getByTestId("location-probe")).toHaveTextContent(
      "/operations/not-a-real-tab",
    );
  });

  it("falls back to the first enabled tab when the requested one is WIP", () => {
    setOverrides({ "operations.change-requests": "wip" });
    renderAt("/operations/change_requests");

    expect(screen.getByTestId("active-key")).toHaveTextContent("service_requests");
  });

  it("falls back to the first enabled tab when the requested one is hidden", () => {
    setOverrides({ "operations.change-requests": "hidden" });
    renderAt("/operations/change_requests");

    expect(screen.getByTestId("active-key")).toHaveTextContent("service_requests");
    // Hidden tabs are dropped from the list entirely, same as useQueryTabs.
    expect(screen.getByTestId("tab-keys")).not.toHaveTextContent("change_requests");
  });

  it("select() navigates to basePath/<key>, pushing a real path-segment URL", () => {
    renderAt("/operations");

    fireEvent.click(screen.getByRole("button", { name: /go to change requests/i }));

    expect(screen.getByTestId("location-probe")).toHaveTextContent(
      "/operations/change_requests",
    );
    expect(screen.getByTestId("active-key")).toHaveTextContent("change_requests");
  });
});
