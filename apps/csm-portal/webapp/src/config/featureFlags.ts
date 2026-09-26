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
 * Per-page feature flags, resolved against the navigation tree.
 *
 * A deployment lists nav-node ids it wants to restrict in the runtime
 * `CSM_PORTAL_FEATURE_OVERRIDES` map (`public/config.js`):
 *
 * ```js
 * CSM_PORTAL_FEATURE_OVERRIDES: {
 *   "operations": "wip",
 *   "operations.incidents": "enabled",
 *   "admin.roles": "hidden",
 * }
 * ```
 *
 * Anything absent from that map is a normally working feature — no entry means
 * enabled, so a page only ever needs configuring when it should be restricted.
 * Ids are the dotted `CsmNavNode.id` values in `csmNavItems.ts`; the map is
 * deliberately flat (one level, dotted keys) so there is no deep-merge
 * behaviour to reason about and a config diff between environments stays
 * readable.
 */

import {
  CSM_NAV_ITEMS,
  type CsmNavNode,
  flattenNavNodes,
  navNodeForPath,
} from "@config/csmNavItems";
import type { ComponentType } from "react";
import type { PortalAccess } from "@context/current-user/portalAccess";

/**
 * How a page behaves in this deployment.
 *
 * - `enabled` — normal. The default for anything not listed in the overrides.
 * - `wip` — advertised but not usable: shown in the sidebar/tab strip greyed
 *   out with a "work in progress" tooltip, dropped from Quick-nav, and its
 *   routes render the shared "coming soon" page.
 * - `hidden` — absent: no nav entry anywhere, and its routes redirect away.
 */
export type FeatureState = "enabled" | "wip" | "hidden";

const FEATURE_STATES: readonly string[] = ["enabled", "wip", "hidden"];

function isFeatureState(value: unknown): value is FeatureState {
  return typeof value === "string" && FEATURE_STATES.includes(value);
}

/**
 * Reads the raw override map. Accepts either an object literal (how
 * `public/config.js` writes it) or a JSON string, because platform config
 * injection often only carries string values.
 */
function readRawOverrides(): unknown {
  const raw = window.config?.CSM_PORTAL_FEATURE_OVERRIDES;
  if (typeof raw !== "string") return raw;
  try {
    return JSON.parse(raw) as unknown;
  } catch {
    console.warn(
      "[featureFlags] CSM_PORTAL_FEATURE_OVERRIDES is not valid JSON; ignoring it.",
    );
    return undefined;
  }
}

/**
 * Validates the raw map, dropping entries that name an unknown page or an
 * unknown state. Both are almost always typos, and a silently ignored typo in a
 * flag that is meant to hide something is the failure mode worth shouting
 * about, so each one is warned about at startup.
 */
function parseOverrides(): Record<string, FeatureState> {
  const raw = readRawOverrides();
  if (typeof raw !== "object" || raw === null || Array.isArray(raw)) return {};

  const knownIds = new Set(flattenNavNodes().map((node) => node.id));
  const parsed: Record<string, FeatureState> = {};

  for (const [id, value] of Object.entries(raw as Record<string, unknown>)) {
    if (!knownIds.has(id)) {
      console.warn(
        `[featureFlags] CSM_PORTAL_FEATURE_OVERRIDES names an unknown page "${id}"; ignoring it.`,
      );
      continue;
    }
    if (!isFeatureState(value)) {
      console.warn(
        `[featureFlags] CSM_PORTAL_FEATURE_OVERRIDES["${id}"] must be one of ${FEATURE_STATES.join(
          ", ",
        )}; ignoring it.`,
      );
      continue;
    }
    parsed[id] = value;
  }

  return parsed;
}

/**
 * Resolves every node in the tree to its effective state.
 *
 * Rules, in order:
 * 1. A `hidden` ancestor hides its whole subtree. A child override cannot
 *    resurrect a page whose section does not exist.
 * 2. Otherwise a node's own override wins.
 * 3. Otherwise it inherits its parent's state, so marking a section `wip`
 *    marks its tabs `wip` without listing each one.
 * 4. Otherwise it is `enabled`.
 * 5. Finally, a `wip` section with at least one `enabled` tab is promoted to
 *    `enabled` — the section has something usable in it, so it must stay
 *    clickable or that tab would be reachable only by deep link.
 */
function computeStates(): Map<string, FeatureState> {
  const overrides = parseOverrides();
  const states = new Map<string, FeatureState>();

  const inherit = (nodes: readonly CsmNavNode[], parent: FeatureState): void => {
    for (const node of nodes) {
      const state: FeatureState =
        parent === "hidden" ? "hidden" : (overrides[node.id] ?? parent);
      states.set(node.id, state);
      if (node.children?.length) inherit(node.children, state);
    }
  };
  inherit(CSM_NAV_ITEMS, "enabled");

  const promote = (nodes: readonly CsmNavNode[]): void => {
    for (const node of nodes) {
      if (!node.children?.length) continue;
      promote(node.children);
      const hasUsableChild = node.children.some(
        (child) => states.get(child.id) === "enabled",
      );
      if (states.get(node.id) === "wip" && hasUsableChild) {
        states.set(node.id, "enabled");
      }
    }
  };
  promote(CSM_NAV_ITEMS);

  return states;
}

let cachedStates: Map<string, FeatureState> | undefined;

function states(): Map<string, FeatureState> {
  cachedStates ??= computeStates();
  return cachedStates;
}

/**
 * Clears the memoised resolution so a test can swap `window.config` between
 * cases. Not for application code — the config is fixed for a page load.
 */
export function resetFeatureStatesForTests(): void {
  cachedStates = undefined;
}

let cachedRequirements: Map<string, keyof PortalAccess> | undefined;

/**
 * The {@link CsmNavNode.requires} capability that applies to each node — its
 * own, or the nearest ancestor's, so a restricted section restricts its tabs.
 */
function requirements(): Map<string, keyof PortalAccess> {
  if (!cachedRequirements) {
    const map = new Map<string, keyof PortalAccess>();
    const walk = (nodes: readonly CsmNavNode[], inherited?: keyof PortalAccess): void => {
      for (const node of nodes) {
        const need = node.requires ?? inherited;
        if (need) map.set(node.id, need);
        if (node.children?.length) walk(node.children, need);
      }
    };
    walk(CSM_NAV_ITEMS);
    cachedRequirements = map;
  }
  return cachedRequirements;
}

/**
 * Effective state of a page. An id that is not in the nav tree resolves to
 * `enabled`: unknown means unrestricted, never accidentally blocked.
 *
 * When `access` is given, a page needing a capability the user lacks resolves
 * to `hidden` for them, whatever the deployment's flags say. Omit it for the
 * deployment-wide state.
 */
export function featureState(id: string, access?: PortalAccess): FeatureState {
  const state = states().get(id) ?? "enabled";
  if (state === "hidden" || !access) return state;
  const need = requirements().get(id);
  return need && !access[need] ? "hidden" : state;
}

/** True when the page should appear in navigation at all (enabled or WIP). */
export function isFeatureVisible(id: string, access?: PortalAccess): boolean {
  return featureState(id, access) !== "hidden";
}

/** True when the page is usable, as opposed to hidden or advertised-but-WIP. */
export function isFeatureEnabled(id: string, access?: PortalAccess): boolean {
  return featureState(id, access) === "enabled";
}

/** Effective state of whichever page owns `pathname`. */
export function featureStateForPath(pathname: string, access?: PortalAccess): FeatureState {
  const node = navNodeForPath(pathname);
  return node ? featureState(node.id, access) : "enabled";
}

/** Top-level sections that should render in the sidebar. */
export function visibleNavSections(access?: PortalAccess): typeof CSM_NAV_ITEMS {
  return CSM_NAV_ITEMS.filter((section) => isFeatureVisible(section.id, access));
}

/** A node's tabs that should render in its tab strip. */
export function visibleNavChildren(node: CsmNavNode, access?: PortalAccess): CsmNavNode[] {
  return (node.children ?? []).filter((child) => isFeatureVisible(child.id, access));
}

/** A node's tabs that are actually usable. */
export function enabledNavChildren(node: CsmNavNode, access?: PortalAccess): CsmNavNode[] {
  return (node.children ?? []).filter((child) => isFeatureEnabled(child.id, access));
}

/** A page the Quick-nav palette can offer as a destination. */
export interface NavigableNavNode {
  id: string;
  label: string;
  /** Owning section's label, for a second-level tab. */
  sublabel?: string;
  href: string;
  icon: ComponentType<{ size?: number | string }>;
}

/**
 * Every enabled descendant of `node`, flattened, each carrying its immediate
 * parent's label as its sublabel (so a grandchild reads as "Users / User
 * management" rather than "Users / Settings") and inheriting an icon down the
 * chain until a node declares its own.
 */
function navigableDescendants(
  node: CsmNavNode,
  icon: ComponentType<{ size?: number | string }>,
  access?: PortalAccess,
): NavigableNavNode[] {
  return enabledNavChildren(node, access).flatMap((child) => {
    const self: NavigableNavNode = {
      id: child.id,
      label: child.label,
      sublabel: node.label,
      href: child.href,
      icon: child.icon ?? icon,
    };
    return [self, ...navigableDescendants(child, self.icon, access)];
  });
}

/**
 * Every enabled destination — sections and every level of their tabs —
 * flattened for the Quick-nav palette. Tabs inherit their parent's icon and
 * carry its label as a sublabel so "Users" reads as "Users / User management"
 * rather than as a bare word.
 */
export function navigableNavNodes(access?: PortalAccess): NavigableNavNode[] {
  return CSM_NAV_ITEMS.flatMap((section) => {
    if (!isFeatureEnabled(section.id, access)) return [];
    const self: NavigableNavNode = {
      id: section.id,
      label: section.label,
      href: section.href,
      icon: section.icon,
    };
    return [self, ...navigableDescendants(section, section.icon, access)];
  });
}

/**
 * Where to send someone who lands on a hidden page: the first destination this
 * deployment actually offers. `undefined` when the config hides everything —
 * callers must handle that rather than redirect into a page that is itself
 * hidden, which would bounce the router between two hidden paths forever.
 */
export function firstEnabledDestination(access?: PortalAccess): string | undefined {
  return navigableNavNodes(access)[0]?.href;
}
