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
import { ApiQueryKeys } from "@constants/apiConstants";
import { useBackendApi } from "@api/backend/client";
import type {
  BeCaseEscalation,
  BeCaseEscalationCreatePayload,
} from "@api/backend/types";

/**
 * Escalate or de-escalate a case via `POST /cases/{id}/escalations`
 * (ServiceNow data source only — the caller surfaces a rejection on another
 * source). `reason` is required when escalating (action omitted defaults to
 * `"ESCALATE"` server-side) and optional when de-escalating; the backend
 * rejects an escalate call with no reason.
 *
 * The backend also records a case work note as a side effect of a
 * successful call — this hook does not post any comment itself.
 *
 * On success, invalidates both the case detail (so the level chip refreshes)
 * and this case's escalation-history query (so the new entry shows without a
 * manual refetch).
 */
export function usePostCsmCaseEscalation(
  caseId: string | undefined,
): UseMutationResult<BeCaseEscalation, Error, BeCaseEscalationCreatePayload> {
  const api = useBackendApi();
  const queryClient = useQueryClient();

  return useMutation<BeCaseEscalation, Error, BeCaseEscalationCreatePayload>({
    mutationFn: async (input): Promise<BeCaseEscalation> => {
      if (!caseId) {
        throw new Error("Cannot escalate a case without an id.");
      }
      return api.post<BeCaseEscalationCreatePayload, BeCaseEscalation>(
        `/cases/${encodeURIComponent(caseId)}/escalations`,
        input,
      );
    },
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.CSM_CASE_DETAIL, caseId ?? ""],
      });
      queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.CSM_CASE_ESCALATIONS, caseId ?? ""],
      });
      // The escalation also writes a case work note server-side, so refresh
      // the activity/field-change lane too — same reasoning as
      // usePatchCsmCase's own CSM_CASE_ACTIVITIES invalidation.
      queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.CSM_CASE_ACTIVITIES, caseId ?? ""],
      });
      queryClient.invalidateQueries({ queryKey: [ApiQueryKeys.CSM_CASES] });
    },
  });
}
