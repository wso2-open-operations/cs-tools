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
import type { CaseSlaRecalcResponse } from "@features/csm-sla/types/workItemSla";

/**
 * Admin-only: recomputes one case's currently active (IN_PROGRESS or PAUSED)
 * SLA clocks' pause totals by replaying the case's real state-change
 * history, via `POST /cases/{id}/sla/recalculate`. See entity-service's own
 * `CaseSLARecalcService` for what gets corrected and why a case with no
 * activity history (`hasHistory: false` in the response) is left untouched.
 *
 * On success, invalidates this case's SLA list so the table reflects any
 * corrected totals without a manual refresh.
 */
export function usePostCaseSlaRecalculation(
  caseId: string | undefined,
): UseMutationResult<CaseSlaRecalcResponse, Error, void> {
  const api = useBackendApi();
  const queryClient = useQueryClient();

  return useMutation<CaseSlaRecalcResponse, Error, void>({
    mutationFn: async (): Promise<CaseSlaRecalcResponse> => {
      if (!caseId) {
        throw new Error("Cannot recalculate SLA for a case without an id.");
      }
      return api.post<Record<string, never>, CaseSlaRecalcResponse>(
        `/cases/${encodeURIComponent(caseId)}/sla/recalculate`,
        {},
      );
    },
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.WORK_ITEM_SLAS, caseId ?? ""],
      });
    },
  });
}
