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

import { useMutation, type UseMutationResult } from "@tanstack/react-query";
import { useAsgardeo } from "@asgardeo/react";
import { useAuthApiClient } from "@/hooks/useAuthApiClient";
import { useLogger } from "@hooks/useLogger";
import type { ValidateContactRequest } from "@features/settings/types/users";
import type { ValidateContactResponse } from "@features/settings/types/users";

/**
 * Hook to validate a project contact email (POST /projects/:projectId/contacts/validate).
 *
 * Returns the parsed response on 200 OK (may include contactDetails for existing deactivated contacts).
 * Throws on 409 Conflict (contact already exists as Invited/Registered).
 *
 * @param {string} projectId - The ID of the project.
 * @returns {UseMutationResult<ValidateContactResponse, Error, ValidateContactRequest>} Mutation result.
 */
export function useValidateProjectContact(
  projectId: string,
): UseMutationResult<ValidateContactResponse, Error, ValidateContactRequest> {
  const logger = useLogger();
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();

  return useMutation<ValidateContactResponse, Error, ValidateContactRequest>({
    mutationFn: async (body): Promise<ValidateContactResponse> => {
      logger.debug("[useValidateProjectContact] Request payload:", body);

      try {
        if (!isSignedIn || isAuthLoading) {
          throw new Error("User must be signed in to validate a contact");
        }

        const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;
        if (!baseUrl) {
          throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
        }

        const requestUrl = `${baseUrl}/projects/${projectId}/contacts/validate`;
        const response = await authFetch(requestUrl, {
          method: "POST",

          body: JSON.stringify(body),
        });

        logger.debug(
          `[useValidateProjectContact] Response status: ${response.status}`,
        );

        if (!response.ok) {
          const text = await response.text();
          let errorMessage = `Validation failed: ${response.status} ${response.statusText}`;
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

        const data = (await response.json()) as ValidateContactResponse;
        return data;
      } catch (error) {
        logger.error("[useValidateProjectContact] Error:", error);
        throw error;
      }
    },
  });
}
