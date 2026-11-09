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

import { useCallback, useEffect } from "react";
import {
  keepPreviousData,
  useInfiniteQuery,
  useQueries,
  useQuery,
  type UseQueryResult,
} from "@tanstack/react-query";
import {
  countArticles,
  getArticle,
  getRelatedArticles,
  listKnowledgeBases,
  searchArticles,
} from "@api/backend/client";
import { isValidId } from "@api/backend/types";
import type { KBArticle, SearchKBArticlesRequest } from "@api/backend/types";
import { getNextOffset, MAX_AUTO_EMPTY_PAGES, PAGE_SIZE, trailingEmptyPages } from "@api/pagination";

const KB_KEY = "kb";
const FIVE_MINUTES = 5 * 60 * 1000;

export const useKnowledgeBases = () =>
  useQuery({
    queryKey: [KB_KEY, "knowledge-bases"],
    queryFn: ({ signal }) => listKnowledgeBases(signal),
    staleTime: FIVE_MINUTES,
  });

export interface ArticleSearchFilters {
  knowledgeBaseId?: string;
  searchQuery?: string;
}

/**
 * Shared paging engine. Pages accumulate (`articles` is every page loaded so
 * far, deduplicated by id because newly published articles shift offsets
 * between requests) and `loadMore` fetches the next one. While a new filter is
 * loading the previous results stay on screen instead of collapsing to a
 * spinner. Empty pages that still report `hasMore` are skipped automatically,
 * up to MAX_AUTO_EMPTY_PAGES in a row.
 */
function usePagedSearch(
  keyParts: readonly string[],
  baseRequest: Omit<SearchKBArticlesRequest, "offset">,
) {
  const query = useInfiniteQuery({
    queryKey: [KB_KEY, "search", ...keyParts],
    initialPageParam: 0,
    queryFn: ({ pageParam, signal }) =>
      searchArticles({ ...baseRequest, offset: pageParam }, signal),
    getNextPageParam: getNextOffset,
    placeholderData: keepPreviousData,
  });

  const pages = query.data?.pages ?? [];
  const last = pages[pages.length - 1];
  const hasMore = query.hasNextPage;
  const emptyRun = trailingEmptyPages(pages);

  // Derived from the data, not stored: there is no render in which a skipped
  // empty page shows an empty state before the next page is requested.
  const isSkippingEmpty =
    !!last &&
    hasMore &&
    emptyRun > 0 &&
    emptyRun < MAX_AUTO_EMPTY_PAGES &&
    !query.isPlaceholderData &&
    !query.isError;

  const { fetchNextPage, isFetching } = query;
  useEffect(() => {
    if (isSkippingEmpty && !isFetching) void fetchNextPage();
  }, [isSkippingEmpty, isFetching, fetchNextPage, pages.length]);

  const seen = new Set<string>();
  const articles: KBArticle[] = [];
  for (const page of pages) {
    for (const article of page.articles) {
      if (seen.has(article.id)) continue;
      seen.add(article.id);
      articles.push(article);
    }
  }

  return {
    articles,
    /** Backend's count. An upper bound only when no knowledge base filter is applied. */
    total: last?.total ?? 0,
    hasMore,
    /** The backend has more, but the offset cap means they cannot be paged to. */
    isCapped: !!last && last.hasMore && !hasMore,
    /** Empty pages are being skipped over; show progress, not an empty state. */
    isSkippingEmpty,
    loadMore: () => void query.fetchNextPage(),
    refetch: () => void query.refetch(),
    isLoading: query.isLoading,
    /** True while a filter change is loading and old results are still shown. */
    isRefreshing: query.isPlaceholderData && query.isFetching,
    isLoadingMore: query.isFetchingNextPage,
    isError: query.isError,
    /** The failed request was a "load more", so earlier pages are intact. */
    isLoadMoreError: query.isFetchNextPageError,
    error: query.error,
  };
}

export function useArticleSearch({ knowledgeBaseId, searchQuery }: ArticleSearchFilters) {
  return usePagedSearch(["browse", knowledgeBaseId ?? "", searchQuery ?? ""], {
    ...(knowledgeBaseId ? { knowledgeBaseId } : {}),
    ...(searchQuery ? { searchQuery } : {}),
    limit: PAGE_SIZE,
  });
}

/** Newest articles first. `limit` is the page size requested, not how many to show. */
export const useRecentArticles = (limit: number) =>
  usePagedSearch(["recent", String(limit)], { sortBy: "recent", limit });

export const useArticle = (id: string | undefined) =>
  useQuery<KBArticle>({
    queryKey: [KB_KEY, "article", id ?? ""],
    queryFn: ({ signal }) => getArticle(id as string, signal),
    enabled: isValidId(id),
  });

export const useRelatedArticles = (id: string | undefined) =>
  useQuery({
    queryKey: [KB_KEY, "related", id ?? ""],
    queryFn: ({ signal }) => getRelatedArticles(id as string, signal),
    enabled: isValidId(id),
    staleTime: FIVE_MINUTES,
  });

/** Suggestions shown while typing: enough to be useful, few enough to scan. */
export const SUGGESTION_LIMIT = 5;
/** Shorter queries match too much to be worth a round trip. */
export const MIN_SUGGESTION_CHARS = 2;

/**
 * Type-ahead results for the search box. `enabled` lets the caller pause it
 * (input blurred, dropdown dismissed). Not retried: the next keystroke is the
 * retry, and a failed suggestion must never block a submitted search.
 */
export function useSearchSuggestions(query: string, enabled: boolean) {
  return useQuery({
    queryKey: [KB_KEY, "suggest", query],
    queryFn: ({ signal }) => searchArticles({ searchQuery: query, limit: SUGGESTION_LIMIT }, signal),
    enabled: enabled && query.length >= MIN_SUGGESTION_CHARS,
    placeholderData: keepPreviousData,
    retry: false,
  });
}

export type KnowledgeBaseCounts = Record<string, number | undefined>;

export interface KnowledgeBaseCountsResult {
  /** Loaded counts by knowledge base id; absent while loading or after a failure. */
  counts: KnowledgeBaseCounts;
  /** True for a knowledge base whose count is still being fetched, so a failed one is not mistaken for a pending one. */
  pending: Record<string, boolean>;
}

/**
 * Article count per knowledge base, one request each, cached for five
 * minutes. A count that fails to load is simply absent: the cards and filters
 * render without a number rather than with an error.
 */
export function useKnowledgeBaseCounts(ids: readonly string[]): KnowledgeBaseCountsResult {
  const key = ids.join("\u0000");
  const combine = useCallback(
    (results: UseQueryResult<number>[]): KnowledgeBaseCountsResult => ({
      counts: Object.fromEntries(ids.map((id, index) => [id, results[index]?.data])),
      pending: Object.fromEntries(ids.map((id, index) => [id, results[index]?.isPending ?? false])),
    }),
    // `key` stands in for the array identity.
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [key],
  );
  return useQueries({
    queries: ids.map((id) => ({
      queryKey: [KB_KEY, "count", id],
      queryFn: ({ signal }: { signal: AbortSignal }) => countArticles(id, signal),
      staleTime: FIVE_MINUTES,
      retry: false,
    })),
    combine,
  });
}
