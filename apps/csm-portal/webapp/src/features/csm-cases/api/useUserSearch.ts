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
  useInfiniteQuery,
} from "@tanstack/react-query";
import { useMemo } from "react";
import { useBackendApi } from "@api/backend/client";
import {
  normalizeUserSearchResponse,
  type NormalizedUserSearchResult,
  type SearchUsersRequest,
  type SearchUsersResponse,
} from "@features/csm-users/types/csmUsers";

/** Page size for the lazy-loaded (scroll-to-load-more) assignee filter. */
export const USER_PAGE_SIZE = 10;

/** A single directory match: name (label) + email (the filter value). */
export interface UserSearchOption {
  name: string;
  email: string;
}

/** Flattened, paginated result for the lazy-loaded assignee filter. */
export interface InfiniteUserSearch {
  /** All matching users loaded so far, de-duplicated by email. */
  users: UserSearchOption[];
  isFetching: boolean;
  isFetchingNextPage: boolean;
  hasNextPage: boolean;
  isError: boolean;
  /** Fetch the next page (wired to the dropdown's scroll). */
  fetchNextPage: () => void;
}

/**
 * Paginated user-directory search for the cases assignee filter — the
 * assignee-side twin of {@link useInfiniteProjectSearch}. Loads the first
 * {@link USER_PAGE_SIZE} users as soon as it is enabled (the dropdown opens) —
 * no typing required — and pages through the rest on demand via
 * {@link InfiniteUserSearch.fetchNextPage} (wired to the listbox scroll). An
 * empty query lists the directory a page at a time; a typed query sends
 * `searchQuery` to `POST /users/search` so anyone is findable, not just the
 * first page of users. The `oneOf` PG/SN response is normalized so the picker
 * never branches on the live data source.
 */
export function useInfiniteUserSearch(
  query: string,
  enabled: boolean,
): InfiniteUserSearch {
  const api = useBackendApi();
  const q = query.trim();

  const result = useInfiniteQuery<NormalizedUserSearchResult, Error>({
    queryKey: ["csm-users", "assignee-search", q],
    queryFn: async ({ pageParam }) => {
      const request: SearchUsersRequest = {
        pagination: { offset: pageParam as number, limit: USER_PAGE_SIZE },
        ...(q.length > 0 ? { filters: { searchQuery: q } } : {}),
      };
      const res = await api.post<SearchUsersRequest, SearchUsersResponse>(
        "/users/search",
        request,
      );
      return normalizeUserSearchResponse(res);
    },
    initialPageParam: 0,
    getNextPageParam: (lastPage, allPages) => {
      if (!lastPage.hasMore) return undefined;
      // Stop if the last page came back empty: the offset wouldn't advance, so
      // scroll-fetching would otherwise loop on the same page forever.
      if ((lastPage.users?.length ?? 0) === 0) return undefined;
      // Next offset = rows already loaded; robust even if the backend does not
      // echo back the offset we sent.
      return allPages.reduce((n, p) => n + (p.users?.length ?? 0), 0);
    },
    enabled,
    // Keep prior pages on screen while a new query's first page loads, so the
    // dropdown doesn't flash empty between searches.
    placeholderData: keepPreviousData,
    staleTime: 60_000,
  });

  // Flatten every fetched page, keeping only rows with both a name (the label)
  // and an email (the filter value), de-duplicated by email so paging can't
  // surface the same person twice.
  const users = useMemo(() => {
    const seen = new Set<string>();
    const out: UserSearchOption[] = [];
    for (const u of (result.data?.pages ?? []).flatMap((p) => p.users)) {
      if (!u.email || !u.name || seen.has(u.email)) continue;
      seen.add(u.email);
      out.push({ name: u.name, email: u.email });
    }
    return out;
  }, [result.data]);

  return {
    users,
    isFetching: result.isFetching,
    isFetchingNextPage: result.isFetchingNextPage,
    hasNextPage: result.hasNextPage,
    isError: result.isError,
    fetchNextPage: () => {
      void result.fetchNextPage();
    },
  };
}
