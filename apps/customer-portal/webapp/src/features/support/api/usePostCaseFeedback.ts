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

import { useMutation } from "@tanstack/react-query";
import { useAuthApiClient } from "@/hooks/useAuthApiClient";
import type {
  CaseFeedbackPayload,
  CaseFeedbackResponse,
} from "@features/support/types/feedback";

export type CaseFeedbackApiError = Error & { status: number };

/**
 * Submits feedback for a case (POST /cases/{caseId}/feedback).
 *
 * @param {string} caseId - The case ID to submit feedback for.
 * @returns Mutation result for the feedback submission.
 */
export function usePostCaseFeedback(caseId: string) {
  const authFetch = useAuthApiClient();

  return useMutation<CaseFeedbackResponse, CaseFeedbackApiError, CaseFeedbackPayload>({
    mutationFn: async (payload) => {
      const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL ?? "";
      const response = await authFetch(`${baseUrl}/cases/${caseId}/feedback`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload),
      });
      if (!response.ok) {
        const body = (await response.json().catch(() => ({}))) as { message?: string };
        const err = new Error(
          body?.message ?? `Feedback submission failed: ${response.statusText}`,
        ) as CaseFeedbackApiError;
        err.status = response.status;
        throw err;
      }
      return response.json() as Promise<CaseFeedbackResponse>;
    },
  });
}
