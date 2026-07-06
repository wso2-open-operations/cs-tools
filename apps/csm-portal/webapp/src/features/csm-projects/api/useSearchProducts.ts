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
import { ApiQueryKeys, BE_MAX_PAGE_LIMIT } from "@constants/apiConstants";
import { useBackendApi } from "@api/backend/client";
import type { BeProduct, BeProductSearchPayload, BeProductSearchResponse } from "@api/backend/types";

const PAGE_LIMIT = BE_MAX_PAGE_LIMIT;

/**
 * Loads all products via `POST /products/search`. Used as the product picker
 * in {@link CreateDeployedProductDialog}. The full list is fetched once (paged)
 * and then filtered locally in the Select component, which is acceptable because
 * the product catalogue is a bounded, small set.
 */
export function useSearchProducts(): UseQueryResult<BeProduct[], Error> {
  const api = useBackendApi();

  return useQuery<BeProduct[], Error>({
    queryKey: [ApiQueryKeys.PRODUCTS],
    queryFn: async (): Promise<BeProduct[]> => {
      const all: BeProduct[] = [];
      for (let offset = 0; ; offset += PAGE_LIMIT) {
        const res = await api.post<BeProductSearchPayload, BeProductSearchResponse>(
          "/products/search",
          { pagination: { offset, limit: PAGE_LIMIT } },
        );
        const page = res.products ?? [];
        all.push(...page);
        if (page.length < PAGE_LIMIT) break;
      }
      return all;
    },
    staleTime: 5 * 60_000,
  });
}
