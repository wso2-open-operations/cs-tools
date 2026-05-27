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
import { useQuery } from "@tanstack/react-query";

import { useProject } from "@context/project";

import { cases } from "@features/cases/api/cases.queries";
import { changeRequests } from "@features/changes/api/changes.queries";
import { computeDashboardStats, type DashboardStats } from "@features/dashboard/services/dashboardStats.service";

import { CASE_TYPES } from "@shared/constants";
import { useNavigation } from "@shared/hooks";

export function useDashboardStats(): {
  stats: DashboardStats;
  features: ReturnType<typeof useProject>["features"];
  navigateBySeverity: (id: string | number, label: string) => void;
  navigateByOperationsType: (id: string | number, type: string) => void;
} {
  const { toBySeverity, toOutstandingServiceRequests, toOutstandingChangeRequests } = useNavigation();
  const { projectId, features } = useProject();

  const { hasServiceRequestReadAccess, hasChangeRequestReadAccess, hasEngagementsReadAccess, hasSraReadAccess } =
    features ?? {};

  const { data: defaultCaseTypeStats } = useQuery(cases.stats(projectId!, { caseTypes: [CASE_TYPES.DEFAULT] }));

  const { data: engagementCaseTypeStats } = useQuery({
    ...cases.stats(projectId!, { caseTypes: [CASE_TYPES.ENGAGEMENT] }),
    enabled: !!hasEngagementsReadAccess,
  });

  const { data: serviceRequestCaseTypeStats } = useQuery({
    ...cases.stats(projectId!, { caseTypes: [CASE_TYPES.SERVICE_REQUEST] }),
    enabled: !!hasServiceRequestReadAccess,
  });

  const { data: changeRequestCaseTypeStats } = useQuery({
    ...changeRequests.stats(projectId!),
    enabled: !!hasChangeRequestReadAccess,
  });

  const { data: multipleCaseTypesStats } = useQuery(
    cases.stats(projectId!, {
      caseTypes: [
        CASE_TYPES.DEFAULT,
        ...(hasSraReadAccess ? [CASE_TYPES.SECURITY_REPORT_ANALYSIS] : []),
        ...(hasEngagementsReadAccess ? [CASE_TYPES.ENGAGEMENT] : []),
        ...(hasServiceRequestReadAccess ? [CASE_TYPES.SERVICE_REQUEST] : []),
      ],
    }),
  );

  const stats = computeDashboardStats(
    multipleCaseTypesStats,
    defaultCaseTypeStats,
    engagementCaseTypeStats,
    serviceRequestCaseTypeStats,
    changeRequestCaseTypeStats,
    features,
  );

  const navigateByOperationsType = (type: string | number) => {
    if (type === CASE_TYPES.SERVICE_REQUEST) toOutstandingServiceRequests();
    if (type === CASE_TYPES.CHANGE_REQUEST) toOutstandingChangeRequests();
  };

  return { stats, features, navigateBySeverity: toBySeverity, navigateByOperationsType };
}
