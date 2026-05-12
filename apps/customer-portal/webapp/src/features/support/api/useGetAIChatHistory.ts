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

import { useInfiniteQuery } from "@tanstack/react-query";
import { useAsgardeo } from "@asgardeo/react";
import { useAuthApiClient } from "@/hooks/useAuthApiClient";
import { useLogger } from "@hooks/useLogger";
import { ApiQueryKeys } from "@constants/apiConstants";
import { useEffect, useMemo } from "react";
import type {
  CaseActivitiesResponse,
  CaseCommentsResponse,
  CaseComment,
} from "@features/support/types/cases";

export type UseGetAIChatHistoryResult = {
  comments: CaseComment[];
  isLoading: boolean;
  isError: boolean;
  error: Error | null;
};

/**
 * Fetches all AI chat history (case activity comments) using infinite query with page size 20.
 * Automatically fetches all pages and returns all comments flattened.
 *
 * @param {string} projectId - The project id.
 * @param {string} caseId - The case id.
 * @returns {UseGetAIChatHistoryResult} Flattened comments and loading/error state.
 */
export default function useGetAIChatHistory(
  projectId: string,
  caseId: string,
): UseGetAIChatHistoryResult {
  const logger = useLogger();
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();

  const query = useInfiniteQuery<CaseCommentsResponse, Error>({
    queryKey: [
      ApiQueryKeys.CASE_COMMENTS,
      projectId,
      caseId,
      "ai-chat-history",
    ],
    initialPageParam: 0,
    getNextPageParam: (lastPage) => {
      const nextOffset = lastPage.offset + lastPage.limit;
      return nextOffset < lastPage.totalRecords ? nextOffset : undefined;
    },
    queryFn: async ({ pageParam }): Promise<CaseCommentsResponse> => {
      const offset = pageParam as number;
      logger.debug(
        `Fetching AI chat history: projectId=${projectId}, caseId=${caseId}, offset=${offset}`,
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
          `Error fetching AI chat history: ${response.status} ${response.statusText}`,
        );
      }

      const data: CaseActivitiesResponse = await response.json();
      logger.debug("[useGetAIChatHistory] Page received:", data);

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

  // Auto-fetch all remaining pages once each page loads
  useEffect(() => {
    if (query.hasNextPage && !query.isFetchingNextPage) {
      void query.fetchNextPage();
    }
  }, [query.hasNextPage, query.isFetchingNextPage, query.fetchNextPage]);

  const comments = useMemo(
    () => query.data?.pages.flatMap((p) => p.comments) ?? [],
    [query.data?.pages],
  );

  return {
    comments,
    isLoading: query.isLoading || query.hasNextPage || query.isFetchingNextPage,
    isError: query.isError,
    error: query.error,
  };
}
