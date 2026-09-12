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

import { Box, Button, Typography } from "@wso2/oxygen-ui";
import { ArrowLeft } from "@wso2/oxygen-ui-icons-react";
import { type JSX, Suspense, useMemo } from "react";
import { Outlet, useLocation, useNavigate } from "react-router";
import RouteSuspenseFallback from "@components/route-fallback/RouteSuspenseFallback";
import SectionTabs from "@components/section-tabs/SectionTabs";
import { useRouteTabs } from "@hooks/useSectionTabs";
import { useCurrentUser } from "@context/current-user/CurrentUserContext";
import { hasDashboardBuilderAccess } from "@features/csm-admin/dashboards/utils/dashboardBuilderAccess";

/** The tile-grid landing route — the default Back target for a directory
 * page reached via a tile, when no more specific origin (`state.from`) is
 * known (e.g. a bookmarked/direct link straight into `/roles`). */
const USER_MANAGEMENT_INDEX_PATH = "/admin/user-management";

/**
 * Settings shell. A single top-level tab strip, driven by the navigation
 * tree, for [User management, Dashboards]. User management no longer has its
 * own nested tab strip underneath it — its Users/Roles/Groups/Teams/
 * Permissions directories are chosen from a tile grid instead
 * (`CsmUserManagementLandingPage`, rendered at the `user-management` index
 * route). Since a directory page reached via a tile has no tab strip to
 * click back through, this shell adds an explicit Back link at the top of
 * the page -- before the "Settings" title, matching every other page's Back
 * button position -- whenever the current route is one level or more below
 * that index route.
 *
 * Like every other page's Back button, this one prefers `location.state.from`
 * (set by whatever page actually linked here — e.g. a dashboard widget's
 * "user" click-through) over the tile-grid fallback, and forwards
 * `state.parentState` on so a multi-hop chain (dashboard → this directory
 * page → a person's profile → Back → Back) restores correctly rather than
 * silently landing back on the tile grid partway through. This used to be a
 * fixed "Back to User management" link regardless of how the page was
 * reached — genuinely reported as wrong, since a directory page CAN be
 * reached from somewhere other than its own tile (see `CsmUsersPage.tsx`'s
 * dashboard click-through support). The label is plain "Back" now, per this
 * app's own convention: a destination-specific label is only for a button
 * whose target is genuinely always the same place, which this one no longer is.
 *
 * The "Dashboards" tab is additionally filtered by the signed-in user's own
 * admin role (frontend-only — see `dashboardBuilderAccess.ts` for why this
 * tab specifically needs it, unlike its sibling). This never removes a tab
 * `CSM_PORTAL_FEATURE_OVERRIDES` itself hid/marked WIP — it only ever narrows
 * what a non-admin sees further.
 */
export default function CsmAdminLayout(): JSX.Element {
  const { user } = useCurrentUser();
  const isAdmin = hasDashboardBuilderAccess(user?.roles);
  const allTabs = useRouteTabs("admin");
  const tabs = useMemo(() => {
    const visible = allTabs.tabs.filter((tab) => tab.node.id !== "admin.dashboards" || isAdmin);
    // `allTabs.activeKey` was resolved against the UNFILTERED list — if
    // filtering it out here just removed the active one (a non-admin whose
    // URL still names it), fall back to this narrower list's own first tab
    // rather than handing `<Tabs>` a `value` with no matching `<Tab>`.
    const activeKey = visible.some((tab) => tab.key === allTabs.activeKey)
      ? allTabs.activeKey
      : (visible[0]?.key ?? "");
    return { ...allTabs, tabs: visible, activeKey };
  }, [allTabs, isAdmin]);

  const location = useLocation();
  const { pathname } = location;
  const navigate = useNavigate();
  const showBackToUserManagement = pathname.startsWith(`${USER_MANAGEMENT_INDEX_PATH}/`);
  const backState = location.state as
    | { from?: string; parentState?: unknown }
    | undefined;
  const backTarget = backState?.from ?? USER_MANAGEMENT_INDEX_PATH;

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      {/* Top of the page, before the title -- same position every other
          page's Back button uses (see `UserProfilePage.tsx`/
          `CsmProjectDetailPage.tsx`), not tucked below the tab strip. */}
      {showBackToUserManagement && (
        <Button
          variant="text"
          size="small"
          startIcon={<ArrowLeft size={16} />}
          onClick={() => navigate(backTarget, { state: backState?.parentState ?? undefined })}
          sx={{ alignSelf: "flex-start" }}
        >
          Back
        </Button>
      )}

      <Typography variant="h5">Settings</Typography>

      <SectionTabs {...tabs} ariaLabel="Settings tabs" scrollable />

      <Suspense fallback={<RouteSuspenseFallback />}>
        <Outlet />
      </Suspense>
    </Box>
  );
}
