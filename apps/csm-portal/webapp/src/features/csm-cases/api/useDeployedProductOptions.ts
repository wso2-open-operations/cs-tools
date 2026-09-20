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
  BeDeployedProductSearchPayload,
  BeDeployedProductSearchResponse,
} from "@api/backend/types";

const PAGE_LIMIT = BE_MAX_PAGE_LIMIT;

export interface DeployedProductOption {
  /** Deployed-product id — the value the case-create payload needs. */
  id: string;
  /** Human label: "{product name} {version}". */
  label: string;
  /**
   * Opaque category code carried straight through from `BeDeployedProduct.category`
   * (e.g. "ms", "pc"), or `null`/`undefined` when the record has none. Used by
   * callers (e.g. the service-request create page) that need to narrow the
   * option list to a project's `srProductCategories`.
   */
  category?: string | null;
}

/**
 * Selectable deployed products for a deployment. The deployed-product records
 * (`POST /deployments/{id}/products/search`) embed the resolved `product` and
 * `version` objects, so the readable label is built straight from those — no
 * secondary product/version lookups needed. Disabled until a deployment id is
 * provided.
 *
 * `productCategories`, when given a non-empty list, is forwarded to the
 * search so the entity service itself narrows results to those categories
 * (e.g. a cloud/PDP project's service-request picker) — omit it for
 * unfiltered results, identical to today's behaviour.
 */
export function useDeployedProductOptions(
  deploymentId: string | undefined,
  productCategories?: string[],
): UseQueryResult<DeployedProductOption[], Error> {
  const api = useBackendApi();
  // Normalized to a stable, comparable key: an empty/undefined list must
  // produce the same query key as "no filter" so callers that pass `[]`
  // before metadata has loaded don't churn the cache once the real list
  // arrives, and so the key stays a plain string join rather than an
  // array identity that changes every render.
  const categoriesKey =
    productCategories && productCategories.length > 0
      ? [...productCategories].sort().join(",")
      : "";

  return useQuery<DeployedProductOption[], Error>({
    queryKey: [ApiQueryKeys.DEPLOYMENT_PRODUCTS, deploymentId ?? "", categoriesKey],
    queryFn: async (): Promise<DeployedProductOption[]> => {
      const dpRes = await api.post<
        BeDeployedProductSearchPayload,
        BeDeployedProductSearchResponse
      >(`/deployments/${encodeURIComponent(deploymentId as string)}/products/search`, {
        pagination: { offset: 0, limit: PAGE_LIMIT },
        ...(productCategories && productCategories.length > 0
          ? { productCategories }
          : {}),
      });
      const deployed = dpRes.deployedProducts ?? [];

      return deployed.map((d) => {
        // Fall back to the product id (not a generic "Product") so distinct
        // products with missing names stay distinguishable in the selector.
        const name = d.product?.name || d.product?.id || "Product";
        const ver = d.version?.name ?? "";
        return {
          id: d.id,
          label: ver ? `${name} ${ver}` : name,
          category: d.category,
        };
      });
    },
    enabled: !!deploymentId,
    staleTime: 60_000,
  });
}
