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

import { useInfiniteQuery, type InfiniteData } from "@tanstack/react-query";
import { useAsgardeo } from "@asgardeo/react";
import { useLogger } from "@hooks/useLogger";
import { ApiQueryKeys } from "@constants/apiConstants";
import { useAuthApiClient } from "@context/AuthApiContext";
import type { CaseCommentsResponse } from "@models/responses";

const PAGE_SIZE = 10;

/**
 * Fetches change request comments with pagination (GET /change-requests/:changeRequestId/comments).
 * Uses infinite query to support server-side pagination.
 *
 * @param {string} changeRequestId - The change request ID.
 * @returns {UseInfiniteQueryResult} Infinite query result with change request comments.
 */
export function useInfiniteChangeRequestComments(changeRequestId: string) {
  const logger = useLogger();
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const fetchFn = useAuthApiClient();

  return useInfiniteQuery<
    CaseCommentsResponse,
    Error,
    InfiniteData<CaseCommentsResponse>,
    readonly (string | number)[],
    number
  >({
    queryKey: [
      ApiQueryKeys.CHANGE_REQUEST_COMMENTS,
      changeRequestId,
      "infinite",
    ],
    queryFn: async ({ pageParam, signal }): Promise<CaseCommentsResponse> => {
      logger.debug(
        `[useInfiniteChangeRequestComments] Fetching comments for change request ${changeRequestId}, offset: ${pageParam}`,
      );

      try {
        const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;
        if (!baseUrl) {
          throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
        }

        const params = new URLSearchParams({
          limit: String(PAGE_SIZE),
          offset: String(pageParam),
        });
        const encodedChangeRequestId = encodeURIComponent(changeRequestId);
        const requestUrl = `${baseUrl}/change-requests/${encodedChangeRequestId}/comments?${params}`;
        const response = await fetchFn(requestUrl, { method: "GET", signal });

        logger.debug(
          `[useInfiniteChangeRequestComments] Response status: ${response.status}`,
        );

        if (!response.ok) {
          throw new Error(
            `Error fetching change request comments: ${response.statusText}`,
          );
        }

        const data: CaseCommentsResponse = await response.json();
        logger.debug(
          `[useInfiniteChangeRequestComments] Page received: ${data.comments?.length ?? 0} comments, offset ${data.offset}, total ${data.totalRecords}`,
        );
        return data;
      } catch (error) {
        const errorMessage =
          error instanceof Error ? error.message : "Unknown error";
        logger.error(
          `[useInfiniteChangeRequestComments] Error: ${errorMessage}`,
        );
        throw error;
      }
    },
    enabled: !!isSignedIn && !isAuthLoading && !!changeRequestId,
    initialPageParam: 0,
    getNextPageParam: (lastPage) => {
      const currentOffset = lastPage.offset ?? 0;
      const currentLimit = lastPage.limit ?? PAGE_SIZE;
      const totalRecords = lastPage.totalRecords ?? 0;
      const nextOffset = currentOffset + currentLimit;

      logger.debug(
        `[useInfiniteChangeRequestComments] getNextPageParam: currentOffset=${currentOffset}, limit=${currentLimit}, total=${totalRecords}, nextOffset=${nextOffset}`,
      );

      if (nextOffset >= totalRecords) {
        return undefined;
      }
      return nextOffset;
    },
    gcTime: 5 * 60 * 1000,
    staleTime: 30 * 1000,
  });
}
