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
import { useAuthApiClient } from "@api/useAuthApiClient";
import { useLogger } from "@hooks/useLogger";
import { ApiQueryKeys } from "@constants/apiConstants";
import type { ChangeRequestDetails } from "@models/responses";

/**
 * Fetches a single change request by id.
 *
 * @param {string} changeRequestId - The change request id.
 * @returns {UseQueryResult<ChangeRequestDetails, Error>} Query result with change request details.
 */
export default function useGetChangeRequestDetails(
  changeRequestId: string,
): UseQueryResult<ChangeRequestDetails, Error> {
  const logger = useLogger();
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();

  return useQuery<ChangeRequestDetails, Error>({
    queryKey: [ApiQueryKeys.CHANGE_REQUEST_DETAILS, changeRequestId],
    queryFn: async (): Promise<ChangeRequestDetails> => {
      logger.debug(
        `Fetching change request details: changeRequestId=${changeRequestId}`,
      );

      try {
        const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;
        if (!baseUrl) {
          throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
        }

        const requestUrl = `${baseUrl}/change-requests/${encodeURIComponent(changeRequestId)}`;
        const response = await authFetch(requestUrl, {
          method: "GET",
        });

        if (!response.ok) {
          throw new Error(
            `Error fetching change request details: ${response.status} ${response.statusText}`,
          );
        }

        const data: ChangeRequestDetails = await response.json();
        logger.debug("[useGetChangeRequestDetails] Data received", {
          id: data.id,
          state: data.state?.label,
        });
        return data;
      } catch (error) {
        logger.error(
          "[useGetChangeRequestDetails] Error:",
          error instanceof Error ? error.message : String(error),
        );
        throw error;
      }
    },
    enabled: !!changeRequestId && isSignedIn && !isAuthLoading,
    staleTime: 5 * 60 * 1000,
  });
}
