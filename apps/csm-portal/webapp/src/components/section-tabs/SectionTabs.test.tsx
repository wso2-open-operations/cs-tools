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
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { JSX } from "react";
import { MemoryRouter, Route, Routes, useLocation } from "react-router";
import "@testing-library/jest-dom/vitest";
import { resetFeatureStatesForTests } from "@config/featureFlags";

// `SectionTabs.tsx` imports `@hooks/useSectionTabs`, which now also exports
// `useRouteTabs` (reads `usePortalAccess`) -- pulling in the real
// `@api/backend/client`/`@config/apiConfig` transitively at module load, even
// though this file only exercises the unrelated `LegacyQueryTabRedirect`.
// Same boilerplate this repo's own convention already uses wherever a test
// transitively imports those two modules.
vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ get: vi.fn(), post: vi.fn() }),
}));
vi.mock("@config/apiConfig", () => ({
  apiConfig: { backendUrl: "https://example.test" },
}));

import { LegacyQueryTabRedirect } from "@components/section-tabs/SectionTabs";

function setOverrides(value: unknown): void {
  window.config = {
    ...window.config,
    CSM_PORTAL_FEATURE_OVERRIDES: value,
  } as Window["config"];
  resetFeatureStatesForTests();
}

beforeEach(() => setOverrides(undefined));
afterEach(() => setOverrides(undefined));

function LocationProbe(): JSX.Element {
  const location = useLocation();
  return (
    <div data-testid="location-probe">
      {location.pathname}
      {location.search}
      {location.hash}
    </div>
  );
}

function renderAt(initialEntry: string) {
  return render(
    <MemoryRouter initialEntries={[initialEntry]}>
      <Routes>
        <Route
          path="/operations"
          element={
            <LegacyQueryTabRedirect sectionId="operations" basePath="/operations" />
          }
        />
        <Route path="/operations/:tab" element={<div>tab page</div>} />
      </Routes>
      <LocationProbe />
    </MemoryRouter>,
  );
}

describe("LegacyQueryTabRedirect", () => {
  it("sends a bare section URL (no ?tab=) to the first enabled tab's path segment", () => {
    renderAt("/operations");
    expect(screen.getByTestId("location-probe")).toHaveTextContent(
      "/operations/service-requests",
    );
  });

  it("translates an old ?tab= link to the new path-segment form", () => {
    renderAt("/operations?tab=incidents");
    expect(screen.getByTestId("location-probe")).toHaveTextContent(
      "/operations/incidents",
    );
  });

  it("falls back to the first enabled tab for an unrecognised legacy ?tab= value", () => {
    renderAt("/operations?tab=not_a_real_tab");
    expect(screen.getByTestId("location-probe")).toHaveTextContent(
      "/operations/service-requests",
    );
  });

  it("falls back to the first enabled tab for a legacy ?tab= value naming a restricted tab", () => {
    setOverrides({ "operations.incidents": "hidden" });
    renderAt("/operations?tab=incidents");
    expect(screen.getByTestId("location-probe")).toHaveTextContent(
      "/operations/service-requests",
    );
  });

  it("preserves other existing search params and the hash on the redirect", () => {
    renderAt("/operations?tab=incidents&severity=s1#top");
    const probe = screen.getByTestId("location-probe").textContent ?? "";
    expect(probe).toContain("/operations/incidents");
    expect(probe).toContain("severity=s1");
    expect(probe).not.toContain("tab=incidents");
    expect(probe).toContain("#top");
  });

  it("falls back to the dashboard but keeps other search params and the hash when every tab is restricted", () => {
    setOverrides({ operations: "hidden" });
    renderAt("/operations?tab=incidents&severity=s1#top");
    const probe = screen.getByTestId("location-probe").textContent ?? "";
    expect(probe).toContain("/dashboard");
    expect(probe).toContain("severity=s1");
    expect(probe).not.toContain("tab=incidents");
    expect(probe).toContain("#top");
  });
});
