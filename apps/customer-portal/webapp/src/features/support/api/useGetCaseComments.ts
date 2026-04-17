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
import { useAuthApiClient } from "@/utils/useAuthApiClient";
import { useLogger } from "@hooks/useLogger";
import { ApiQueryKeys } from "@/constants/apiConstants";
import type { CaseCommentsResponse } from "@features/support/types/cases";
import type { UseGetCaseCommentsOptions } from "@features/support/types/supportApi";

export type { UseGetCaseCommentsOptions };

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
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();
  const { offset = 0, limit = 20 } = options ?? {};

  return useQuery<CaseCommentsResponse, Error>({
    queryKey: [ApiQueryKeys.CASE_COMMENTS, projectId, caseId, offset, limit],
    queryFn: async (): Promise<CaseCommentsResponse> => {
      logger.debug(
        `Fetching case comments: projectId=${projectId}, caseId=${caseId}`,
      );

      try {
        const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;
        if (!baseUrl) {
          throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
        }

        const params = new URLSearchParams();
        if (offset !== undefined) params.set("offset", String(offset));
        if (limit !== undefined) params.set("limit", String(limit));
        const query = params.toString();
        const requestUrl = `${baseUrl}/cases/${caseId}/comments${query ? `?${query}` : ""}`;
        const response = await authFetch(requestUrl, {
          method: "GET",
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
    enabled: !!projectId && !!caseId && isSignedIn && !isAuthLoading,
    staleTime: 5 * 60 * 1000, // 5 minutes
  });
}
