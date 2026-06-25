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
  BeCatalogRef,
  BeSearchCatalogsPayload,
  BeSearchCatalogsResponse,
} from "@api/backend/types";

const PAGE_LIMIT = BE_MAX_PAGE_LIMIT;

/**
 * Service catalogs available for a deployed product, via `POST /catalogs/search`.
 * Catalogs (and the catalog items they embed) only exist for ServiceNow-sourced
 * deployed products; managed-cloud products resolve to an empty list. Pages
 * through so catalogs beyond the first page stay selectable. Disabled until a
 * deployed-product id is provided.
 */
export function useSearchCatalogs(
  deployedProductId: string | undefined,
): UseQueryResult<BeCatalogRef[], Error> {
  const api = useBackendApi();

  return useQuery<BeCatalogRef[], Error>({
    queryKey: [ApiQueryKeys.CATALOGS_SEARCH, deployedProductId ?? ""],
    queryFn: async (): Promise<BeCatalogRef[]> => {
      const all: BeCatalogRef[] = [];
      for (let offset = 0; ; offset += PAGE_LIMIT) {
        const res = await api.post<
          BeSearchCatalogsPayload,
          BeSearchCatalogsResponse
        >("/catalogs/search", {
          deployedProductId: deployedProductId as string,
          pagination: { offset, limit: PAGE_LIMIT },
        });
        const page = res.catalogs ?? [];
        all.push(...page);
        if (page.length < PAGE_LIMIT) break;
      }
      return all;
    },
    enabled: !!deployedProductId,
    staleTime: 60_000,
  });
}
