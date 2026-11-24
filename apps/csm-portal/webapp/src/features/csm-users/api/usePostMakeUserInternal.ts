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
import { useBackendApi } from "@api/backend/client";
import type { BeMakeUserInternalResponse } from "@api/backend/types";

/**
 * Flips an existing @wso2.com user from external to internal via
 * `POST /users/{id}/internalize` — admin-only on the backend (`PermAdmin`),
 * which also re-checks the target is a @wso2.com account before forwarding
 * to entity-service. A 403/400 reaches this hook as an ordinary error; the
 * backend is what actually protects the endpoint, not whatever shows this
 * control (`usePortalAccess().canMakeUserInternal`).
 *
 * On success, invalidates the profile query so the user's "Customer Portal
 * roles" chips (Internal, no more External/Customer/Partner) refresh without
 * a manual reload.
 */
export function usePostMakeUserInternal(): UseMutationResult<
  BeMakeUserInternalResponse,
  Error,
  string
> {
  const api = useBackendApi();
  const queryClient = useQueryClient();

  return useMutation<BeMakeUserInternalResponse, Error, string>({
    mutationFn: (userId) =>
      api.postEmpty<BeMakeUserInternalResponse>(
        `/users/${encodeURIComponent(userId)}/internalize`,
      ),
    onSuccess: (_data, userId) => {
      queryClient.invalidateQueries({ queryKey: ["csm-user-by-id", userId] });
    },
  });
}
