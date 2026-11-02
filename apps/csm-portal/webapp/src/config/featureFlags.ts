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
  isNavShortcut,
  navNodeById,
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
 *    marks its tabs `wip` without listing each one. A shortcut (see
 *    `CsmNavNode.shortcutTo`) is the exception: it is not one of its section's
 *    tabs, so it does not inherit a `wip` section's state (a `hidden` section
 *    still hides it, rule 1). Whether a WIP section offers its shortcut at all
 *    is decided per user, see `isShortcutOffered`.
 * 4. Otherwise it is `enabled`.
 * 5. Finally, a `wip` section with at least one `enabled` tab is promoted to
 *    `enabled` — the section has something usable in it, so it must stay
 *    clickable or that tab would be reachable only by deep link. A shortcut
 *    does not count as a tab here either.
 */
function computeStates(): Map<string, FeatureState> {
  const overrides = parseOverrides();
  const states = new Map<string, FeatureState>();

  const inherit = (nodes: readonly CsmNavNode[], parent: FeatureState): void => {
    for (const node of nodes) {
      const inherited: FeatureState = isNavShortcut(node) ? "enabled" : parent;
      const state: FeatureState =
        parent === "hidden" ? "hidden" : (overrides[node.id] ?? inherited);
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
        (child) => !isNavShortcut(child) && states.get(child.id) === "enabled",
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
 * True when some node under (but not including) `node` has an active preview
 * override. Lets a `hidden`/`wip` ancestor get promoted too when the admin
 * has only switched on preview for one of its descendants: `listPreviewableFeatures`
 * offers every restricted node individually, tabs included, so an admin can
 * (and reasonably would) preview just `"operations.incidents"` without also
 * previewing its `"operations"` section — but `visibleNavSections`/
 * `navigableNavNodes` filter a section out of the tree before ever looking at
 * its children, so a previewed tab under a still-hidden section would
 * otherwise be enabled in {@link featureState} yet unreachable from anywhere
 * in the UI. Only promotes the ancestor itself; a sibling tab with no preview
 * of its own stays exactly as restricted as before.
 *
 * A shortcut (see `CsmNavNode.shortcutTo`) does not count, for the same reason
 * it does not count in `computeStates`' own promotion (rule 5): it is not one
 * of its section's tabs, and it has no preview toggle of its own (see
 * {@link listPreviewableFeatures}), so a stray stored value for one must not
 * surface its section.
 */
function hasPreviewedDescendant(
  node: CsmNavNode | undefined,
  previewOverrides: Record<string, boolean> | undefined,
): boolean {
  if (!node?.children?.length || !previewOverrides) return false;
  return node.children.some(
    (child) =>
      !isNavShortcut(child) &&
      (previewOverrides[child.id] === true ||
        hasPreviewedDescendant(child, previewOverrides)),
  );
}

/**
 * Effective state of a page. An id that is not in the nav tree resolves to
 * `enabled`: unknown means unrestricted, never accidentally blocked.
 *
 * When `access` is given, a page needing a capability the user lacks resolves
 * to `hidden` for them, whatever the deployment's flags say. Omit it for the
 * deployment-wide state.
 *
 * `previewOverrides` (the Feature Preview dialog's per-admin, browser-local
 * toggles — see `featurePreview.ts`) can PROMOTE a `hidden`/`wip` node to
 * `enabled` for that one admin, but can never demote anything and can never
 * bypass a genuine capability failure: a node whose `requires` capability the
 * caller lacks resolves to `hidden` regardless of any override. Promotion
 * also requires `access.isAdmin` — this is deliberately not reachable by
 * anyone else, and never touches the deployment-wide `states()` map itself,
 * so every other caller (every call site that doesn't pass this third
 * argument) sees exactly the same result as before this parameter existed.
 *
 * A node's own override promotes it directly; failing that, a previewed
 * descendant (see {@link hasPreviewedDescendant}) promotes it too, so
 * previewing a tab under a hidden section surfaces the section it needs to
 * be reached through, not just the tab's own (otherwise unreachable) state.
 *
 * A shortcut (see `CsmNavNode.shortcutTo`) resolves to the narrower of its
 * own state and its owner's. It has no preview toggle of its own (see
 * {@link listPreviewableFeatures}), so `previewOverrides` reaches it only
 * through its owner — and, being the narrower of the two, never lifts it past
 * its own `requires` or deployment state.
 */
export function featureState(
  id: string,
  access?: PortalAccess,
  previewOverrides?: Record<string, boolean>,
): FeatureState {
  const owner = shortcutOwners().get(id);
  if (owner === undefined) return ownFeatureState(id, access, previewOverrides);
  return narrower(ownFeatureState(id, access), ownFeatureState(owner, access, previewOverrides));
}

/** {@link featureState} for one node, before a shortcut is narrowed to its owner's state. */
function ownFeatureState(
  id: string,
  access?: PortalAccess,
  previewOverrides?: Record<string, boolean>,
): FeatureState {
  const state = states().get(id) ?? "enabled";
  const need = requirements().get(id);
  const capabilityOk = !access || !need || access[need];
  if (!capabilityOk) return "hidden";
  if (state !== "hidden" && state !== "wip") return state;
  if (!access?.isAdmin) return state;
  if (previewOverrides?.[id] === true) return "enabled";
  if (hasPreviewedDescendant(navNodeById(id), previewOverrides)) return "enabled";
  return state;
}

const STATE_RANK: Record<FeatureState, number> = { enabled: 0, wip: 1, hidden: 2 };

/** The more restrictive of two states: hidden over wip over enabled. */
function narrower(a: FeatureState, b: FeatureState): FeatureState {
  return STATE_RANK[a] >= STATE_RANK[b] ? a : b;
}

interface ShortcutLinks {
  /** Shortcut id → the id of the node that owns its page. */
  owners: Map<string, string>;
  /** Shortcut id → the id of the section it is listed under. */
  sections: Map<string, string>;
}

let cachedShortcutLinks: ShortcutLinks | undefined;

/**
 * Where each shortcut (see {@link CsmNavNode.shortcutTo}) points and where it
 * sits. A shortcut is never more visible than its owner: hiding or WIP-ing the
 * owner through a deployment override, or a role that cannot reach the owner,
 * applies to the shortcut too.
 */
function shortcutLinks(): ShortcutLinks {
  if (!cachedShortcutLinks) {
    const links: ShortcutLinks = { owners: new Map(), sections: new Map() };
    const walk = (nodes: readonly CsmNavNode[], parentId?: string): void => {
      for (const node of nodes) {
        if (node.shortcutTo) {
          links.owners.set(node.id, node.shortcutTo);
          if (parentId) links.sections.set(node.id, parentId);
        }
        if (node.children?.length) walk(node.children, node.id);
      }
    };
    walk(CSM_NAV_ITEMS);
    cachedShortcutLinks = links;
  }
  return cachedShortcutLinks;
}

function shortcutOwners(): Map<string, string> {
  return shortcutLinks().owners;
}

/**
 * True when `node` is a shortcut this user is actually offered in the
 * sidebar: it is visible for them, and the section it sits in is `enabled`
 * for them — a WIP section renders as one disabled entry with no dropdown, so
 * a shortcut inside it could never be reached, and its owner must keep its own
 * entry instead.
 *
 * `previewOverrides` is passed through to both checks, so for an admin the
 * section's state here is the same one the sidebar renders it with. It cannot
 * change anything for anyone else: every preview promotion is admin-only (see
 * {@link featureState}).
 */
function isShortcutOffered(
  node: CsmNavNode,
  access: PortalAccess,
  previewOverrides?: Record<string, boolean>,
): boolean {
  if (!isNavShortcut(node) || !isFeatureVisible(node.id, access, previewOverrides)) return false;
  const sectionId = shortcutLinks().sections.get(node.id);
  return sectionId === undefined || isFeatureEnabled(sectionId, access, previewOverrides);
}

/**
 * The shortcut this user is offered for the page `ownerId` owns, if any — the
 * entry the sidebar shows (and highlights) in place of the owner's own.
 */
export function visibleShortcutTo(
  ownerId: string,
  access: PortalAccess,
  previewOverrides?: Record<string, boolean>,
): CsmNavNode | undefined {
  return flattenNavNodes().find(
    (node) => node.shortcutTo === ownerId && isShortcutOffered(node, access, previewOverrides),
  );
}

/** True when the page should appear in navigation at all (enabled or WIP). */
export function isFeatureVisible(
  id: string,
  access?: PortalAccess,
  previewOverrides?: Record<string, boolean>,
): boolean {
  return featureState(id, access, previewOverrides) !== "hidden";
}

/** True when the page is usable, as opposed to hidden or advertised-but-WIP. */
export function isFeatureEnabled(
  id: string,
  access?: PortalAccess,
  previewOverrides?: Record<string, boolean>,
): boolean {
  return featureState(id, access, previewOverrides) === "enabled";
}

/** Effective state of whichever page owns `pathname`. */
export function featureStateForPath(
  pathname: string,
  access?: PortalAccess,
  previewOverrides?: Record<string, boolean>,
): FeatureState {
  const node = navNodeForPath(pathname);
  return node ? featureState(node.id, access, previewOverrides) : "enabled";
}

/**
 * Top-level sections that should render in the sidebar. With `access`, a
 * section this user reaches through a visible shortcut elsewhere in the
 * sidebar is left out, so the page is offered once (see
 * {@link CsmNavNode.shortcutTo}).
 */
export function visibleNavSections(
  access?: PortalAccess,
  previewOverrides?: Record<string, boolean>,
): typeof CSM_NAV_ITEMS {
  return CSM_NAV_ITEMS.filter(
    (section) =>
      isFeatureVisible(section.id, access, previewOverrides) &&
      !(access && visibleShortcutTo(section.id, access, previewOverrides)),
  );
}

/**
 * A node's tabs that should render in its tab strip. Shortcuts are not tabs:
 * their page lives outside the section.
 */
export function visibleNavChildren(
  node: CsmNavNode,
  access?: PortalAccess,
  previewOverrides?: Record<string, boolean>,
): CsmNavNode[] {
  return (node.children ?? []).filter(
    (child) => !isNavShortcut(child) && isFeatureVisible(child.id, access, previewOverrides),
  );
}

/** A node's tabs that are actually usable. Shortcuts are not tabs. */
export function enabledNavChildren(
  node: CsmNavNode,
  access?: PortalAccess,
  previewOverrides?: Record<string, boolean>,
): CsmNavNode[] {
  return (node.children ?? []).filter(
    (child) => !isNavShortcut(child) && isFeatureEnabled(child.id, access, previewOverrides),
  );
}

/**
 * A section's children as sidebar dropdown entries: its tabs plus any
 * shortcut this user is offered.
 */
export function visibleRailChildren(
  node: CsmNavNode,
  access: PortalAccess,
  previewOverrides?: Record<string, boolean>,
): CsmNavNode[] {
  return (node.children ?? []).filter((child) =>
    isNavShortcut(child)
      ? isShortcutOffered(child, access, previewOverrides)
      : isFeatureVisible(child.id, access, previewOverrides),
  );
}

/** True when `section` has a shortcut child this user is offered. */
export function hasVisibleShortcutChild(
  section: CsmNavNode,
  access: PortalAccess,
  previewOverrides?: Record<string, boolean>,
): boolean {
  return (section.children ?? []).some((child) =>
    isShortcutOffered(child, access, previewOverrides),
  );
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
  previewOverrides?: Record<string, boolean>,
): NavigableNavNode[] {
  return enabledNavChildren(node, access, previewOverrides).flatMap((child) => {
    const self: NavigableNavNode = {
      id: child.id,
      label: child.label,
      sublabel: node.label,
      href: child.href,
      icon: child.icon ?? icon,
    };
    return [self, ...navigableDescendants(child, self.icon, access, previewOverrides)];
  });
}

/**
 * Every enabled destination — sections and every level of their tabs —
 * flattened for the Quick-nav palette. Tabs inherit their parent's icon and
 * carry its label as a sublabel so "Users" reads as "Users / User management"
 * rather than as a bare word.
 */
export function navigableNavNodes(
  access?: PortalAccess,
  previewOverrides?: Record<string, boolean>,
): NavigableNavNode[] {
  return CSM_NAV_ITEMS.flatMap((section) => {
    if (!isFeatureEnabled(section.id, access, previewOverrides)) return [];
    const self: NavigableNavNode = {
      id: section.id,
      label: section.label,
      href: section.href,
      icon: section.icon,
    };
    return [self, ...navigableDescendants(section, section.icon, access, previewOverrides)];
  });
}

/**
 * Where to send someone who lands on a hidden page: the first destination this
 * deployment actually offers. `undefined` when the config hides everything —
 * callers must handle that rather than redirect into a page that is itself
 * hidden, which would bounce the router between two hidden paths forever.
 */
export function firstEnabledDestination(
  access?: PortalAccess,
  previewOverrides?: Record<string, boolean>,
): string | undefined {
  return navigableNavNodes(access, previewOverrides)[0]?.href;
}

/**
 * Every nav node this deployment's own `CSM_PORTAL_FEATURE_OVERRIDES` map
 * EXPLICITLY names, and whose resolved state (without any preview override
 * applied) is still `hidden`/`wip` — the candidate list the Feature Preview
 * dialog renders as toggle rows.
 *
 * Deliberately NOT every node whose state merely ends up `hidden`/`wip`
 * through inheritance — a `hidden` section forces every one of its tabs
 * hidden too (rule 1 in `computeStates`' own doc comment), so without this
 * restriction a deployment that hides a handful of sections would offer a
 * toggle for every tab underneath each one as well, turning a short,
 * intentional restriction list into a dialog listing nearly the entire nav
 * tree. Only the ids a deployment actually chose to name are worth a row;
 * previewing a tab that's merely swept up by a hidden ancestor still works
 * (`hasPreviewedDescendant` promotes the ancestor too, see `featureState`),
 * it's just not separately offered here unless the deployment named it too.
 *
 * `access` only narrows by capability (a node a caller can never reach
 * regardless of deployment state is not worth offering a toggle for); the
 * admin-only gate on actually USING a toggle lives in `featureState`'s own
 * promotion check, not here.
 *
 * A shortcut (see `CsmNavNode.shortcutTo`) is never offered, even when the
 * deployment names it: it is a second sidebar entry for its owner's page, and
 * previewing the owner is the toggle that matters (`featureState` ignores a
 * shortcut's own preview value).
 */
export function listPreviewableFeatures(access?: PortalAccess): CsmNavNode[] {
  const overrides = parseOverrides();
  return flattenNavNodes(CSM_NAV_ITEMS).filter((node) => {
    if (isNavShortcut(node) || !(node.id in overrides)) return false;
    const state = featureState(node.id, access);
    return state === "hidden" || state === "wip";
  });
}

/**
 * True when `id` is visible to the signed-in admin right now ONLY because of
 * an active Feature Preview override — this deployment's own
 * `CSM_PORTAL_FEATURE_OVERRIDES` still restricts it (`hidden`/`wip`) for
 * everyone else. Lets a previewed nav element be marked as such wherever it
 * renders, so "I can see this" is never mistaken for "this is live".
 *
 * Compares `id`'s resolved state WITH the real preview overrides against its
 * state with none at all, rather than checking `previewOverrides[id]`
 * directly — `id` may be enabled only because one of its OWN descendants was
 * previewed (`featureState`'s own `hasPreviewedDescendant` promotion, e.g. an
 * admin previews `admin.user-management.users` while `admin.user-management`
 * itself carries no override of its own), which a direct check would miss
 * entirely: the parent tab would then render as plain "enabled" with no
 * Preview chip, indistinguishable from a tab that's actually live for
 * everyone.
 */
export function isPreviewedOnly(
  id: string,
  access?: PortalAccess,
  previewOverrides?: Record<string, boolean>,
): boolean {
  if (!access?.isAdmin) return false;
  const withoutPreview = featureState(id, access);
  if (withoutPreview === "enabled") return false;
  return featureState(id, access, previewOverrides) === "enabled";
}
