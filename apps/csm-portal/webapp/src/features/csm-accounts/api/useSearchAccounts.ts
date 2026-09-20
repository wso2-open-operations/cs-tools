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
} from "@tanstack/react-query";
import { useMemo } from "react";
import { useAuthApiClient } from "@hooks/useAuthApiClient";
import { apiConfig } from "@config/apiConfig";
import { ApiError, parseApiResponseMessage } from "@utils/ApiError";
import type {
  Account,
  SearchAccountsRequest,
  SearchAccountsResponse,
} from "@features/csm-accounts/types/csmAccounts";

export function useSearchAccounts(request: SearchAccountsRequest) {
  const authFetch = useAuthApiClient();

  return useQuery<SearchAccountsResponse, Error>({
    queryKey: ["csm-accounts-search", request],
    queryFn: async () => {
      const res = await authFetch(`${apiConfig.backendUrl}/accounts/search`, {
        method: "POST",
        body: JSON.stringify(request),
      });
      if (!res.ok) {
        const body = await res.text();
        throw new ApiError(
          res.status,
          res.statusText,
          parseApiResponseMessage(body, res.status, res.statusText),
        );
      }
      return (await res.json()) as SearchAccountsResponse;
    },
    placeholderData: keepPreviousData,
    staleTime: 30_000,
  });
}

/** Page size for the lazy-loaded (scroll-to-load-more) account filter — same
 * default as {@link PROJECT_PAGE_SIZE} in `useProjectSearch.ts`, the picker
 * this hook mirrors. */
export const ACCOUNT_PAGE_SIZE = 10;

/** Flattened, paginated result for the lazy-loaded account filter — same
 * shape as `useProjectSearch.ts`'s `InfiniteProjectSearch`. */
export interface InfiniteAccountSearch {
  /** All accounts loaded so far, across every fetched page. */
  accounts: Account[];
  isFetching: boolean;
  isFetchingNextPage: boolean;
  hasNextPage: boolean;
  isError: boolean;
  /** Fetch the next page (wired to the dropdown's scroll). */
  fetchNextPage: () => void;
}

/**
 * Paginated account search for the cases "Account" advanced filter, backed
 * by the same `POST /accounts/search` endpoint {@link useSearchAccounts}
 * already calls for the Accounts list page. Loads the first
 * {@link ACCOUNT_PAGE_SIZE} accounts as soon as it is enabled (the dropdown
 * opens) — no typing required — and pages through the rest on demand via
 * {@link InfiniteAccountSearch.fetchNextPage} (wired to the listbox scroll).
 * Mirrors {@link useInfiniteProjectSearch} in `useProjectSearch.ts`, the
 * reference shape for this "type-to-search, scroll-to-page" picker pattern.
 */
export function useInfiniteAccountSearch(
  query: string,
  enabled: boolean,
): InfiniteAccountSearch {
  const authFetch = useAuthApiClient();
  const q = query.trim();

  const result = useInfiniteQuery<SearchAccountsResponse, Error>({
    queryKey: ["csm-accounts-search-paged", q],
    queryFn: async ({ pageParam }) => {
      const request: SearchAccountsRequest = {
        pagination: { offset: pageParam as number, limit: ACCOUNT_PAGE_SIZE },
        ...(q.length > 0 ? { filters: { searchQuery: q } } : {}),
      };
      const res = await authFetch(`${apiConfig.backendUrl}/accounts/search`, {
        method: "POST",
        body: JSON.stringify(request),
      });
      if (!res.ok) {
        const body = await res.text();
        throw new ApiError(
          res.status,
          res.statusText,
          parseApiResponseMessage(body, res.status, res.statusText),
        );
      }
      return (await res.json()) as SearchAccountsResponse;
    },
    initialPageParam: 0,
    getNextPageParam: (lastPage, allPages) => {
      if (!lastPage.hasMore) return undefined;
      // Stop if the last page came back empty: the offset wouldn't advance, so
      // scroll-fetching would otherwise loop on the same page forever.
      if ((lastPage.accounts?.length ?? 0) === 0) return undefined;
      // Next offset = rows already loaded; robust even if the backend does not
      // echo back the offset we sent.
      return allPages.reduce((n, p) => n + (p.accounts?.length ?? 0), 0);
    },
    enabled,
    // Keep prior pages on screen while a new query's first page loads, so the
    // dropdown doesn't flash empty between searches.
    placeholderData: keepPreviousData,
    staleTime: 60_000,
  });

  const accounts = useMemo(
    () => (result.data?.pages ?? []).flatMap((p) => p.accounts ?? []),
    [result.data],
  );

  return {
    accounts,
    isFetching: result.isFetching,
    isFetchingNextPage: result.isFetchingNextPage,
    hasNextPage: result.hasNextPage,
    isError: result.isError,
    fetchNextPage: () => {
      void result.fetchNextPage();
    },
  };
}
