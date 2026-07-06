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
  BeCaseCreateBody,
  BeCaseCreateResponse,
  BeCreatedCase,
} from "@api/backend/types";

/**
 * Create a case via `POST /cases`. Accepts any supported create body (standard
 * support `case` or catalog `service_request`). The backend wraps the result in
 * a `{ message, case }` envelope, so this unwraps and returns the created case
 * (its `id` drives the post-create redirect). The mutation also invalidates
 * project-scoped case searches so list views refresh.
 */
export function usePostCsmCase(): UseMutationResult<
  BeCreatedCase,
  Error,
  BeCaseCreateBody
> {
  const api = useBackendApi();
  const queryClient = useQueryClient();

  return useMutation<BeCreatedCase, Error, BeCaseCreateBody>({
    mutationFn: async (input): Promise<BeCreatedCase> => {
      const res = await api.post<BeCaseCreateBody, BeCaseCreateResponse>(
        "/cases",
        input,
      );
      return res.case;
    },
    // The create response carries no projectId, so invalidate the project-scoped
    // search using the submitted payload's project instead.
    onSuccess: (_created, variables) => {
      queryClient.invalidateQueries({
        queryKey: [
          ApiQueryKeys.BACKEND_PROJECT_CASES_SEARCH,
          variables.projectId,
        ],
      });
      // And the cross-project CSM cases list (fan-out aggregation).
      queryClient.invalidateQueries({ queryKey: [ApiQueryKeys.CSM_CASES] });
    },
  });
}
