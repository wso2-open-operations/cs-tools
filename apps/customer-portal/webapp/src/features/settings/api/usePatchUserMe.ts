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
import type { PatchUserMeRequest } from "@features/settings/types/users";
import type { UserDetails } from "@features/settings/types/users";
import { parseApiResponseMessage } from "@utils/ApiError";

export interface PatchUserMeResponse {
  phoneNumber?: string;
  timeZone?: string;
}

/**
 * Hook to update current user profile (PATCH /users/me).
 * Only pass changed fields in the request body.
 *
 * @returns {UseMutationResult<PatchUserMeResponse, Error, PatchUserMeRequest>} Mutation result.
 */
export function usePatchUserMe(): UseMutationResult<
  PatchUserMeResponse,
  Error,
  PatchUserMeRequest
> {
  const logger = useLogger();
  const queryClient = useQueryClient();
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();

  return useMutation<PatchUserMeResponse, Error, PatchUserMeRequest>({
    mutationFn: async (
      payload: PatchUserMeRequest,
    ): Promise<PatchUserMeResponse> => {
      logger.debug("[usePatchUserMe] Request payload:", payload);

      if (!isSignedIn || isAuthLoading) {
        throw new Error("User must be signed in to update profile");
      }

      const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;
      if (!baseUrl) {
        throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
      }

      const requestUrl = `${baseUrl}/users/me`;
      const response = await authFetch(requestUrl, {
        method: "PATCH",
        body: JSON.stringify(payload),
      });

      if (!response.ok) {
        const text = await response.text();
        throw new Error(parseApiResponseMessage(text, response.status, response.statusText));
      }
      return (await response.json()) as PatchUserMeResponse;
    },
    onSuccess: (data, variables) => {
      queryClient.setQueryData<UserDetails>(["userDetails"], (old) => {
        if (!old) {
          return old;
        }
        return {
          ...old,
          ...(variables.timeZone !== undefined
            ? {
                timeZone:
                  data.timeZone !== undefined
                    ? data.timeZone
                    : variables.timeZone,
              }
            : {}),
          ...(variables.phoneNumber !== undefined
            ? {
                phoneNumber:
                  data.phoneNumber !== undefined
                    ? data.phoneNumber
                    : variables.phoneNumber,
              }
            : {}),
        };
      });
      void queryClient.invalidateQueries({ queryKey: ["userDetails"] });
      if (variables.timeZone !== undefined) {
        void queryClient.invalidateQueries({
          predicate: (query) => query.queryKey[0] !== "userDetails",
          refetchType: "active",
        });
      }
    },
  });
}
