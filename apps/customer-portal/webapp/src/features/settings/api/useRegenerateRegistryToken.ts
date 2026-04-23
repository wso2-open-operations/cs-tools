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
import type { RegistryTokenCreationResponse } from "@features/settings/types/registryTokens";

/**
 * Hook to regenerate a registry token secret (POST /registry-tokens/:id/regenerate).
 *
 * @param {string} projectId - The project ID (used to invalidate the tokens search cache).
 * @returns {UseMutationResult<RegistryTokenCreationResponse, Error, number>} Mutation result (pass token id to mutate).
 */
export function useRegenerateRegistryToken(
  projectId: string,
): UseMutationResult<RegistryTokenCreationResponse, Error, number> {
  const logger = useLogger();
  const queryClient = useQueryClient();
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();

  return useMutation<RegistryTokenCreationResponse, Error, number>({
    mutationFn: async (tokenId): Promise<RegistryTokenCreationResponse> => {
      logger.debug("[useRegenerateRegistryToken] Regenerating token:", tokenId);

      try {
        if (!isSignedIn || isAuthLoading) {
          throw new Error(
            "User must be signed in to regenerate a registry token",
          );
        }

        const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;
        if (!baseUrl) {
          throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
        }

        const requestUrl = `${baseUrl}/registry-tokens/${tokenId}/regenerate`;
        const response = await authFetch(requestUrl, {
          method: "POST",
        });

        logger.debug(
          `[useRegenerateRegistryToken] Response status: ${response.status}`,
        );

        if (!response.ok) {
          const text = await response.text();
          let errorMessage = `Error regenerating registry token: ${response.status} ${response.statusText}`;
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

        const data: RegistryTokenCreationResponse = await response.json();
        return data;
      } catch (error) {
        logger.error("[useRegenerateRegistryToken] Error:", error);
        throw error;
      }
    },
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.REGISTRY_TOKENS_SEARCH, projectId],
      });
    },
  });
}
