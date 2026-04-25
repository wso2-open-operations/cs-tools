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

import {
  useMutation,
  useQueryClient,
  type UseMutationResult,
} from "@tanstack/react-query";
import { useAsgardeo } from "@asgardeo/react";
import { useAuthApiClient } from "@/hooks/useAuthApiClient";
import { useLogger } from "@hooks/useLogger";
import { ApiQueryKeys } from "@constants/apiConstants";

export interface PatchProjectContactVariables {
  email: string;
  isCsAdmin: boolean;
  isPortalUser: boolean;
  isSecurityContact: boolean;
}

/**
 * Hook to update a project contact's membership roles
 * (PATCH /projects/:projectId/contacts/:email).
 *
 * @param {string} projectId - The ID of the project.
 * @returns {UseMutationResult<void, Error, PatchProjectContactVariables>} Mutation result.
 */
export function usePatchProjectContact(
  projectId: string,
): UseMutationResult<void, Error, PatchProjectContactVariables> {
  const logger = useLogger();
  const queryClient = useQueryClient();
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();

  return useMutation<void, Error, PatchProjectContactVariables>({
    mutationFn: async ({ email, isCsAdmin, isPortalUser, isSecurityContact }): Promise<void> => {
      logger.debug(
        `[usePatchProjectContact] Patching ${email} isCsAdmin=${isCsAdmin} isPortalUser=${isPortalUser} isSecurityContact=${isSecurityContact}`,
      );

      try {
        if (!isSignedIn || isAuthLoading) {
          throw new Error("User must be signed in to update a project contact");
        }

        const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;
        if (!baseUrl) {
          throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
        }

        const requestUrl = `${baseUrl}/projects/${projectId}/contacts/${encodeURIComponent(email)}`;
        const response = await authFetch(requestUrl, {
          method: "PATCH",
          body: JSON.stringify({ isCsAdmin, isPortalUser, isSecurityContact }),
        });

        if (!response.ok) {
          const text = await response.text();
          let errorMessage = `Error updating project contact: ${response.status} ${response.statusText}`;
          try {
            const json = JSON.parse(text) as { message?: string };
            if (typeof json.message === "string") {
              errorMessage = json.message;
            } else if (text) {
              errorMessage += ` - ${text}`;
            }
          } catch {
            if (text) errorMessage += ` - ${text}`;
          }
          throw new Error(errorMessage);
        }
      } catch (error) {
        logger.error("[usePatchProjectContact] Error:", error);
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
