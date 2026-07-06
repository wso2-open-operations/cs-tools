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
  useQuery,
  type UseQueryResult,
} from "@tanstack/react-query";
import { useMemo } from "react";
import { ApiQueryKeys } from "@constants/apiConstants";
import { useBackendApi } from "@api/backend/client";
import type {
  BeProject,
  BeProjectSearchPayload,
  BeProjectSearchResponse,
} from "@api/backend/types";

/** A single page of matches is plenty for a type-ahead picker. */
const PROJECT_SEARCH_LIMIT = 20;

/** Page size for the lazy-loaded (scroll-to-load-more) project filter. */
export const PROJECT_PAGE_SIZE = 10;

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

/** Flattened, paginated result for the lazy-loaded project filter. */
export interface InfiniteProjectSearch {
  /** All projects loaded so far, across every fetched page. */
  projects: BeProject[];
  isFetching: boolean;
  isFetchingNextPage: boolean;
  hasNextPage: boolean;
  isError: boolean;
  /** Fetch the next page (wired to the dropdown's scroll). */
  fetchNextPage: () => void;
}

/**
 * Paginated project search for the cases project filter. Loads the first
 * {@link PROJECT_PAGE_SIZE} projects as soon as it is enabled (the dropdown
 * opens) — no typing required — and pages through the rest on demand via
 * {@link InfiniteProjectSearch.fetchNextPage} (wired to the listbox scroll). An
 * empty query lists the whole catalogue a page at a time; a typed query narrows
 * it and re-pages from the first match.
 */
export function useInfiniteProjectSearch(
  query: string,
  enabled: boolean,
): InfiniteProjectSearch {
  const api = useBackendApi();
  const q = query.trim();

  const result = useInfiniteQuery<BeProjectSearchResponse, Error>({
    queryKey: [ApiQueryKeys.CSM_PROJECTS, "search-paged", q],
    queryFn: ({ pageParam }) => {
      const pagination = { offset: pageParam as number, limit: PROJECT_PAGE_SIZE };
      return api.post<BeProjectSearchPayload, BeProjectSearchResponse>(
        "/projects/search",
        q.length > 0 ? { searchQuery: q, pagination } : { pagination },
      );
    },
    initialPageParam: 0,
    getNextPageParam: (lastPage, allPages) => {
      if (!lastPage.hasMore) return undefined;
      // Stop if the last page came back empty: the offset wouldn't advance, so
      // scroll-fetching would otherwise loop on the same page forever.
      if ((lastPage.projects?.length ?? 0) === 0) return undefined;
      // Next offset = rows already loaded; robust even if the backend does not
      // echo back the offset we sent.
      return allPages.reduce((n, p) => n + (p.projects?.length ?? 0), 0);
    },
    enabled,
    // Keep prior pages on screen while a new query's first page loads, so the
    // dropdown doesn't flash empty between searches.
    placeholderData: keepPreviousData,
    staleTime: 60_000,
  });

  const projects = useMemo(
    () => (result.data?.pages ?? []).flatMap((p) => p.projects ?? []),
    [result.data],
  );

  return {
    projects,
    isFetching: result.isFetching,
    isFetchingNextPage: result.isFetchingNextPage,
    hasNextPage: result.hasNextPage,
    isError: result.isError,
    fetchNextPage: () => {
      void result.fetchNextPage();
    },
  };
}
