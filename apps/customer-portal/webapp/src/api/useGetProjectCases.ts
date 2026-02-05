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
import { mockCases } from "@/models/mockData";
import { ApiQueryKeys, API_MOCK_DELAY } from "@/constants/apiConstants";
import type { CaseSearchRequest } from "@/models/requests";
import type { CaseSearchResponse } from "@/models/responses";

/**
 * Custom hook to search cases for a specific project.
 *
 * @param {string} projectId - The ID of the project to search cases for.
 * @param {CaseSearchRequest} requestBody - The search parameters including filters, pagination, and sorting.
 * @returns {UseQueryResult<CaseSearchResponse, Error>} The query result object.
 */
export default function useGetProjectCases(
  projectId: string,
  requestBody: CaseSearchRequest,
): UseQueryResult<CaseSearchResponse, Error> {
  const logger = useLogger();

  const { getIdToken, isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const { isMockEnabled } = useMockConfig();

  return useQuery<CaseSearchResponse, Error>({
    queryKey: [
      ApiQueryKeys.PROJECT_CASES,
      projectId,
      requestBody,
      isMockEnabled,
    ],
    queryFn: async (): Promise<CaseSearchResponse> => {
      logger.debug(
        `Fetching cases for project: ${projectId} with params:`,
        requestBody,
        `mock: ${isMockEnabled}`,
      );

      if (isMockEnabled) {
        await new Promise((resolve) => setTimeout(resolve, API_MOCK_DELAY));

        // TODO: Filter logic need be implemented here

        const { offset = 0, limit = 5 } = requestBody.pagination;
        const filteredCases = mockCases.filter(
          (cases) => cases.project.id === projectId || projectId === "all",
        );

        // slice for pagination
        const pagedCases = filteredCases.slice(offset, offset + limit);

        const response: CaseSearchResponse = {
          cases: pagedCases.length > 0 ? pagedCases : mockCases.slice(0, limit),
          totalRecords:
            filteredCases.length > 0 ? filteredCases.length : mockCases.length,
          offset,
          limit,
        };

        logger.debug("Cases fetched successfully (mock)", response);
        return response;
      }

      try {
        const idToken = await getIdToken();
        const baseUrl = import.meta.env.CUSTOMER_PORTAL_BACKEND_BASE_URL;

        if (!baseUrl) {
          throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
        }

        const requestUrl = `${baseUrl}/projects/${projectId}/cases/search`;

        const response = await fetch(requestUrl, {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
            Accept: "application/json",
            Authorization: `Bearer ${idToken}`,
            "x-user-id-token": idToken,
          },
          body: JSON.stringify(requestBody),
        });

        logger.debug(
          `[useGetProjectCases] Response status: ${response.status}`,
        );

        if (!response.ok) {
          throw new Error(
            `Error fetching project cases: ${response.statusText}`,
          );
        }

        const data: CaseSearchResponse = await response.json();
        logger.debug("[useGetProjectCases] Data received:", data);
        return data;
      } catch (error) {
        logger.error("[useGetProjectCases] Error:", error);
        throw error;
      }
    },
    enabled: !!projectId && (isMockEnabled || (isSignedIn && !isAuthLoading)),
  });
}
