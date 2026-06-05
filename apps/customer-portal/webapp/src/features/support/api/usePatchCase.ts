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
import type { PatchCaseRequest } from "@features/support/types/cases";
import type { PatchCaseResponse } from "@features/support/types/supportApi";
import { parseApiResponseMessage } from "@utils/ApiError";

export type { PatchCaseResponse };

/**
 * Hook to update a case state (PATCH /cases/:caseId).
 *
 * @param {string} projectId - Project ID for cache invalidation.
 * @param {string} caseId - Case ID.
 * @returns {UseMutationResult<PatchCaseResponse, Error, PatchCaseRequest>} Mutation result.
 */
export function usePatchCase(
  projectId: string,
  caseId: string,
): UseMutationResult<PatchCaseResponse, Error, PatchCaseRequest> {
  const logger = useLogger();
  const queryClient = useQueryClient();
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();

  return useMutation<PatchCaseResponse, Error, PatchCaseRequest>({
    mutationFn: async (body: PatchCaseRequest): Promise<PatchCaseResponse> => {
      logger.debug("[usePatchCase] Request:", { caseId, body });

      if (!caseId || !projectId) {
        throw new Error("Case ID and project ID are required to update a case");
      }
      if (!isSignedIn || isAuthLoading) {
        throw new Error("User must be signed in to update a case");
      }

      const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;
      if (!baseUrl) {
        throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
      }

      const response = await authFetch(`${baseUrl}/cases/${caseId}`, {
        method: "PATCH",

        body: JSON.stringify(body),
      });

      if (!response.ok) {
        const text = await response.text();
        throw new Error(parseApiResponseMessage(text, response.status, response.statusText));
      }

      const data: PatchCaseResponse = await response.json();
      logger.debug("[usePatchCase] Case updated:", data);
      return data;
    },
    onSuccess: (_data, variables) => {
      const matchesProjectCases = (queryKey: readonly unknown[]): boolean => {
        if (queryKey[0] !== ApiQueryKeys.PROJECT_CASES) return false;
        if (queryKey[1] === "page") return queryKey[2] === projectId;
        return queryKey[1] === projectId;
      };

      const isCaseDetailsQuery = (queryKey: readonly unknown[]) =>
        queryKey[0] === ApiQueryKeys.CASE_DETAILS && queryKey[2] === caseId;

      queryClient.invalidateQueries({ predicate: (q) => isCaseDetailsQuery(q.queryKey) });

      if (variables.watchList !== undefined) {
        void queryClient.refetchQueries({
          predicate: (q) => isCaseDetailsQuery(q.queryKey),
          type: "active",
        });
      }
      queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.CASES_STATS, projectId],
      });
      queryClient.invalidateQueries({
        predicate: (query) => matchesProjectCases(query.queryKey),
      });
      void queryClient.refetchQueries({
        queryKey: [ApiQueryKeys.CASES_STATS, projectId],
        type: "active",
      });
      void queryClient.refetchQueries({
        predicate: (query) => matchesProjectCases(query.queryKey),
        type: "active",
      });
    },
  });
}
