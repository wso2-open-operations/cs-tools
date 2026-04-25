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

import {
  useMutation,
  useQueryClient,
  type UseMutationResult,
} from "@tanstack/react-query";
import { useAsgardeo } from "@asgardeo/react";
import { useAuthApiClient } from "@/hooks/useAuthApiClient";
import { useLogger } from "@hooks/useLogger";
import { ApiQueryKeys } from "@constants/apiConstants";
import { parseApiResponseMessage } from "@utils/ApiError";

/**
 * Hook to delete a project contact (DELETE /projects/:projectId/contacts/:email).
 *
 * @param {string} projectId - The ID of the project.
 * @returns {UseMutationResult<void, Error, string>} Mutation result (pass contact email to mutate).
 */
export function useDeleteProjectContact(
  projectId: string,
): UseMutationResult<void, Error, string> {
  const logger = useLogger();
  const queryClient = useQueryClient();
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();

  return useMutation<void, Error, string>({
    mutationFn: async (contactEmail): Promise<void> => {
      logger.debug("[useDeleteProjectContact] Deleting contact:", contactEmail);

      try {
        if (!isSignedIn || isAuthLoading) {
          throw new Error("User must be signed in to remove a project contact");
        }

        const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;
        if (!baseUrl) {
          throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
        }

        const encodedEmail = encodeURIComponent(contactEmail);
        const requestUrl = `${baseUrl}/projects/${projectId}/contacts/${encodedEmail}`;
        const response = await authFetch(requestUrl, {
          method: "DELETE",
        });

        logger.debug(
          `[useDeleteProjectContact] Response status: ${response.status}`,
        );

        if (!response.ok) {
          const text = await response.text();
          throw new Error(parseApiResponseMessage(text, response.status, response.statusText));
        }
      } catch (error) {
        logger.error("[useDeleteProjectContact] Error:", error);
        throw error;
      }
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.PROJECT_CONTACTS, projectId],
      });
    },
  });
}
