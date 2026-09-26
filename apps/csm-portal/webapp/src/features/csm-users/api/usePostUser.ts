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
import type { BeCreateUserPayload, BeUser } from "@api/backend/types";

/**
 * Create a new platform user via `POST /users`. Admin-only on the backend
 * (`PermAdmin`) — a caller without the admin role gets a 403 from the
 * backend regardless of whether the Add User control is shown.
 *
 * On success, invalidates every `useSearchUsers` query (key prefix
 * `"csm-users-search"`) so the new user appears in the list without a manual
 * refresh.
 */
export function usePostUser(): UseMutationResult<BeUser, Error, BeCreateUserPayload> {
  const api = useBackendApi();
  const queryClient = useQueryClient();

  return useMutation<BeUser, Error, BeCreateUserPayload>({
    mutationFn: (input) => api.post<BeCreateUserPayload, BeUser>("/users", input),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["csm-users-search"] });
    },
  });
}
