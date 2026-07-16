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
import type { CaseFeedback } from "@features/support/types/feedback";

/**
 * Fetches previously submitted feedback for a case (GET /cases/{caseId}/feedback).
 * Returns null when no feedback has been submitted yet (backend responds 404).
 *
 * @param {string} caseId - The case ID.
 * @param {boolean} [enabled] - Whether the query should run (e.g. only for closed cases).
 * @returns {UseQueryResult<CaseFeedback | null, Error>} The query result object.
 */
export function useGetCaseFeedback(
  caseId: string,
  enabled = true,
): UseQueryResult<CaseFeedback | null, Error> {
  const logger = useLogger();
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();

  return useQuery<CaseFeedback | null, Error>({
    queryKey: [ApiQueryKeys.CASE_FEEDBACK, caseId],
    queryFn: async (): Promise<CaseFeedback | null> => {
      logger.debug(`[useGetCaseFeedback] Fetching feedback for case ${caseId}`);

      const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;
      if (!baseUrl) {
        throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
      }

      const response = await authFetch(`${baseUrl}/cases/${caseId}/feedback`, {
        method: "GET",
      });

      if (response.status === 404) {
        return null;
      }

      if (!response.ok) {
        throw new Error(`Error fetching case feedback: ${response.statusText}`);
      }

      return response.json() as Promise<CaseFeedback>;
    },
    enabled: enabled && !!caseId && isSignedIn && !isAuthLoading,
    staleTime: 0,
  });
}
