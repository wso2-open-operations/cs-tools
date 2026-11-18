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

import { useQueries } from "@tanstack/react-query";
import { useMemo } from "react";
import { useBackendApi } from "@api/backend/client";
import {
  normalizeUserDetail,
  type NormalizedUserDetail,
  type SnUserDetail,
} from "@features/csm-users/types/csmUsers";

/**
 * Resolves user ids to display names with `GET /users/{id}`, one request per
 * id, sharing {@link useGetUserById}'s cache key. For id-valued pickers whose
 * selection arrives without a name (a filter restored from the URL, a picker
 * remounted by a Quick/Advanced switch), so a chip reads as the person's name
 * instead of a raw UUID. Pass only the ids not already named elsewhere; an id
 * that cannot be resolved is simply absent from the map.
 */
export function useUserNamesById(ids: string[]): Map<string, string> {
  const api = useBackendApi();
  const entries = useQueries({
    queries: ids.map((id) => ({
      queryKey: ["csm-user-by-id", id],
      queryFn: async (): Promise<NormalizedUserDetail | null> => {
        const user = await api.get<SnUserDetail>(`/users/${encodeURIComponent(id)}`);
        return user ? normalizeUserDetail(user) : null;
      },
      staleTime: 30_000,
      retry: false,
    })),
    combine: (results) =>
      JSON.stringify(
        results.flatMap((r, i) => {
          const name = r.data?.name?.trim();
          return name ? [[ids[i], name]] : [];
        }),
      ),
  });
  // Serialized above so the map keeps its identity until a lookup settles.
  return useMemo(() => new Map<string, string>(JSON.parse(entries)), [entries]);
}
