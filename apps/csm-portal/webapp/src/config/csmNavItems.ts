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
  AlertOctagon,
  AlertTriangle,
  Briefcase,
  Bug,
  Building2,
  ChartColumn,
  Clock,
  ClipboardList,
  Cog,
  FileWarning,
  GitPullRequest,
  Headset,
  KeyRound,
  LifeBuoy,
  Megaphone,
  RefreshCw,
  Settings,
  Shield,
  UserCog,
  Users,
  UsersRound,
} from "@wso2/oxygen-ui-icons-react";
import type { ComponentType } from "react";

/**
 * One entry in the navigation tree: either a top-level sidebar section or one
 * of its second-level tabs. Pure structure — this module knows nothing about
 * feature flags; `featureFlags.ts` resolves a {@link CsmNavNode.id} to a
 * visibility state.
 */
export interface CsmNavNode {
  /**
   * Stable, dotted identifier: `"operations"` for a section,
   * `"operations.incidents"` for one of its tabs. This is the key deployments
   * use in `CSM_PORTAL_FEATURE_OVERRIDES`, so renaming one is a breaking
   * config change — treat these as a public contract.
   */
  id: string;
  label: string;
  /** Where selecting this node navigates. May carry a `?tab=` query. */
  href: string;
  /**
   * For sections whose tab strip lives in a query parameter rather than in
   * child routes (Operations, Security Center): the `?tab=` value that selects
   * this node. Absent for route-backed tabs (Customers, Settings).
   */
  tab?: string;
  /**
   * Route path prefixes this node owns beyond its own `href` pathname — the
   * detail/create routes that render outside the tab strip (for example
   * `/operations/incidents/:id`). Used by the route guard so a disabled tab's
   * deep links are disabled with it.
   */
  routes?: string[];
  /**
   * True when this page already renders its own, more specific unavailable
   * message — one that names what it is blocked on, say. The route guard then
   * lets a `wip` page render that instead of replacing it with the generic
   * "coming soon" fallback, which would be a downgrade.
   */
  rendersOwnWipPage?: boolean;
  icon?: ComponentType<{ size?: number | string }>;
  /**
   * Platform roles required to access/view this navigation node (e.g. `["admin"]`).
   * When omitted or empty, the node is accessible to any authenticated user.
   */
  roles?: string[];
  children?: CsmNavNode[];
}

/** A top-level sidebar section. Always carries an icon. */
export interface CsmNavSection extends CsmNavNode {
  icon: ComponentType<{ size?: number | string }>;
}

/**
 * The CSM portal's navigation tree. Single source of truth for the sidebar,
 * every section's tab strip, the Quick-nav palette's "Pages" section, and
 * "Pin this page" title derivation — so a new page or tab is declared here
 * once.
 *
 * Every node is enabled by default. A deployment turns one into a disabled
 * "work in progress" entry, or removes it outright, by listing its `id` in
 * `CSM_PORTAL_FEATURE_OVERRIDES` (see `featureFlags.ts`); nothing here encodes
 * per-environment readiness.
 */
export const CSM_NAV_ITEMS: CsmNavSection[] = [
  {
    id: "dashboard",
    label: "Dashboard",
    href: "/dashboard",
    icon: ChartColumn,
  },
  {
    id: "support",
    label: "Support",
    href: "/cases",
    icon: Headset,
  },
  {
    id: "operations",
    label: "Operations",
    href: "/operations",
    icon: Cog,
    children: [
      {
        id: "operations.service-requests",
        label: "Service requests",
        href: "/operations?tab=service_requests",
        tab: "service_requests",
        routes: ["/operations/service-requests"],
        icon: ClipboardList,
      },
      {
        id: "operations.change-requests",
        label: "Change requests",
        href: "/operations?tab=change_requests",
        tab: "change_requests",
        routes: ["/operations/change-requests"],
        icon: GitPullRequest,
      },
      {
        id: "operations.incidents",
        label: "Incidents",
        href: "/operations?tab=incidents",
        tab: "incidents",
        routes: ["/operations/incidents"],
        icon: AlertTriangle,
      },
      {
        id: "operations.problems",
        label: "Problem management",
        href: "/operations?tab=problems",
        tab: "problems",
        routes: ["/operations/problems"],
        icon: AlertOctagon,
      },
    ],
  },
  {
    id: "engagements",
    label: "Engagements",
    href: "/engagements",
    icon: Briefcase,
  },
  {
    id: "security-center",
    label: "Security Center",
    href: "/security-center",
    icon: Shield,
    children: [
      {
        id: "security-center.reports",
        label: "Security reports",
        href: "/security-center?tab=security_reports",
        tab: "security_reports",
        routes: ["/security-center/reports"],
        icon: FileWarning,
      },
      {
        id: "security-center.vulnerabilities",
        label: "Vulnerabilities",
        href: "/security-center?tab=vulnerabilities",
        tab: "vulnerabilities",
        routes: ["/security-center/vulnerabilities"],
        icon: Bug,
      },
    ],
  },
  {
    id: "updates",
    label: "Updates",
    href: "/updates",
    icon: RefreshCw,
  },
  {
    id: "time-cards",
    label: "Time cards",
    href: "/time-cards",
    icon: Clock,
  },
  {
    id: "announcements",
    label: "Announcements",
    href: "/announcements",
    icon: Megaphone,
  },
  {
    id: "customers",
    label: "Customers",
    href: "/customers",
    icon: Building2,
    children: [
      {
        id: "customers.accounts",
        label: "Accounts",
        href: "/customers/accounts",
      },
      {
        id: "customers.projects",
        label: "Projects",
        href: "/customers/projects",
      },
    ],
  },
  {
    id: "admin",
    label: "Settings",
    href: "/admin",
    icon: Settings,
    roles: ["admin"],
    children: [
      {
        id: "admin.user-management",
        label: "User management",
        href: "/admin/user-management",
        children: [
          {
            id: "admin.user-management.users",
            label: "Users",
            href: "/admin/user-management/users",
            icon: Users,
          },
          {
            id: "admin.user-management.roles",
            label: "Roles",
            href: "/admin/user-management/roles",
            icon: UserCog,
          },
          {
            id: "admin.user-management.groups",
            label: "Groups",
            href: "/admin/user-management/groups",
            icon: UsersRound,
          },
          {
            id: "admin.user-management.teams",
            label: "Teams",
            href: "/admin/user-management/teams",
            icon: Building2,
          },
          // Routes to a placeholder that already names its backend blocker, so
          // it renders itself rather than the generic WIP page.
          {
            id: "admin.user-management.permissions",
            label: "Permissions",
            href: "/admin/user-management/permissions",
            rendersOwnWipPage: true,
            icon: KeyRound,
          },
        ],
      },
      // Admin-role-gated (see `isDashboardBuilderVisibleForRoles` in
      // `csmAdminAccess.ts`) — unlike every sibling tab above, this one is
      // hidden from a non-admin signed-in user rather than merely relying
      // on the backend to reject the action. Deliberate exception to this
      // section's usual "show the action, let the backend reject it" rule
      // (see App.tsx's own comment on the roles/groups/teams member
      // routes): the dashboard builder exposes no privileged backend
      // action at all (everything it does is local to the browser), so
      // there is nothing for a backend gate to enforce here — the ONLY
      // gate is this frontend one.
      {
        id: "admin.dashboards",
        label: "Dashboards",
        href: "/admin/dashboards",
      },
    ],
  },
  {
    id: "help",
    label: "Help",
    href: "/help",
    icon: LifeBuoy,
    // Every topic is an in-page anchor on the single `/help` route (see
    // `HelpPage`), not its own route, so `href` carries a `#<topic>` fragment
    // rather than a path segment — same anchor-vs-path split `navNodeRoutes`
    // already makes for a query-param tab, and handled there the same way.
    children: [
      { id: "help.overview", label: "Overview", href: "/help#overview" },
      {
        id: "help.workspace-basics",
        label: "Navigation & personalization",
        href: "/help#workspace-basics",
      },
      { id: "help.dashboard", label: "Dashboard", href: "/help#dashboard" },
      { id: "help.support", label: "Support", href: "/help#support" },
      { id: "help.operations", label: "Operations", href: "/help#operations" },
      { id: "help.engagements", label: "Engagements", href: "/help#engagements" },
      {
        id: "help.security-center",
        label: "Security Center",
        href: "/help#security-center",
      },
      { id: "help.updates", label: "Updates", href: "/help#updates" },
      { id: "help.time-cards", label: "Time cards", href: "/help#time-cards" },
      {
        id: "help.announcements",
        label: "Announcements",
        href: "/help#announcements",
      },
      { id: "help.customers", label: "Customers", href: "/help#customers" },
      {
        id: "help.people-access",
        label: "People & project access",
        href: "/help#people-access",
      },
      { id: "help.settings", label: "Settings", href: "/help#settings" },
    ],
  },
];

/** The pathname part of `href`, dropping any query string or hash. */
export function navNodePath(node: CsmNavNode): string {
  return node.href.split(/[?#]/)[0];
}

/**
 * Where actually navigating to `node` should go. For a query-param tab
 * (`tab` set), `href` is kept in the legacy `?tab=` shape purely so an old
 * bookmarked/shared link in that form still resolves (see
 * `LegacyQueryTabRedirect`) — the real, canonical destination is its first
 * declared {@link CsmNavNode.routes} entry, the actual path-segment route
 * (`/operations/incidents`, not `/operations?tab=incidents`). Every other
 * node has no such split: `href` already is the real destination.
 */
export function navNodeHref(node: CsmNavNode): string {
  if (node.tab && node.routes?.[0]) return node.routes[0];
  return node.href;
}

/** True when `href` carries a `#` fragment rather than being a distinct path
 * (a Help topic's `/help#operations`) — the node's pathname is only its
 * parent's landing route, shared with every sibling anchor. */
function isAnchorHref(node: CsmNavNode): boolean {
  return node.href.includes("#");
}

/**
 * Route path prefixes a node owns. A query-param tab (`tab` set) or an
 * in-page anchor (`href` with a `#` fragment) owns only its explicit
 * {@link CsmNavNode.routes}: its `href` pathname is the *parent's* landing
 * route, which every sibling shares, so claiming it would make the
 * longest-prefix match in {@link navNodeForPath} ambiguous.
 */
export function navNodeRoutes(node: CsmNavNode): string[] {
  const extra = node.routes ?? [];
  return node.tab || isAnchorHref(node) ? extra : [navNodePath(node), ...extra];
}

/** Depth-first walk of the tree, parents before their children. */
export function flattenNavNodes(
  nodes: readonly CsmNavNode[] = CSM_NAV_ITEMS,
): CsmNavNode[] {
  return nodes.flatMap((node) => [
    node,
    ...(node.children ? flattenNavNodes(node.children) : []),
  ]);
}

/** The node with this id, anywhere in the tree. */
export function navNodeById(id: string): CsmNavNode | undefined {
  return flattenNavNodes().find((node) => node.id === id);
}

function matchesPrefix(pathname: string, prefix: string): boolean {
  return pathname === prefix || pathname.startsWith(`${prefix}/`);
}

/** A nav node together with the route prefix of its that `pathname` matched. */
export interface CsmNavMatch {
  node: CsmNavNode;
  /**
   * The matched prefix — the node's canonical path for this URL. Prefer it over
   * {@link navNodePath} for a query-param tab, whose `href` pathname is the
   * parent's landing route rather than the tab's own routes.
   */
  prefix: string;
}

/**
 * The most specific nav node owning `pathname`, by longest matching route
 * prefix. `/operations/incidents/42` resolves to the Incidents tab rather than
 * to Operations, which is what lets a single finished tab stay reachable inside
 * an otherwise-unfinished section.
 */
export function navNodeMatchForPath(pathname: string): CsmNavMatch | undefined {
  let best: CsmNavMatch | undefined;

  for (const node of flattenNavNodes()) {
    for (const prefix of navNodeRoutes(node)) {
      if (
        matchesPrefix(pathname, prefix) &&
        prefix.length > (best?.prefix.length ?? -1)
      ) {
        best = { node, prefix };
      }
    }
  }

  return best;
}

/** {@link navNodeMatchForPath}, when only the node matters. */
export function navNodeForPath(pathname: string): CsmNavNode | undefined {
  return navNodeMatchForPath(pathname)?.node;
}

/**
 * The child selected by a `?tab=` value in `search`, for sections that keep
 * their tab in the query rather than in child routes. Path matching alone can't
 * find these: every such tab shares its section's pathname.
 */
export function navTabForSearch(
  node: CsmNavNode,
  search: string,
): CsmNavNode | undefined {
  const tab = new URLSearchParams(search).get("tab");
  if (!tab) return undefined;
  return node.children?.find((child) => child.tab === tab);
}

/** The top-level section owning `pathname`, ignoring its tabs. */
export function navSectionForPath(pathname: string): CsmNavSection | undefined {
  return CSM_NAV_ITEMS.find((section) =>
    navNodeRoutes(section).some((prefix) => matchesPrefix(pathname, prefix)),
  );
}
