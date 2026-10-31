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
import { useBackendApi } from "@api/backend/client";
import { skillKeys } from "@features/csm-skills/api/skillKeys";
import type {
  SearchUserSkillsRequest,
  SearchUserSkillsResponse,
} from "@features/csm-skills/types";

const SEARCH_PATH = "/skills/users/search";

function isForbidden(error: unknown): boolean {
  return typeof error === "object" && error !== null && (error as { status?: unknown }).status === 403;
}

/**
 * Whether the caller is a skill manager. There is no flag for it, so this asks
 * the manager search for one row: a success means yes, a 403 means no (the
 * backend is the gate). Cached for the session; any other failure leaves the
 * query in error, which callers treat as "do not show the tab".
 */
export function useIsSkillManager(): UseQueryResult<boolean, Error> {
  const api = useBackendApi();
  return useQuery<boolean, Error>({
    queryKey: skillKeys.managerProbe,
    queryFn: async () => {
      try {
        await api.post<SearchUserSkillsRequest, SearchUserSkillsResponse>(SEARCH_PATH, {
          pagination: { limit: 1 },
        });
        return true;
      } catch (e) {
        if (isForbidden(e)) return false;
        throw e;
      }
    },
    retry: false,
    staleTime: Infinity,
    gcTime: Infinity,
  });
}

/** `POST /skills/users/search`: person x skill rows, filtered and paged by the backend. */
export function useSearchUserSkills(
  request: SearchUserSkillsRequest,
): UseQueryResult<SearchUserSkillsResponse, Error> {
  const api = useBackendApi();
  return useQuery<SearchUserSkillsResponse, Error>({
    queryKey: skillKeys.people(request),
    queryFn: async () =>
      (await api.post<SearchUserSkillsRequest, SearchUserSkillsResponse>(SEARCH_PATH, request)) ?? {
        items: [],
        total: 0,
        limit: request.pagination?.limit ?? 0,
        offset: request.pagination?.offset ?? 0,
      },
    placeholderData: keepPreviousData,
  });
}
