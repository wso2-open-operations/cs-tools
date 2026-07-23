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
import { useBackendApi } from "@api/backend/client";
import type { BeSearchTagsResponse, BeTag } from "@api/backend/types";

/** Cap on how many matching tags a search returns — this is a type-ahead list, not a paged browse. */
const TAG_SEARCH_LIMIT = 20;

/**
 * Searches existing free-text tag labels via `GET /tags/search?q=&limit=`, for
 * the {@link AddTagDialog} type-ahead. Tags are genuinely free-text on the
 * backing data source (SN's generic label mechanism) — this only offers
 * already-used labels as suggestions; the dialog still allows creating a
 * label with no match. Runs even for an empty query (lists recently/commonly
 * used tags) while `enabled` — callers gate that on the dialog being open.
 */
export function useSearchTags(
  query: string,
  enabled: boolean,
): UseQueryResult<BeTag[], Error> {
  const api = useBackendApi();
  const q = query.trim();

  return useQuery<BeTag[], Error>({
    queryKey: ["csm-tags-search", q],
    queryFn: async (): Promise<BeTag[]> => {
      const params = new URLSearchParams();
      if (q) params.set("q", q);
      params.set("limit", String(TAG_SEARCH_LIMIT));
      const res = await api.get<BeSearchTagsResponse>(
        `/tags/search?${params.toString()}`,
      );
      return res?.tags ?? [];
    },
    enabled,
    staleTime: 30_000,
    placeholderData: keepPreviousData,
  });
}
