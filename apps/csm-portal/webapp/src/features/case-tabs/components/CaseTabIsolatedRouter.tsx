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

import {
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
  type JSX,
  type ReactNode,
} from "react";
import type { NavigateOptions, To } from "react-router";
import { matchCaseLocation } from "@context/case-tabs/caseRoutePatterns";
import { useCaseTabsControllerRef } from "@context/case-tabs/CaseTabsContext";
import { CaseRouteOverrideProvider } from "@context/case-tabs/CaseRouteOverrideContext";
import type { CaseRouteKind, CaseTabState } from "@context/case-tabs/caseTabsTypes";
import { tabElementId, tabPanelElementId } from "@features/case-tabs/utils/tabElementIds";
import { useNavTransition } from "@hooks/useNavTransition";

function toHref(to: To): string {
  if (typeof to === "string") return to;
  return `${to.pathname ?? ""}${to.search ?? ""}${to.hash ?? ""}`;
}

/** Splits a concrete `CaseTabState.path` (e.g. "/cases/CS1?tab=activities#x")
 * into the pieces `CaseRouteOverrideValue` wants broken out — shared by this
 * tab's initial seed and its outside-reactivation resync (see the effect
 * below) so both derive `routeState` from `tab.path` identically. */
function parseTabPath(path: string): { pathname: string; search: string; hash: string } {
  const [pathnameAndSearch, hash = ""] = path.split("#");
  const [pathname, search = ""] = pathnameAndSearch.split("?");
  return {
    pathname,
    search: search ? `?${search}` : "",
    hash: hash ? `#${hash}` : "",
  };
}

export interface CaseTabIsolatedRouterProps {
  tab: CaseTabState;
  isVisible: boolean;
  /** The page to render for this tab — `CsmCaseDetailPage` in production,
   * swappable in tests so this component's mechanics can be verified without
   * pulling in the real (very large) page. */
  children: ReactNode;
}

/**
 * Mounts one case-detail page "kept alive" in the background, giving it its
 * own private `caseId`/location/navigate — WITHOUT a second react-router
 * `<Router>`. react-router refuses to render a `<Router>` inside another
 * `<Router>` (an unconditional invariant), and the app already has exactly
 * one (`<BrowserRouter>`, in `App.tsx`) — an earlier version of this
 * component tried exactly that (a low-level `<Router>` per tab) and crashed
 * the moment any case was opened for it. See `CaseRouteOverrideContext`'s
 * own doc comment for the full explanation of why a plain Context works
 * where a second Router cannot: `CsmCaseDetailPage` still reads the REAL
 * `useParams`/`useLocation`/`useNavigate` (there is only ever one, real,
 * app-wide match), it just prefers this override's values when one is
 * present in context.
 *
 * In-page navigation (the misrouted-case redirect, the dashless-id repair in
 * `useNormalizedIdParam`, the header's own Back button, ...) is intercepted
 * by the `navigate` function passed through the override rather than
 * reaching the real browser history:
 *   - if it resolves to the SAME caseId this tab represents, the tab's own
 *     `path`/`kind` are updated in place (`updateTabPath`) — covers the
 *     redirect/repair cases, and keeps this tab's identity stable.
 *   - if it resolves to a DIFFERENT caseId (e.g. following a related-case
 *     link — though in practice those render as real react-router `<Link>`s
 *     bound to the real router regardless of which tab they're clicked from,
 *     so they already open/activate a tab via `CaseDetailRouteSync` before
 *     ever reaching this code path), it is treated as opening a new tab (or
 *     activating an existing one for that case) rather than retargeting this
 *     one — avoiding the need to ever change a tab's React key mid-life
 *     (which would force a real remount and defeat the point of this
 *     component).
 *   - if it doesn't resolve to a case route AT ALL — leaving the case
 *     entirely, e.g. the page's own Back button returning to the case list/
 *     dashboard, or a "create new" shortcut — it must reach the REAL router,
 *     not just this tab's local override state: `CaseTabsContentHost` decides
 *     whether to show this tab's content at all by comparing the REAL
 *     `location.pathname` against a case-route match, so updating only the
 *     override's internal `pathname` here left the real URL (and so that
 *     visibility check) completely unchanged — Back looked unresponsive
 *     because, from the real router's point of view, nothing had moved. The
 *     tab itself is left open (not closed) so it's still available to
 *     reactivate from the tab strip, matching how leaving a case via the
 *     pinned "current location" tab or the app nav already behaves.
 *
 * The active tab's real-URL sync (so a reload/bookmark on `/cases/:id`
 * still works — see this feature's design notes) is owned by the caller
 * (`CaseTabsWorkspace`), not here: this component never touches
 * `window.location` itself.
 */
export default function CaseTabIsolatedRouter({
  tab,
  isVisible,
  children,
}: CaseTabIsolatedRouterProps): JSX.Element {
  const initialPath = useMemo(() => parseTabPath(tab.path), [tab.path]);

  const [routeState, setRouteState] = useState<{
    pathname: string;
    search: string;
    hash: string;
    kind: CaseRouteKind;
    state: unknown;
  }>({ ...initialPath, kind: tab.kind, state: tab.state });

  const controllerRef = useCaseTabsControllerRef();

  // Re-syncs `routeState` (and so the override this tab's page actually
  // reads) from `tab.path`/`tab.kind`/`tab.state` whenever those change from
  // OUTSIDE this tab's own `navigate` below — i.e. an outside navigation
  // that REACTIVATES this already-open tab with different route info (a
  // bookmark or a related-case link to the same case but a different
  // `?tab=` section or `#hash` — see `caseTabsReducer`'s `OPEN_OR_ACTIVATE`,
  // which already updates the TAB RECORD correctly for this). Without this,
  // that update never reached the actually-mounted page: the address bar
  // and the tab record both showed the new section, but this component's
  // own `routeState` — seeded once and otherwise only ever written by this
  // tab's OWN in-tab `navigate` — kept serving the OLD `search`/`hash` to
  // `CsmCaseDetailPage`'s `useSectionTabs`-backed tab strip.
  //
  // Render-time state adjustment (not a `useEffect` — this repo's lint
  // config forbids `setState` inside one, same `react-hooks/set-state-in-
  // effect` class of constraint as `CaseTabsContext`'s own `openTab` fix
  // earlier this session; same pattern `useCurrentLocationTab` already uses
  // elsewhere in this feature): compares `tab.path`/`kind`/`state` against
  // the CURRENT `routeState` synchronously within this render, and calls
  // `setRouteState` directly — not in an effect — whenever they differ. A
  // no-op for in-tab navigation itself: `navigate` below already updates
  // `routeState` directly to the identical derived values it also reports
  // to the controller via `updateTabPath`, so this comparison already finds
  // them equal by the time it runs.
  const parsedFromTab = parseTabPath(tab.path);
  if (
    routeState.pathname !== parsedFromTab.pathname ||
    routeState.search !== parsedFromTab.search ||
    routeState.hash !== parsedFromTab.hash ||
    routeState.kind !== tab.kind ||
    routeState.state !== tab.state
  ) {
    setRouteState({ ...parsedFromTab, kind: tab.kind, state: tab.state });
  }

  // `tab.id` and `tab.caseId` are both invariant for the lifetime of a given
  // tab instance: this component is keyed by `tab.id` (never changes by
  // definition), and an in-page navigation to a DIFFERENT case is handled by
  // opening/activating a different tab (see `navigate` below) rather than
  // ever retargeting this one — so `tab.caseId` never changes here either.
  // Safe to close over both directly in the memo below (with the lint
  // suppression that implies) instead of keeping them in refs.
  const realNavigate = useNavTransition();

  const navigate = useMemo(() => {
    return (to: To | number, options?: NavigateOptions): void => {
      if (typeof to === "number") {
        // No independent back/forward stack per in-app tab; not meaningful
        // here (nothing in CsmCaseDetailPage calls navigate(-1)/(1) today).
        return;
      }
      const href = toHref(to);
      const pathname = href.split(/[?#]/)[0];
      const search = href.includes("?") ? `?${href.split("?")[1]?.split("#")[0]}` : "";
      const hash = href.includes("#") ? `#${href.split("#")[1]}` : "";
      const match = matchCaseLocation(pathname);
      if (!match) {
        // Leaving this case entirely (e.g. the page's own Back button, or a
        // "create new" shortcut) — this MUST reach the real router. See this
        // component's own doc comment above for why updating only the local
        // override state here left Back looking completely unresponsive:
        // `CaseTabsContentHost` gates this tab's visibility on the REAL
        // `location.pathname`, which never moved otherwise. The tab stays
        // open in the background (not closed) for the strip to reactivate.
        realNavigate(to, options);
        return;
      }
      if (match.caseId === tab.caseId) {
        setRouteState(() => ({
          pathname,
          search,
          hash,
          kind: match.kind,
          state: options?.state,
        }));
        controllerRef.current.updateTabPath(tab.id, match.kind, href);
        return;
      }
      // Different case referenced from inside this tab: open/activate it as
      // its own tab, leave this tab exactly where it was.
      controllerRef.current.openTab(match.caseId, match.kind, href, options?.state);
    };
    // Stable for the lifetime of this tab instance — reads the latest
    // controller via `controllerRef` rather than depending on it directly.
    // `realNavigate` (from `useNavTransition`) is itself stable for the
    // lifetime of the app, so including it doesn't defeat that.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [realNavigate]);

  const overrideValue = useMemo(
    () => ({
      caseId: tab.caseId,
      kind: routeState.kind,
      pathname: routeState.pathname,
      search: routeState.search,
      hash: routeState.hash,
      state: routeState.state,
      navigate,
    }),
    [tab.caseId, routeState, navigate],
  );

  // This panel is its OWN scroll container — not just a flex child of
  // `AppLayout`'s single shared `mainContentRef` region (which every
  // non-tab page scrolls via, and which `AppLayout` resets to the top on
  // every route change, tab switches included). Without this, a tab's
  // scroll offset lived on `mainContentRef` itself, which is the SAME DOM
  // node for every tab — so switching away and back always came back at the
  // top, even though the tab's other state (drafts, open dialogs) correctly
  // persisted (this component is kept alive, never unmounted, purely
  // toggling `display`/`hidden`).
  //
  // Giving each tab its own scrolling element is necessary but NOT
  // sufficient on its own: per the CSSOM View spec, an element with no
  // layout box (`display: none`, which this panel gets while hidden) has no
  // defined `scrollTop`, and browsers are inconsistent about restoring it
  // once the element becomes visible again — some do, some silently reset
  // it to 0 (see the WPT test `scrollTop-display-change.html`; this is
  // implementation-defined, not standardized). So the position is captured
  // and reapplied explicitly instead of trusting the browser to remember it
  // through the toggle: `handleScroll` keeps `savedScrollTopRef` current
  // continuously (not just at the moment this panel hides — by the time a
  // `display: none` commit has already landed, this panel's own `scrollTop`
  // may already be unreadable/zeroed per the above, so there is no reliable
  // "capture on hide" moment to hook), and the layout effect below writes
  // that saved value back onto the real DOM node the instant this panel
  // becomes visible again, overwriting whatever the browser did or didn't
  // preserve on its own.
  const panelRef = useRef<HTMLDivElement>(null);
  const savedScrollTopRef = useRef(0);
  const wasVisibleRef = useRef(isVisible);

  useLayoutEffect(() => {
    if (isVisible && !wasVisibleRef.current && panelRef.current) {
      panelRef.current.scrollTop = savedScrollTopRef.current;
    }
    wasVisibleRef.current = isVisible;
  }, [isVisible]);

  const handleScroll = (): void => {
    if (panelRef.current) savedScrollTopRef.current = panelRef.current.scrollTop;
  };

  return (
    <div
      ref={panelRef}
      hidden={!isVisible}
      // `id`/`role="tabpanel"`/`aria-labelledby` complete the standard ARIA
      // tabs wiring `CaseTabStrip`'s own chip starts (its `id`/
      // `aria-controls`, built from these exact same helpers) — this is what
      // makes the pairing a real `tablist`/`tab`/`tabpanel` relationship, not
      // just visually-tab-shaped chips. `data-testid` kept identical to the
      // existing `id` (this codebase's tests already select by it) rather
      // than introducing a second, parallel identifier for the same node.
      id={tabPanelElementId(tab.id)}
      role="tabpanel"
      aria-labelledby={tabElementId(tab.id)}
      data-testid={tabPanelElementId(tab.id)}
      onScroll={handleScroll}
      style={{
        display: isVisible ? "flex" : "none",
        flexDirection: "column",
        flex: 1,
        minHeight: 0,
        overflowY: "auto",
      }}
    >
      <CaseRouteOverrideProvider value={overrideValue}>{children}</CaseRouteOverrideProvider>
    </div>
  );
}
