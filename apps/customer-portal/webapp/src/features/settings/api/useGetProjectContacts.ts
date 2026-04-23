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
import type { ProjectContact } from "@features/settings/types/users";

/**
 * Hook to fetch project contacts (GET /projects/:projectId/contacts).
 *
 * @param {string} projectId - The ID of the project.
 * @returns {UseQueryResult<ProjectContact[], Error>} Query result.
 */
export default function useGetProjectContacts(
  projectId: string,
): UseQueryResult<ProjectContact[], Error> {
  const logger = useLogger();
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();

  return useQuery<ProjectContact[], Error>({
    queryKey: [ApiQueryKeys.PROJECT_CONTACTS, projectId],
    queryFn: async (): Promise<ProjectContact[]> => {
      logger.debug(
        `[useGetProjectContacts] Fetching contacts for project: ${projectId}`,
      );

      try {
        const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;
        if (!baseUrl) {
          throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
        }

        const requestUrl = `${baseUrl}/projects/${projectId}/contacts`;
        const response = await authFetch(requestUrl, {
          method: "GET",
        });

        logger.debug(
          `[useGetProjectContacts] Response status: ${response.status}`,
        );

        if (!response.ok) {
          const text = await response.text();
          throw new Error(
            `Error fetching project contacts: ${response.status} ${response.statusText}${text ? ` - ${text}` : ""}`,
          );
        }

        const data: ProjectContact[] = await response.json();
        return data;
      } catch (error) {
        logger.error("[useGetProjectContacts] Error:", error);
        throw error;
      }
    },
    enabled: !!projectId && !!isSignedIn && !isAuthLoading,
    staleTime: 0,
  });
}
