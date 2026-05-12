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
import type { ProjectFeatures } from "@features/project-hub/types/projects";
import { PRIMARY_PRODUCTION_DEPLOYMENT_TYPE_LABEL } from "@constants/permissionConstants";
import type {
  GetProjectPermissionsOptions,
  ProjectOperationsStatsResult,
  ProjectPermissions,
} from "@/types/permission";
import { ProjectClosureState, ProjectType } from "@/types/permission";
import { ProductCategory } from "@features/project-details/types/deployments";
import { convertMinutesToHours } from "@features/project-details/utils/projectDetails";
import { formatBackendTimestampForDisplay } from "@utils/dateTime";

export { PRIMARY_PRODUCTION_DEPLOYMENT_TYPE_LABEL } from "@constants/permissionConstants";
export { ProjectClosureState, ProjectType } from "@/types/permission";
export { ProductCategory } from "@features/project-details/types/deployments";
export type {
  GetProjectPermissionsOptions,
  ProjectOperationsStatsResult,
  ProjectPermissions,
} from "@/types/permission";

export type ProjectSeverityPolicy = {
  excludeS0: boolean;
  restrictSeverityToLow: boolean;
};

const CATASTROPHIC_SEVERITY_TAG = "(P0)";
const LOW_SEVERITY_TAG = "(P4)";
const CATASTROPHIC_SEVERITY_ID = "14";
const LOW_SEVERITY_ID = "13";
const NOT_APPLICABLE_ONBOARDING_STATUS = "not-applicable";

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
    hasSraWriteAccess: false,
    showOutstandingOpsChart: false,
    includeChangeRequestsInDashboardTotals: false,
    includeS0InSupportMetrics: false,
    showServiceHoursAllocationsCard: false,
    hasEngagements: false,
    hasUpdates: false,
  };
}

/**
 * Returns UI and stats permissions for a project type label (API string).
 *
 * @param projectTypeLabel - Kept for API parity with legacy callers; ignored when
 * permissions are derived from project features.
 * @param options - Optional API-driven feature payload from `/projects/{projectId}/features`.
 * @returns Permission flags for conditional rendering and aggregations.
 */
export function getProjectPermissions(
  _projectTypeLabel: string | null | undefined,
  options?: GetProjectPermissionsOptions,
): ProjectPermissions {
  const features = options?.projectFeatures;
  if (!features) return restrictivePermissions();

  const hasSR = features.hasServiceRequestReadAccess;
  const hasCR = features.hasChangeRequestReadAccess;
  const hasDeployments =
    features.hasDeploymentReadAccess || features.hasDeploymentWriteAccess;
  const hasTimeLogs = features.hasTimeLogsReadAccess;
  const hasCatastrophicSeverity = hasAcceptedSeverityLabel(
    features,
    CATASTROPHIC_SEVERITY_TAG,
  );

  return {
    hasOperations: hasSR || hasCR,
    hasSR,
    hasCR,
    hasDeployments,
    hasQueryHours: hasTimeLogs,
    hasTimeLogs,
    hasSecurityReportAnalysis: features.hasSraReadAccess,
    hasSraWriteAccess: features.hasSraWriteAccess,
    showOutstandingOpsChart: hasSR || hasCR,
    includeChangeRequestsInDashboardTotals: hasCR,
    includeS0InSupportMetrics: hasCatastrophicSeverity,
    showServiceHoursAllocationsCard: hasTimeLogs,
    hasEngagements: features.hasEngagementsReadAccess,
    hasUpdates: features.hasUpdatesReadAccess,
  };
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
 * @param projectTypeLabel - Kept for API parity with legacy callers; ignored when
 * severity policy is derived from project features.
 * @returns True when severity must be forced to S4.
 */
export function shouldForceSeverityS4(
  _projectTypeLabel: string | null | undefined,
  options?: GetProjectPermissionsOptions,
): boolean {
  const acceptedSeverities =
    options?.projectFeatures?.acceptedSeverityValues ?? [];
  if (acceptedSeverities.length !== 1) return false;
  return hasSeverityMatch(
    acceptedSeverities[0],
    LOW_SEVERITY_ID,
    LOW_SEVERITY_TAG,
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
  options?: GetProjectPermissionsOptions,
): ProjectSeverityPolicy {
  return {
    excludeS0: shouldExcludeS0(projectTypeLabel, options),
    restrictSeverityToLow: shouldForceSeverityS4(projectTypeLabel, options),
  };
}

function hasAcceptedSeverityLabel(
  features: ProjectFeatures,
  labelFragment: string,
): boolean {
  const severityId =
    labelFragment === CATASTROPHIC_SEVERITY_TAG
      ? CATASTROPHIC_SEVERITY_ID
      : LOW_SEVERITY_ID;
  return features.acceptedSeverityValues.some((severity) =>
    hasSeverityMatch(severity, severityId, labelFragment),
  );
}

function hasSeverityMatch(
  severity: { id?: string; label?: string },
  expectedId: string,
  fallbackLabelTag: string,
): boolean {
  if ((severity.id ?? "").trim() === expectedId) {
    return true;
  }
  const normalizedLabel = (severity.label ?? "").trim().toLowerCase();
  const normalizedFallback = fallbackLabelTag.trim().toLowerCase();
  return normalizedLabel.includes(normalizedFallback);
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
    projectTypeLabel === ProjectType.CLOUD_EVALUATION_SUPPORT
  );
}

/**
 * Whether the project is a cloud support project (Cloud Support, Cloud Subscription, or Cloud Evaluation).
 *
 * @param projectTypeLabel - Value from project.type.label.
 * @returns True when the project is a cloud support type.
 */
export function isCloudSupportProject(
  projectTypeLabel: string | null | undefined,
): boolean {
  return projectTypeLabel === ProjectType.CLOUD_SUPPORT;
}

/**
 * Whether onboarding-specific UI should be hidden for the project.
 *
 * @param onboardingStatus - Project onboarding status value from API.
 * @returns True when onboarding status is "Not-Applicable".
 */
export function shouldHideOnboardingData(
  onboardingStatus: string | null | undefined,
): boolean {
  const normalized = (onboardingStatus ?? "").trim().toLowerCase();
  return normalized === NOT_APPLICABLE_ONBOARDING_STATUS;
}

/**
 * Returns the product categories to pass when fetching products for case creation.
 * Only Cloud Subscription and Cloud Evaluation Support projects filter by Cloud products.
 *
 * @param projectTypeLabel - Value from project.type.label.
 * @returns Product category filter array, or undefined when no filter should be applied.
 */
export function getProductCategoriesForCaseCreation(
  projectTypeLabel: string | null | undefined,
): ProductCategory[] | undefined {
  if (
    projectTypeLabel === ProjectType.CLOUD_SUPPORT ||
    projectTypeLabel === ProjectType.CLOUD_EVALUATION_SUPPORT
  ) {
    return [ProductCategory.CLOUD];
  }
  return undefined;
}

/**
 * Returns the product categories to pass when fetching products for service request creation.
 * Only Cloud Subscription and Cloud Evaluation Support projects use PDP products.
 *
 * @param projectTypeLabel - Value from project.type.label.
 * @returns Product category filter array, or undefined when no filter should be applied.
 */
export function getProductCategoriesForServiceRequest(
  projectTypeLabel: string | null | undefined,
): ProductCategory[] | undefined {
  if (
    projectTypeLabel === ProjectType.CLOUD_SUPPORT ||
    projectTypeLabel === ProjectType.CLOUD_EVALUATION_SUPPORT
  ) {
    return [ProductCategory.PDP];
  }
  return undefined;
}

/**
 * Whether the project is in a Restricted closure state.
 * When restricted, action buttons (create SR, add deployment, add user, etc.) must be hidden.
 *
 * @param closureState - Value from project.closureState.
 * @returns True when the project is restricted.
 */
export function isProjectRestricted(
  closureState: string | null | undefined,
): boolean {
  return closureState === ProjectClosureState.RESTRICTED;
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
  options?: GetProjectPermissionsOptions,
): ActivityItem[] {
  const permissions = getProjectPermissions(projectTypeLabel, options);

  const formatDateTime = (dateString: string): string => {
    if (!dateString) return "";
    const dateStr = formatBackendTimestampForDisplay(dateString, {
      month: "short",
      day: "numeric",
      year: "numeric",
    });
    const timeStr = formatBackendTimestampForDisplay(dateString, {
      hour: "numeric",
      minute: "2-digit",
      hour12: true,
    });
    if (!dateStr || !timeStr) return "";
    return `${dateStr} at ${timeStr}`;
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
