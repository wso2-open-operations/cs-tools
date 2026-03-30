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

import { PROJECT_TYPE_LABELS } from "@constants/projectTypeConstants";
import type { ActivityItem } from "@constants/projectDetailsConstants";
import type { ProjectStatsResponse } from "@models/responses";
import { convertMinutesToHours } from "@utils/projectDetails";
import type { ProjectPermissions } from "@/types/subscriptionTypes";

/**
 * Restrictive defaults for unknown or unlisted project types.
 */
function restrictivePermissions(): ProjectPermissions {
  return {
    hasOperations: false,
    hasSR: false,
    hasCR: false,
    hasDeployments: false,
    hasQueryHours: false,
    hasTimeLogs: false,
    showOutstandingOpsChart: false,
    includeChangeRequestsInDashboardTotals: false,
    includeS0InSupportMetrics: false,
    showServiceHoursAllocationsCard: false,
  };
}

/**
 * Returns UI and stats permissions for a project type label (API string).
 *
 * @param projectTypeLabel - Value from project.type.label.
 * @returns Permission flags for conditional rendering and aggregations.
 */
export function getProjectPermissions(
  projectTypeLabel: string | null | undefined,
): ProjectPermissions {
  const permissions = restrictivePermissions();
  const label = projectTypeLabel ?? "";

  switch (label) {
    case PROJECT_TYPE_LABELS.MANAGED_CLOUD_SUBSCRIPTION:
      permissions.hasOperations = true;
      permissions.hasSR = true;
      permissions.hasCR = true;
      permissions.hasDeployments = true;
      permissions.hasQueryHours = true;
      permissions.hasTimeLogs = true;
      permissions.showOutstandingOpsChart = true;
      permissions.includeChangeRequestsInDashboardTotals = true;
      permissions.includeS0InSupportMetrics = true;
      permissions.showServiceHoursAllocationsCard = true;
      break;

    case PROJECT_TYPE_LABELS.CLOUD_SUPPORT:
      permissions.hasOperations = true;
      permissions.hasSR = true;
      permissions.hasCR = false;
      permissions.hasDeployments = false;
      permissions.hasQueryHours = true;
      permissions.hasTimeLogs = false;
      permissions.showOutstandingOpsChart = true;
      permissions.includeChangeRequestsInDashboardTotals = false;
      permissions.includeS0InSupportMetrics = false;
      permissions.showServiceHoursAllocationsCard = false;
      break;

    case PROJECT_TYPE_LABELS.CLOUD_EVALUATION_SUPPORT:
    case PROJECT_TYPE_LABELS.EVALUATION_SUBSCRIPTION:
    case PROJECT_TYPE_LABELS.SUBSCRIPTION:
      break;

    default:
      break;
  }

  return permissions;
}

/**
 * Whether S0 (catastrophic) should be hidden in support lists and filters.
 *
 * @param projectTypeLabel - Value from project.type.label.
 * @returns True when S0 must be excluded (all types except Managed Cloud).
 */
export function shouldExcludeS0(
  projectTypeLabel: string | null | undefined,
): boolean {
  return !getProjectPermissions(projectTypeLabel).includeS0InSupportMetrics;
}

export interface ProjectOperationsStatsResult {
  total: number;
  serviceRequests: number;
  changeRequests: number;
}

/**
 * Computes outstanding operations counts; ignores CR when the project type has no CR.
 *
 * @param permissions - Project permission flags.
 * @param srCount - Service request total count.
 * @param crCount - Change request total count.
 * @returns Totals for dashboard operations chart and related UI.
 */
export function calculateProjectStats(
  permissions: ProjectPermissions,
  srCount: number,
  crCount: number,
): ProjectOperationsStatsResult {
  const serviceRequests = srCount;
  const changeRequests = permissions.hasCR ? crCount : 0;
  const total = permissions.hasCR ? serviceRequests + changeRequests : serviceRequests;

  return {
    total,
    serviceRequests,
    changeRequests,
  };
}

/**
 * Builds recent activity rows for the project overview card from stats API data.
 *
 * @param activity - Recent activity payload from project stats.
 * @param projectTypeLabel - Value from project.type.label.
 * @returns Activity rows to render.
 */
export function getRecentActivityItems(
  activity?: ProjectStatsResponse["recentActivity"],
  projectTypeLabel?: string | null,
): ActivityItem[] {
  const permissions = getProjectPermissions(projectTypeLabel);

  const formatDateTime = (dateString: string): string => {
    if (!dateString) return "";
    try {
      const date = new Date(dateString.replace(" ", "T"));
      if (isNaN(date.getTime())) return "";
      const dateStr = date.toLocaleDateString("en-US", {
        month: "short",
        day: "numeric",
        year: "numeric",
      });
      const timeStr = date.toLocaleTimeString("en-US", {
        hour: "numeric",
        minute: "2-digit",
        hour12: true,
      });
      return `${dateStr} at ${timeStr}`;
    } catch {
      return "";
    }
  };

  const items: ActivityItem[] = [];

  if (permissions.hasTimeLogs) {
    items.push(
      {
        label: "Total Time Logged",
        value:
          activity?.totalHours !== undefined
            ? `${convertMinutesToHours(activity.totalHours)} hrs`
            : "N/A",
        type: "text",
      },
      {
        label: "Billable Hours",
        value:
          activity?.billableHours !== undefined
            ? `${convertMinutesToHours(activity.billableHours)} hrs`
            : "N/A",
        type: "text",
      },
    );
  }

  if (permissions.hasDeployments) {
    items.push({
      label: "Last Deployment",
      value: activity?.lastDeploymentOn
        ? formatDateTime(activity.lastDeploymentOn)
        : "N/A",
      type: "text",
    });
  }

  return items;
}
