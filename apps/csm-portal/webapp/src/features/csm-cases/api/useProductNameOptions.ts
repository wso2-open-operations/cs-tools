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
import type { BeProductSearchPayload, BeProductSearchResponse } from "@api/backend/types";

const PAGE_LIMIT = BE_MAX_PAGE_LIMIT;

/**
 * Distinct product **family names** for the cases product filter.
 *
 * `POST /products/search` returns one row per product model/version (hundreds
 * of rows, the same short `name` repeated many times, e.g. dozens of "API
 * Manager" variants). The case product filter matches on the family name
 * (`filters.productNames` → ServiceNow `product.name IN ...`, all versions), so
 * this hook fetches the full (bounded) catalogue once and reduces it to the set
 * of distinct, non-empty names, sorted for a stable dropdown order.
 */
export function useProductNameOptions(): UseQueryResult<string[], Error> {
  const api = useBackendApi();

  return useQuery<string[], Error>({
    queryKey: [ApiQueryKeys.PRODUCTS, "family-names"],
    queryFn: async (): Promise<string[]> => {
      const names = new Set<string>();
      for (let offset = 0; ; offset += PAGE_LIMIT) {
        const res = await api.post<BeProductSearchPayload, BeProductSearchResponse>(
          "/products/search",
          { pagination: { offset, limit: PAGE_LIMIT } },
        );
        const page = res.products ?? [];
        for (const p of page) {
          const name = p.name?.trim();
          if (name) names.add(name);
        }
        if (page.length < PAGE_LIMIT) break;
      }
      return [...names].sort((a, b) => a.localeCompare(b));
    },
    staleTime: 5 * 60_000,
  });
}
