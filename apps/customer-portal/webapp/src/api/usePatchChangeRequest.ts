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
import type { PatchChangeRequestRequest } from "@/types/changeRequests";
import type { PatchChangeRequestResponse } from "@/types/changeRequests";

/**
 * Hook to update change request planned start (PATCH /change-requests/:id).
 *
 * @param {string} changeRequestId - The change request id.
 * @returns {UseMutationResult<PatchChangeRequestResponse, Error, PatchChangeRequestRequest>} Mutation result.
 */
export function usePatchChangeRequest(
  changeRequestId: string,
): UseMutationResult<
  PatchChangeRequestResponse,
  Error,
  PatchChangeRequestRequest
> {
  const logger = useLogger();
  const queryClient = useQueryClient();
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();

  return useMutation<
    PatchChangeRequestResponse,
    Error,
    PatchChangeRequestRequest
  >({
    mutationFn: async (
      payload: PatchChangeRequestRequest,
    ): Promise<PatchChangeRequestResponse> => {
      logger.debug("[usePatchChangeRequest] Request payload:", payload);

      try {
        if (!isSignedIn || isAuthLoading) {
          throw new Error("User must be signed in to update change request");
        }

        const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;
        if (!baseUrl) {
          throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
        }

        const requestUrl = `${baseUrl}/change-requests/${encodeURIComponent(changeRequestId)}`;

        const response = await authFetch(requestUrl, {
          method: "PATCH",
          body: JSON.stringify(payload),
        });

        logger.debug(
          `[usePatchChangeRequest] Response status: ${response.status}`,
        );

        if (!response.ok) {
          const text = await response.text();
          let errorMessage = `Error updating change request: ${response.status} ${response.statusText}`;
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

        const data: PatchChangeRequestResponse = await response.json();
        logger.debug("[usePatchChangeRequest] Change request updated:", data);
        return data;
      } catch (error) {
        logger.error("[usePatchChangeRequest] Error:", error);
        throw error;
      }
    },
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.CHANGE_REQUEST_DETAILS, changeRequestId],
      });
    },
  });
}
