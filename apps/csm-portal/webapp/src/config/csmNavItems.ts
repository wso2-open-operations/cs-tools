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
  Award,
  BarChart3,
  Briefcase,
  BookOpen,
  Bug,
  Building,
  Building2,
  CalendarDays,
  ChartColumn,
  Clock,
  ClipboardList,
  Cog,
  FileWarning,
  FolderOpen,
  GitPullRequest,
  Headset,
  HeartPulse,
  KeyRound,
  LifeBuoy,
  Megaphone,
  RefreshCw,
  ScrollText,
  Settings,
  Shield,
  UserCheck,
  UserCog,
  Users,
  UsersRound,
} from "@wso2/oxygen-ui-icons-react";
import type { ComponentType } from "react";
import type { PortalAccess } from "@context/current-user/portalAccess";

/**
 * One entry in the navigation tree: either a top-level sidebar section or one
 * of its second-level tabs. Pure structure — this module knows nothing about
 * feature flags; `featureFlags.ts` resolves a {@link CsmNavNode.id} to a
 * visibility state.
 */
import { PLG_NAV_SECTION } from "@features/plg/config/plgNavItems";

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
   * The {@link PortalAccess} capability a user needs to see this node; without
   * it the node is hidden for that user (nav entry and route), and so is
   * everything under it. Per user, unlike the per-deployment feature flags.
   */
  requires?: keyof PortalAccess;
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
  /**
   * Marks this node as a second sidebar entry for a page another node owns:
   * the id of that owning node. A shortcut owns no routes (the route guard,
   * the active-item highlight, Recent and Pin all keep resolving the page to
   * its owner), is never a tab in its section's in-page tab strip, and is left
   * out of Quick-nav (the owner is already listed there). In the sidebar it is
   * a child of its section, which then opens as a dropdown; when it is shown,
   * the owner's own top-level entry is dropped so the page is offered once.
   * Its visibility is the narrower of its own (`requires`, deployment
   * overrides) and its owner's.
   */
  shortcutTo?: string;
  icon?: ComponentType<{ size?: number | string }>;
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
    requires: "canUseOperations",
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
        routes: ["/operations/incidents", "/operations/incident-tasks"],
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
      {
        id: "operations.outages",
        label: "Outages",
        href: "/operations?tab=outages",
        tab: "outages",
        routes: ["/operations/outages"],
        icon: Megaphone,
      },
      {
        id: "operations.runbooks",
        label: "Runbooks",
        href: "/operations?tab=runbooks",
        tab: "runbooks",
        icon: ScrollText,
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
    id: "allocations",
    label: "Allocations",
    href: "/allocations",
    icon: UserCheck,
  },
  {
    id: "announcements",
    label: "Announcements",
    href: "/announcements",
    icon: Megaphone,
  },
  {
    id: "security-center",
    label: "Security Center",
    href: "/security-center",
    icon: Shield,
    requires: "canUseSecurityCenter",
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
    id: "customers",
    label: "Customers",
    href: "/customers",
    icon: Building2,
    children: [
      {
        id: "customers.accounts",
        label: "Accounts",
        href: "/customers/accounts",
        // Shown only where Customers opens as a sidebar dropdown (and in
        // Quick-nav); the in-page tab strip draws no icons. Same icons Recent
        // uses for an account and a project (csm-recent/kindMeta.tsx).
        icon: Building,
      },
      {
        id: "customers.projects",
        label: "Projects",
        href: "/customers/projects",
        icon: FolderOpen,
      },
      {
        // Viewers reach Customer Health from the Customers dropdown instead
        // of from its own top-level entry -- see
        // showCustomerHealthUnderCustomers's own doc comment. The page itself
        // still belongs to the top-level "customer-health" node (see
        // CsmNavNode.shortcutTo).
        id: "customers.customer-health",
        label: "Customer Health",
        href: "/customer-health",
        shortcutTo: "customer-health",
        requires: "showCustomerHealthUnderCustomers",
        icon: HeartPulse,
      },
    ],
  },
  {
    id: "kb-articles",
    label: "Knowledge",
    href: "/knowledge",
    // Staff-only -- see canViewStaffSections's own doc comment.
    requires: "canViewStaffSections",
    icon: BookOpen,
    children: [
      {
        id: "kb-articles.all",
        label: "All",
        href: "/knowledge/all",
      },
      {
        id: "kb-articles.to-review",
        label: "To Review",
        href: "/knowledge/to-review",
      },
      {
        id: "kb-articles.my",
        label: "My",
        href: "/knowledge/my-articles",
      },
      {
        id: "kb-articles.admin",
        label: "Admin",
        href: "/knowledge/admin",
      },
    ],
  },
  // PLG Customer Success Portal. Declared in
  // features/plg/config/plgNavItems so a change to PLG's pages does not
  // touch this file. Hide the whole section with
  // CSM_PORTAL_FEATURE_OVERRIDES: { "plg": "hidden" }.
  PLG_NAV_SECTION,
  {
    id: "updates",
    label: "Updates",
    href: "/updates",
    requires: "canUseTimeCardsAndUpdates",
    icon: RefreshCw,
  },
  {
    id: "time-cards",
    label: "Time cards",
    href: "/time-cards",
    requires: "canUseTimeCardsAndUpdates",
    icon: Clock,
  },
  {
    id: "team-schedule",
    label: "Team Schedule",
    href: "/team-schedule",
    // An explicit allow-list (cs_engineer/admin/comment_updater), not just
    // "not a plain viewer" -- see canViewTeamSchedule's own doc comment.
    // Editing it is a lead's job and will gate on its own flag when the
    // write routes land.
    requires: "canViewTeamSchedule",
    icon: CalendarDays,
  },
  {
    id: "skills",
    label: "Skills",
    href: "/skills",
    // Everyone who can write keeps a skills profile (the backend puts the
    // skills routes' changes under the write permission). Its tabs are not
    // nav-tree children: the Skill Manager tab depends on a per-user probe
    // (see CsmSkillsPage), not on a role.
    requires: "canWrite",
    icon: Award,
  },
  {
    id: "customer-health",
    label: "Customer Health",
    href: "/customer-health",
    // Ex-Support Portal Lite section with no modern/entity-service-backed
    // equivalent. Its audience is the viewer role plus cs_engineer and admin
    // (PermViewCustomerHealth server-side) -- see canViewCustomerHealth's own
    // doc comment. Like Usage Metrics below, it sits just above Settings and
    // Help at the end of the sidebar -- except for a viewer, who gets it under
    // Customers instead (the "customers.customer-health" shortcut). This node
    // still owns the page and its routes for every role.
    requires: "canViewCustomerHealth",
    icon: HeartPulse,
  },
  {
    id: "usage-metrics",
    label: "Usage Metrics",
    href: "/usage-metrics",
    // Reported live: unlike its ex-Support-Portal-Lite siblings, a plain
    // viewer must NOT see this one -- only cs_engineer/admin and the
    // dedicated usage_metrics_viewer role do. See canViewUsageMetrics's own
    // doc comment.
    requires: "canViewUsageMetrics",
    icon: BarChart3,
  },
  {
    id: "admin",
    label: "Settings",
    href: "/admin",
    // cs_engineer/admin only -- see canViewSettings's own doc comment.
    requires: "canViewSettings",
    icon: Settings,
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
      // Settings is also open to a CS engineer. This tab is admin only, so
      // the sidebar and Quick-nav hide it the same way the Settings tabs do.
      {
        id: "admin.banners",
        label: "Banners",
        href: "/admin/banners",
        icon: Megaphone,
        requires: "isAdmin",
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
      {
        id: "help.operations",
        label: "Operations",
        href: "/help#operations",
        requires: "canUseOperations",
      },
      { id: "help.engagements", label: "Engagements", href: "/help#engagements" },
      {
        id: "help.security-center",
        label: "Security Center",
        href: "/help#security-center",
        requires: "canUseSecurityCenter",
      },
      {
        id: "help.updates",
        label: "Updates",
        href: "/help#updates",
        requires: "canUseTimeCardsAndUpdates",
      },
      {
        id: "help.time-cards",
        label: "Time cards",
        href: "/help#time-cards",
        requires: "canUseTimeCardsAndUpdates",
      },
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
      {
        id: "help.settings",
        label: "Settings",
        href: "/help#settings",
        // Help topics follow the page they document -- see canViewSettings's
        // own doc comment on why Settings itself is cs_engineer/admin-only.
        requires: "canViewSettings",
      },
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

/** True when `node` is a second sidebar entry for a page another node owns. */
export function isNavShortcut(node: CsmNavNode): boolean {
  return Boolean(node.shortcutTo);
}

/**
 * Route path prefixes a node owns. A query-param tab (`tab` set) or an
 * in-page anchor (`href` with a `#` fragment) owns only its explicit
 * {@link CsmNavNode.routes}: its `href` pathname is the *parent's* landing
 * route, which every sibling shares, so claiming it would make the
 * longest-prefix match in {@link navNodeForPath} ambiguous.
 */
export function navNodeRoutes(node: CsmNavNode): string[] {
  // A shortcut points at a page its owner already claims; claiming it too
  // would make two nodes tie for the same prefix.
  if (node.shortcutTo) return [];
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
 *
 * On a length tie, the later match wins (`>=`, not `>`) — `flattenNavNodes`
 * yields parents before their children, and a section whose `href` is just an
 * alias for its own landing child (e.g. "plg"'s href and "plg.dashboard"'s
 * href are both "/plg/dashboard", since the section has no dedicated landing
 * page of its own) would otherwise have the parent win a same-length tie
 * against the more specific child it's aliasing — surfacing as the child
 * never being the one reported active for its own path.
 */
export function navNodeMatchForPath(pathname: string): CsmNavMatch | undefined {
  let best: CsmNavMatch | undefined;

  for (const node of flattenNavNodes()) {
    for (const prefix of navNodeRoutes(node)) {
      if (
        matchesPrefix(pathname, prefix) &&
        prefix.length >= (best?.prefix.length ?? -1)
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
