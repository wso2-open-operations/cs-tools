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
import { getUserFacingErrorMessage } from "@utils/errorMessages";
import { ApiQueryKeys } from "@constants/apiConstants";
import type {
  ProjectDeploymentItem,
  ProjectDeploymentsListResponse,
} from "@models/responses";

/**
 * Fetches deployments for a project.
 *
 * @param {string} projectId - The project ID.
 * @returns {UseQueryResult<ProjectDeploymentsListResponse, Error>} The query result.
 */
export function useGetDeployments(
  projectId: string,
): UseQueryResult<ProjectDeploymentsListResponse, Error> {
  const logger = useLogger();
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();

  return useQuery<ProjectDeploymentsListResponse, Error>({
    queryKey: [ApiQueryKeys.DEPLOYMENTS, projectId],
    queryFn: async (): Promise<ProjectDeploymentsListResponse> => {
      logger.debug(`Fetching deployments for project ID: ${projectId}`);

      try {
        const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;

        if (!baseUrl) {
          throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
        }

        const requestUrl = `${baseUrl}/projects/${projectId}/deployments`;

        const response = await authFetch(requestUrl, {
          method: "GET",
        });

        logger.debug(`[useGetDeployments] Response status: ${response.status}`);

        if (!response.ok) {
          throw new Error(`Error fetching deployments: ${response.statusText}`);
        }

        const raw = await response.json();
        const data: ProjectDeploymentsListResponse = Array.isArray(raw)
          ? { deployments: raw as ProjectDeploymentItem[] }
          : (raw as ProjectDeploymentsListResponse);
        logger.debug("[useGetDeployments] Data received:", data);
        return data;
      } catch (error) {
        logger.error("[useGetDeployments] Error:", error);
        throw new Error(
          getUserFacingErrorMessage(error, "Failed to load deployments."),
        );
      }
    },
    enabled: !!projectId && isSignedIn && !isAuthLoading,
    staleTime: 5 * 60 * 1000,
  });
}
