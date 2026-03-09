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
import { useAuthApiClient } from "@api/useAuthApiClient";
import { useLogger } from "@hooks/useLogger";
import type { CaseClassificationRequest } from "@models/requests";
import type { CaseClassificationResponse } from "@models/responses";

/**
 * Classifies case details based on chat history and environment context.
 *
 * @returns {UseMutationResult<CaseClassificationResponse, Error, CaseClassificationRequest>} Mutation result.
 */
export function usePostCaseClassifications(): UseMutationResult<
  CaseClassificationResponse,
  Error,
  CaseClassificationRequest
> {
  const logger = useLogger();
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();

  return useMutation<
    CaseClassificationResponse,
    Error,
    CaseClassificationRequest
  >({
    mutationFn: async (
      requestBody: CaseClassificationRequest,
    ): Promise<CaseClassificationResponse> => {
      const { chatHistory, ...restOfRequest } = requestBody;
      logger.debug(
        "[usePostCaseClassifications] Request payload (chatHistory redacted):",
        {
          ...restOfRequest,
          chatHistorySummary: {
            length: chatHistory.length,
            hasContent: chatHistory.trim().length > 0,
          },
        },
      );

      try {
        if (!isSignedIn || isAuthLoading) {
          throw new Error("User must be signed in to classify case details");
        }

        const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;
        if (!baseUrl) {
          throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
        }

        const requestUrl = `${baseUrl}/cases/classify`;

        const response = await authFetch(requestUrl, {
          method: "POST",

          body: JSON.stringify(requestBody),
        });

        logger.debug(
          `[usePostCaseClassifications] Response status: ${response.status}`,
        );

        if (!response.ok) {
          throw new Error(
            `Error classifying case details: ${response.status} ${response.statusText}`,
          );
        }

        const data: CaseClassificationResponse = await response.json();
        logger.debug("[usePostCaseClassifications] Data received:", data);
        return data;
      } catch (error) {
        logger.error("[usePostCaseClassifications] Error:", error);
        throw error;
      }
    },
  });
}
