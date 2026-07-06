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
import type {
  BeProductVersion,
  BeProductVersionSearchPayload,
  BeProductVersionSearchResponse,
} from "@api/backend/types";

const PAGE_LIMIT = BE_MAX_PAGE_LIMIT;

/**
 * Loads versions for a specific product via
 * `POST /products/{id}/versions/search`. Disabled until `productId` is
 * provided so the query is naturally lazy (fires only after a product is
 * chosen in the create dialog).
 */
export function useSearchProductVersions(
  productId: string | undefined,
): UseQueryResult<BeProductVersion[], Error> {
  const api = useBackendApi();

  return useQuery<BeProductVersion[], Error>({
    queryKey: [ApiQueryKeys.PRODUCT_VERSIONS_SEARCH, productId ?? ""],
    queryFn: async (): Promise<BeProductVersion[]> => {
      const all: BeProductVersion[] = [];
      for (let offset = 0; ; offset += PAGE_LIMIT) {
        const res = await api.post<BeProductVersionSearchPayload, BeProductVersionSearchResponse>(
          `/products/${encodeURIComponent(productId as string)}/versions/search`,
          { pagination: { offset, limit: PAGE_LIMIT } },
        );
        const page = res.productVersions ?? [];
        all.push(...page);
        if (page.length < PAGE_LIMIT) break;
      }
      return all;
    },
    enabled: !!productId,
    staleTime: 5 * 60_000,
  });
}
