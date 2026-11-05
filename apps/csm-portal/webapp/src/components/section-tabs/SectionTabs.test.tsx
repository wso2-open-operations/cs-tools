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
// The redirect reads the signed-in user's access (an admin opens a Preview
// tab), and Operations needs a role that can use it.
const currentRoles = vi.hoisted(() => ({ value: ["cs_engineer"] as string[] }));
vi.mock("@context/current-user/CurrentUserContext", () => ({
  useCurrentUser: () => ({
    user: { id: "user-1", roles: currentRoles.value },
    isLoading: false,
    isError: false,
    error: null,
  }),
}));

import SectionTabs, { LegacyQueryTabRedirect } from "@components/section-tabs/SectionTabs";
import { navNodeById } from "@config/csmNavItems";
import type { SectionTab } from "@hooks/useSectionTabs";

function setOverrides(value: unknown): void {
  window.config = {
    ...window.config,
    CSM_PORTAL_FEATURE_OVERRIDES: value,
  } as Window["config"];
  resetFeatureStatesForTests();
}

beforeEach(() => {
  currentRoles.value = ["cs_engineer"];
  setOverrides(undefined);
});
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

describe("LegacyQueryTabRedirect for a Preview section", () => {
  beforeEach(() => setOverrides({ operations: "preview" }));

  it("sends an admin to the first tab, which is in Preview", () => {
    currentRoles.value = ["admin"];
    renderAt("/operations");
    expect(screen.getByTestId("location-probe")).toHaveTextContent("/operations/service-requests");
  });

  it("sends everyone else to the dashboard, since no tab is open to them", () => {
    currentRoles.value = ["cs_engineer"];
    renderAt("/operations");
    expect(screen.getByTestId("location-probe")).toHaveTextContent("/dashboard");
  });
});

describe("SectionTabs chips", () => {
  function tab(id: string, state: SectionTab["state"], previewedOnly: boolean): SectionTab {
    const node = navNodeById(id)!;
    return { key: id, label: node.label, state, node, previewedOnly };
  }

  function renderTabs(tabs: SectionTab[]) {
    return render(
      <MemoryRouter>
        <SectionTabs tabs={tabs} activeKey={tabs[0]!.key} select={vi.fn()} ariaLabel="Test tabs" />
      </MemoryRouter>,
    );
  }

  it("shows a disabled Preview chip for a tab the user cannot open", () => {
    renderTabs([
      tab("operations.incidents", "enabled", false),
      tab("operations.problems", "preview", false),
    ]);
    const problems = screen.getByRole("tab", { name: /Problem management/ });
    expect(problems).toBeDisabled();
    expect(problems).toHaveTextContent("Preview");
    expect(screen.queryByText("WIP")).not.toBeInTheDocument();
  });

  it("shows a Preview chip on a tab an admin can open, and leaves it clickable", () => {
    renderTabs([
      tab("operations.incidents", "enabled", false),
      tab("operations.problems", "enabled", true),
    ]);
    const problems = screen.getByRole("tab", { name: /Problem management/ });
    expect(problems).not.toBeDisabled();
    expect(problems).toHaveTextContent("Preview");
    expect(screen.getByRole("tab", { name: "Incidents" })).not.toHaveTextContent("Preview");
  });
});
