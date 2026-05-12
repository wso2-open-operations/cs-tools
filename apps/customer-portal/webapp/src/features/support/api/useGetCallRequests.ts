// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License. You may obtain a copy of the License
// at
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
import { useAuthApiClient } from "@/hooks/useAuthApiClient";
import { ApiQueryKeys } from "@constants/apiConstants";
import { useLogger } from "@hooks/useLogger";
import type { CallRequestsResponse } from "@features/support/types/calls";
import { parseApiResponseMessage } from "@utils/ApiError";

/** Page size for POST .../call-requests/search (must match backend max). */
export const CALL_REQUESTS_PAGE_SIZE = 10;

const LIMIT = CALL_REQUESTS_PAGE_SIZE;

/**
 * Hook to fetch call requests for a specific case using infinite query.
 * Uses POST /cases/:caseId/call-requests/search with pagination (max limit 10).
 *
 * @param {string} projectId - The ID of the project (used for query key only).
 * @param {string} caseId - The ID of the case.
 * @param {number[]} [stateKeys] - When provided, filters results to only these call request state IDs.
 * @returns {UseInfiniteQueryResult<InfiniteData<CallRequestsResponse>, Error>} Infinite query result.
 */
export function useGetCallRequests(
  projectId: string,
  caseId: string,
  stateKeys?: number[],
): UseInfiniteQueryResult<InfiniteData<CallRequestsResponse>, Error> {
  const logger = useLogger();
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();

  return useInfiniteQuery<
    CallRequestsResponse,
    Error,
    InfiniteData<CallRequestsResponse>,
    readonly (string | number)[],
    number
  >({
    queryKey: [
      ApiQueryKeys.CASE_CALL_REQUESTS,
      projectId,
      caseId,
      JSON.stringify(stateKeys ?? []),
    ],
    queryFn: async ({ pageParam }): Promise<CallRequestsResponse> => {
      logger.debug(
        `[useGetCallRequests] Fetching call requests for case: ${caseId}, offset: ${pageParam}`,
      );

      try {
        if (!isSignedIn || isAuthLoading) {
          throw new Error("User must be signed in to fetch call requests");
        }

        const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;
        if (!baseUrl) {
          throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
        }

        const requestUrl = `${baseUrl}/cases/${caseId}/call-requests/search`;
        const bodyObj: Record<string, unknown> = {
          pagination: { limit: LIMIT, offset: pageParam },
        };
        if (stateKeys && stateKeys.length > 0) {
          bodyObj.filters = { stateKeys };
        }
        const body = JSON.stringify(bodyObj);

        const response = await authFetch(requestUrl, {
          method: "POST",

          body,
        });

        logger.debug(
          `[useGetCallRequests] Response status: ${response.status}`,
        );

        if (!response.ok) {
          const text = await response.text();
          throw new Error(parseApiResponseMessage(text, response.status, response.statusText));
        }

        const data: CallRequestsResponse = await response.json();
        return data;
      } catch (error) {
        logger.error("[useGetCallRequests] Error:", error);
        throw error;
      }
    },
    initialPageParam: 0,
    getNextPageParam: (lastPage) => {
      const { offset, limit, totalRecords } = lastPage;
      if (
        offset === undefined ||
        limit === undefined ||
        totalRecords === undefined
      ) {
        logger.warn(
          "[useGetCallRequests] Missing pagination metadata: offset, limit, or totalRecords undefined. Stopping pagination.",
          { offset, limit, totalRecords },
        );
        return undefined;
      }
      const nextOffset = offset + limit;
      return nextOffset < totalRecords ? nextOffset : undefined;
    },
    enabled: !!caseId && !isAuthLoading && isSignedIn,
    staleTime: 0,
  });
}
