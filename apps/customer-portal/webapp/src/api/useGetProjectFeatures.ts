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
import { ApiError } from "@utils/ApiError";
import { useLogger } from "@hooks/useLogger";
import { ApiQueryKeys } from "@constants/apiConstants";
import type { ProjectFeatures } from "@features/project-hub/types/projects";

/**
 * Fetches API-driven project feature permissions and accepted severities.
 *
 * @param {string} projectId - The project identifier.
 * @returns {UseQueryResult<ProjectFeatures, Error>} Feature response query result.
 */
export default function useGetProjectFeatures(
  projectId: string,
): UseQueryResult<ProjectFeatures, Error> {
  const logger = useLogger();
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();

  return useQuery<ProjectFeatures, Error>({
    queryKey: [ApiQueryKeys.PROJECT_FEATURES, projectId],
    queryFn: async (): Promise<ProjectFeatures> => {
      logger.debug(
        `[useGetProjectFeatures] Fetching features for project ID: ${projectId}`,
      );
      try {
        const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;
        if (!baseUrl) {
          throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
        }

        const response = await authFetch(
          `${baseUrl}/projects/${projectId}/features`,
          {
            method: "GET",
          },
        );

        logger.debug(
          `[useGetProjectFeatures] Response status for ${projectId}: ${response.status}`,
        );

        if (!response.ok) {
          let apiMessage: string | undefined;
          try {
            const errBody = await response.json();
            if (typeof errBody?.message === "string") {
              apiMessage = errBody.message;
            }
          } catch {
            // ignore - body may not be JSON
          }

          throw new ApiError(
            response.status,
            response.statusText,
            apiMessage ??
              `Error fetching project features: ${response.statusText}`,
          );
        }

        const data: ProjectFeatures = await response.json();

        logger.debug("[useGetProjectFeatures] Data received:", data);
        return data;
      } catch (error) {
        logger.error("[useGetProjectFeatures] Error:", error);
        throw error;
      }
    },
    enabled: !!projectId && isSignedIn && !isAuthLoading,
    staleTime: 5 * 60 * 1000,
  });
}
