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

import {
  useInfiniteQuery,
  type UseInfiniteQueryResult,
  type InfiniteData,
} from "@tanstack/react-query";
import { useAsgardeo } from "@asgardeo/react";
import { useMockConfig } from "@providers/MockConfigProvider";
import { useLogger } from "@hooks/useLogger";
import { mockCases } from "@models/mockData";
import { ApiQueryKeys, API_MOCK_DELAY } from "@constants/apiConstants";
import { addApiHeaders } from "@utils/apiUtils";
import type { CaseSearchRequest } from "@models/requests";
import type { CaseSearchResponse } from "@models/responses";

/**
 * Custom hook to search cases for a specific project using infinite query.
 *
 * @param {string} projectId - The ID of the project to search cases for.
 * @param {Omit<CaseSearchRequest, 'pagination'>} baseRequest - The search parameters excluding pagination.
 * @returns {UseInfiniteQueryResult<CaseSearchResponse, Error>} The infinite query result object.
 */
export default function useGetProjectCases(
  projectId: string,
  baseRequest: Omit<CaseSearchRequest, "pagination">,
): UseInfiniteQueryResult<InfiniteData<CaseSearchResponse>, Error> {
  const logger = useLogger();

  const { getIdToken, isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const { isMockEnabled } = useMockConfig();

  return useInfiniteQuery<CaseSearchResponse, Error>({
    queryKey: [
      ApiQueryKeys.PROJECT_CASES,
      projectId,
      baseRequest,
      isMockEnabled,
    ],
    queryFn: async ({ pageParam = 0 }): Promise<CaseSearchResponse> => {
      const requestBody: CaseSearchRequest = {
        ...baseRequest,
        pagination: {
          offset: pageParam as number,
          limit: 10,
        },
      };

      logger.debug(
        `Fetching cases for project: ${projectId} with params:`,
        requestBody,
        `mock: ${isMockEnabled}`,
      );

      if (isMockEnabled) {
        await new Promise((resolve) => setTimeout(resolve, API_MOCK_DELAY));

        const { offset = 0, limit = 10 } = requestBody.pagination;
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
        const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;

        if (!baseUrl) {
          throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
        }

        const requestUrl = `${baseUrl}/projects/${projectId}/cases/search`;

        const response = await fetch(requestUrl, {
          method: "POST",
          headers: addApiHeaders(idToken),
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
    initialPageParam: 0,
    getNextPageParam: (lastPage) => {
      const nextOffset = lastPage.offset + lastPage.limit;
      return nextOffset < lastPage.totalRecords ? nextOffset : undefined;
    },
    enabled: !!projectId && (isMockEnabled || (isSignedIn && !isAuthLoading)),
    staleTime: 5 * 60 * 1000,
    refetchOnWindowFocus: false,
  });
}
