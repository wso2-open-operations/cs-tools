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
import { useLogger } from "@hooks/useLogger";
import { ApiQueryKeys } from "@constants/apiConstants";
import { isMockMode, useBackendApi } from "@api/backend/client";

/**
 * Minimal user-directory entry: just what UI affordances like the cases
 * assignee filter need. Intentionally narrower than the admin `CsmUser` so
 * features can look up people without depending on the csm-admin module.
 */
export interface DirectoryUser {
  name: string;
  email: string;
}

const MOCK_LATENCY_MS = 150;

// A small mock directory so the assignee picker is populated in mock mode
// without coupling to the csm-admin mock data set.
const MOCK_DIRECTORY: DirectoryUser[] = [
  { name: "Sajith Ekanayaka", email: "sajithe@wso2.com" },
  { name: "Renee Park", email: "renee.park@example.com" },
  { name: "Marcus Webb", email: "marcus.webb@example.com" },
  { name: "Lena Fischer", email: "lena.fischer@example.com" },
  { name: "Arjun Nair", email: "arjun.nair@example.com" },
  { name: "Priya Sharma", email: "priya.sharma@example.com" },
];

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
 * Shared user-directory lookup. Backed by `POST /users/search` in live mode
 * (the canonical backend route — there is no `GET /users`), and a small mock
 * set under the mock toggle. Returns a lightweight {@link DirectoryUser} list.
 */
export function useDirectoryUsers(): UseQueryResult<DirectoryUser[], Error> {
  const logger = useLogger();
  const api = useBackendApi();

  return useQuery<DirectoryUser[], Error>({
    queryKey: [ApiQueryKeys.CSM_ADMIN_USERS, "directory"],
    queryFn: async (): Promise<DirectoryUser[]> => {
      if (isMockMode()) {
        logger.debug("[useDirectoryUsers] returning mock directory");
        await new Promise((r) => setTimeout(r, MOCK_LATENCY_MS));
        return MOCK_DIRECTORY;
      }
      const res = await api.post<Record<string, never>, unknown>(
        "/users/search",
        {},
      );
      const rows: RawDirectoryUser[] = Array.isArray(res)
        ? (res as RawDirectoryUser[])
        : ((res as { users?: RawDirectoryUser[] })?.users ?? []);
      return rows.map(toDirectoryUser).filter((u) => u.name);
    },
    staleTime: 30_000,
  });
}
