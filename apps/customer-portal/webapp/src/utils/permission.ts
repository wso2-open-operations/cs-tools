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

import type { ActivityItem } from "@features/project-details/types/projectDetails";
import type { ProjectStatsResponse } from "@features/project-hub/types/projects";
import { PRIMARY_PRODUCTION_DEPLOYMENT_TYPE_LABEL } from "@constants/permissionConstants";
import type {
  GetProjectPermissionsOptions,
  ProjectOperationsStatsResult,
  ProjectPermissions,
} from "@/types/permission";
import { ProjectType } from "@/types/permission";
import { convertMinutesToHours } from "@features/project-details/utils/projectDetails";

export { PRIMARY_PRODUCTION_DEPLOYMENT_TYPE_LABEL } from "@constants/permissionConstants";
export { ProjectType } from "@/types/permission";
export type {
  GetProjectPermissionsOptions,
  ProjectOperationsStatsResult,
  ProjectPermissions,
} from "@/types/permission";

export type ProjectSeverityPolicy = {
  excludeS0: boolean;
  restrictSeverityToLow: boolean;
};

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
    hasSecurityReportAnalysis: false,
    showOutstandingOpsChart: false,
    includeChangeRequestsInDashboardTotals: false,
    includeS0InSupportMetrics: false,
    showServiceHoursAllocationsCard: false,
    hasEngagements: false,
  };
}

/**
 * Returns UI and stats permissions for a project type label (API string).
 *
 * @param projectTypeLabel - Value from project.type.label.
 * @param options - Optional flags from project search/details (e.g. `hasPdpSubscription` for Cloud SR).
 * @returns Permission flags for conditional rendering and aggregations.
 */
export function getProjectPermissions(
  projectTypeLabel: string | null | undefined,
  options?: GetProjectPermissionsOptions,
): ProjectPermissions {
  const permissions = restrictivePermissions();
  const label = projectTypeLabel ?? "";
  const hasPdpSubscription = options?.hasPdpSubscription === true;

  switch (label) {
    case ProjectType.MANAGED_CLOUD_SUBSCRIPTION:
      permissions.hasOperations = true;
      permissions.hasSR = true;
      permissions.hasCR = true;
      permissions.hasDeployments = true;
      permissions.hasQueryHours = true;
      permissions.hasTimeLogs = true;
      permissions.hasSecurityReportAnalysis = true;
      permissions.showOutstandingOpsChart = true;
      permissions.includeChangeRequestsInDashboardTotals = true;
      permissions.includeS0InSupportMetrics = true;
      permissions.showServiceHoursAllocationsCard = true;
      permissions.hasEngagements = true;
      break;

    case ProjectType.CLOUD_SUPPORT:
    case ProjectType.CLOUD_SUBSCRIPTION: {
      permissions.hasOperations = true;
      permissions.hasSR = hasPdpSubscription;
      permissions.hasCR = false;
      permissions.hasDeployments = false;
      permissions.hasQueryHours = true;
      permissions.hasTimeLogs = false;
      permissions.hasSecurityReportAnalysis = true;
      permissions.showOutstandingOpsChart = hasPdpSubscription;
      permissions.includeChangeRequestsInDashboardTotals = false;
      permissions.includeS0InSupportMetrics = false;
      permissions.showServiceHoursAllocationsCard = false;
      permissions.hasEngagements = true;
      break;
    }

    case ProjectType.CLOUD_EVALUATION_SUPPORT:
    case ProjectType.EVALUATION_SUBSCRIPTION:
      break;

    case ProjectType.DEVELOPMENT_SUPPORT:
      permissions.hasSecurityReportAnalysis = true;
      break;

    case ProjectType.PROFESSIONAL_SERVICES:
      permissions.hasSecurityReportAnalysis = true;
      break;

    case ProjectType.SUBSCRIPTION:
      permissions.hasOperations = false;
      permissions.hasSR = false;
      permissions.hasCR = false;
      permissions.hasDeployments = true;
      permissions.hasQueryHours = true;
      permissions.hasTimeLogs = true;
      permissions.hasSecurityReportAnalysis = true;
      permissions.showOutstandingOpsChart = false;
      permissions.includeChangeRequestsInDashboardTotals = false;
      permissions.includeS0InSupportMetrics = false;
      permissions.showServiceHoursAllocationsCard = true;
      permissions.hasEngagements = true;
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
  options?: GetProjectPermissionsOptions,
): boolean {
  return !getProjectPermissions(projectTypeLabel, options)
    .includeS0InSupportMetrics;
}

/**
 * Whether the severity must be locked to S4 (Low) for case creation.
 * Development Support projects always use S4 and the field must not be editable.
 *
 * @param projectTypeLabel - Value from project.type.label.
 * @returns True when severity must be forced to S4.
 */
export function shouldForceSeverityS4(
  projectTypeLabel: string | null | undefined,
): boolean {
  return (
    projectTypeLabel === ProjectType.DEVELOPMENT_SUPPORT ||
    projectTypeLabel === ProjectType.PROFESSIONAL_SERVICES
  );
}

/**
 * Centralized severity rendering policy for list filters, tables, and create flows.
 *
 * @param projectTypeLabel - Value from project.type.label.
 * @returns Severity policy flags for conditional UI behavior.
 */
export function getProjectSeverityPolicy(
  projectTypeLabel: string | null | undefined,
): ProjectSeverityPolicy {
  return {
    excludeS0: shouldExcludeS0(projectTypeLabel),
    restrictSeverityToLow: shouldForceSeverityS4(projectTypeLabel),
  };
}

/**
 * Whether case/SR/security report flows should be restricted to primary
 * production deployments only (Cloud Support and Cloud Evaluation Support).
 *
 * @param projectTypeLabel - Value from project.type.label.
 * @returns True when only primary production deployments should be used.
 */
export function shouldRestrictToPrimaryProductionDeployments(
  projectTypeLabel: string | null | undefined,
): boolean {
  return (
    projectTypeLabel === ProjectType.CLOUD_SUPPORT ||
    projectTypeLabel === ProjectType.CLOUD_SUBSCRIPTION ||
    projectTypeLabel === ProjectType.CLOUD_EVALUATION_SUPPORT
  );
}

/**
 * Filters a deployment list to only primary production entries when the
 * project type requires it; otherwise returns the list unchanged.
 *
 * @param deployments - Project deployments.
 * @param projectTypeLabel - Value from project.type.label.
 * @returns Deployments narrowed per project type rules.
 */
export function filterDeploymentsForCaseCreation<
  T extends { type?: { label?: string | null } | null },
>(
  deployments: T[] | undefined,
  projectTypeLabel: string | null | undefined,
): T[] {
  if (projectTypeLabel === undefined) return [];
  const list = deployments ?? [];
  if (!shouldRestrictToPrimaryProductionDeployments(projectTypeLabel)) {
    return list;
  }
  const target = PRIMARY_PRODUCTION_DEPLOYMENT_TYPE_LABEL.toLowerCase();
  return list.filter(
    (d) => (d.type?.label ?? "").trim().toLowerCase() === target,
  );
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
  const serviceRequests = permissions.hasSR ? srCount : 0;
  const changeRequests = permissions.hasCR ? crCount : 0;
  const total = serviceRequests + changeRequests;

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
