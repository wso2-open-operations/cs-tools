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
import { useAuthApiClient } from "@/hooks/useAuthApiClient";
import { useLogger } from "@hooks/useLogger";
import { ApiQueryKeys } from "@constants/apiConstants";
import type {
  CaseActivitiesResponse,
  CaseCommentsResponse,
} from "@features/support/types/cases";
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

  const mapActivitiesToCommentsResponse = (
    data: CaseActivitiesResponse,
  ): CaseCommentsResponse => ({
    comments: (data.activities ?? []).map((activity) => ({
      id: activity.id,
      content: activity.content,
      createdOn: activity.createdOn,
      createdBy: activity.createdBy,
      createdByFirstName: activity.createdByFirstName,
      createdByLastName: activity.createdByLastName,
      createdByFullName: activity.createdByFullName,
      type: activity.type,
      fileName: activity.fileName,
      contentType: activity.contentType,
      sizeBytes: activity.sizeBytes,
      downloadUrl: activity.downloadUrl,
      isEscalated: false,
    })),
    totalRecords: data.totalRecords,
    offset: data.offset,
    limit: data.limit,
  });

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

        const requestUrl = `${baseUrl}/cases/${caseId}/activities/search`;
        const response = await authFetch(requestUrl, {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
          },
          body: JSON.stringify({
            pagination: {
              offset,
              limit,
            },
          }),
        });

        if (!response.ok) {
          throw new Error(
            `Error fetching case comments: ${response.status} ${response.statusText}`,
          );
        }

        const data: CaseActivitiesResponse = await response.json();
        logger.debug("[useGetCaseComments] Data received:", data);
        return mapActivitiesToCommentsResponse(data);
      } catch (error) {
        logger.error("[useGetCaseComments] Error:", error);
        throw error;
      }
    },
    enabled: !!projectId && !!caseId && isSignedIn && !isAuthLoading,
    staleTime: 0,
  });
}
