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
  useMutation,
  useQueryClient,
  type UseMutationResult,
} from "@tanstack/react-query";
import { ApiQueryKeys } from "@constants/apiConstants";
import { useBackendApi } from "@api/backend/client";
import type { Account } from "@features/csm-accounts/types/csmAccounts";

/**
 * `PATCH /accounts/{id}` request body. Both fields are independently
 * optional, but at least one must be provided. Omitting a field (or sending
 * `null`) leaves that team's assignment unchanged — the backend has no way
 * to clear a team back to "no team" yet, so a caller must never send `null`
 * expecting it to clear anything. The two teams are unrelated account
 * attributes and must stay independently editable.
 */
export interface AccountTeamsPatch {
  creTeamId?: string | null;
  sreTeamId?: string | null;
}

/**
 * Update an account's CRE/SRE team assignment via `PATCH /accounts/{id}`.
 * Mirrors `usePatchCsmCase`'s shape (bound to a single id up front, throws
 * if the id is missing at call time) and its cache-invalidation approach: on
 * success, invalidate this account's detail query so it refetches the
 * authoritative state.
 */
export function usePatchAccountTeams(
  accountId: string | undefined,
): UseMutationResult<Account, Error, AccountTeamsPatch> {
  const api = useBackendApi();
  const queryClient = useQueryClient();

  return useMutation<Account, Error, AccountTeamsPatch>({
    mutationFn: async (patch): Promise<Account> => {
      if (!accountId) {
        throw new Error("Cannot update an account without an id.");
      }
      return api.patch<AccountTeamsPatch, Account>(
        `/accounts/${encodeURIComponent(accountId)}`,
        patch,
      );
    },
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.CSM_ACCOUNT_DETAIL, accountId ?? ""],
      });
      // The accounts list page (`useSearchAccounts.ts`) keys its cache with
      // a raw literal ("csm-accounts-search"), not `ApiQueryKeys.CSM_ACCOUNTS`
      // — match that existing key here too so the list's CRE/SRE team column
      // also refreshes after an edit instead of only the detail page.
      queryClient.invalidateQueries({
        predicate: (q) => q.queryKey[0] === "csm-accounts-search",
      });
    },
  });
}
