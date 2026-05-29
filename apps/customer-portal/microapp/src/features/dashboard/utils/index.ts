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

import type { CasesStatsDto } from "@features/case-types/cases/types/case.dto";
import type { ChangeRequestsStatsDto } from "@features/case-types/change-requests/types/change.dto";
import type { PieDataItem } from "@features/dashboard/components";
import type { ProjectFeaturesDto } from "@features/projects/types/project.dto";

import { overrideOrDefault } from "@shared/utils/string.utils";

import {
  CASE_TYPES,
  CHANGE_REQUESTS_LABEL,
  ENGAGEMENTS_TYPE_PIE_COLORS,
  PROJECT_SEVERITY_PIE_COLORS,
  SERVICE_REQUESTS_LABEL,
} from "@shared/constants";

export type DashboardStats = {
  actionRequired: number | undefined;
  outstanding: number | undefined;
  resolvedThisMonth: number | undefined;
  averageResponseTime: number | undefined;
  outstandingSupportCasesPieData: (PieDataItem & { id: string | number })[] | undefined;
  outstandingEngagementsPieData: (PieDataItem & { id: string | number })[] | undefined;
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
  const { hasChangeRequestReadAccess = false, hasServiceRequestReadAccess } = features ?? {};

  const loading =
    multipleCaseTypesStats === undefined || (hasChangeRequestReadAccess && changeRequestCaseTypeStats === undefined);

  const actionRequired = loading
    ? undefined
    : (multipleCaseTypesStats?.actionRequiredCount ?? 0) +
      (hasChangeRequestReadAccess ? (changeRequestCaseTypeStats?.actionRequiredCount ?? 0) : 0);

  const outstanding = loading
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

  const outstandingEngagementsPieData: (PieDataItem & { id: string | number })[] | undefined =
    engagementCaseTypeStats?.outstandingEngagementTypeCount.map((item) => ({
      id: item.id,
      label: overrideOrDefault(item.label),
      value: item.count,
      color: ENGAGEMENTS_TYPE_PIE_COLORS[item.label] || colors.grey[500],
    })) ?? undefined;

  const operationsData: (PieDataItem & { id: string | number })[] = [];

  if (hasServiceRequestReadAccess) {
    operationsData.push({
      id: CASE_TYPES.SERVICE_REQUEST,
      label: SERVICE_REQUESTS_LABEL,
      value: serviceRequestCaseTypeStats?.outstandingCount ?? 0,
      color: colors.orange[500],
    });
  }
  if (hasChangeRequestReadAccess) {
    operationsData.push({
      id: CASE_TYPES.CHANGE_REQUEST,
      label: CHANGE_REQUESTS_LABEL,
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
    actionRequired,
    outstanding,
    resolvedThisMonth,
    averageResponseTime,
    outstandingSupportCasesPieData,
    outstandingEngagementsPieData,
    outstandingOperationsPieData,
  };
}
