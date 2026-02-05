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
import { useMockConfig } from "@/providers/MockConfigProvider";
import { useLogger } from "@/hooks/useLogger";
import { mockProjectDetails } from "@/models/mockData";
import { ApiQueryKeys, API_MOCK_DELAY } from "@/constants/apiConstants";
import type { ProjectDetails } from "@/models/responses";

/**
 * Custom hook to fetch detailed project information by ID.
 *
 * @param {string} projectId - The ID of the project to fetch details for.
 * @returns {UseQueryResult<ProjectDetails, Error>} The query result object.
 */
export default function useGetProjectDetails(
  projectId: string,
): UseQueryResult<ProjectDetails, Error> {
  const logger = useLogger();
  const { getIdToken, isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const { isMockEnabled } = useMockConfig();

  return useQuery<ProjectDetails, Error>({
    queryKey: [ApiQueryKeys.PROJECT_DETAILS, projectId, isMockEnabled],
    queryFn: async (): Promise<ProjectDetails> => {
      logger.debug(
        `Fetching project details for project ID: ${projectId}, mock: ${isMockEnabled}`,
      );

      if (isMockEnabled) {
        // Mock behavior: simulate network latency.
        await new Promise((resolve) => setTimeout(resolve, API_MOCK_DELAY));

        // Find project by ID from mock data.
        const project = mockProjectDetails.find((p) => p.id === projectId);

        if (!project) {
          logger.error(`Project not found for ID: ${projectId}`);
          throw new Error(`Project with ID ${projectId} not found`);
        }

        logger.debug(
          `Project details fetched successfully for project ID: ${projectId} (mock)`,
          project,
        );

        return project;
      }

      try {
        const idToken = await getIdToken();
        const baseUrl = import.meta.env.CUSTOMER_PORTAL_BACKEND_BASE_URL;

        if (!baseUrl) {
          throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
        }

        const requestUrl = `${baseUrl}/projects/${projectId}`;

        const response = await fetch(requestUrl, {
          method: "GET",
          headers: {
            "Content-Type": "application/json",
            Accept: "application/json",
            Authorization: `Bearer ${idToken}`,
            "x-user-id-token": idToken,
          },
        });

        logger.debug(
          `[useGetProjectDetails] Response status for ${projectId}: ${response.status}`,
        );

        if (!response.ok) {
          throw new Error(
            `Error fetching project details: ${response.statusText}`,
          );
        }

        const data: ProjectDetails = await response.json();
        logger.debug("[useGetProjectDetails] Data received:", data);
        return data;
      } catch (error) {
        logger.error("[useGetProjectDetails] Error:", error);
        throw error;
      }
    },
    enabled: !!projectId && (isMockEnabled || (isSignedIn && !isAuthLoading)),
    staleTime: 5 * 60 * 1000, // 5 minutes
  });
}
