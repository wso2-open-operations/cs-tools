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

import { useQuery, type UseQueryResult } from "@tanstack/react-query";
import { ApiQueryKeys } from "@constants/apiConstants";
import { useBackendApi } from "@api/backend/client";
import type {
  ProductIssueSearchRequest,
  ProductIssueSearchResponse,
  ProductIssueSummary,
} from "@features/csm-cases/types/productWorkItems";

export const MY_WOC_PAGE_SIZE = 50;

/** Search body of the "My WOC Git issues" view: waiting on CS, opened by the
 * signed-in user or on a case assigned to them, oldest pause first. */
export const MY_WOC_SEARCH_BODY: ProductIssueSearchRequest = {
  filters: { boardStatuses: ["WAITING_ON_CS"], mine: true, sort: "pausedSince" },
  pagination: { limit: MY_WOC_PAGE_SIZE, offset: 0 },
};

export interface MyWocProductIssues {
  items: ProductIssueSummary[];
  total: number;
}

function isUnavailable(err: unknown): boolean {
  const status = (err as { status?: number } | null)?.status;
  return status === 404 || status === 403;
}

/**
 * The signed-in user's product issues that wait on CS, via
 * `POST /product-work-items/search`. `null` means the routes are not there
 * (404, `GIT_INTERNALS_ENABLED` off) or the caller is not internal (403): the
 * panel renders nothing. `[]` is a real empty answer.
 */
export function useMyWocProductIssues(): UseQueryResult<MyWocProductIssues | null, Error> {
  const api = useBackendApi();

  return useQuery<MyWocProductIssues | null, Error>({
    queryKey: [ApiQueryKeys.CSM_MY_WOC_PRODUCT_ISSUES],
    queryFn: async (): Promise<MyWocProductIssues | null> => {
      try {
        const res = await api.post<ProductIssueSearchRequest, ProductIssueSearchResponse>(
          "/product-work-items/search",
          MY_WOC_SEARCH_BODY,
        );
        const items = res?.items ?? [];
        return { items, total: res?.total ?? items.length };
      } catch (err) {
        if (isUnavailable(err)) return null;
        throw err;
      }
    },
    staleTime: 30_000,
  });
}
