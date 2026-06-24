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
import type {
  BeCaseUpdatePayload,
  BeUpdateCaseResponse,
} from "@api/backend/types";

const MOCK_LATENCY_MS = 200;

/**
 * Update a case via `PATCH /cases/{id}` — state transitions, priority changes,
 * assignee (`assigneeEmail`), or watch list (`watchList`). The backend requires
 * **exactly one** of those fields per call, so callers must not combine them.
 * The response is `BeUpdateCaseResponse` ({ message, case }), but on success we
 * ignore the body and invalidate this case's detail query and the cross-project
 * list so both refetch the authoritative state (incl. fresh `nextStates`).
 *
 * In MOCK mode the mutation resolves without persisting (mock detail data is
 * static), so the caller's optimistic feedback is the only visible effect.
 */
export function usePatchCsmCase(
  caseId: string | undefined,
): UseMutationResult<BeUpdateCaseResponse, Error, BeCaseUpdatePayload> {
  const api = useBackendApi();
  const queryClient = useQueryClient();

  return useMutation<BeUpdateCaseResponse, Error, BeCaseUpdatePayload>({
    mutationFn: async (input): Promise<BeUpdateCaseResponse> => {
      if (!caseId) {
        throw new Error("Cannot update a case without an id.");
      }
      if (isMockMode()) {
        await new Promise((r) => setTimeout(r, MOCK_LATENCY_MS));
        // The success handler ignores the body and refetches, so the mock only
        // needs the id. (Echoing input.state/priority would not type-check
        // against the single-field union and isn't used anyway.)
        return { message: "Case updated (mock).", case: { id: caseId } };
      }
      return api.patch<BeCaseUpdatePayload, BeUpdateCaseResponse>(
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
