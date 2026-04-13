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

import { useQuery, type UseQueryResult } from "@tanstack/react-query";
import { useAsgardeo } from "@asgardeo/react";
import { useAuthApiClient } from "@api/useAuthApiClient";
import { useLogger } from "@hooks/useLogger";
import { ApiQueryKeys } from "@constants/apiConstants";
import type { ProjectCasesStats } from "@/types/cases";
import { CaseType } from "@constants/supportConstants";

export { DASHBOARD_CASE_TYPE_LABELS } from "@constants/dashboardConstants";

export interface UseGetProjectCasesStatsOptions {
  incidentId?: string;
  queryId?: string;
  caseTypes?: Array<`${(typeof CaseType)[keyof typeof CaseType]}` | string>;
  createdByMe?: boolean;
  enabled?: boolean;
}

/**
 * Custom hook to fetch project case statistics by ID.
 * API expects ?caseTypes=queryId&caseTypes=incidentId (both required for filtered stats).
 *
 * @param {string} id - The ID of the project.
 * @param {UseGetProjectCasesStatsOptions} [options] - incidentId, queryId, enabled.
 * @returns {UseQueryResult<ProjectCasesStats, Error>} The query result object.
 */
export function useGetProjectCasesStats(
  id: string,
  options?: UseGetProjectCasesStatsOptions,
): UseQueryResult<ProjectCasesStats, Error> {
  const logger = useLogger();
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();
  const { incidentId, queryId, caseTypes, createdByMe, enabled = true } = options ?? {};

  return useQuery<ProjectCasesStats, Error>({
    queryKey: [ApiQueryKeys.CASES_STATS, id, incidentId, queryId, caseTypes, createdByMe],
    queryFn: async (): Promise<ProjectCasesStats> => {
      logger.debug(`Fetching case stats for project ID: ${id}`);

      try {
        const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;

        if (!baseUrl) {
          throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
        }

        let requestUrl = `${baseUrl}/projects/${id}/stats/cases`;

        const params = new URLSearchParams();

        if (Array.isArray(caseTypes) && caseTypes.length > 0) {
          caseTypes.forEach((type) => {
            if (type) {
              params.append("caseTypes", type);
            }
          });
        } else if (incidentId && queryId) {
          params.append("caseTypes", queryId);
          params.append("caseTypes", incidentId);
        }

        if (createdByMe) {
          params.append("createdBy", "me");
        }

        const queryString = params.toString();
        if (queryString) {
          requestUrl += `?${queryString}`;
        }

        const response = await authFetch(requestUrl, {
          method: "GET",
        });

        logger.debug(
          `[useGetProjectCasesStats] Response status for ${id}: ${response.status}`,
        );

        if (!response.ok) {
          throw new Error(`Error fetching case stats: ${response.statusText}`);
        }

        const raw = (await response.json()) as any;

        const data: ProjectCasesStats = {
          totalCases: raw?.totalCases ?? raw?.totalCount ?? 0,
          totalCount: raw?.totalCount,
          activeCount: raw?.activeCount,
          outstandingCount: raw?.outstandingCount,
          averageResponseTime: raw?.averageResponseTime ?? 0,
          resolvedCases: {
            total: raw?.resolvedCases?.total ?? 0,
            currentMonth: raw?.resolvedCases?.currentMonth ?? 0,
            pastThirtyDays: raw?.resolvedCases?.pastThirtyDays,
          },
          changeRate: raw?.changeRate,
          stateCount: raw?.stateCount ?? [],
          severityCount: raw?.severityCount ?? [],
          outstandingSeverityCount: raw?.outstandingSeverityCount ?? [],
          caseTypeCount: raw?.caseTypeCount ?? [],
          casesTrend: raw?.casesTrend ?? [],
          engagementTypeCount: raw?.engagementTypeCount ?? [],
          outstandingEngagementTypeCount:
            raw?.outstandingEngagementTypeCount ?? [],
        };

        logger.debug("[useGetProjectCasesStats] Data received:", data);
        return data;
      } catch (error) {
        logger.error("[useGetProjectCasesStats] Error:", error);
        throw error;
      }
    },
    enabled: !!id && isSignedIn && !isAuthLoading && enabled,
    staleTime: 5 * 60 * 1000,
    gcTime: 10 * 60 * 1000,
    refetchOnWindowFocus: false,
    refetchOnReconnect: false,
  });
}
