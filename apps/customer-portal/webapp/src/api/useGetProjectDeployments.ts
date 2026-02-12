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
import { API_MOCK_DELAY } from "@constants/apiConstants";
import { addApiHeaders } from "@utils/apiUtils";
import { getMockProjectDeployments } from "@models/mockFunctions";
import type { ProjectDeploymentItem } from "@models/responses";

/**
 * Fetches project deployments (array with id, name, project, type.label).
 * Use type.label for case classification API environments.
 *
 * @param {string} projectId - The project ID.
 * @returns {UseQueryResult<ProjectDeploymentItem[], Error>} The query result.
 */
export function useGetProjectDeployments(
  projectId: string,
): UseQueryResult<ProjectDeploymentItem[], Error> {
  const logger = useLogger();
  const { getIdToken, isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const { isMockEnabled } = useMockConfig();

  return useQuery<ProjectDeploymentItem[], Error>({
    queryKey: ["project-deployments", projectId, isMockEnabled],
    queryFn: async (): Promise<ProjectDeploymentItem[]> => {
      logger.debug(
        `Fetching project deployments for project ID: ${projectId}, mock: ${isMockEnabled}`,
      );

      if (isMockEnabled) {
        await new Promise((resolve) => setTimeout(resolve, API_MOCK_DELAY));
        const data = getMockProjectDeployments(projectId);
        logger.debug(
          `Project deployments fetched for project ID: ${projectId} (mock)`,
          data,
        );
        return data;
      }

      try {
        const idToken = await getIdToken();
        const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;

        if (!baseUrl) {
          throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
        }

        const requestUrl = `${baseUrl}/projects/${projectId}/deployments`;

        const response = await fetch(requestUrl, {
          method: "GET",
          headers: addApiHeaders(idToken),
        });

        logger.debug(
          `[useGetProjectDeployments] Response status: ${response.status}`,
        );

        if (!response.ok) {
          throw new Error(
            `Error fetching project deployments: ${response.statusText}`,
          );
        }

        const data: ProjectDeploymentItem[] = await response.json();
        logger.debug("[useGetProjectDeployments] Data received:", data);
        return data;
      } catch (error) {
        logger.error("[useGetProjectDeployments] Error:", error);
        throw error;
      }
    },
    enabled: !!projectId && (isMockEnabled || (isSignedIn && !isAuthLoading)),
    staleTime: 5 * 60 * 1000,
  });
}
