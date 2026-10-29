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
 * Operations' header, the Change requests tab's sub-navigation (the change
 * request list and Standard changes, whose catalogue and proposals are one
 * sub-view) and where the old links (the separate standard changes tab, and
 * the two sub-views that came after it) land.
 * The tabs' own contents are stubbed: each has its own tests.
 */

import { fireEvent, render, screen, within } from "@testing-library/react";
import { MemoryRouter, Route, Routes, useLocation, useNavigate } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { JSX } from "react";
import "@testing-library/jest-dom/vitest";
import { resetFeatureStatesForTests } from "@config/featureFlags";
import { currentPageEntry } from "@features/csm-recent/currentPageEntry";

vi.mock("@features/csm-cases/components/CsmIssuesView", () => ({
  default: () => <div data-testid="tab-content">service requests</div>,
}));
vi.mock("@features/csm-operations/components/ChangeRequestsTab", () => ({
  default: () => <div data-testid="tab-content">change request list</div>,
}));
vi.mock("@features/csm-operations/components/IncidentsTab", () => ({
  default: () => <div data-testid="tab-content">incidents</div>,
}));
vi.mock("@features/csm-operations/components/ProblemsTab", () => ({
  default: () => <div data-testid="tab-content">problems</div>,
}));
vi.mock("@features/csm-operations/components/OutagesTab", () => ({
  default: () => <div data-testid="tab-content">outages</div>,
}));
vi.mock("@features/csm-operations/components/RunbooksTab", () => ({
  default: () => <div data-testid="tab-content">runbooks</div>,
}));
vi.mock("@features/csm-operations/components/StandardChangesTab", () => ({
  default: ({ show }: { show: string }) => <div data-testid="tab-content">{`standard changes: ${show}`}</div>,
}));

import OperationsPage from "@features/csm-operations/pages/OperationsPage";
import {
  OperationsIndexRedirect,
  StandardChangesPathRedirect,
} from "@features/csm-operations/pages/OperationsLegacyRedirects";

/** The page's URL, and a way to go Back in history from the test. */
function Probe(): JSX.Element {
  const location = useLocation();
  const navigate = useNavigate();
  return (
    <>
      <div data-testid="location">{`${location.pathname}${location.search}${location.hash}`}</div>
      <button onClick={() => navigate(-1)}>history back</button>
    </>
  );
}

/** The Operations routes of App.tsx that matter here. */
function renderAt(entry: string): void {
  render(
    <MemoryRouter initialEntries={[entry]}>
      <Routes>
        <Route path="/operations">
          <Route index element={<OperationsIndexRedirect />} />
          <Route
            path=":tab"
            element={
              <>
                <OperationsPage />
                <Probe />
              </>
            }
          />
          <Route path="standard-changes" element={<StandardChangesPathRedirect />} />
          <Route
            path="standard-changes/:id"
            element={
              <>
                <div data-testid="tab-content">template page</div>
                <Probe />
              </>
            }
          />
          <Route
            path="standard-change-proposals/:id"
            element={
              <>
                <div data-testid="tab-content">proposal page</div>
                <Probe />
              </>
            }
          />
        </Route>
      </Routes>
    </MemoryRouter>,
  );
}

const currentUrl = (): string => screen.getByTestId("location").textContent ?? "";
const content = (): string => screen.getByTestId("tab-content").textContent ?? "";
const subNav = (): HTMLElement => screen.getByRole("tablist", { name: "Change requests" });

beforeEach(() => {
  window.config = { ...window.config, CSM_PORTAL_FEATURE_OVERRIDES: undefined } as Window["config"];
  resetFeatureStatesForTests();
});

const OPERATIONS_SUBTITLE = "Service requests, change requests, incidents, problems, and runbooks across customers.";

/** The page's heading and the line under it. */
function heading(): { title: string; subtitle: string } {
  const title = screen.getByRole("heading", { level: 5 });
  return { title: title.textContent ?? "", subtitle: title.nextElementSibling?.textContent ?? "" };
}

describe("OperationsPage — header", () => {
  it.each([
    ["/operations/service-requests"],
    ["/operations/change-requests"],
    ["/operations/change-requests?crQ=gateway"],
    ["/operations/change-requests?view=nonsense"],
    ["/operations/incidents"],
    ["/operations/problems"],
    ["/operations/outages"],
    ["/operations/runbooks"],
  ])("keeps the Operations heading on %s", (entry) => {
    renderAt(entry);
    expect(heading()).toEqual({ title: "Operations", subtitle: OPERATIONS_SUBTITLE });
  });

  it.each([
    ["/operations/change-requests?view=standard"],
    ["/operations/change-requests?view=standard&show=proposals"],
    ["/operations/change-requests?view=catalogue"],
    ["/operations/change-requests?view=proposals"],
  ])("on %s it is about standard changes, and names nothing else Operations holds", (entry) => {
    renderAt(entry);
    expect(heading()).toEqual({
      title: "Standard changes",
      subtitle: "Pre-authorised change requests, raised from templates the CAB has approved.",
    });
    expect(screen.queryByText("Operations")).not.toBeInTheDocument();
    expect(screen.queryByText(OPERATIONS_SUBTITLE)).not.toBeInTheDocument();
    expect(screen.queryByText(/service requests|incidents|problems|runbooks/i)).not.toBeInTheDocument();
  });

  it("switches the heading with the sub-view", () => {
    renderAt("/operations/change-requests");
    expect(heading().title).toBe("Operations");
    fireEvent.click(within(subNav()).getByRole("tab", { name: "Standard changes" }));
    expect(heading().title).toBe("Standard changes");
    fireEvent.click(within(subNav()).getByRole("tab", { name: "Change requests" }));
    expect(heading()).toEqual({ title: "Operations", subtitle: OPERATIONS_SUBTITLE });
  });

  it("offers no sub-navigation outside Change requests", () => {
    renderAt("/operations/incidents");
    expect(content()).toBe("incidents");
    expect(screen.queryByRole("tablist", { name: "Change requests" })).not.toBeInTheDocument();
  });
});

describe("OperationsPage — Change requests sub-navigation", () => {
  it("reads exactly: Change requests | Standard changes, with the list first and shown by default", () => {
    renderAt("/operations/change-requests");
    expect(content()).toBe("change request list");
    const tabs = within(subNav()).getAllByRole("tab");
    expect(tabs.map((t) => t.textContent)).toEqual(["Change requests", "Standard changes"]);
    expect(tabs[0]).toHaveAttribute("aria-selected", "true");
  });

  it("keeps the list and its own URL filters as they were when no sub-view is asked for", () => {
    renderAt("/operations/change-requests?crQ=gateway&crStates=new");
    expect(content()).toBe("change request list");
    expect(currentUrl()).toBe("/operations/change-requests?crQ=gateway&crStates=new");
  });

  it("treats a sub-view it does not know as the list", () => {
    renderAt("/operations/change-requests?view=nonsense");
    expect(content()).toBe("change request list");
    expect(within(subNav()).getByRole("tab", { name: "Change requests" })).toHaveAttribute("aria-selected", "true");
  });

  it.each([
    ["/operations/change-requests?view=standard", "templates"],
    ["/operations/change-requests?view=standard&show=proposals", "proposals"],
  ])("opens %s from its own link, on the %s", (entry, show) => {
    renderAt(entry);
    expect(content()).toBe(`standard changes: ${show}`);
    expect(currentUrl()).toBe(entry);
    expect(within(subNav()).getByRole("tab", { name: "Standard changes" })).toHaveAttribute("aria-selected", "true");
  });

  it.each([
    ["/operations/change-requests?view=catalogue", "/operations/change-requests?view=standard", "templates"],
    ["/operations/change-requests?view=proposals", "/operations/change-requests?view=standard&show=proposals", "proposals"],
    [
      "/operations/change-requests?view=proposals&scope=awaiting",
      "/operations/change-requests?view=standard&show=proposals&scope=awaiting",
      "proposals",
    ],
    [
      "/operations/change-requests?crQ=gateway&view=catalogue",
      "/operations/change-requests?view=standard&crQ=gateway",
      "templates",
    ],
  ])("rewrites the old %s link to %s, on the %s", (entry, to, show) => {
    renderAt(entry);
    expect(currentUrl()).toBe(to);
    expect(content()).toBe(`standard changes: ${show}`);
    expect(within(subNav()).getByRole("tab", { name: "Standard changes" })).toHaveAttribute("aria-selected", "true");
  });

  it("rewrites an old link in place, without a history entry of its own", () => {
    render(
      <MemoryRouter initialEntries={["/operations/incidents", "/operations/change-requests?view=proposals"]}>
        <Routes>
          <Route
            path="/operations/:tab"
            element={
              <>
                <OperationsPage />
                <Probe />
              </>
            }
          />
        </Routes>
      </MemoryRouter>,
    );
    expect(currentUrl()).toBe("/operations/change-requests?view=standard&show=proposals");
    fireEvent.click(screen.getByRole("button", { name: "history back" }));
    expect(currentUrl()).toBe("/operations/incidents");
  });

  it("keeps the list's filters while on Standard changes, so the list comes back exactly as it was", () => {
    renderAt("/operations/change-requests?crQ=gateway&crStates=new");

    fireEvent.click(within(subNav()).getByRole("tab", { name: "Standard changes" }));
    expect(currentUrl()).toBe("/operations/change-requests?view=standard&crQ=gateway&crStates=new");
    expect(content()).toBe("standard changes: templates");
    // They are not Standard changes' filters: a pin of it names the place alone.
    expect(currentPageEntry("/operations/change-requests", "?view=standard&crQ=gateway&crStates=new").title).toBe(
      "Standard changes",
    );

    fireEvent.click(within(subNav()).getByRole("tab", { name: "Change requests" }));
    expect(currentUrl()).toBe("/operations/change-requests?crQ=gateway&crStates=new");
    expect(content()).toBe("change request list");
  });

  it("drops Standard changes' own parameters on the way back to the list, keeping the list's", () => {
    renderAt("/operations/change-requests?view=standard&show=proposals&scope=awaiting&state=draft&crQ=gateway");
    fireEvent.click(within(subNav()).getByRole("tab", { name: "Change requests" }));
    expect(currentUrl()).toBe("/operations/change-requests?crQ=gateway");
    expect(content()).toBe("change request list");

    fireEvent.click(within(subNav()).getByRole("tab", { name: "Standard changes" }));
    expect(currentUrl()).toBe("/operations/change-requests?view=standard&crQ=gateway");
    expect(content()).toBe("standard changes: templates");
  });

  it("makes each sub-view its own history entry", () => {
    renderAt("/operations/change-requests");
    fireEvent.click(within(subNav()).getByRole("tab", { name: "Standard changes" }));
    expect(currentUrl()).toBe("/operations/change-requests?view=standard");
    fireEvent.click(screen.getByRole("button", { name: "history back" }));
    expect(currentUrl()).toBe("/operations/change-requests");
    expect(content()).toBe("change request list");
  });

  it("no longer has a standard changes tab of its own", () => {
    renderAt("/operations/standard-changes");
    // The old tab's path is redirected into Change requests, never rendered as a tab.
    expect(currentUrl()).toBe("/operations/change-requests?view=standard");
    expect(content()).toBe("standard changes: templates");
  });
});

describe("old links to the standard changes tab", () => {
  it.each([
    ["/operations?tab=standard_changes", "/operations/change-requests?view=standard", "templates"],
    ["/operations?tab=standard_changes&view=proposals", "/operations/change-requests?view=standard&show=proposals", "proposals"],
    ["/operations?tab=standard-changes", "/operations/change-requests?view=standard", "templates"],
    ["/operations?tab=standard-changes&view=proposals", "/operations/change-requests?view=standard&show=proposals", "proposals"],
    ["/operations/standard-changes", "/operations/change-requests?view=standard", "templates"],
    ["/operations/standard-changes?view=proposals", "/operations/change-requests?view=standard&show=proposals", "proposals"],
    ["/operations/standard-changes?view=bogus", "/operations/change-requests?view=standard", "templates"],
    ["/operations?tab=standard_changes&foo=1#top", "/operations/change-requests?view=standard&foo=1#top", "templates"],
  ])("%s lands on %s", (from, to, show) => {
    renderAt(from);
    expect(currentUrl()).toBe(to);
    expect(content()).toBe(`standard changes: ${show}`);
  });

  it("leaves every other old ?tab= link to the usual redirect", () => {
    renderAt("/operations?tab=change_requests&crQ=gateway");
    expect(currentUrl()).toBe("/operations/change-requests?crQ=gateway");
    expect(content()).toBe("change request list");
  });

  it("does not redirect a template's own page", () => {
    renderAt("/operations/standard-changes/tpl-1");
    expect(currentUrl()).toBe("/operations/standard-changes/tpl-1");
    expect(content()).toBe("template page");
  });

  it.each([
    ["/operations/standard-changes/tpl-1", "template page"],
    ["/operations/standard-change-proposals/scp-1", "proposal page"],
  ])("%s is its own route, outside the Operations page and its heading", (entry, page) => {
    renderAt(entry);
    expect(content()).toBe(page);
    expect(screen.queryByText("Operations")).not.toBeInTheDocument();
    expect(screen.queryByText(OPERATIONS_SUBTITLE)).not.toBeInTheDocument();
  });
});
