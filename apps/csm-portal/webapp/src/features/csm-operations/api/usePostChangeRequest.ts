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
  BeCreateChangeRequestPayload,
  BeCreateChangeRequestResponse,
} from "@api/backend/types";

/**
 * Create a change request via `POST /change-requests` (ServiceNow data
 * source only). Unlike service requests (which are a case `type` handled by
 * `usePostCsmCase`), change requests are their own entity with no project
 * cascade in the create payload — the BE resolves project/deployment context
 * from the ServiceNow-side CI/service, not from IDs the portal sends. On
 * success the change-requests list is invalidated so the new record appears.
 */
export function usePostChangeRequest(): UseMutationResult<
  BeCreateChangeRequestResponse,
  Error,
  BeCreateChangeRequestPayload
> {
  const api = useBackendApi();
  const queryClient = useQueryClient();

  return useMutation<
    BeCreateChangeRequestResponse,
    Error,
    BeCreateChangeRequestPayload
  >({
    mutationFn: (payload): Promise<BeCreateChangeRequestResponse> =>
      api.post<BeCreateChangeRequestPayload, BeCreateChangeRequestResponse>(
        "/change-requests",
        payload,
      ),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.CHANGE_REQUESTS],
      });
    },
  });
}
