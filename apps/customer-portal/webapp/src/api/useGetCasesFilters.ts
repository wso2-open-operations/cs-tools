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
import { useMockConfig } from "@/providers/MockConfigProvider";
import { useLogger } from "@/hooks/useLogger";
import { mockCaseMetadata } from "@/models/mockData";
import { ApiQueryKeys, API_MOCK_DELAY } from "@/constants/apiConstants";
import type { CaseMetadataResponse } from "@/models/responses";

/**
 * Custom hook to fetch case filters for a specific project.
 *
 * @param {string} projectId - The ID of the project to fetch filters for.
 * @returns {UseQueryResult<CaseMetadataResponse, Error>} The query result object.
 */
export default function useGetCasesFilters(
  projectId: string,
): UseQueryResult<CaseMetadataResponse, Error> {
  const logger = useLogger();
  const { getIdToken, isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const { isMockEnabled } = useMockConfig();

  return useQuery<CaseMetadataResponse, Error>({
    queryKey: [ApiQueryKeys.PROJECT_CASES, "filters", projectId, isMockEnabled],
    queryFn: async (): Promise<CaseMetadataResponse> => {
      logger.debug(
        `Fetching case filters for project: ${projectId} mock: ${isMockEnabled}`,
      );

      if (isMockEnabled) {
        await new Promise((resolve) => setTimeout(resolve, API_MOCK_DELAY));
        logger.debug(
          "Case filters fetched successfully (mock)",
          mockCaseMetadata,
        );
        return mockCaseMetadata;
      }

      try {
        const idToken = await getIdToken();
        const baseUrl = import.meta.env.CUSTOMER_PORTAL_BACKEND_BASE_URL;

        if (!baseUrl) {
          throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
        }

        const requestUrl = `${baseUrl}/projects/${projectId}/cases/filters`;

        const response = await fetch(requestUrl, {
          method: "GET",
          headers: {
            "Content-Type": "application/json",
            Accept: "application/json",
            Authorization: `Bearer ${idToken}`,
            "x-user-id-token": idToken,
          },
        });

        if (!response.ok) {
          throw new Error(
            `Error fetching case filters: ${response.statusText}`,
          );
        }

        const data: CaseMetadataResponse = await response.json();
        logger.debug("[useGetCasesFilters] Data received:", data);
        return data;
      } catch (error) {
        logger.error("[useGetCasesFilters] Error:", error);
        throw error;
      }
    },
    enabled: !!projectId && (isMockEnabled || (isSignedIn && !isAuthLoading)),
    staleTime: 10 * 60 * 1000, // Filters don't change often, keep for 10 mins
  });
}
