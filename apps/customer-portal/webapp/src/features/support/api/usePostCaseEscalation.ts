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

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useAuthApiClient } from "@/hooks/useAuthApiClient";
import { ApiQueryKeys } from "@constants/apiConstants";
import type { CreateEscalationResponse } from "@features/support/types/cases";

export type EscalationApiError = Error & { status: number };

/**
 * Creates a new escalation for a case.
 *
 * @param {string} caseId - The case ID to escalate.
 * @returns Mutation result for POST /cases/{caseId}/escalations.
 */
export function usePostCaseEscalation(caseId: string) {
  const authFetch = useAuthApiClient();
  const queryClient = useQueryClient();

  return useMutation<CreateEscalationResponse, EscalationApiError, { reason: string }>({
    mutationFn: async ({ reason }) => {
      const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL ?? "";
      const response = await authFetch(
        `${baseUrl}/cases/${caseId}/escalations`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ reason }),
        },
      );
      if (!response.ok) {
        const body = await response.json().catch(() => ({})) as { message?: string };
        const err = new Error(
          body?.message ?? `Escalation failed: ${response.statusText}`,
        ) as EscalationApiError;
        err.status = response.status;
        throw err;
      }
      return response.json() as Promise<CreateEscalationResponse>;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.CASE_ESCALATIONS_SEARCH, caseId],
      });
      void queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.CASE_DETAILS],
      });
    },
  });
}
