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

import type { JSX } from "react";
import WallboardDashboard from "@features/csm-dashboard/components/WallboardDashboard";

/** The `cs-overview` dashboard's own id — the one dashboard this page shows.
 * A fixed constant, not a route param: unlike `/dashboard/:dashboardId`,
 * this route has no switcher and is never meant to show anything else. */
const CS_OVERVIEW_DASHBOARD_ID = "cs-overview";

/**
 * `/cs-monitor-dashboard` — a standalone, full-screen rendering of the
 * `cs-overview` dashboard, styled to match `digiops-cs`'s Wallboard.tsx
 * (see `CS_Dashboard.png`). Requires real sign-in (routed through
 * `<AuthGuard bare />` in App.tsx) but renders NOTHING else — no header, no
 * sidebar, no banners, no "Dashboard" title, no dashboard/team switcher —
 * just this one component, edge to edge. `CsmDashboardPage.tsx` (the
 * normal, chrome-wrapped `/dashboard` route) renders `cs-overview` through
 * the standard `AgentsLandingPagePilot` grid instead, matching every other
 * dashboard; the two routes intentionally show the same underlying data
 * two different ways, for two different audiences (an engineer browsing
 * the portal vs. a wallboard/kiosk display).
 */
export default function CsMonitorDashboardPage(): JSX.Element {
  return <WallboardDashboard dashboardId={CS_OVERVIEW_DASHBOARD_ID} />;
}
