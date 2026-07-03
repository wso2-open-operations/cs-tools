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
  Briefcase,
  Building2,
  ChartColumn,
  Clock,
  Cog,
  Headset,
  RefreshCw,
  Settings,
  Shield,
} from "@wso2/oxygen-ui-icons-react";
import type { ComponentType } from "react";
import { DISABLE_WIP_FEATURES } from "@config/featureFlagsConfig";

export interface CsmNavItem {
  id: string;
  label: string;
  path: string;
  icon: ComponentType<{ size?: number | string }>;
  /**
   * Marks a still-in-progress section. When the `CSM_PORTAL_DISABLE_WIP_FEATURES`
   * runtime flag is on, these stay visible in the sidebar but are disabled (with
   * a "work in progress" tooltip), dropped from the quick-nav palette, and their
   * routes render the shared "coming soon" page instead of the unfinished
   * feature (see `isWipDisabled`, `navigableNavItems`, `isDisabledWipPath`, and
   * `App.tsx`'s `WipRouteGuard`).
   */
  wip?: boolean;
}

/**
 * The CSM portal's top-level pages. Single source of truth for the sidebar nav,
 * the Quick-nav palette's "Pages" section, and "Pin this page" title/kind
 * derivation — so a new page only has to be added here once.
 *
 * Dashboard, Support, Updates and Time cards are the shipped sections; the rest
 * are flagged `wip` so a deployment can disable them via
 * `CSM_PORTAL_DISABLE_WIP_FEATURES` until they are ready.
 */
export const CSM_NAV_ITEMS: CsmNavItem[] = [
  { id: "dashboard", label: "Dashboard", path: "/dashboard", icon: ChartColumn },
  { id: "support", label: "Support", path: "/cases", icon: Headset },
  { id: "operations", label: "Operations", path: "/operations", icon: Cog, wip: true },
  { id: "engagements", label: "Engagements", path: "/engagements", icon: Briefcase, wip: true },
  { id: "security-center", label: "Security Center", path: "/security-center", icon: Shield, wip: true },
  { id: "updates", label: "Updates", path: "/updates", icon: RefreshCw },
  { id: "time-cards", label: "Time cards", path: "/time-cards", icon: Clock },
  { id: "customers", label: "Customers", path: "/customers", icon: Building2, wip: true },
  { id: "admin", label: "Settings", path: "/admin", icon: Settings, wip: true },
];

/** True when `item` is a WIP section the current config disables. */
export function isWipDisabled(item: CsmNavItem): boolean {
  return DISABLE_WIP_FEATURES && item.wip === true;
}

/**
 * Nav items that can actually be navigated to — the full list minus any WIP
 * section the current config disables. Used by the quick-nav palette so it only
 * offers reachable destinations. The sidebar still renders the full
 * {@link CSM_NAV_ITEMS} (disabling WIP items in place), and title/kind
 * derivation keeps using the full list so a directly-navigated page still
 * resolves its title.
 */
export function navigableNavItems(): CsmNavItem[] {
  return DISABLE_WIP_FEATURES
    ? CSM_NAV_ITEMS.filter((item) => !item.wip)
    : CSM_NAV_ITEMS;
}

/** The nav item whose path is (a prefix of) `pathname`, if any. */
export function navItemForPath(pathname: string): CsmNavItem | undefined {
  return CSM_NAV_ITEMS.find(
    (item) => pathname === item.path || pathname.startsWith(`${item.path}/`),
  );
}

/**
 * True when `pathname` belongs to a WIP section the current config disables.
 * Used to redirect direct/deep links to disabled sections back to the dashboard.
 */
export function isDisabledWipPath(pathname: string): boolean {
  if (!DISABLE_WIP_FEATURES) return false;
  return navItemForPath(pathname)?.wip === true;
}
