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
import { useAuthApiClient } from "@api/useAuthApiClient";
import { useLogger } from "@hooks/useLogger";
import { ApiQueryKeys } from "@constants/apiConstants";
import type {
  CaseAttachmentsResponse,
  CaseAttachment,
} from "@models/responses";

const PAGE_SIZE = 10;

/**
 * Fetches case attachments with pagination (GET /cases/:caseId/attachments).
 * Uses infinite query to support server-side pagination.
 *
 * @param {string} caseId - The case ID.
 * @returns {UseInfiniteQueryResult} Infinite query result with case attachments.
 */
export function useGetCaseAttachments(caseId: string) {
  const logger = useLogger();
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();

  return useInfiniteQuery<
    CaseAttachmentsResponse,
    Error,
    InfiniteData<CaseAttachmentsResponse>,
    readonly (string | number)[],
    number
  >({
    queryKey: [ApiQueryKeys.CASE_ATTACHMENTS, caseId, "infinite"],
    queryFn: async ({
      pageParam,
      signal,
    }): Promise<CaseAttachmentsResponse> => {
      logger.debug(
        `[useGetCaseAttachments] Fetching attachments for case ${caseId}, offset: ${pageParam}`,
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
        const encodedCaseId = encodeURIComponent(caseId);
        const requestUrl = `${baseUrl}/cases/${encodedCaseId}/attachments?${params}`;
        const response = await authFetch(requestUrl, {
          method: "GET",
          signal,
        });

        logger.debug(
          `[useGetCaseAttachments] Response status: ${response.status}`,
        );

        if (!response.ok) {
          throw new Error(
            `Error fetching case attachments: ${response.statusText}`,
          );
        }

        const data: CaseAttachmentsResponse = await response.json();
        logger.debug(
          `[useGetCaseAttachments] Page received: ${data.attachments?.length ?? 0} attachments, offset ${data.offset}, total ${data.totalRecords}`,
        );
        return data;
      } catch (error) {
        const errorMessage =
          error instanceof Error ? error.message : "Unknown error";
        logger.error(`[useGetCaseAttachments] Error: ${errorMessage}`);
        throw error;
      }
    },
    initialPageParam: 0,
    getNextPageParam: (lastPage) => {
      const { offset, limit, totalRecords } = lastPage;
      if (limit <= 0) return undefined;
      const nextOffset = offset + limit;
      return nextOffset < totalRecords ? nextOffset : undefined;
    },
    enabled: !!caseId && isSignedIn && !isAuthLoading,
    staleTime: 5 * 60 * 1000,
  });
}

/** Flattens all pages of case attachments into a single array. */
export function flattenCaseAttachments(
  data: InfiniteData<CaseAttachmentsResponse> | undefined,
): CaseAttachment[] {
  if (!data?.pages) return [];
  return data.pages.flatMap(
    (page: CaseAttachmentsResponse) => page.attachments ?? [],
  );
}
