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
import { isMockMode, useBackendApi } from "@api/backend/client";
import type { BeCase, BeCaseUpdatePayload } from "@api/backend/types";

const MOCK_LATENCY_MS = 200;

/**
 * Update a case via `PATCH /cases/{id}` — used for state transitions (and
 * priority changes). On success it invalidates this case's detail query and the
 * cross-project list so both reflect the new state.
 *
 * In MOCK mode the mutation resolves without persisting (mock detail data is
 * static), so the caller's optimistic feedback is the only visible effect.
 */
export function usePatchCsmCase(
  caseId: string | undefined,
): UseMutationResult<BeCase, Error, BeCaseUpdatePayload> {
  const api = useBackendApi();
  const queryClient = useQueryClient();

  return useMutation<BeCase, Error, BeCaseUpdatePayload>({
    mutationFn: async (input): Promise<BeCase> => {
      if (!caseId) {
        throw new Error("Cannot update a case without an id.");
      }
      if (isMockMode()) {
        await new Promise((r) => setTimeout(r, MOCK_LATENCY_MS));
        return {
          id: caseId,
          state: input.state,
          priority: input.priority,
        };
      }
      return api.patch<BeCaseUpdatePayload, BeCase>(
        `/cases/${encodeURIComponent(caseId)}`,
        input,
      );
    },
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.CSM_CASE_DETAIL, caseId ?? ""],
      });
      queryClient.invalidateQueries({ queryKey: [ApiQueryKeys.CSM_CASES] });
    },
  });
}
