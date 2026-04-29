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

import { colors } from "@wso2/oxygen-ui";
import type { PieDataItem } from "@features/dashboard/components/PieChartWidget";
import type { CasesStatsDto } from "@features/cases/types/case.dto";
import type { ChangeRequestsStatsDto } from "@features/changes/types/change.dto";
import type { ProjectFeaturesDto } from "@features/projects/types/project.dto";
import { overrideOrDefault } from "@shared/utils/string.utils";
import { ENGAGEMENTS_TYPE_PIE_COLORS, PROJECT_SEVERITY_PIE_COLORS } from "@config/constants";

export type DashboardStats = {
  totalInteractions: number | undefined;
  activeInteractions: number | undefined;
  resolvedThisMonth: number | undefined;
  averageResponseTime: number | undefined;
  outstandingSupportCasesPieData: (PieDataItem & { id: string | number })[] | undefined;
  outstandingEngagementsPieData: (PieDataItem & { id: string | number })[];
  outstandingOperationsPieData: (PieDataItem & { id: string | number })[] | undefined;
};

export function computeDashboardStats(
  multipleCaseTypesStats: CasesStatsDto | undefined,
  defaultCaseTypeStats: CasesStatsDto | undefined,
  engagementCaseTypeStats: CasesStatsDto | undefined,
  serviceRequestCaseTypeStats: CasesStatsDto | undefined,
  changeRequestCaseTypeStats: ChangeRequestsStatsDto | undefined,
  features: Pick<ProjectFeaturesDto, "hasChangeRequestReadAccess" | "hasServiceRequestReadAccess"> | undefined,
): DashboardStats {
  const hasChangeRequestReadAccess = features?.hasChangeRequestReadAccess ?? false;
  const hasServiceRequestReadAccess = features?.hasServiceRequestReadAccess ?? false;

  const isInteractionsLoading =
    multipleCaseTypesStats === undefined ||
    (hasChangeRequestReadAccess && changeRequestCaseTypeStats === undefined);

  const totalInteractions = isInteractionsLoading
    ? undefined
    : (multipleCaseTypesStats?.actionRequiredCount ?? 0) +
      (hasChangeRequestReadAccess ? (changeRequestCaseTypeStats?.actionRequiredCount ?? 0) : 0);

  const activeInteractions = isInteractionsLoading
    ? undefined
    : (multipleCaseTypesStats?.outstandingCount ?? 0) +
      (hasChangeRequestReadAccess ? (changeRequestCaseTypeStats?.outstandingCount ?? 0) : 0);

  const resolvedThisMonth =
    multipleCaseTypesStats?.resolvedCases?.pastThirtyDays === undefined &&
    (!hasChangeRequestReadAccess || changeRequestCaseTypeStats?.resolvedCount?.pastThirtyDays === undefined)
      ? undefined
      : (multipleCaseTypesStats?.resolvedCases?.pastThirtyDays ?? 0) +
        (hasChangeRequestReadAccess ? (changeRequestCaseTypeStats?.resolvedCount?.pastThirtyDays ?? 0) : 0);

  const averageResponseTime = multipleCaseTypesStats?.averageResponseTime;

  const outstandingSupportCasesPieData = defaultCaseTypeStats?.outstandingSeverityCount.map((item) => ({
    id: item.id,
    label: overrideOrDefault(item.label),
    value: item.count,
    color: PROJECT_SEVERITY_PIE_COLORS[item.id] || colors.grey[500],
  }));

  const outstandingEngagementsPieData: (PieDataItem & { id: string | number })[] =
    engagementCaseTypeStats?.outstandingEngagementTypeCount.map((item) => ({
      id: item.id,
      label: overrideOrDefault(item.label),
      value: item.count,
      color: ENGAGEMENTS_TYPE_PIE_COLORS[item.label] || colors.grey[500],
    })) ?? [];

  const operationsData: (PieDataItem & { id: string | number })[] = [];
  if (hasServiceRequestReadAccess) {
    operationsData.push({
      id: "service",
      label: "Service Requests",
      value: serviceRequestCaseTypeStats?.outstandingCount ?? 0,
      color: colors.orange[500],
    });
  }
  if (hasChangeRequestReadAccess) {
    operationsData.push({
      id: "change",
      label: "Change Requests",
      value: changeRequestCaseTypeStats?.outstandingCount ?? 0,
      color: colors.blue[500],
    });
  }

  const outstandingOperationsPieData =
    serviceRequestCaseTypeStats?.outstandingCount != undefined ||
    changeRequestCaseTypeStats?.outstandingCount != undefined
      ? operationsData
      : undefined;

  return {
    totalInteractions,
    activeInteractions,
    resolvedThisMonth,
    averageResponseTime,
    outstandingSupportCasesPieData,
    outstandingEngagementsPieData,
    outstandingOperationsPieData,
  };
}
