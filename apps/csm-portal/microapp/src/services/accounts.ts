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

// Read-only Accounts browsing (Customers > Accounts). Paged via infinite
// scroll, mirroring services/announcements.ts and services/engagements.ts —
// the webapp's own CsmAccountsPage uses classic offset pagination, but every
// other list page in this app is infinite-scrolled, so this follows the
// mobile convention instead of the desktop one.

import { infiniteQueryOptions, queryOptions } from "@tanstack/react-query";
import { ACCOUNTS_SEARCH_ENDPOINT, ACCOUNT_ENDPOINT } from "@config/endpoints";
import type { AccountDto, AccountSearchPayloadDto, AccountSearchResponseDto } from "@src/types";
import { toAccount, type Account } from "@src/types";
import apiClient from "./apiClient";

export interface AccountSearchResult {
  items: Account[];
  total: number;
  offset: number;
  limit: number;
  hasMore: boolean;
}

const ACCOUNTS_PAGE_LIMIT = 20;

async function searchAccounts(searchQuery: string, offset: number): Promise<AccountSearchResult> {
  const q = searchQuery.trim();
  const payload: AccountSearchPayloadDto = {
    pagination: { offset, limit: ACCOUNTS_PAGE_LIMIT },
    ...(q ? { searchQuery: q } : {}),
  };
  const { data } = await apiClient.post<AccountSearchResponseDto>(ACCOUNTS_SEARCH_ENDPOINT, payload);
  const items = data.accounts.map(toAccount);
  return {
    items,
    total: data.total,
    offset: data.offset,
    limit: data.limit,
    // Some data sources omit hasMore from the search envelope (see cases.ts's getAllCases and
    // adminUsers.ts's searchUsers); derive it from offset/total when that happens, and never
    // report hasMore on an empty page (see services/engagements.ts's own fix for this).
    hasMore: items.length > 0 && (data.hasMore ?? data.offset + items.length < data.total),
  };
}

const getAccount = async (id: string): Promise<Account> => {
  const { data } = await apiClient.get<AccountDto>(ACCOUNT_ENDPOINT(id));
  return toAccount(data);
};

export const accounts = {
  infinite: (searchQuery: string) =>
    infiniteQueryOptions({
      queryKey: ["accounts", "infinite", searchQuery],
      queryFn: ({ pageParam }) => searchAccounts(searchQuery, pageParam),
      initialPageParam: 0,
      getNextPageParam: (last) => (last.hasMore ? last.offset + last.limit : undefined),
    }),

  get: (id: string) =>
    queryOptions({
      queryKey: ["account", id],
      queryFn: () => getAccount(id),
    }),
};
