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
import type { CaseDetails } from "@models/responses";

/**
 * Fetches a single case by id. When projectId is provided, it is included in the query key.
 * When projectId is omitted (e.g. for old-URL redirect by caseId only), the API still
 * returns case details including project.id for redirect.
 *
 * @param {string | undefined} projectId - The project id (optional when fetching by caseId only).
 * @param {string} caseId - The case id.
 * @returns {UseQueryResult<CaseDetails, Error>} Query result with case details.
 */
export default function useGetCaseDetails(
  projectId: string | undefined,
  caseId: string,
): UseQueryResult<CaseDetails, Error> {
  const logger = useLogger();
  const { isLoading: isAuthLoading, isSignedIn } = useAsgardeo();
  const authFetch = useAuthApiClient();

  return useQuery<CaseDetails, Error>({
    queryKey: [ApiQueryKeys.CASE_DETAILS, projectId ?? "byCaseId", caseId],
    queryFn: async (): Promise<CaseDetails> => {
      logger.debug(
        `Fetching case details: projectId=${projectId}, caseId=${caseId}`,
      );

      try {
        const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;
        if (!baseUrl) {
          throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
        }

        const requestUrl = `${baseUrl}/cases/${caseId}`;
        const response = await authFetch(requestUrl, {
          method: "GET",
        });

        if (!response.ok) {
          throw new Error(
            `Error fetching case details: ${response.status} ${response.statusText}`,
          );
        }

        const data: CaseDetails = await response.json();
        logger.debug("[useGetCaseDetails] Data received:", data);
        return data;
      } catch (error) {
        logger.error("[useGetCaseDetails] Error:", error);
        throw error;
      }
    },
    enabled: !!caseId && !isAuthLoading && isSignedIn,
    staleTime: 5 * 60 * 1000,
  });
}
