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
  useQuery,
  type UseQueryResult,
} from "@tanstack/react-query";
import { ApiQueryKeys } from "@constants/apiConstants";
import { useBackendApi } from "@api/backend/client";
import type {
  ProductIssueDashboard,
  ProductIssueDashboardParams,
} from "@features/csm-dashboard/types/productIssueDashboard";

/** The request path: the filter lists are omitted when empty. */
export function productIssueDashboardPath(params: ProductIssueDashboardParams): string {
  const query = new URLSearchParams({ from: params.from, to: params.to, bucket: params.bucket });
  if (params.products.length > 0) query.set("products", params.products.join(","));
  return `/product-work-items/dashboard?${query.toString()}`;
}

/**
 * The Product issues lead dashboard, via `GET /product-work-items/dashboard`.
 * `null` means the route is not there (404, git internals off). A 403 (no
 * `cs_lead`/`admin` role) rejects with the `BackendApiError`; the page shows
 * the standard 403 page for it.
 */
export function useProductIssueDashboard(
  params: ProductIssueDashboardParams,
): UseQueryResult<ProductIssueDashboard | null, Error> {
  const api = useBackendApi();
  const path = productIssueDashboardPath(params);

  return useQuery<ProductIssueDashboard | null, Error>({
    queryKey: [
      ApiQueryKeys.CSM_PRODUCT_ISSUE_DASHBOARD,
      params.from,
      params.to,
      params.bucket,
      params.products,
    ],
    queryFn: () => api.get<ProductIssueDashboard>(path),
    staleTime: 60_000,
  });
}
