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
import type { BeCreateProblemPayload, BeProblemDetail } from "@api/backend/types";

/**
 * Create a problem via `POST /problems` (ServiceNow data source only). Unlike
 * change requests/incidents, the response is the full problem-detail object
 * directly (no wrapper envelope) — the same shape `GET /problems/{id}`
 * returns. On success the problems list is invalidated so the new record
 * appears.
 */
export function usePostProblem(): UseMutationResult<
  BeProblemDetail,
  Error,
  BeCreateProblemPayload
> {
  const api = useBackendApi();
  const queryClient = useQueryClient();

  return useMutation<BeProblemDetail, Error, BeCreateProblemPayload>({
    mutationFn: (payload): Promise<BeProblemDetail> =>
      api.post<BeCreateProblemPayload, BeProblemDetail>("/problems", payload),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: [ApiQueryKeys.PROBLEMS] });
    },
  });
}
