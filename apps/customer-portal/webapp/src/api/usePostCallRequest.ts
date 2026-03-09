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

import {
  useMutation,
  useQueryClient,
  type UseMutationResult,
} from "@tanstack/react-query";
import { useAsgardeo } from "@asgardeo/react";
import { useAuthApiClient } from "@api/useAuthApiClient";
import { useLogger } from "@hooks/useLogger";
import { ApiQueryKeys } from "@constants/apiConstants";
import type { CreateCallRequest } from "@models/requests";
import type { CreateCallResponse } from "@models/responses";

/**
 * Hook to create a new call request for a case (POST /cases/:caseId/call-requests).
 *
 * @param {string} projectId - The ID of the project (used for cache invalidation).
 * @param {string} caseId - The ID of the case.
 * @returns {UseMutationResult<CreateCallResponse, Error, CreateCallRequest>} Mutation result.
 */
export function usePostCallRequest(
  projectId: string,
  caseId: string,
): UseMutationResult<CreateCallResponse, Error, CreateCallRequest> {
  const logger = useLogger();
  const queryClient = useQueryClient();
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();

  return useMutation<CreateCallResponse, Error, CreateCallRequest>({
    mutationFn: async (
      body: CreateCallRequest,
    ): Promise<CreateCallResponse> => {
      logger.debug("[usePostCallRequest] Request payload:", body);

      try {
        if (!isSignedIn || isAuthLoading) {
          throw new Error("User must be signed in to create a call request");
        }

        const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;
        if (!baseUrl) {
          throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
        }

        const requestUrl = `${baseUrl}/cases/${caseId}/call-requests`;

        const response = await authFetch(requestUrl, {
          method: "POST",

          body: JSON.stringify(body),
        });

        logger.debug(
          `[usePostCallRequest] Response status: ${response.status}`,
        );

        if (!response.ok) {
          const text = await response.text();
          let errorMessage = `Error creating call request: ${response.status} ${response.statusText}`;
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

        const data: CreateCallResponse = await response.json();
        logger.debug("[usePostCallRequest] Call request created:", data);
        return data;
      } catch (error) {
        logger.error("[usePostCallRequest] Error:", error);
        throw error;
      }
    },
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.CASE_CALL_REQUESTS, projectId, caseId],
      });
    },
  });
}
