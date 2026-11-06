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
import { useBackendApi } from "@api/backend/client";
import { ApiQueryKeys } from "@constants/apiConstants";
import { fetchAllPages } from "@features/csm-products/api/fetchAllPages";
import type { CatalogVersion, CatalogVersionWire } from "@features/csm-products/types/productCatalog";
import { fromWireVersion } from "@features/csm-products/utils/catalogWire";

/** Live (not retired) versions, one row per deployment profile, of one product or of all. */
function useLiveVersions(productId: string | null, enabled: boolean): UseQueryResult<CatalogVersion[], Error> {
  const api = useBackendApi();
  // "all" keeps the every-product list apart from one product's: a disabled
  // per-product query with no product must not read the full list's cache.
  const key = [ApiQueryKeys.PRODUCT_CATALOG, ApiQueryKeys.PRODUCT_CATALOG_VERSIONS, productId ?? "all"];
  return useQuery<CatalogVersion[], Error>({
    queryKey: key,
    queryFn: async () => {
      const body: Record<string, unknown> = { excludeRetired: true };
      if (productId) body.productId = productId;
      const rows = await fetchAllPages<CatalogVersionWire>(api, "/product-versions/search", body, "productVersions", 1000);
      return rows.map(fromWireVersion);
    },
    enabled,
    staleTime: 60_000,
  });
}

/** One product's live versions. `enabled` lets a caller wait for a product to be picked. */
export function useGetCatalogVersions(productId: string, enabled = true): UseQueryResult<CatalogVersion[], Error> {
  return useLiveVersions(productId, enabled && Boolean(productId));
}

/** Every product's live versions, ordered by product name, release date (newest first), version and profile. */
export function useGetAllCatalogVersions(): UseQueryResult<CatalogVersion[], Error> {
  return useLiveVersions(null, true);
}
