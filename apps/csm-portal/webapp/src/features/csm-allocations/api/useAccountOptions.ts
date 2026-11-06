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

import { useInfiniteAccountSearch } from "@features/csm-accounts/api/useSearchAccounts";
import type { Account } from "@features/csm-accounts/types/csmAccounts";

/**
 * Type-ahead account search in the `useSearch` shape `AsyncEntitySelect`
 * expects, backed by the portal's existing account search.
 */
export function useAccountOptions(
  query: string,
  enabled: boolean,
): { data: Account[] | undefined; isFetching: boolean; isError: boolean } {
  const { accounts, isFetching, isError } = useInfiniteAccountSearch(query, enabled);
  return { data: accounts, isFetching, isError };
}
