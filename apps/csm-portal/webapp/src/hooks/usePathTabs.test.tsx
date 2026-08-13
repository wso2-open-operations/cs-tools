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

import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import type { JSX } from "react";
import { MemoryRouter, Route, Routes, useLocation } from "react-router";
import "@testing-library/jest-dom/vitest";
import { usePathTabs } from "@hooks/useSectionTabs";

type TabId = "one" | "two" | "three";
const TABS: readonly TabId[] = ["one", "two", "three"];

// Renders the real react-router stack (no mock on the navigation hook), so
// these tests prove an actual navigation happened, not just that some
// navigate function was called with the expected arguments — same convention
// as useNormalizedIdParam.test.tsx.
function LocationProbe(): JSX.Element {
  const location = useLocation();
  return (
    <div data-testid="location-probe">
      {location.pathname}
      {location.hash}
    </div>
  );
}

function Probe({ basePath }: { basePath: string }): JSX.Element {
  const { activeTab, setActiveTab } = usePathTabs<TabId>(basePath, TABS, "one");
  return (
    <div>
      <div data-testid="active-tab">{activeTab}</div>
      <button type="button" onClick={() => setActiveTab("two")}>
        Go to two
      </button>
      <button type="button" onClick={() => setActiveTab("three", { replace: true })}>
        Replace with three
      </button>
    </div>
  );
}

function renderAt(initialEntry: string): ReturnType<typeof render> {
  return render(
    <MemoryRouter initialEntries={[initialEntry]}>
      <LocationProbe />
      <Routes>
        <Route path="/things/:id/:tab?" element={<Probe basePath="/things/thing-1" />} />
      </Routes>
    </MemoryRouter>,
  );
}

describe("usePathTabs", () => {
  it("resolves the default tab when the URL carries no tab segment", () => {
    renderAt("/things/thing-1");
    expect(screen.getByTestId("active-tab")).toHaveTextContent("one");
  });

  it("resolves the active tab straight off a known URL segment", () => {
    renderAt("/things/thing-1/two");
    expect(screen.getByTestId("active-tab")).toHaveTextContent("two");
  });

  it("falls back to the default tab for an unknown segment, without redirecting the URL", () => {
    renderAt("/things/thing-1/not-a-real-tab");

    expect(screen.getByTestId("active-tab")).toHaveTextContent("one");
    // No redirect/correction: the URL the caller landed on is left exactly
    // as-is, just rendered as the default tab — this is what rules out a
    // redirect loop, since the hook never issues a navigation on its own.
    expect(screen.getByTestId("location-probe")).toHaveTextContent(
      "/things/thing-1/not-a-real-tab",
    );
  });

  it("setActiveTab navigates to basePath/<id>, pushing a new history entry by default", () => {
    renderAt("/things/thing-1");

    fireEvent.click(screen.getByRole("button", { name: /go to two/i }));

    expect(screen.getByTestId("location-probe")).toHaveTextContent(
      "/things/thing-1/two",
    );
    expect(screen.getByTestId("active-tab")).toHaveTextContent("two");
  });

  it("setActiveTab navigates to basePath/<id> when { replace: true } is passed", () => {
    renderAt("/things/thing-1");

    fireEvent.click(screen.getByRole("button", { name: /replace with three/i }));

    expect(screen.getByTestId("location-probe")).toHaveTextContent(
      "/things/thing-1/three",
    );
    expect(screen.getByTestId("active-tab")).toHaveTextContent("three");
  });

  it("preserves the current hash across a tab switch, so a permalink fragment survives", () => {
    renderAt("/things/thing-1#some-entry");

    fireEvent.click(screen.getByRole("button", { name: /go to two/i }));

    expect(screen.getByTestId("location-probe")).toHaveTextContent(
      "/things/thing-1/two#some-entry",
    );
  });

  it("is reusable for a nested two-level basePath (e.g. a work-items sub-tab)", () => {
    // `basePath` here is already resolved past the first-level sub-tab
    // (`work-items/tasks`) — the hook has no opinion on how many path
    // segments came before it, only that the final one is its own tab.
    render(
      <MemoryRouter initialEntries={["/projects/proj-1/work-items/tasks/two"]}>
        <LocationProbe />
        <Routes>
          <Route
            path="/projects/:projectId/work-items/:subTab/:tab?"
            element={<Probe basePath="/projects/proj-1/work-items/tasks" />}
          />
        </Routes>
      </MemoryRouter>,
    );

    expect(screen.getByTestId("active-tab")).toHaveTextContent("two");
  });
});
