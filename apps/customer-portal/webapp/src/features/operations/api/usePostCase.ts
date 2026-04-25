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
import { useAuthApiClient } from "@/hooks/useAuthApiClient";
import { useLogger } from "@hooks/useLogger";
import type { CreateCaseRequest } from "@features/support/types/cases";
import type { CreateServiceRequestPayload } from "@features/operations/types/serviceRequests";
import type { CreateCaseResponse } from "@features/support/types/cases";
import { parseApiResponseMessage } from "@utils/ApiError";

/**
 * Posts a new support case or service request to the backend.
 *
 * @returns {UseMutationResult<CreateCaseResponse, Error, CreateCaseRequest | CreateServiceRequestPayload>} Mutation result.
 */
export function usePostCase(): UseMutationResult<
  CreateCaseResponse,
  Error,
  CreateCaseRequest | CreateServiceRequestPayload
> {
  const logger = useLogger();
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();

  return useMutation<
    CreateCaseResponse,
    Error,
    CreateCaseRequest | CreateServiceRequestPayload
  >({
    mutationFn: async (
      body: CreateCaseRequest | CreateServiceRequestPayload,
    ): Promise<CreateCaseResponse> => {
      logger.debug("[usePostCase] Request payload summary:", {
        requestType: "type" in body ? body.type : "case",
        projectId: body.projectId,
        deploymentId: body.deploymentId,
        deployedProductId: body.deployedProductId,
        descriptionPreview:
          "description" in body && body.description
            ? `${body.description.slice(0, 80)}...`
            : undefined,
        variableCount: "variables" in body ? body.variables.length : undefined,
        attachmentCount: body.attachments?.length ?? 0,
      });

      try {
        if (!isSignedIn || isAuthLoading) {
          throw new Error("User must be signed in to create a case");
        }

        const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;
        if (!baseUrl) {
          throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
        }

        const requestUrl = `${baseUrl}/cases`;

        const response = await authFetch(requestUrl, {
          method: "POST",

          body: JSON.stringify(body),
        });

        logger.debug(`[usePostCase] Response status: ${response.status}`);

        if (!response.ok) {
          const text = await response.text();
          throw new Error(parseApiResponseMessage(text, response.status, response.statusText));
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
