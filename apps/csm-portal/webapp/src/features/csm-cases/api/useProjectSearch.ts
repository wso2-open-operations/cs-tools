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
  keepPreviousData,
  useQuery,
  type UseQueryResult,
} from "@tanstack/react-query";
import { ApiQueryKeys } from "@constants/apiConstants";
import { useBackendApi } from "@api/backend/client";
import type {
  BeProject,
  BeProjectSearchPayload,
  BeProjectSearchResponse,
} from "@api/backend/types";

/** A single page of matches is plenty for a type-ahead picker. */
const PROJECT_SEARCH_LIMIT = 20;

/**
 * Type-ahead project search for the cases project filter. Unlike
 * {@link useProjectOptions} (which pages through the whole catalogue), this
 * sends the typed term to `POST /projects/search` and returns just the first
 * page of matches. The query stays disabled until the user has typed, so the
 * project list is never loaded up front — only what the user searches for.
 */
export function useProjectSearch(
  query: string,
  enabled: boolean,
): UseQueryResult<BeProject[], Error> {
  const api = useBackendApi();
  const q = query.trim();

  return useQuery<BeProject[], Error>({
    queryKey: [ApiQueryKeys.CSM_PROJECTS, "search", q],
    queryFn: async (): Promise<BeProject[]> => {
      const res = await api.post<
        BeProjectSearchPayload,
        BeProjectSearchResponse
      >("/projects/search", {
        searchQuery: q,
        pagination: { offset: 0, limit: PROJECT_SEARCH_LIMIT },
      });
      return res.projects ?? [];
    },
    enabled: enabled && q.length > 0,
    // Keep the prior matches on screen while the next keystroke's query runs,
    // so the dropdown doesn't flash empty between searches.
    placeholderData: keepPreviousData,
    staleTime: 60_000,
  });
}
