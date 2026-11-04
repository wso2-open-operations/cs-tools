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
import type { CatalogProduct } from "@features/csm-products/types/productCatalog";

/** Every product with its catalogue fields and version counts, ordered by name. */
export function useGetCatalogProducts(): UseQueryResult<CatalogProduct[], Error> {
  const api = useBackendApi();
  return useQuery<CatalogProduct[], Error>({
    queryKey: [ApiQueryKeys.PRODUCT_CATALOG, ApiQueryKeys.PRODUCT_CATALOG_PRODUCTS],
    queryFn: async () => {
      const products = await fetchAllPages<CatalogProduct>(api, "/products/search", {}, "products", 500);
      return products.sort((a, b) => a.name.trim().localeCompare(b.name.trim()) || a.id.localeCompare(b.id));
    },
    staleTime: 60_000,
  });
}
