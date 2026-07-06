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

import { useQuery, type UseQueryResult } from "@tanstack/react-query";
import { ApiQueryKeys, BE_MAX_PAGE_LIMIT } from "@constants/apiConstants";
import { useBackendApi } from "@api/backend/client";

/**
 * Minimal user-directory entry: just what UI affordances like the cases
 * assignee filter need. Intentionally narrower than the admin `CsmUser` so
 * features can look up people without depending on the csm-admin module.
 */
export interface DirectoryUser {
  name: string;
  email: string;
}

/** Raw user shape the backend may return; mapped defensively to DirectoryUser. */
interface RawDirectoryUser {
  name?: string;
  firstName?: string;
  lastName?: string;
  email?: string;
}

function toDirectoryUser(u: RawDirectoryUser): DirectoryUser {
  const name =
    u.name?.trim() ||
    `${u.firstName ?? ""} ${u.lastName ?? ""}`.trim();
  return { name, email: u.email ?? "" };
}

/**
 * Shared user-directory lookup. Backed by `POST /users/search`
 * (the canonical backend route — there is no `GET /users`). Returns a
 * lightweight {@link DirectoryUser} list.
 */
export function useDirectoryUsers(): UseQueryResult<DirectoryUser[], Error> {
  const api = useBackendApi();

  return useQuery<DirectoryUser[], Error>({
    queryKey: [ApiQueryKeys.CSM_ADMIN_USERS, "directory"],
    queryFn: async (): Promise<DirectoryUser[]> => {
      // No filters: fetch the broadest page the BE allows (default is only 10).
      const res = await api.post<{ pagination: { limit: number } }, unknown>(
        "/users/search",
        { pagination: { limit: BE_MAX_PAGE_LIMIT } },
      );
      const rows: RawDirectoryUser[] = Array.isArray(res)
        ? (res as RawDirectoryUser[])
        : ((res as { users?: RawDirectoryUser[] })?.users ?? []);
      // Require both name (the option label) and email (the value the assignee
      // filter sends as `assignedTo`) — a name-only user would yield an empty,
      // invalid filter value.
      return rows.map(toDirectoryUser).filter((u) => u.name && u.email);
    },
    staleTime: 30_000,
  });
}
