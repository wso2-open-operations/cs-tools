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

import { keepPreviousData, useQuery, type UseQueryResult } from "@tanstack/react-query";
import { ApiQueryKeys } from "@constants/apiConstants";
import { useBackendApi } from "@api/backend/client";
import type {
  BeUser,
  BeUserSearchPayload,
  BeUserSearchResponse,
} from "@api/backend/types";
import { INTERNAL_USER_ROLES } from "@features/csm-users/types/csmUsers";

/** A single page of matches is plenty for a type-ahead picker. */
const USER_SEARCH_LIMIT = 20;

/** Optional server-side scoping beyond the typed search term — e.g. an
 * internal-only picker that should never offer a customer/external user (see
 * {@link useSearchInternalUsersByName}). Mirrors `useUserSearch`'s own
 * `UserSearchScope`. */
export interface UserSearchByNameScope {
  roleIds?: string[];
  active?: boolean;
}

/** Shared fetch behind both hooks below — kept out of `useSearchUsersByName`'s
 * own signature so that hook's type stays exactly `(query, enabled) => ...`,
 * matching `AsyncEntitySelect`/`AsyncEntityMultiSelect`'s
 * `useSearch: (query, enabled, extra?: string) => ...` prop shape for every
 * existing caller that passes `useSearchUsersByName` directly (adding a
 * third, non-`string` parameter to that hook's own type would silently break
 * assignability there). */
function useUsersByNameSearch(
  query: string,
  enabled: boolean,
  scope?: UserSearchByNameScope,
): UseQueryResult<BeUser[], Error> {
  const api = useBackendApi();
  const q = query.trim();

  return useQuery<BeUser[], Error>({
    queryKey: [ApiQueryKeys.USERS_SEARCH_BY_NAME, q, scope?.roleIds, scope?.active],
    queryFn: async (): Promise<BeUser[]> => {
      const res = await api.post<BeUserSearchPayload, BeUserSearchResponse>(
        "/users/search",
        {
          filters: {
            searchQuery: q,
            ...(scope?.roleIds && { roleIds: scope.roleIds }),
            ...(scope?.active !== undefined && { active: scope.active }),
          },
          pagination: { offset: 0, limit: USER_SEARCH_LIMIT },
        },
      );
      return (res.users ?? []).filter((u) => !!u.id);
    },
    enabled,
    placeholderData: keepPreviousData,
    staleTime: 60_000,
  });
}

/**
 * Type-ahead user search (`POST /users/search`, `filters.searchQuery`) that
 * returns each match's portal `id` — unlike {@link useDirectoryUsers} (which
 * loads the whole directory keyed by email for the cases assignee filter),
 * this is for pickers that need a user's UUID directly (change-request
 * "Requested by" / "Assigned to"). Fires as soon as the dropdown opens, even
 * with an empty query, so the picker shows a default page of people instead
 * of looking broken until the caller types something. Unscoped — searches the
 * full user directory, internal and external alike; see
 * {@link useSearchInternalUsersByName} for the internal-only twin.
 */
export function useSearchUsersByName(
  query: string,
  enabled: boolean,
): UseQueryResult<BeUser[], Error> {
  return useUsersByNameSearch(query, enabled);
}

/**
 * Internal-staff-only twin of {@link useSearchUsersByName}, scoped
 * server-side to `{ roleIds: INTERNAL_USER_ROLES, active: true }` so a picker
 * built on it can never offer a customer/external user. Same
 * `(query, enabled, extra?: string) => ...` shape as
 * `AsyncEntitySelect`/`AsyncEntityMultiSelect`'s `useSearch` prop expects (the
 * unused `extra` param is accepted for that compatibility, same as any other
 * `useSearch` implementation that doesn't need it). Use this wherever a user
 * picker must stay internal-only — i.e. everywhere except the admin
 * user-management page and the customer-contacts picker, which use different
 * search paths entirely.
 */
export function useSearchInternalUsersByName(
  query: string,
  enabled: boolean,
): UseQueryResult<BeUser[], Error> {
  return useUsersByNameSearch(query, enabled, {
    roleIds: INTERNAL_USER_ROLES,
    active: true,
  });
}
