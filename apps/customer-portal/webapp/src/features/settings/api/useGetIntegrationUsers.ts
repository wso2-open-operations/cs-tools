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
// software distributed under the License is distributed on
// an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

import { useQuery, type UseQueryResult } from "@tanstack/react-query";
import { useAsgardeo } from "@asgardeo/react";
import { useAuthApiClient } from "@/hooks/useAuthApiClient";
import { useLogger } from "@hooks/useLogger";
import { ApiQueryKeys } from "@constants/apiConstants";
import type { IntegrationUser } from "@features/settings/types/users";

/**
 * Hook to fetch integration users for a project (GET /projects/:projectId/integration-users).
 *
 * @param {string} projectId - The ID of the project.
 * @param {boolean} [enabled=true] - Whether the query should run.
 * @returns {UseQueryResult<IntegrationUser[], Error>} Query result.
 */
export function useGetIntegrationUsers(
  projectId: string,
  enabled = true,
): UseQueryResult<IntegrationUser[], Error> {
  const logger = useLogger();
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();

  return useQuery<IntegrationUser[], Error>({
    queryKey: [ApiQueryKeys.INTEGRATION_USERS, projectId],
    queryFn: async (): Promise<IntegrationUser[]> => {
      logger.debug(
        `[useGetIntegrationUsers] Fetching integration users for project: ${projectId}`,
      );

      try {
        const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;
        if (!baseUrl) {
          throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
        }

        const requestUrl = `${baseUrl}/projects/${projectId}/integration-users`;
        const response = await authFetch(requestUrl, {
          method: "GET",
        });

        logger.debug(
          `[useGetIntegrationUsers] Response status: ${response.status}`,
        );

        if (!response.ok) {
          const text = await response.text();
          throw new Error(
            `Error fetching integration users: ${response.status} ${response.statusText}${text ? ` - ${text}` : ""}`,
          );
        }

        const data: IntegrationUser[] = await response.json();
        return data;
      } catch (error) {
        logger.error("[useGetIntegrationUsers] Error:", error);
        throw error;
      }
    },
    enabled: !!projectId && !!isSignedIn && !isAuthLoading && enabled,
    staleTime: 0,
  });
}
