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
import { mockCaseAttachments } from "@models/mockData";
import { ApiQueryKeys, API_MOCK_DELAY } from "@constants/apiConstants";
import { addApiHeaders } from "@utils/apiUtils";
import type { CaseAttachmentsResponse } from "@models/responses";

export interface UseGetCaseAttachmentsOptions {
  limit?: number;
  offset?: number;
  enabled?: boolean;
}

/**
 * Fetches attachments for a case.
 *
 * @param {string} caseId - The case id.
 * @param {UseGetCaseAttachmentsOptions} options - Optional limit and offset for pagination.
 * @returns {UseQueryResult<CaseAttachmentsResponse, Error>} Query result with attachments.
 */
export default function useGetCaseAttachments(
  caseId: string,
  options?: UseGetCaseAttachmentsOptions,
): UseQueryResult<CaseAttachmentsResponse, Error> {
  const logger = useLogger();
  const { getIdToken, isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const { isMockEnabled } = useMockConfig();
  const { limit = 50, offset = 0, enabled: optionsEnabled = true } = options ?? {};

  return useQuery<CaseAttachmentsResponse, Error>({
    queryKey: [
      ApiQueryKeys.CASE_ATTACHMENTS,
      caseId,
      limit,
      offset,
      isMockEnabled,
    ],
    queryFn: async (): Promise<CaseAttachmentsResponse> => {
      logger.debug(
        `Fetching case attachments: caseId=${caseId}, limit=${limit}, offset=${offset}, mock=${isMockEnabled}`,
      );

      if (isMockEnabled) {
        await new Promise((resolve) => setTimeout(resolve, API_MOCK_DELAY));
        const sliced = mockCaseAttachments.slice(offset, offset + limit);
        return {
          attachments: sliced.length > 0 ? sliced : mockCaseAttachments,
          totalRecords: mockCaseAttachments.length,
          limit,
          offset,
        };
      }

      try {
        const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;
        if (!baseUrl) {
          throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
        }

        const idToken = await getIdToken();
        const params = new URLSearchParams();
        params.set("limit", String(limit));
        params.set("offset", String(offset));
        const requestUrl = `${baseUrl}/cases/${caseId}/attachments?${params.toString()}`;
        const response = await fetch(requestUrl, {
          method: "GET",
          headers: addApiHeaders(idToken),
        });

        if (!response.ok) {
          throw new Error(
            `Error fetching case attachments: ${response.status} ${response.statusText}`,
          );
        }

        const data: CaseAttachmentsResponse = await response.json();
        logger.debug("[useGetCaseAttachments] Data received:", data);
        return data;
      } catch (error) {
        logger.error("[useGetCaseAttachments] Error:", error);
        throw error;
      }
    },
    enabled:
      optionsEnabled &&
      !!caseId &&
      (isMockEnabled || (isSignedIn && !isAuthLoading)),
    staleTime: 5 * 60 * 1000, // 5 minutes
  });
}
