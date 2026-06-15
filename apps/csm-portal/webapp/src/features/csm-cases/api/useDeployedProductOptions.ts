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
  BeProductSearchPayload,
  BeProductSearchResponse,
  BeProductVersionSearchPayload,
  BeProductVersionSearchResponse,
} from "@api/backend/types";

const PAGE_LIMIT = BE_MAX_PAGE_LIMIT;
const PRODUCTS_LIMIT = BE_MAX_PAGE_LIMIT;
// Bound the catalog scan: if referenced product ids can't be resolved (deleted
// or bad data), don't page through an unbounded catalog. ~2000 products covered.
// Doubled from 20 when the page limit halved to BE_MAX_PAGE_LIMIT, so the total
// scan ceiling (pages * limit) is unchanged.
const MAX_CATALOG_PAGES = 40;

export interface DeployedProductOption {
  /** Deployed-product id — the value the case-create payload needs. */
  id: string;
  /** Human label: "{product name} {version}". */
  label: string;
}

/**
 * Selectable deployed products for a deployment. The deployed-product records
 * (`POST /deployments/{id}/products/search`) carry only `productId` /
 * `productVersionId`, so this resolves readable labels via `POST
 * /products/search` (id → name) and `POST /products/{id}/versions/search`
 * (id → version). Disabled until a deployment id is provided.
 */
export function useDeployedProductOptions(
  deploymentId: string | undefined,
): UseQueryResult<DeployedProductOption[], Error> {
  const api = useBackendApi();

  return useQuery<DeployedProductOption[], Error>({
    queryKey: [ApiQueryKeys.DEPLOYMENT_PRODUCTS, deploymentId ?? ""],
    queryFn: async (): Promise<DeployedProductOption[]> => {
      const dpRes = await api.post<
        BeDeployedProductSearchPayload,
        BeDeployedProductSearchResponse
      >(`/deployments/${encodeURIComponent(deploymentId as string)}/products/search`, {
        pagination: { offset: 0, limit: PAGE_LIMIT },
      });
      const deployed = dpRes.deployedProducts ?? [];
      if (deployed.length === 0) return [];

      const productIds = [
        ...new Set(
          deployed.map((d) => d.productId).filter((v): v is string => !!v),
        ),
      ];

      // `/products/search` has no id filter, so page through it collecting
      // names for the referenced product ids only — stop once every id is
      // resolved or the catalog is exhausted. Without this, products beyond
      // the first page collapse to a generic fallback label.
      const productName = new Map<string, string>();
      const unresolved = new Set(productIds);
      const resolveProductNames = async (): Promise<void> => {
        let offset = 0;
        for (let page = 0; unresolved.size > 0 && page < MAX_CATALOG_PAGES; page += 1) {
          const res = await api.post<
            BeProductSearchPayload,
            BeProductSearchResponse
          >("/products/search", {
            pagination: { offset, limit: PRODUCTS_LIMIT },
          });
          const products = res.products ?? [];
          for (const p of products) {
            if (unresolved.has(p.id)) {
              productName.set(p.id, p.name ?? p.id);
              unresolved.delete(p.id);
            }
          }
          if (products.length < PRODUCTS_LIMIT) break;
          offset += PRODUCTS_LIMIT;
        }
        // Any still-unresolved ids fall back to the id as their label below.
      };

      const [, versionLists] = await Promise.all([
        resolveProductNames(),
        Promise.all(
          productIds.map((pid) =>
            api
              .post<
                BeProductVersionSearchPayload,
                BeProductVersionSearchResponse
              >(`/products/${encodeURIComponent(pid)}/versions/search`, {
                pagination: { offset: 0, limit: PAGE_LIMIT },
              })
              .then((r) => r.productVersions ?? [])
              .catch(() => []),
          ),
        ),
      ]);

      const versionLabel = new Map<string, string>();
      for (const versions of versionLists) {
        for (const v of versions) versionLabel.set(v.id, v.version ?? "");
      }

      return deployed.map((d) => {
        // Fall back to the product id (not a generic "Product") so distinct
        // unresolved products stay distinguishable in the selector.
        const name =
          (d.productId && productName.get(d.productId)) ||
          d.productId ||
          "Product";
        const ver =
          (d.productVersionId && versionLabel.get(d.productVersionId)) || "";
        return { id: d.id, label: ver ? `${name} ${ver}` : name };
      });
    },
    enabled: !!deploymentId,
    staleTime: 60_000,
  });
}
