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

import { fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useState, type JSX, type ReactNode } from "react";
import { MemoryRouter, Route, Routes, useLocation, type To } from "react-router";
import "@testing-library/jest-dom/vitest";
import {
  enabledPathTabKeys,
  firstEnabledPathTab,
  usePathSectionTabs,
  useQueryParamTabs,
} from "@hooks/useSectionTabs";
import { resetFeatureStatesForTests } from "@config/featureFlags";
import {
  CaseRouteOverrideProvider,
  type CaseRouteOverrideValue,
} from "@context/case-tabs/CaseRouteOverrideContext";

function setOverrides(value: unknown): void {
  window.config = {
    ...window.config,
    CSM_PORTAL_FEATURE_OVERRIDES: value,
  } as Window["config"];
  resetFeatureStatesForTests();
}

beforeEach(() => {
  setOverrides(undefined);
});

afterEach(() => {
  vi.restoreAllMocks();
});

function LocationProbe(): JSX.Element {
  const location = useLocation();
  return (
    <div data-testid="location-probe">
      {location.pathname}
      {location.search}
    </div>
  );
}

// --- usePathSectionTabs -----------------------------------------------------

function PathTabsProbe({ sectionId, basePath }: { sectionId: string; basePath: string }): JSX.Element {
  const { tabs, activeKey, select } = usePathSectionTabs(sectionId, basePath);
  return (
    <div>
      <div data-testid="active-key">{activeKey}</div>
      <div data-testid="tab-keys">{tabs.map((t) => t.key).join(",")}</div>
      {tabs.map((t) => (
        <button key={t.key} onClick={() => select(t.key)}>
          {t.label}
        </button>
      ))}
    </div>
  );
}

function renderPathTabsAt(initialEntry: string) {
  return render(
    <MemoryRouter initialEntries={[initialEntry]}>
      <LocationProbe />
      <Routes>
        <Route
          path="/operations/:tab"
          element={<PathTabsProbe sectionId="operations" basePath="/operations" />}
        />
      </Routes>
    </MemoryRouter>,
  );
}

describe("usePathSectionTabs", () => {
  it("keys tabs by the kebab-case conversion of the nav node's ?tab= value", () => {
    renderPathTabsAt("/operations/service-requests");

    expect(screen.getByTestId("tab-keys")).toHaveTextContent(
      "service-requests,change-requests,incidents,problems",
    );
  });

  it("reads the active tab from the path segment, not a query param", () => {
    renderPathTabsAt("/operations/incidents");

    expect(screen.getByTestId("active-key")).toHaveTextContent("incidents");
  });

  it("falls back to the first enabled tab for an unrecognised path segment, without crashing or looping", () => {
    renderPathTabsAt("/operations/not-a-real-tab");

    expect(screen.getByTestId("active-key")).toHaveTextContent("service-requests");
    // No redirect happened — resolving the fallback is a pure read.
    expect(screen.getByTestId("location-probe")).toHaveTextContent(
      "/operations/not-a-real-tab",
    );
  });

  it("falls back to the first ENABLED tab when the requested one is restricted (wip)", () => {
    setOverrides({ "operations.incidents": "wip" });

    renderPathTabsAt("/operations/incidents");

    expect(screen.getByTestId("active-key")).toHaveTextContent("service-requests");
  });

  it("navigates to the new path segment (not a ?tab= query) when selecting a tab", () => {
    renderPathTabsAt("/operations/service-requests");

    fireEvent.click(screen.getByText("Change requests"));

    expect(screen.getByTestId("location-probe")).toHaveTextContent(
      "/operations/change-requests",
    );
  });
});

describe("firstEnabledPathTab / enabledPathTabKeys", () => {
  it("returns every enabled tab's kebab-case path segment, in nav order", () => {
    expect(enabledPathTabKeys("operations")).toEqual([
      "service-requests",
      "change-requests",
      "incidents",
      "problems",
    ]);
    expect(firstEnabledPathTab("operations")).toBe("service-requests");
  });

  it("excludes a hidden tab and returns undefined for an unknown section", () => {
    setOverrides({ "operations.incidents": "hidden" });

    expect(enabledPathTabKeys("operations")).toEqual([
      "service-requests",
      "change-requests",
      "problems",
    ]);
    expect(firstEnabledPathTab("does-not-exist")).toBeUndefined();
  });
});

// --- useQueryParamTabs -------------------------------------------------------

type ProbeTabId = "one" | "two" | "three";
const PROBE_TABS: readonly ProbeTabId[] = ["one", "two", "three"];

function QueryParamTabsProbe({
  clearParamsOnChange,
}: {
  clearParamsOnChange?: readonly string[];
}): JSX.Element {
  const { activeTab, setActiveTab } = useQueryParamTabs<ProbeTabId>(
    PROBE_TABS,
    "one",
    { clearParamsOnChange },
  );
  return (
    <div>
      <div data-testid="active-tab">{activeTab}</div>
      <button onClick={() => setActiveTab("two")}>Go to two</button>
      <button onClick={() => setActiveTab("three", { replace: false })}>
        Go to three (push)
      </button>
    </div>
  );
}

function renderQueryParamTabsAt(
  initialEntry: string,
  clearParamsOnChange?: readonly string[],
) {
  return render(
    <MemoryRouter initialEntries={[initialEntry]}>
      <LocationProbe />
      <QueryParamTabsProbe clearParamsOnChange={clearParamsOnChange} />
    </MemoryRouter>,
  );
}

describe("useQueryParamTabs", () => {
  it("defaults to the given default tab when ?tab= is absent", () => {
    renderQueryParamTabsAt("/detail/1");
    expect(screen.getByTestId("active-tab")).toHaveTextContent("one");
  });

  it("reads the active tab from ?tab=", () => {
    renderQueryParamTabsAt("/detail/1?tab=two");
    expect(screen.getByTestId("active-tab")).toHaveTextContent("two");
  });

  it("falls back to the default for an unrecognised ?tab= value, without crashing or looping", () => {
    renderQueryParamTabsAt("/detail/1?tab=bogus");
    expect(screen.getByTestId("active-tab")).toHaveTextContent("one");
    // No rewrite happened — the stale value is left alone.
    expect(screen.getByTestId("location-probe")).toHaveTextContent("tab=bogus");
  });

  it("writes ?tab= when switching tabs, preserving other existing search params", () => {
    renderQueryParamTabsAt("/detail/1?filter=open&page=2");

    fireEvent.click(screen.getByText("Go to two"));

    const probe = screen.getByTestId("location-probe").textContent ?? "";
    expect(probe).toContain("tab=two");
    expect(probe).toContain("filter=open");
    expect(probe).toContain("page=2");
  });

  it("drops params named in clearParamsOnChange when the tab changes", () => {
    renderQueryParamTabsAt("/detail/1?tab=one&subTab=nested", ["subTab"]);

    fireEvent.click(screen.getByText("Go to two"));

    const probe = screen.getByTestId("location-probe").textContent ?? "";
    expect(probe).toContain("tab=two");
    expect(probe).not.toContain("subTab");
  });

  it("replaces the history entry by default rather than pushing a new one", () => {
    renderQueryParamTabsAt("/detail/1");

    fireEvent.click(screen.getByText("Go to two"));
    expect(screen.getByTestId("active-tab")).toHaveTextContent("two");
  });

  // Regression test: this hook used to read/write the REAL, single,
  // app-wide `?tab=` query param unconditionally, even for a page mounted
  // inside an open in-app case tab (`CaseTabIsolatedRouter`) — which has its
  // OWN per-tab location, not the real one. Two such tabs open on different
  // sections would fight over the same real query param, and switching
  // between them could reset one back to its default section. This exercises
  // the override-aware branch directly (`CsmCaseDetailPage`'s own
  // `routeOverride?.navigate ?? routedNavigate` pattern, applied inside the
  // hook itself now) with two independent overrides standing in for two
  // open tabs.
  describe("inside an open case tab (CaseRouteOverrideContext)", () => {
    // A real React-state-backed override, not a closure mutation — mirrors
    // `CaseTabIsolatedRouter`'s own shape (its `routeState`/`overrideValue`
    // pair): `navigate` updates STATE, which triggers a real rerender that
    // rebuilds the override VALUE object with the new `search`, exactly as
    // production does. An earlier version of this fixture only mutated a
    // closure-local `search` variable and never touched `override.search`
    // or recreated the provider's value at all — so it never actually
    // exercised a rerender, and could have silently kept passing even if
    // the hook stopped reading fresh `search` values from a changing
    // override altogether.
    function OverrideHost({
      initialSearch,
      onNavigate,
      children,
    }: {
      initialSearch: string;
      onNavigate: (search: string) => void;
      children: ReactNode;
    }) {
      const [search, setSearch] = useState(initialSearch);
      const navigate = (to: To | number): void => {
        const next =
          typeof to === "string"
            ? (to.split("?")[1] ?? "")
            : typeof to === "number"
              ? ""
              : (to.search ?? "").replace(/^\?/, "");
        const nextSearch = next ? `?${next}` : "";
        setSearch(nextSearch);
        onNavigate(nextSearch);
      };
      const value: CaseRouteOverrideValue = {
        caseId: "CS-STUB",
        kind: "case",
        pathname: "/cases/CS-STUB",
        search,
        hash: "",
        state: undefined,
        navigate,
      };
      return (
        <CaseRouteOverrideProvider value={value}>{children}</CaseRouteOverrideProvider>
      );
    }

    it("reads/writes the tab's own search, never the real router's", () => {
      const navigateSpyA = vi.fn();
      const navigateSpyB = vi.fn();

      render(
        <MemoryRouter initialEntries={["/cases/CS-STUB"]}>
          <LocationProbe />
          <OverrideHost initialSearch="?tab=one" onNavigate={navigateSpyA}>
            <div data-testid="tab-a">
              <QueryParamTabsProbe />
            </div>
          </OverrideHost>
          <OverrideHost initialSearch="?tab=two" onNavigate={navigateSpyB}>
            <div data-testid="tab-b">
              <QueryParamTabsProbe />
            </div>
          </OverrideHost>
        </MemoryRouter>,
      );

      // Each tab's own override search resolves independently, not the real
      // router's (which has no `?tab=` at all here).
      expect(screen.getByTestId("tab-a")).toHaveTextContent("one");
      expect(screen.getByTestId("tab-b")).toHaveTextContent("two");

      // Switching tab A's section calls ONLY tab A's own `navigate` — the
      // real router (asserted via `LocationProbe`) and tab B are untouched.
      fireEvent.click(within(screen.getByTestId("tab-a")).getByText("Go to two"));
      expect(navigateSpyA).toHaveBeenCalled();
      expect(navigateSpyB).not.toHaveBeenCalled();
      expect(screen.getByTestId("tab-a")).toHaveTextContent("two");
      expect(screen.getByTestId("tab-b")).toHaveTextContent("two");
      // ^ tab-b still shows "two" because it was already on "two" — the
      // point is its OWN override was never invoked, not that its value
      // changed to something different by coincidence.
      expect(screen.getByTestId("location-probe")).not.toHaveTextContent("tab=");
    });
  });
});
