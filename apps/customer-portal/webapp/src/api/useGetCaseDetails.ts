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
import { mockCaseDetails } from "@models/mockData";
import { ApiQueryKeys, API_MOCK_DELAY } from "@constants/apiConstants";
import { addApiHeaders } from "@utils/apiUtils";
import type { CaseDetails } from "@models/responses";

/**
 * Fetches a single case by id for a project.
 *
 * @param {string} projectId - The project id.
 * @param {string} caseId - The case id.
 * @returns {UseQueryResult<CaseDetails, Error>} Query result with case details.
 */
export default function useGetCaseDetails(
  projectId: string,
  caseId: string,
): UseQueryResult<CaseDetails, Error> {
  const logger = useLogger();
  const { getIdToken, isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const { isMockEnabled } = useMockConfig();

  return useQuery<CaseDetails, Error>({
    queryKey: [ApiQueryKeys.CASE_DETAILS, projectId, caseId, isMockEnabled],
    queryFn: async (): Promise<CaseDetails> => {
      logger.debug(
        `Fetching case details: projectId=${projectId}, caseId=${caseId}, mock=${isMockEnabled}`,
      );

      if (isMockEnabled) {
        await new Promise((resolve) => setTimeout(resolve, API_MOCK_DELAY));
        return { ...mockCaseDetails, id: caseId };
      }

      try {
        const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;
        if (!baseUrl) {
          throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
        }

        const idToken = await getIdToken();
        const requestUrl = `${baseUrl}/projects/${projectId}/cases/${caseId}`;
        const response = await fetch(requestUrl, {
          method: "GET",
          headers: addApiHeaders(idToken),
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
    enabled:
      !!projectId &&
      !!caseId &&
      (isMockEnabled || (isSignedIn && !isAuthLoading)),
    staleTime: 5 * 60 * 1000,
  });
}
