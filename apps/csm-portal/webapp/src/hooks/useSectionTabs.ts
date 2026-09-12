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
 * Drives a section's second-level tab strip from the navigation tree, so the
 * tabs a deployment is allowed to see are decided in one place
 * (`CSM_PORTAL_FEATURE_OVERRIDES`) rather than per page.
 *
 * Three flavours, matching the tab idioms in the app: `useQueryTabs` for a
 * legacy `?tab=` reader (kept only for the redirect off old Operations/
 * Security Center links — see `usePathSectionTabs`), `usePathSectionTabs` for
 * a section whose tab strip is itself a real path segment (Operations,
 * Security Center), and `useRouteTabs` for sections whose tabs are child
 * routes with their own nested layout (Customers, Settings).
 *
 * A detail page's own static tab strip (Case/Incident/Change Request/Project
 * detail — not driven by the nav tree at all, just a caller-supplied list) is
 * `useQueryParamTabs`, at the bottom of this file.
 */

import { useCallback } from "react";
import { useLocation, useParams, useSearchParams } from "react-router";
import {
  type CsmNavNode,
  navNodeById,
  navNodePath,
} from "@config/csmNavItems";
import {
  type FeatureState,
  enabledNavChildren,
  featureState,
  visibleNavChildren,
} from "@config/featureFlags";
import { useNavTransition } from "@hooks/useNavTransition";
import { useCaseRouteOverride } from "@context/case-tabs/CaseRouteOverrideContext";

/** One rendered tab: the nav node plus the state that decides how it looks. */
export interface SectionTab {
  /** Value the `<Tabs>` strip is keyed by. */
  key: string;
  label: string;
  state: FeatureState;
  node: CsmNavNode;
}

export interface SectionTabsState {
  /** Tabs to render — enabled and WIP ones; hidden tabs are gone entirely. */
  tabs: SectionTab[];
  /** Key of the selected tab, or `""` when the section has no visible tabs. */
  activeKey: string;
  select: (key: string) => void;
}

/**
 * Picks the tab to show. Honours the caller's request only when that tab is
 * usable, so a link to a tab this deployment marked WIP or hidden lands on the
 * first working tab instead of on a dead panel. Falls back to the first visible
 * tab when nothing is enabled, so the strip still renders something.
 */
function resolveActiveKey(tabs: SectionTab[], requested: string | null): string {
  const requestedTab = requested
    ? tabs.find((tab) => tab.key === requested)
    : undefined;
  if (requestedTab?.state === "enabled") return requestedTab.key;
  return (
    tabs.find((tab) => tab.state === "enabled")?.key ?? tabs[0]?.key ?? ""
  );
}

function tabsFor(
  sectionId: string,
  keyOf: (node: CsmNavNode) => string,
): SectionTab[] {
  const section = navNodeById(sectionId);
  if (!section) return [];
  return visibleNavChildren(section).map((node) => ({
    key: keyOf(node),
    label: node.label,
    state: featureState(node.id),
    node,
  }));
}

/** Tab strip for a section that keeps its selection in the `?tab=` query. */
export function useQueryTabs(sectionId: string): SectionTabsState {
  const [searchParams, setSearchParams] = useSearchParams();
  // Query tabs declare their `?tab=` value on the nav node; a node without one
  // can't be selected, so it is keyed by id and simply never matches.
  const tabs = tabsFor(sectionId, (node) => node.tab ?? node.id);
  const activeKey = resolveActiveKey(tabs, searchParams.get("tab"));

  return {
    tabs,
    activeKey,
    select: (key: string) =>
      setSearchParams((prev) => {
        prev.set("tab", key);
        return prev;
      }),
  };
}

/**
 * A node's `?tab=` value (e.g. `service_requests`) converted to the kebab-case
 * path segment `usePathSectionTabs` keys its tabs by (`service-requests`) —
 * the same convention every hand-declared detail route under a path-tab
 * section already uses (`/operations/service-requests/:caseId`,
 * `/security-center/vulnerabilities/:id`, …), so a tab's own landing route and
 * its detail routes share one prefix without the nav tree needing a second,
 * redundant field for it.
 */
function pathTabKey(node: CsmNavNode): string {
  return (node.tab ?? node.id).replace(/_/g, "-");
}

/**
 * Tab strip for a section whose tab strip is itself a real path segment
 * (`/operations/:tab`, `/security-center/:tab`) rather than a `?tab=` query —
 * see the module doc comment for why these two sections get a path segment.
 * Reuses `tabsFor`/`resolveActiveKey` — the exact same nav-tree/feature-flag
 * gating `useQueryTabs` applies — so a WIP or hidden tab behaves identically
 * either way; only where the active tab is read from (`useParams` vs
 * `useSearchParams`) and how selecting one navigates differ.
 */
export function usePathSectionTabs(
  sectionId: string,
  basePath: string,
): SectionTabsState {
  const { tab: rawTab } = useParams();
  const navigate = useNavTransition();
  const tabs = tabsFor(sectionId, pathTabKey);
  const activeKey = resolveActiveKey(tabs, rawTab ?? null);

  return {
    tabs,
    activeKey,
    select: (key: string) => navigate(`${basePath}/${key}`),
  };
}

/**
 * The first usable tab's path segment for a `usePathSectionTabs` section —
 * the path-segment analogue of `firstEnabledTabHref` (which returns the
 * legacy `?tab=` href and so is only still used by `useRouteTabs` sections'
 * `SectionIndexRedirect`). `undefined` when every tab is restricted, same as
 * `firstEnabledTabHref`.
 */
export function firstEnabledPathTab(sectionId: string): string | undefined {
  return enabledPathTabKeys(sectionId)[0];
}

/**
 * Every usable tab's path segment for a `usePathSectionTabs` section, in nav
 * order — lets a caller (the legacy `?tab=` redirect) check whether a
 * requested key names a tab this deployment actually offers, not just fetch
 * the first one.
 */
export function enabledPathTabKeys(sectionId: string): string[] {
  const section = navNodeById(sectionId);
  if (!section) return [];
  return enabledNavChildren(section).map(pathTabKey);
}

/** Tab strip for a section whose tabs are child routes. */
export function useRouteTabs(sectionId: string): SectionTabsState {
  const { pathname } = useLocation();
  const navigate = useNavTransition();
  const tabs = tabsFor(sectionId, (node) => navNodePath(node));

  const current = tabs.find(
    (tab) => pathname === tab.key || pathname.startsWith(`${tab.key}/`),
  );
  const activeKey = resolveActiveKey(tabs, current?.key ?? null);

  return {
    tabs,
    activeKey,
    select: (key: string) => void navigate(key),
  };
}

/**
 * Where a section's index route should land: its first usable tab. Returns
 * `undefined` when every tab is restricted, leaving the caller to decide.
 */
export function firstEnabledTabHref(sectionId: string): string | undefined {
  const section = navNodeById(sectionId);
  return section ? enabledNavChildren(section)[0]?.href : undefined;
}

/**
 * A detail page's own static tab strip, kept in a `?tab=` query param — for a
 * page whose tabs are NOT nav-tree driven (Case/Incident/Change
 * Request/Project detail): the caller supplies its own fixed tab id list, not
 * one resolved from `CSM_PORTAL_FEATURE_OVERRIDES`.
 */
export interface QueryParamTabsState<TId extends string> {
  activeTab: TId;
  /**
   * `replace: true` (the default) swaps the current history entry rather than
   * pushing a new one, matching how every existing hand-rolled `?tab=` reader
   * in this app behaved before this hook existed — switching tabs is not a
   * distinct back-button stop. Pass `replace: false` for the rare case where
   * it should be (none of this app's current callers need it).
   */
  setActiveTab: (next: TId, options?: { replace?: boolean }) => void;
}

/**
 * Reads/writes a caller-supplied tab id in `searchParams.get(paramName)`
 * (`"tab"` by default), preserving every other existing search param (filters,
 * pagination, an unrelated page's own params) — the update is always applied
 * against the *current* `URLSearchParams`, never a fresh one, so nothing
 * unrelated is clobbered. A missing or unrecognised value falls back to
 * `defaultTab` without writing anything back to the URL itself (so a
 * bookmarked/shared link with a stale tab value doesn't get silently
 * rewritten out from under whoever shared it) — this can never loop, since
 * resolving the fallback is a pure read, not a navigation.
 *
 * `clearParamsOnChange` drops other params (e.g. a nested sub-tab's own query
 * param) whenever the tab itself changes, since a sub-tab selection made
 * under a *different* parent tab no longer means anything once you've
 * switched away from it.
 *
 * Override-aware: when called from inside an open in-app case tab (a
 * `CaseTabIsolatedRouter` instance — see `CaseRouteOverrideContext`), this
 * reads/writes that tab's OWN `search` string and navigates through its own
 * `navigate`, instead of the real, single, app-wide `useSearchParams()`.
 * Without this, every open tab shared the same real `?tab=` query param —
 * two case tabs open on different sections (one on "Details", one on
 * "Activities") would fight over it, and switching between them could reset
 * whichever one wasn't just written to back to its default section. Outside
 * a tab (the override is `undefined` — a directly-routed page, or any page
 * that isn't part of the case-tabs mechanism at all) this behaves exactly as
 * before, against the real router.
 */
export function useQueryParamTabs<TId extends string>(
  tabs: readonly TId[],
  defaultTab: TId,
  options: { paramName?: string; clearParamsOnChange?: readonly string[] } = {},
): QueryParamTabsState<TId> {
  const { paramName = "tab", clearParamsOnChange = [] } = options;
  const routeOverride = useCaseRouteOverride();
  // Real router hooks — called unconditionally regardless of `routeOverride`
  // (rules of hooks), same pattern as `CsmCaseDetailPage`'s own top-level
  // override check; their values are simply unused when an override is
  // present.
  const [routedSearchParams, setRoutedSearchParams] = useSearchParams();

  const activeSearchParams = routeOverride
    ? new URLSearchParams(routeOverride.search)
    : routedSearchParams;
  const raw = activeSearchParams.get(paramName);
  const activeTab: TId =
    raw && (tabs as readonly string[]).includes(raw) ? (raw as TId) : defaultTab;

  const setActiveTab = useCallback(
    (next: TId, setOptions?: { replace?: boolean }): void => {
      if (routeOverride) {
        const params = new URLSearchParams(routeOverride.search);
        params.set(paramName, next);
        for (const dropped of clearParamsOnChange) params.delete(dropped);
        const nextSearch = params.toString();
        routeOverride.navigate(
          {
            pathname: routeOverride.pathname,
            search: nextSearch ? `?${nextSearch}` : "",
            hash: routeOverride.hash,
          },
          { replace: setOptions?.replace ?? true, state: routeOverride.state },
        );
        return;
      }
      setRoutedSearchParams(
        (prev) => {
          const params = new URLSearchParams(prev);
          params.set(paramName, next);
          for (const dropped of clearParamsOnChange) params.delete(dropped);
          return params;
        },
        { replace: setOptions?.replace ?? true },
      );
    },
    // clearParamsOnChange is passed fresh by most callers (an inline array
    // literal), so it's deliberately excluded here — including it would
    // rebuild setActiveTab (and anything memoized on it) on every render for
    // those callers, defeating the point of memoizing it at all. Every
    // current caller passes a `const` module-level array, so this is safe in
    // practice; a caller with a genuinely dynamic clear list should build one
    // itself rather than relying on this hook to react to it.
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [routeOverride, setRoutedSearchParams, paramName],
  );

  return { activeTab, setActiveTab };
}
