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
import { useAuthApiClient } from "@/hooks/useAuthApiClient";
import { useLogger } from "@hooks/useLogger";
import { ApiQueryKeys } from "@constants/apiConstants";
import type { PatchChangeRequestRequest } from "@features/operations/types/changeRequests";
import type { PatchChangeRequestResponse } from "@features/operations/types/changeRequests";
import {
  ApiError,
  parseApiResponseErrorCode,
  parseApiResponseMessage,
} from "@utils/ApiError";

/**
 * Hook to patch a change request (PATCH /change-requests/:id): the customer's
 * answer (approve / reject, review successful / unsuccessful) or a proposed
 * implementation window.
 *
 * A failed request rejects with an {@link ApiError}, so callers can tell a
 * conflict (409) or refusal (403) from an invalid request (400). On success the
 * mutation settles only after the change request, the change request lists and
 * their stats have been refetched, so whoever awaits it sees the new state.
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
  const { isSignedIn } = useAsgardeo();
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
        // Not gated on the provider's `isLoading`: it flips on the SDK's own
        // background token handling (seen on the local mock identity provider,
        // where it was true on about one click in five), so a customer who is
        // plainly signed in had Approve / Propose refused. `authFetch` gets the
        // token itself and sends an expired session to sign-in.
        if (!isSignedIn) {
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
          // The refusal's machine-readable name rides with the message: the
          // caller classifies by it, never by the wording.
          throw new ApiError(
            response.status,
            response.statusText,
            parseApiResponseMessage(text, response.status, response.statusText),
            undefined,
            parseApiResponseErrorCode(text),
          );
        }

        const data: PatchChangeRequestResponse = await response.json();
        logger.debug("[usePatchChangeRequest] Change request updated:", data);
        return data;
      } catch (error) {
        logger.error("[usePatchChangeRequest] Error:", error);
        throw error;
      }
    },
    // Returned so the mutation stays pending until the refetch has landed: the
    // page then never shows the old state with live buttons, and a second click
    // cannot be sent against a change request that has already moved on.
    onSuccess: () =>
      Promise.all([
        queryClient.invalidateQueries({
          queryKey: [ApiQueryKeys.CHANGE_REQUEST_DETAILS, changeRequestId],
        }),
        queryClient.invalidateQueries({
          queryKey: [ApiQueryKeys.CHANGE_REQUESTS],
        }),
        queryClient.invalidateQueries({
          queryKey: [ApiQueryKeys.CHANGE_REQUEST_STATS],
        }),
      ]),
    // A 409 / 403 means the change request is no longer what the page showed:
    // refetch so the buttons follow the real state.
    onError: (error) => {
      if (
        error instanceof ApiError &&
        (error.status === 409 || error.status === 403)
      ) {
        void queryClient.invalidateQueries({
          queryKey: [ApiQueryKeys.CHANGE_REQUEST_DETAILS, changeRequestId],
        });
      }
    },
  });
}
