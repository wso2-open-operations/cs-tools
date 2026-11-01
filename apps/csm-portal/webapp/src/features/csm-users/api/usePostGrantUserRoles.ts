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
import type { BeGrantUserRolesPayload, BeGrantUserRolesResponse } from "@api/backend/types";

export interface GrantUserRolesInput {
  userId: string;
  roles: string[];
}

/**
 * Grant one or more portal roles (see `GET /roles/grantable`) to an
 * already-existing user via `POST /users/{id}/roles` — admin-only on the
 * backend (`PermAdmin`), same as `POST /users`'s own "Portal roles" grant at
 * creation time. A 403 reaches this hook as an ordinary error; the backend is
 * what actually protects the endpoint, not whatever shows this control.
 *
 * The backend reports success per-role (`granted`/`failed`) rather than
 * failing the whole call for a partial SCIM failure (HTTP 200 vs 207 — both
 * resolve here, the caller reads the body to tell them apart). On success,
 * invalidates the profile query so the newly granted roles show up in
 * `UserProfilePage.tsx`'s "CSM Platform roles" list without a manual refresh.
 */
export function usePostGrantUserRoles(): UseMutationResult<
  BeGrantUserRolesResponse,
  Error,
  GrantUserRolesInput
> {
  const api = useBackendApi();
  const queryClient = useQueryClient();

  return useMutation<BeGrantUserRolesResponse, Error, GrantUserRolesInput>({
    mutationFn: ({ userId, roles }) =>
      api.post<BeGrantUserRolesPayload, BeGrantUserRolesResponse>(
        `/users/${encodeURIComponent(userId)}/roles`,
        { roles },
      ),
    onSuccess: (_data, { userId }) => {
      queryClient.invalidateQueries({ queryKey: ["csm-user-by-id", userId] });
    },
  });
}
