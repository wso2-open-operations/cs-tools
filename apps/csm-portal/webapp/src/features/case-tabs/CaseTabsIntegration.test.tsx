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

import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useState } from "react";
import { BrowserRouter, Route, Routes, useNavigate, useParams } from "react-router";
import "@testing-library/jest-dom/vitest";
import { useCaseRouteOverride } from "@context/case-tabs/CaseRouteOverrideContext";
import { CaseTabsProvider, useCaseTabsController } from "@context/case-tabs/CaseTabsContext";
import {
  CaseTabsBehaviorProvider,
  useCaseTabsBehavior,
} from "@context/case-tabs/CaseTabsBehaviorContext";
import { ErrorBannerProvider } from "@context/error-banner/ErrorBannerContext";
import CaseDetailRouteSync from "@features/case-tabs/components/CaseDetailRouteSync";
import {
  CaseTabsContentHost,
  CaseTabStripBar,
} from "@features/case-tabs/components/CaseTabsWorkspace";
import { useReportCaseTabMeta } from "@features/case-tabs/hooks/useReportCaseTabMeta";
import { useQueryParamTabs } from "@hooks/useSectionTabs";
import { MAX_OPEN_CASE_TABS } from "@context/case-tabs/caseTabsTypes";

const ENABLED_STORAGE_KEY = "csm.caseTabs.enabled";
const CAP_MODE_STORAGE_KEY = "csm.caseTabs.capMode";

/**
 * Stand-in for `CsmCaseDetailPage`, wired in via the SAME lazy-loaded module
 * `App.tsx`'s real routes and `CaseTabsWorkspace`'s real keep-alive host
 * both import — so tests here exercise the actual production wiring end to
 * end (route match -> `CaseDetailRouteSync` -> `CaseTabsContext` ->
 * `CaseTabsContentHost`'s `CaseTabIsolatedRouter`), not a hand-rolled
 * substitute for it. Reports its own label via `useReportCaseTabMeta` the
 * exact same way the real page does now (see that hook's doc comment for
 * why), which is what makes the bug-1 regression test below meaningful.
 */
const STUB_SECTIONS = ["details", "activities"] as const;
type StubSection = (typeof STUB_SECTIONS)[number];

function StubCaseDetailPage() {
  const override = useCaseRouteOverride();
  const routedNavigate = useNavigate();
  const { caseId: routedCaseId } = useParams();
  const caseId = override?.caseId ?? routedCaseId;
  const navigate = override?.navigate ?? routedNavigate;
  // A fake "still loading" -> "loaded" transition, matching a real
  // `useGetCsmCaseDetail` query — the label should appear once this
  // resolves, without needing the user to switch tabs away and back.
  const [label, setLabel] = useState<string | undefined>(undefined);
  useReportCaseTabMeta(caseId, { label, internalId: undefined, subject: undefined });
  // Same `useQueryParamTabs` call `CsmCaseDetailPage` itself makes for its
  // own section strip (Details/Activities/...) — what makes the item-4
  // regression test below meaningful: it exercises the SAME override-aware
  // path a real open tab's section selection goes through.
  const { activeTab: section, setActiveTab: setSection } = useQueryParamTabs<StubSection>(
    STUB_SECTIONS,
    "details",
  );
  return (
    <div>
      <div data-testid="stub-page-case-id">{caseId}</div>
      <div data-testid="stub-page-section">{section}</div>
      <button onClick={() => setLabel(`Label for ${caseId}`)}>resolve-label</button>
      <button onClick={() => setSection("activities")}>go-to-activities-{caseId}</button>
      {/* Same shape as `CsmCaseDetailPage`'s own Back button: navigates via
          whichever `navigate` is in scope (the tab override's, when this
          instance is an open tab) to a bare non-case route. */}
      <button onClick={() => navigate("/cases")}>stub-back-button</button>
    </div>
  );
}

vi.mock("@features/case-tabs/tabPageRegistry", () => ({
  pageComponentForKind: () => StubCaseDetailPage,
}));

function Opener({ caseId }: { caseId: string }) {
  const { openTab } = useCaseTabsController();
  return <button onClick={() => openTab(caseId, "case", `/cases/${caseId}`)}>open-{caseId}</button>;
}

/** A real `<Link>`-equivalent click, same as a case-list row's own
 * navigation — exercises the real router, not a manual `history.pushState`
 * (which `BrowserRouter` doesn't observe the same way). */
function NavigateButton({ to, label }: { to: string; label: string }) {
  const navigate = useNavigate();
  return <button onClick={() => navigate(to)}>{label}</button>;
}

/**
 * End-to-end smoke test for the real `App.tsx`/`AppLayout` wiring shape:
 * one real `<BrowserRouter>` (as the app always has exactly one), the same
 * case-detail route pattern App.tsx registers, `CaseTabsContentHost`
 * (App.tsx and AppLayout render this in the app shell), and
 * `CaseDetailRouteSync` as the route's own element (App.tsx's actual
 * `element={<CaseDetailRouteSync kind="case" />}`).
 *
 * This is also the regression test for the "You cannot render a <Router>
 * inside another <Router>" crash a previous version of this feature shipped
 * with: that bug was only reachable through this exact real-router nesting,
 * which a standalone component render (this feature's other test files)
 * cannot reproduce.
 */
function App({ initialPath }: { initialPath: string }) {
  window.history.pushState({}, "", initialPath);
  return (
    <BrowserRouter>
      <ErrorBannerProvider>
        <CaseTabsBehaviorProvider>
          <CaseTabsProvider>
            <CaseTabStripBar />
            <CaseTabsContentHost />
            <Routes>
              <Route path="/cases/:caseId" element={<CaseDetailRouteSync kind="case" />} />
            </Routes>
          </CaseTabsProvider>
        </CaseTabsBehaviorProvider>
      </ErrorBannerProvider>
    </BrowserRouter>
  );
}

describe("case tabs — real BrowserRouter integration", () => {
  beforeEach(() => {
    // These tests exercise the tab mechanism itself; opt into it being on
    // explicitly rather than relying on whatever the default happens to be
    // (see the dedicated describe block below for a test of that default
    // specifically). Cap mode is pinned to "evict-oldest" (rather than left
    // at its own default) so the cap tests below are deterministic about
    // WHICH tab gets evicted.
    localStorage.setItem(ENABLED_STORAGE_KEY, "1");
    localStorage.setItem(CAP_MODE_STORAGE_KEY, "evict-oldest");
  });

  it("opening a case via the tab mechanism renders it, without the nested-Router crash", async () => {
    expect(() => render(<App initialPath="/cases/CS0001" />)).not.toThrow();
    await waitFor(() =>
      expect(screen.getByTestId("stub-page-case-id")).toHaveTextContent("CS0001"),
    );
    // A tab for it is now open and shown in the strip, alongside the
    // permanent pinned "current location" tab (see `useCurrentLocationTab`).
    expect(screen.getByRole("tablist", { name: "Open cases" })).toBeInTheDocument();
    expect(screen.getByText("CS0001")).toBeInTheDocument();
  });

  // Regression test for bug: a tab's chip label stayed on the raw caseId
  // until the user switched to a different tab and back — i.e. it didn't
  // update in place while the tab stayed the active/visible one.
  it("bug 1 — a tab's label updates once its data resolves, without switching tabs away and back", async () => {
    render(<App initialPath="/cases/CS0001" />);
    await waitFor(() => screen.getByTestId("stub-page-case-id"));

    // Label not resolved yet: the chip falls back to "Loading…", not the
    // raw caseId/UUID.
    expect(screen.getByText("Loading…")).toBeInTheDocument();

    // Simulate the page's data query resolving (matches
    // `CsmCaseDetailPage`'s own data -> label flow via `useReportCaseTabMeta`).
    fireEvent.click(screen.getByText("resolve-label"));

    await waitFor(() => expect(screen.getByText("Label for CS0001")).toBeInTheDocument());
    // Still exactly one CASE tab (plus the permanent pinned one) — this
    // wasn't achieved by closing/reopening or switching away.
    expect(screen.getAllByRole("tab")).toHaveLength(2);
  });

  // Regression test for bug: a detail page's own section tab strip
  // (Details/Activities/...) reads/writes the REAL shared router `?tab=`
  // query param, not a per-tab one — with two case tabs open on different
  // sections, switching between them could reset one back to its default
  // section because they were fighting over the same param. Fixed by making
  // `useQueryParamTabs` itself override-aware (see that hook's own doc
  // comment) — this exercises it end to end, through the real tab strip and
  // the real keep-alive host, not just the hook in isolation (see
  // `useSectionTabs.test.tsx` for that).
  it("bug 4 — each open case tab keeps its own section selection independently", async () => {
    function AppWithTwoCaseRoutes() {
      window.history.pushState({}, "", "/cases/CS0001");
      return (
        <BrowserRouter>
          <ErrorBannerProvider>
            <CaseTabsBehaviorProvider>
              <CaseTabsProvider>
                <NavigateButton to="/cases/CS0002" label="go-to-cs0002" />
                <NavigateButton to="/cases/CS0001" label="go-to-cs0001" />
                <CaseTabStripBar />
                <CaseTabsContentHost />
                <Routes>
                  <Route path="/cases/:caseId" element={<CaseDetailRouteSync kind="case" />} />
                </Routes>
              </CaseTabsProvider>
            </CaseTabsBehaviorProvider>
          </ErrorBannerProvider>
        </BrowserRouter>
      );
    }
    render(<AppWithTwoCaseRoutes />);
    await waitFor(() => expect(screen.getByTestId("stub-page-case-id")).toBeInTheDocument());

    // Once a second tab is open, both pages stay mounted in the background
    // (keep-alive) — `stub-page-section` then matches TWO elements, so
    // reading it always means the currently VISIBLE tab's panel specifically
    // (`CaseTabIsolatedRouter` marks the hidden one(s) with a real `hidden`
    // attribute — see that component's own doc comment).
    const visibleSection = (): string | null => {
      const panels = document.querySelectorAll('[data-testid^="case-tab-panel-"]');
      for (const panel of Array.from(panels)) {
        if (!panel.hasAttribute("hidden")) {
          return panel.querySelector('[data-testid="stub-page-section"]')?.textContent ?? null;
        }
      }
      return null;
    };

    // CS0001 (the only, and so visible, tab right now): switch its own
    // section to "activities".
    fireEvent.click(screen.getByText("go-to-activities-CS0001"));
    await waitFor(() => expect(visibleSection()).toBe("activities"));

    // Open a second, distinct case — a real navigation, exactly like a
    // case-list row click.
    fireEvent.click(screen.getByText("go-to-cs0002"));
    await waitFor(() => expect(window.location.pathname).toBe("/cases/CS0002"));
    // CS0002's own section starts at its own default ("details"), NOT
    // inheriting CS0001's "activities" — each tab's `?tab=` is independent.
    await waitFor(() => expect(visibleSection()).toBe("details"));

    // Switch back to CS0001 via its own TAB STRIP CHIP — not another bare
    // `NavigateButton` click. A chip click (`CaseTabsWorkspace`'s own
    // `onActivate`) navigates to the tab's own STORED path (which already
    // carries its `?tab=activities`), the same way a real tab-strip click
    // does; a bare link/bookmark to `/cases/CS0001` (no `?tab=`) is instead
    // exactly the OUTSIDE-navigation-reactivates-an-open-tab case that's
    // now correctly treated as "start this tab over at its default
    // section" (see `caseTabsReducer`'s `OPEN_OR_ACTIVATE` and
    // `CaseTabIsolatedRouter`'s resync effect) — this test isn't exercising
    // that path here, `CaseTabIsolatedRouter.test.tsx`'s own reactivation
    // test does.
    fireEvent.click(screen.getAllByRole("tab")[1]);
    await waitFor(() => expect(window.location.pathname).toBe("/cases/CS0001"));
    await waitFor(() => expect(visibleSection()).toBe("activities"));

    // And CS0002's own section is untouched by any of this — switched to
    // the same way, via its own chip.
    fireEvent.click(screen.getAllByRole("tab")[2]);
    await waitFor(() => expect(window.location.pathname).toBe("/cases/CS0002"));
    await waitFor(() => expect(visibleSection()).toBe("details"));
  });

  // Regression test for bug: at the open-tab cap, clicking one more distinct
  // case silently kept showing a DIFFERENT, already-open tab's content (and
  // even redirected the URL to it) instead of the case that was actually
  // clicked. Cap-behavior no longer has a "refuse the new one" mode at all
  // (see `CaseTabsCapMode`'s own doc comment) — this now exercises the
  // eviction path instead: the new case must open in its OWN new tab
  // (evicting an existing one per the active `CaseTabsCapMode`), not get
  // silently redirected onto a stale already-open tab.
  it("bug 2 — a new distinct case past the cap opens in its own tab (evicting one), never a stale open tab", async () => {
    function AppWithFullTabs() {
      window.history.pushState({}, "", "/cases/CS0000");
      return (
        <BrowserRouter>
          <ErrorBannerProvider>
            <CaseTabsBehaviorProvider>
              <CaseTabsProvider>
                {Array.from({ length: MAX_OPEN_CASE_TABS }, (_, i) => (
                  <Opener key={i} caseId={`CS000${i}`} />
                ))}
                <NavigateButton to="/cases/CS-OVERFLOW" label="click-overflow-case" />
                <CaseTabStripBar />
                <CaseTabsContentHost />
                <Routes>
                  <Route path="/cases/:caseId" element={<CaseDetailRouteSync kind="case" />} />
                </Routes>
              </CaseTabsProvider>
            </CaseTabsBehaviorProvider>
          </ErrorBannerProvider>
        </BrowserRouter>
      );
    }
    render(<AppWithFullTabs />);
    for (let i = 0; i < MAX_OPEN_CASE_TABS; i++) {
      fireEvent.click(screen.getByText(`open-CS000${i}`));
    }
    // The 10 case tabs, plus the permanent pinned "current location" tab.
    await waitFor(() =>
      expect(screen.getAllByRole("tab")).toHaveLength(MAX_OPEN_CASE_TABS + 1),
    );

    // Navigate (as a case-list row click would) to one more, never-opened case.
    fireEvent.click(screen.getByText("click-overflow-case"));

    // The URL must land on the case that was actually clicked — not get
    // silently redirected back to whichever tab was previously active.
    await waitFor(() => expect(window.location.pathname).toBe("/cases/CS-OVERFLOW"));
    await waitFor(() => expect(screen.getByText("CS-OVERFLOW")).toBeInTheDocument());
    // Rendered INSIDE a tab panel now (not the un-tabbed fallback, which no
    // longer exists for a capacity reason) — a genuine new tab, not a stale
    // already-open one repurposed in place.
    expect(
      screen.getByText("CS-OVERFLOW").closest('[data-testid^="case-tab-panel-"]'),
    ).not.toBeNull();
    // Still exactly at the cap: the new tab opened, evicting exactly one of
    // the originally-opened ones to make room (which one is `CaseTabsCapMode`
    // eviction-order detail, not this regression's concern — see
    // `caseTabsReducer.test.ts` for that). Every still-open tab's page stays
    // mounted in the background (keep-alive — see `CaseTabIsolatedRouter`),
    // so an evicted tab's page unmounting is what actually proves it closed,
    // not just the strip's chip count.
    expect(screen.getAllByRole("tab")).toHaveLength(MAX_OPEN_CASE_TABS + 1);
    // (Only the case tabs themselves render `StubCaseDetailPage` — the
    // permanent pinned "current location" tab doesn't — so this is
    // `MAX_OPEN_CASE_TABS`, not `+1`.)
    const openCaseIds = screen
      .getAllByTestId("stub-page-case-id")
      .map((el) => el.textContent);
    expect(openCaseIds).toHaveLength(MAX_OPEN_CASE_TABS);
    expect(openCaseIds).toContain("CS-OVERFLOW");
    const originalCaseIds = Array.from(
      { length: MAX_OPEN_CASE_TABS },
      (_, i) => `CS000${i}`,
    );
    const survivingOriginalCount = originalCaseIds.filter((id) =>
      openCaseIds.includes(id),
    ).length;
    expect(survivingOriginalCount).toBe(MAX_OPEN_CASE_TABS - 1);
  });

  // Regression test for a reported "Back button in Case view is
  // unresponsive" bug: clicking Back from an open case TAB used to do
  // nothing at all, because `CaseTabIsolatedRouter`'s `navigate` only
  // handled staying on the same case (in-place update) or switching to a
  // DIFFERENT case (open/activate a tab) — a target that isn't a case route
  // at all (the case list, the dashboard, ...) fell into the "same case"
  // branch by default and only updated this tab's own LOCAL override state,
  // never the real router. `CaseTabsContentHost` decides whether to show
  // this tab's content at all by comparing the REAL `location.pathname`
  // against a case-route match, so the real URL — and the visible content —
  // never changed: from the user's point of view, the click did nothing.
  it("bug 3 — the Back button from an open case tab actually leaves the case, reaching the real router", async () => {
    // Isolation from the tests above, which also open real tabs
    // (`CaseTabsProvider` persists open tabs to sessionStorage — see its own
    // doc comment) — without this, a leftover tab set (e.g. the cap-eviction
    // test's own MAX_OPEN_CASE_TABS tabs) could evict CS0001 before this
    // test ever gets to click its Back button.
    sessionStorage.clear();
    function AppWithCaseList() {
      window.history.pushState({}, "", "/cases/CS0001");
      return (
        <BrowserRouter>
          <ErrorBannerProvider>
            <CaseTabsBehaviorProvider>
              <CaseTabsProvider>
                <CaseTabStripBar />
                <CaseTabsContentHost />
                <Routes>
                  <Route path="/cases" element={<div>stub-case-list</div>} />
                  <Route path="/cases/:caseId" element={<CaseDetailRouteSync kind="case" />} />
                </Routes>
              </CaseTabsProvider>
            </CaseTabsBehaviorProvider>
          </ErrorBannerProvider>
        </BrowserRouter>
      );
    }
    render(<AppWithCaseList />);
    await waitFor(() =>
      expect(screen.getByTestId("stub-page-case-id")).toHaveTextContent("CS0001"),
    );

    fireEvent.click(screen.getByText("stub-back-button"));

    // The real URL actually moved...
    await waitFor(() => expect(window.location.pathname).toBe("/cases"));
    // ...the list view is what's now showing...
    await waitFor(() => expect(screen.getByText("stub-case-list")).toBeInTheDocument());
    // ...the case tab's own content is hidden (kept mounted in the
    // background per `CaseTabIsolatedRouter`'s keep-alive design, not
    // unmounted or showing on top of the list)...
    const tabPanel = screen.getByTestId("stub-page-case-id").closest(
      '[data-testid^="case-tab-panel-"]',
    );
    expect(tabPanel).toHaveAttribute("hidden");
    // ...but the tab itself is still open, ready to be reactivated — Back
    // doesn't close it.
    expect(screen.getByText("CS0001")).toBeInTheDocument();
  });
});

/**
 * Regression test for the single most consequential behavioral change in
 * this feature: the case-tabs mechanism now defaults to ON. (It originally
 * shipped "beta, off by default" — flipped to on-by-default later per
 * explicit user instruction, once the feature had settled; `false` is now
 * an explicit opt-out, not the shipped default.) A fresh browser/session
 * (empty localStorage) must see the tab strip and the pinned "current
 * location" tab, and open a visited case into its own in-app tab, rather
 * than the old pre-feature plain-full-page-navigation behavior — that
 * older behavior is still reachable (see the explicit-opt-out test below),
 * just no longer what a fresh session sees on its own.
 */
describe("case tabs — default (mode 'on') behavior", () => {
  beforeEach(() => {
    localStorage.removeItem(ENABLED_STORAGE_KEY);
    localStorage.removeItem(CAP_MODE_STORAGE_KEY);
    // Isolation from the tests above (and from each other, between the two
    // `render()` calls in the test below) — this describe block's own
    // fixture state stays independent regardless of what earlier tests, or
    // an earlier render within the SAME test, left behind.
    sessionStorage.clear();
  });

  it("renders the tab strip and pinned tab, opening a visited case into its own tab, on a fresh session", async () => {
    render(<App initialPath="/cases/CS0001" />);
    await waitFor(() => expect(screen.getByTestId("stub-page-case-id")).toBeInTheDocument());
    expect(screen.getByRole("tablist", { name: "Open cases" })).toBeInTheDocument();
    // The case tab, plus the permanent pinned "current location" tab.
    expect(screen.getAllByRole("tab")).toHaveLength(2);
  });

  it("opens each distinct case visited into its own tab, by default, with no opt-in step needed", async () => {
    function AppAtTwoCasesInTurn({ path }: { path: string }) {
      window.history.pushState({}, "", path);
      return (
        <BrowserRouter>
          <ErrorBannerProvider>
            <CaseTabsBehaviorProvider>
              <CaseTabsProvider>
                <CaseTabStripBar />
                <CaseTabsContentHost />
                <Routes>
                  <Route path="/cases/:caseId" element={<CaseDetailRouteSync kind="case" />} />
                </Routes>
              </CaseTabsProvider>
            </CaseTabsBehaviorProvider>
          </ErrorBannerProvider>
        </BrowserRouter>
      );
    }
    const first = render(<AppAtTwoCasesInTurn path="/cases/CS0001" />);
    await waitFor(() =>
      expect(first.getByTestId("stub-page-case-id")).toHaveTextContent("CS0001"),
    );
    expect(first.getByRole("tablist")).toBeInTheDocument();
    expect(first.getAllByRole("tab")).toHaveLength(2);
    first.unmount();

    // A second, distinct case renders the same way, from a clean slate —
    // this clears the FIRST render's own persisted tab (see
    // `CaseTabsContext`'s sessionStorage code) so this app instance doesn't
    // rehydrate CS0001 alongside CS0002; the point here is that EACH
    // instance opens its own visited case by default, not that they share
    // accumulated state (they don't, and never did).
    sessionStorage.clear();
    const second = render(<AppAtTwoCasesInTurn path="/cases/CS0002" />);
    await waitFor(() =>
      expect(second.getByTestId("stub-page-case-id")).toHaveTextContent("CS0002"),
    );
    expect(second.getByRole("tablist")).toBeInTheDocument();
    expect(second.getAllByRole("tab")).toHaveLength(2);
  });

  // Regression test for bug: enabling the mechanism (disabled -> enabled)
  // while already viewing a case, in the SAME commit, used to silently
  // leave that case un-tabbed for the rest of the session — nothing ever
  // retried it. Root cause: `CaseDetailRouteSync` (a descendant of
  // `CaseTabsProvider`) read the fresh `enabled` value directly and called
  // `openTab` from its own effect in that same commit, but React flushes
  // descendant effects before ancestor effects — so `CaseTabsProvider`'s own
  // effect hadn't yet synced its `enabledRef` from `false` to `true`, and
  // `openTab` saw the stale ref and refused. Fixed by having `openTab` read
  // `enabled`/`capMode` directly as `useCallback` dependencies (always
  // fresh for the render that created it) instead of via an effect-synced
  // ref — see `CaseTabsContext`'s own comment on `openTab` for why a
  // render-time ref mutation wasn't a valid alternative here.
  //
  // Starting state is an EXPLICIT opt-out (`ENABLED_STORAGE_KEY` set to
  // `"0"`), not just an absent key — with tabs on by default, an absent key
  // no longer means disabled, so this is what a real user who deliberately
  // turned the mechanism off, then back on, actually goes through.
  it("re-enabling tabs (after an explicit opt-out) while already on a case route opens a tab immediately", async () => {
    function EnableToggle() {
      const { setEnabled } = useCaseTabsBehavior();
      return <button onClick={() => setEnabled(true)}>enable-tabs</button>;
    }
    function AppStartingDisabled({ initialPath }: { initialPath: string }) {
      window.history.pushState({}, "", initialPath);
      return (
        <BrowserRouter>
          <ErrorBannerProvider>
            <CaseTabsBehaviorProvider>
              <CaseTabsProvider>
                <EnableToggle />
                <CaseTabStripBar />
                <CaseTabsContentHost />
                <Routes>
                  <Route path="/cases/:caseId" element={<CaseDetailRouteSync kind="case" />} />
                </Routes>
              </CaseTabsProvider>
            </CaseTabsBehaviorProvider>
          </ErrorBannerProvider>
        </BrowserRouter>
      );
    }

    localStorage.setItem(ENABLED_STORAGE_KEY, "0");
    localStorage.removeItem(CAP_MODE_STORAGE_KEY);
    sessionStorage.clear();

    render(<AppStartingDisabled initialPath="/cases/CS0001" />);
    await waitFor(() => expect(screen.getByTestId("stub-page-case-id")).toBeInTheDocument());
    expect(screen.queryByRole("tablist")).not.toBeInTheDocument();

    fireEvent.click(screen.getByText("enable-tabs"));

    await waitFor(() =>
      expect(screen.getByRole("tablist", { name: "Open cases" })).toBeInTheDocument(),
    );
    expect(screen.getByText("CS0001")).toBeInTheDocument();
  });

  // Regression test for the flip itself: the old pre-feature
  // plain-full-page-navigation behavior (no tab strip, no open-tab
  // bookkeeping) is still reachable — just via an explicit opt-out now,
  // not the default a fresh session sees on its own.
  it("an explicit opt-out (enabled=0) still renders every case directly, never opening a tab", async () => {
    localStorage.setItem(ENABLED_STORAGE_KEY, "0");
    render(<App initialPath="/cases/CS0001" />);
    await waitFor(() => expect(screen.getByTestId("stub-page-case-id")).toBeInTheDocument());
    expect(screen.queryByRole("tablist", { name: "Open cases" })).not.toBeInTheDocument();
    expect(screen.queryAllByRole("tab")).toHaveLength(0);
  });
});
