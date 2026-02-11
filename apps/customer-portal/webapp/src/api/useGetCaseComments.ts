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
import { useMockConfig } from "@providers/MockConfigProvider";
import { useLogger } from "@hooks/useLogger";
import { mockCaseComments } from "@models/mockData";
import { ApiQueryKeys, API_MOCK_DELAY } from "@constants/apiConstants";
import { addApiHeaders } from "@utils/apiUtils";
import type { CaseCommentsResponse } from "@models/responses";

export interface UseGetCaseCommentsOptions {
  offset?: number;
  limit?: number;
}

/**
 * Fetches comments for a case.
 *
 * @param {string} projectId - The project id.
 * @param {string} caseId - The case id.
 * @param {UseGetCaseCommentsOptions} options - Optional offset and limit for pagination.
 * @returns {UseQueryResult<CaseCommentsResponse, Error>} Query result with comments.
 */
export default function useGetCaseComments(
  projectId: string,
  caseId: string,
  options?: UseGetCaseCommentsOptions,
): UseQueryResult<CaseCommentsResponse, Error> {
  const logger = useLogger();
  const { getIdToken, isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const { isMockEnabled } = useMockConfig();
  const { offset = 0, limit = 20 } = options ?? {};

  return useQuery<CaseCommentsResponse, Error>({
    queryKey: [
      ApiQueryKeys.CASE_COMMENTS,
      projectId,
      caseId,
      offset,
      limit,
      isMockEnabled,
    ],
    queryFn: async (): Promise<CaseCommentsResponse> => {
      logger.debug(
        `Fetching case comments: projectId=${projectId}, caseId=${caseId}, mock=${isMockEnabled}`,
      );

      if (isMockEnabled) {
        await new Promise((resolve) => setTimeout(resolve, API_MOCK_DELAY));
        const sliced = mockCaseComments.slice(offset, offset + limit);
        return {
          comments: sliced.length > 0 ? sliced : mockCaseComments,
          totalRecords: mockCaseComments.length,
          offset,
          limit,
        };
      }

      try {
        const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;
        if (!baseUrl) {
          throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
        }

        const idToken = await getIdToken();
        const params = new URLSearchParams();
        if (offset !== undefined) params.set("offset", String(offset));
        if (limit !== undefined) params.set("limit", String(limit));
        const query = params.toString();
        const requestUrl = `${baseUrl}/projects/${projectId}/cases/${caseId}/comments${query ? `?${query}` : ""}`;
        const response = await fetch(requestUrl, {
          method: "GET",
          headers: addApiHeaders(idToken),
        });

        if (!response.ok) {
          throw new Error(
            `Error fetching case comments: ${response.status} ${response.statusText}`,
          );
        }

        const data: CaseCommentsResponse = await response.json();
        logger.debug("[useGetCaseComments] Data received:", data);
        return data;
      } catch (error) {
        logger.error("[useGetCaseComments] Error:", error);
        throw error;
      }
    },
    enabled:
      !!projectId &&
      !!caseId &&
      (isMockEnabled || (isSignedIn && !isAuthLoading)),
    staleTime: 5 * 60 * 1000, // 5 minutes
  });
}
