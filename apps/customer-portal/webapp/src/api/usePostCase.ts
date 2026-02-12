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

import { useMutation, type UseMutationResult } from "@tanstack/react-query";
import { useAsgardeo } from "@asgardeo/react";
import { useMockConfig } from "@providers/MockConfigProvider";
import { useLogger } from "@hooks/useLogger";
import { addApiHeaders } from "@utils/apiUtils";
import type { CreateCaseRequest } from "@models/requests";
import type { CreateCaseResponse } from "@models/responses";

/**
 * Posts a new support case to the backend. When mock is enabled, the mutation
 * throws without calling the API; the create-case page should disable the
 * submit button when mock is enabled.
 *
 * @returns {UseMutationResult<CreateCaseResponse, Error, CreateCaseRequest>} Mutation result.
 */
export function usePostCase(): UseMutationResult<
  CreateCaseResponse,
  Error,
  CreateCaseRequest
> {
  const logger = useLogger();
  const { getIdToken, isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const { isMockEnabled } = useMockConfig();

  return useMutation<CreateCaseResponse, Error, CreateCaseRequest>({
    mutationFn: async (
      body: CreateCaseRequest,
    ): Promise<CreateCaseResponse> => {
      logger.debug("[usePostCase] Request payload:", {
        ...body,
        description: body.description
          ? `${body.description.slice(0, 80)}...`
          : "",
      });

      if (isMockEnabled) {
        throw new Error(
          "Create case is not available when mock is enabled. Disable mock to create a case.",
        );
      }

      try {
        if (!isSignedIn || isAuthLoading) {
          throw new Error("User must be signed in to create a case");
        }

        const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;
        if (!baseUrl) {
          throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
        }

        const idToken = await getIdToken();
        const requestUrl = `${baseUrl}/cases`;

        const response = await fetch(requestUrl, {
          method: "POST",
          headers: addApiHeaders(idToken),
          body: JSON.stringify(body),
        });

        logger.debug(`[usePostCase] Response status: ${response.status}`);

        if (!response.ok) {
          const text = await response.text();
          throw new Error(
            `Error creating case: ${response.status} ${response.statusText}${text ? ` - ${text}` : ""}`,
          );
        }

        const data: CreateCaseResponse = await response.json();
        logger.debug("[usePostCase] Case created:", data);
        return data;
      } catch (error) {
        logger.error("[usePostCase] Error:", error);
        throw error;
      }
    },
  });
}
