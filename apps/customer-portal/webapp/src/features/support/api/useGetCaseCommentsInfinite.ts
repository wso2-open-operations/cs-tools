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
  type InfiniteData,
  type UseInfiniteQueryResult,
} from "@tanstack/react-query";
import { useAsgardeo } from "@asgardeo/react";
import { useAuthApiClient } from "@/hooks/useAuthApiClient";
import { useLogger } from "@hooks/useLogger";
import { ApiQueryKeys } from "@constants/apiConstants";
import type {
  CaseActivitiesResponse,
  CaseCommentsResponse,
} from "@features/support/types/cases";

/**
 * Infinite query for case comments, loading 10 at a time.
 * Call `fetchNextPage()` to load older comments.
 */
export default function useGetCaseCommentsInfinite(
  projectId: string,
  caseId: string,
): UseInfiniteQueryResult<InfiniteData<CaseCommentsResponse>, Error> {
  const logger = useLogger();
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();

  return useInfiniteQuery<CaseCommentsResponse, Error, InfiniteData<CaseCommentsResponse>>({
    queryKey: [ApiQueryKeys.CASE_COMMENTS, projectId, caseId, "infinite"],
    initialPageParam: 0,
    getNextPageParam: (lastPage) => {
      const nextOffset = lastPage.offset + lastPage.limit;
      return nextOffset < lastPage.totalRecords ? nextOffset : undefined;
    },
    queryFn: async ({ pageParam }): Promise<CaseCommentsResponse> => {
      const offset = pageParam as number;
      logger.debug(
        `Fetching case comments (infinite): projectId=${projectId}, caseId=${caseId}, offset=${offset}`,
      );

      const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;
      if (!baseUrl) {
        throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
      }

      const response = await authFetch(
        `${baseUrl}/cases/${caseId}/activities/search`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ pagination: { offset, limit: 10 } }),
        },
      );

      if (!response.ok) {
        throw new Error(
          `Error fetching case comments: ${response.status} ${response.statusText}`,
        );
      }

      const data: CaseActivitiesResponse = await response.json();
      return {
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
      };
    },
    enabled: !!projectId && !!caseId && isSignedIn && !isAuthLoading,
    staleTime: 0,
  });
}
