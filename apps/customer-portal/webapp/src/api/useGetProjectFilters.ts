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
import type { CaseMetadataResponse } from "@models/responses";

/**
 * Custom hook to fetch all filters (cases, conversations, change requests, etc.) for a specific project.
 *
 * @param {string} projectId - The ID of the project to fetch filters for.
 * @returns {UseQueryResult<CaseMetadataResponse, Error>} The query result object containing all filter metadata.
 */
export default function useGetProjectFilters(
  projectId: string,
): UseQueryResult<CaseMetadataResponse, Error> {
  const logger = useLogger();
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();

  return useQuery<CaseMetadataResponse, Error>({
    queryKey: [ApiQueryKeys.PROJECT_CASES, "filters", projectId],
    queryFn: async (): Promise<CaseMetadataResponse> => {
      logger.debug(`Fetching project filters for project: ${projectId}`);

      try {
        const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;

        if (!baseUrl) {
          throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
        }

        const requestUrl = `${baseUrl}/projects/${projectId}/filters`;

        const response = await authFetch(requestUrl, {
          method: "GET",
        });

        if (!response.ok) {
          throw new Error(
            `Error fetching project filters: ${response.statusText}`,
          );
        }

        const data: CaseMetadataResponse = await response.json();
        logger.debug("[useGetProjectFilters] Data received:", data);
        return data;
      } catch (error) {
        logger.error("[useGetProjectFilters] Error:", error);
        throw error;
      }
    },
    enabled: !!projectId && isSignedIn && !isAuthLoading,
    staleTime: 10 * 60 * 1000, // Filters don't change often, keep for 10 mins
    gcTime: 10 * 60 * 1000,
    refetchOnMount: false,
    refetchOnWindowFocus: false,
    refetchOnReconnect: false,
  });
}
