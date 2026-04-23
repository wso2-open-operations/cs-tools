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

import type { TabOption } from "@components/tab-bar/TabBar";
import type { ProjectPermissions } from "@/types/permission";
import { ProjectDetailsTabId } from "@features/project-details/types/projectDetails";

export type ProjectDetailsLoadingParams = {
  isAuthLoading: boolean;
  isProjectLoading: boolean;
  isStatsLoading: boolean;
  project: unknown;
  projectError: unknown;
  stats: unknown;
  statsError: unknown;
};

/**
 * Whether the project details shell should show as loading (global loader).
 *
 * @param params - Auth, query, and data presence flags.
 * @returns True while initial loads are in flight or data is not yet resolved.
 */
export function getProjectDetailsLoadingState(
  params: ProjectDetailsLoadingParams,
): boolean {
  const {
    isAuthLoading,
    isProjectLoading,
    isStatsLoading,
    project,
    projectError,
    stats,
    statsError,
  } = params;

  if (isAuthLoading || isProjectLoading || isStatsLoading) {
    return true;
  }
  if (!project && !projectError) {
    return true;
  }
  if (!stats && !statsError) {
    return true;
  }

  return false;
}

/**
 * Filters project detail tab definitions by permission flags.
 *
 * @param tabs - Full tab list from constants.
 * @param permissions - Resolved project permissions.
 * @returns Tabs the user is allowed to see.
 */
export function filterProjectDetailsTabsByPermissions(
  tabs: TabOption[],
  permissions: ProjectPermissions,
): TabOption[] {
  return tabs.filter((tab) => {
    switch (tab.id) {
      case ProjectDetailsTabId.DEPLOYMENTS:
        return permissions.hasDeployments;
      case ProjectDetailsTabId.TIME_TRACKING:
        return permissions.hasTimeLogs;
      default:
        return true;
    }
  });
}
