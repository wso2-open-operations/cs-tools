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
import { useCallback } from "react";
import { ApiQueryKeys } from "@constants/apiConstants";
import { useBackendApi } from "@api/backend/client";
import type {
  BeCaseUpdatePayload,
  BeUpdateCaseResponse,
} from "@api/backend/types";

/**
 * Update a case via `PATCH /cases/{id}` — state transitions, priority changes,
 * work sub-state (`workState`), assignee (`assigneeEmail`), or watch list
 * (`watchList`). The backend requires **exactly one** of those fields per call,
 * so callers must not combine them.
 * The response is `BeUpdateCaseResponse` ({ message, case }), but on success we
 * ignore the body and invalidate this case's detail query and the cross-project
 * list so both refetch the authoritative state (incl. fresh `nextStates`).
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
      // A state/severity/assignee/watcher patch is audited server-side, so
      // refresh the activity/field-change lane too — otherwise the new
      // lifecycle entry wouldn't show until the next unrelated refetch.
      queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.CSM_CASE_ACTIVITIES, caseId ?? ""],
      });
    },
  });
}

/**
 * Patch an **arbitrary** case by id (not bound to a single case like
 * {@link usePatchCsmCase}). Used when one action must update other cases too —
 * e.g. pausing the engineer's other ongoing case(s) when they start work on a
 * new one. Same single-field-per-call contract; invalidates the patched case's
 * detail and the cross-project list so both refetch.
 */
export function usePatchCsmCaseById(): (
  caseId: string,
  input: BeCaseUpdatePayload,
) => Promise<void> {
  const api = useBackendApi();
  const queryClient = useQueryClient();

  return useCallback(
    async (caseId: string, input: BeCaseUpdatePayload): Promise<void> => {
      if (!caseId) throw new Error("Cannot update a case without an id.");
      await api.patch<BeCaseUpdatePayload, BeUpdateCaseResponse>(
        `/cases/${encodeURIComponent(caseId)}`,
        input,
      );
      queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.CSM_CASE_DETAIL, caseId],
      });
      queryClient.invalidateQueries({ queryKey: [ApiQueryKeys.CSM_CASES] });
      queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.CSM_CASE_ACTIVITIES, caseId],
      });
    },
    [api, queryClient],
  );
}
