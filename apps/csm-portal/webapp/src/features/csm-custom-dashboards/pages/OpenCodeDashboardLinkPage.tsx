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
import { Navigate } from "react-router";
import { savedCodeDashboardPath } from "@features/csm-custom-dashboards/api/useCodeDashboards";
import {
  sharedCodeDashboardPath,
  useOpenCodeDashboardLink,
} from "@features/csm-custom-dashboards/api/useCodeDashboardShares";
import OpenDashboardLinkPage from "@features/csm-custom-dashboards/pages/OpenDashboardLinkPage";
import { useCanViewAdvancedDashboards } from "@features/csm-custom-dashboards/utils/advancedDashboardAccess";

/** Where an opened advanced dashboard link goes: the saved advanced page when it is the caller's own, else Shared Dashboards. */
function codeDashboardPath(dashboardId: string, own: boolean): string {
  return own ? savedCodeDashboardPath(dashboardId) : sharedCodeDashboardPath(dashboardId);
}

/**
 * An advanced dashboard's link (`/dashboard/shared/code-link/:linkId`,
 * CODE_DASHBOARD_LINK_PATH): OpenDashboardLinkPage with the advanced
 * dashboards' open call (`POST /dashboards/shared/code/link`) and pages. The
 * 404 page for a user who cannot view advanced dashboards
 * (`canViewAdvancedDashboards`): the backend's route does not exist for them
 * either.
 */
export default function OpenCodeDashboardLinkPage(): JSX.Element {
  if (!useCanViewAdvancedDashboards()) return <Navigate to="/404" replace />;
  return <OpenDashboardLinkPage useOpenLink={useOpenCodeDashboardLink} dashboardPath={codeDashboardPath} />;
}
