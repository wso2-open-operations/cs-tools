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

import { isCodeDashboardGenerationEnabled } from "@config/codeDashboardConfig";
import type { PortalAccess } from "@context/current-user/portalAccess";
import { usePortalAccess } from "@context/current-user/usePortalAccess";

/**
 * Whether a user may generate an advanced dashboard (the code approach,
 * `/dashboard/my/new-code`): the deployment has it on
 * (DASHBOARD_GENERATE_CODE_ENABLED) and the user is a CS engineer or an
 * admin (`canWrite`, as for `POST /dashboards/generate-code`). Who gets it is
 * not decided yet; this is the one place to narrow it.
 */
export function canGenerateAdvancedDashboards(access: PortalAccess): boolean {
  return isCodeDashboardGenerationEnabled() && access.canWrite;
}

/** {@link canGenerateAdvancedDashboards} for the signed-in user. */
export function useCanGenerateAdvancedDashboards(): boolean {
  return canGenerateAdvancedDashboards(usePortalAccess());
}

/**
 * Whether a user may view advanced dashboards others shared with them (under
 * Shared Dashboards, and through an advanced dashboard's link): the
 * deployment has the code approach on. No role check of its own: the same
 * users as Shared Dashboards itself (no nav `requires`), as the backend's
 * viewer routes are PermView like the custom dashboards' shared routes.
 * Making, saving, editing and sharing stay under
 * {@link canGenerateAdvancedDashboards}.
 */
export function canViewAdvancedDashboards(): boolean {
  return isCodeDashboardGenerationEnabled();
}

/** {@link canViewAdvancedDashboards} as a hook, for pages (and their tests) to read. */
export function useCanViewAdvancedDashboards(): boolean {
  return canViewAdvancedDashboards();
}
