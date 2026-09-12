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
  BePatchProblemResponse,
  BeUpdateProblemPayload,
} from "@api/backend/types";

export interface PatchProblemInput {
  id: string;
  patch: BeUpdateProblemPayload;
}

/**
 * Update a problem via `PATCH /problems/{id}` (ServiceNow data source only).
 * The BE requires at least one field in `patch`.
 *
 * The detail and the problems list are invalidated on both success *and*
 * error (`onSettled`, not just `onSuccess`) — same reasoning as
 * `usePatchChangeRequest`: a state transition can be rejected/reverted by a
 * live ServiceNow business rule (the response's `state` always reflects the
 * real post-write value, per `CHANGES-problem-update.md` §2), so trusting a
 * stale cached state after a failed attempt would make an already-attempted
 * transition button look available again. Refetching after every attempt
 * keeps the next render honest regardless of which side of that ambiguity a
 * given failure falls on.
 *
 * The response itself (`BeProblemUpdateView`) is intentionally narrower than
 * the full problem detail — it doesn't echo `causeNotes`/`fixNotes`/
 * `workaround`/`targetResolutionDate` back — so the invalidation, not the
 * mutation's own return value, is what makes those show up on screen.
 */
export function usePatchProblem(): UseMutationResult<
  BePatchProblemResponse,
  Error,
  PatchProblemInput
> {
  const api = useBackendApi();
  const queryClient = useQueryClient();

  return useMutation<BePatchProblemResponse, Error, PatchProblemInput>({
    mutationFn: (input): Promise<BePatchProblemResponse> =>
      api.patch<BeUpdateProblemPayload, BePatchProblemResponse>(
        `/problems/${encodeURIComponent(input.id)}`,
        input.patch,
      ),
    onSettled: (_data, _error, variables) => {
      void queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.PROBLEM_DETAILS, variables.id],
      });
      void queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.PROBLEMS],
      });
    },
  });
}
