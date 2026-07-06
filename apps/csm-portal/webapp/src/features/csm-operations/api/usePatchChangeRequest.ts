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
  BePatchChangeRequestPayload,
  BePatchChangeRequestResponse,
} from "@api/backend/types";

export interface PatchChangeRequestInput {
  id: string;
  patch: BePatchChangeRequestPayload;
}

/**
 * Update a change request via `PATCH /change-requests/{id}` (ServiceNow data
 * source only). The BE requires at least one field in `patch`. On success the
 * detail and any cached list/search are invalidated so the new values show.
 */
export function usePatchChangeRequest(): UseMutationResult<
  BePatchChangeRequestResponse,
  Error,
  PatchChangeRequestInput
> {
  const api = useBackendApi();
  const queryClient = useQueryClient();

  return useMutation<
    BePatchChangeRequestResponse,
    Error,
    PatchChangeRequestInput
  >({
    mutationFn: (input): Promise<BePatchChangeRequestResponse> =>
      api.patch<BePatchChangeRequestPayload, BePatchChangeRequestResponse>(
        `/change-requests/${encodeURIComponent(input.id)}`,
        input.patch,
      ),
    onSuccess: (_data, variables) => {
      void queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.CHANGE_REQUEST_DETAILS, variables.id],
      });
      void queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.CHANGE_REQUESTS],
      });
    },
  });
}
