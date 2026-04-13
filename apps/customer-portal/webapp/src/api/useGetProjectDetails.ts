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
import { ApiError } from "@api/ApiError";
import { useLogger } from "@hooks/useLogger";
import { ApiQueryKeys } from "@constants/apiConstants";
import type { ProjectDetails } from "@/types/projects";

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
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();

  return useQuery<ProjectDetails, Error>({
    queryKey: [ApiQueryKeys.PROJECT_DETAILS, projectId],
    queryFn: async (): Promise<ProjectDetails> => {
      logger.debug(`Fetching project details for project ID: ${projectId}`);

      try {
        const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;

        if (!baseUrl) {
          throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
        }

        const requestUrl = `${baseUrl}/projects/${projectId}`;

        const response = await authFetch(requestUrl, {
          method: "GET",
        });

        logger.debug(
          `[useGetProjectDetails] Response status for ${projectId}: ${response.status}`,
        );

        if (!response.ok) {
          let apiMessage: string | undefined;
          try {
            const errBody = await response.json();
            if (typeof errBody?.message === "string") {
              apiMessage = errBody.message;
            }
          } catch {
            // ignore – body may not be JSON
          }
          throw new ApiError(
            response.status,
            response.statusText,
            apiMessage ?? `Error fetching project details: ${response.statusText}`,
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
    enabled: !!projectId && isSignedIn && !isAuthLoading,
    staleTime: 5 * 60 * 1000, // 5 minutes
  });
}
