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
 * Four flavours, matching the tab idioms in the app: `useQueryTabs` for
 * sections that still keep the selection in `?tab=`, `usePathSectionTabs` for
 * a nav-tree-driven section whose selection lives in a real URL path segment
 * instead (Operations, Security Center), `useRouteTabs` for sections whose
 * tabs are child routes (Customers, Settings), and `usePathTabs` for a single
 * detail page whose own sub-tabs are real URL path segments
 * (Case/Incident/Change-Request detail) so a tab can be linked and reopens
 * directly on it.
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
 * Tab strip for a section whose selection lives in a URL path segment rather
 * than in `?tab=` (Operations, Security Center) — a bookmarked or shared link
 * reopens on the exact tab. Unlike `usePathTabs`, the tab list itself still
 * comes from the nav tree via `tabsFor`/`resolveActiveKey`, same as
 * `useQueryTabs`, so WIP/hidden gating and the fallback-to-first-enabled-tab
 * behaviour are identical between the two — only where the selection is read
 * from and written to differs.
 *
 * `basePath` is the section's own route root (e.g. `/operations`); the tab
 * segment is expected directly under it, matching a `${basePath}/:tab?`
 * route registered in `App.tsx`. Keys match `useQueryTabs`'s (`node.tab ??
 * node.id`) so a caller can translate a legacy `?tab=<key>` value straight
 * into `${basePath}/<key>` without any remapping.
 */
export function usePathSectionTabs(
  sectionId: string,
  basePath: string,
): SectionTabsState {
  const { tab } = useParams<{ tab?: string }>();
  const navigate = useNavTransition();
  const tabs = tabsFor(sectionId, (node) => node.tab ?? node.id);
  const activeKey = resolveActiveKey(tabs, tab ?? null);

  return {
    tabs,
    activeKey,
    select: (key: string) => void navigate(`${basePath}/${key}`),
  };
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

/** `usePathTabs`'s return shape: the resolved tab plus how to switch it. */
export interface PathTabsState<TId extends string> {
  /** The URL's tab segment when it names one of `tabs`, otherwise
   * `defaultTabId` — always a member of `tabs`, never a raw/unknown segment. */
  activeTab: TId;
  /**
   * Navigates to `${basePath}/${id}`, preserving the current hash (so a
   * permalink fragment like `#comment-5` survives a tab correction) but not
   * the search string. `replace` is for a programmatic correction (e.g.
   * falling back off a tab this data/gating doesn't allow) so it doesn't
   * grow history the way a genuine tab click should.
   */
  setActiveTab: (id: TId, options?: { replace?: boolean }) => void;
}

/**
 * Tab strip for a single detail page whose tabs are real URL path segments,
 * e.g. `/cases/:caseId/:tab?` — a bookmarked or shared link reopens on the
 * exact tab. Unlike `useQueryTabs`/`useRouteTabs` above (which read the
 * *section* nav tree), this is generic over any caller-supplied tab id list,
 * for a single page's own sub-tabs rather than a nav-driven section strip.
 *
 * `basePath` is passed in already resolved (e.g. a case detail page computes
 * it per mount point — `/cases/:id`, `/engagements/:id`, etc.) rather than
 * hardcoded here, so this hook has no opinion on the route pattern it's used
 * under — including a later two-level nesting like
 * `/projects/:id/work-items/:subTab`, where `basePath` would itself already
 * contain the resolved `:subTab`.
 *
 * An unknown or missing tab segment resolves to `defaultTabId` for rendering
 * only — this never itself navigates/redirects the URL, so there's no risk of
 * a redirect loop; callers that want the URL corrected (e.g. a tab that
 * turns out to be gated once data loads) do so explicitly via `setActiveTab`.
 */
export function usePathTabs<TId extends string>(
  basePath: string,
  tabs: readonly TId[],
  defaultTabId: TId,
): PathTabsState<TId> {
  const { tab } = useParams<{ tab?: string }>();
  const { hash } = useLocation();
  const navigate = useNavTransition();

  const activeTab: TId =
    tab && (tabs as readonly string[]).includes(tab) ? (tab as TId) : defaultTabId;

  const setActiveTab = useCallback(
    (id: TId, options?: { replace?: boolean }) => {
      navigate({ pathname: `${basePath}/${id}`, hash }, { replace: options?.replace });
    },
    [basePath, hash, navigate],
  );

  return { activeTab, setActiveTab };
}

/**
 * Where a section's index route should land: its first usable tab. Returns
 * `undefined` when every tab is restricted, leaving the caller to decide.
 */
export function firstEnabledTabHref(sectionId: string): string | undefined {
  const section = navNodeById(sectionId);
  return section ? enabledNavChildren(section)[0]?.href : undefined;
}
