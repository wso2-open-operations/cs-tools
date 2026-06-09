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
  BeDeployedProductSearchPayload,
  BeDeployedProductSearchResponse,
  BeProductSearchPayload,
  BeProductSearchResponse,
  BeProductVersionSearchPayload,
  BeProductVersionSearchResponse,
} from "@api/backend/types";

const PAGE_LIMIT = 100;
const PRODUCTS_LIMIT = 100; // backend caps pagination limit at 100

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

      const [productsRes, versionLists] = await Promise.all([
        api.post<BeProductSearchPayload, BeProductSearchResponse>(
          "/products/search",
          { pagination: { offset: 0, limit: PRODUCTS_LIMIT } },
        ),
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

      const productName = new Map(
        (productsRes.products ?? []).map((p) => [p.id, p.name ?? p.id]),
      );
      const versionLabel = new Map<string, string>();
      for (const versions of versionLists) {
        for (const v of versions) versionLabel.set(v.id, v.version ?? "");
      }

      return deployed.map((d) => {
        const name = (d.productId && productName.get(d.productId)) || "Product";
        const ver =
          (d.productVersionId && versionLabel.get(d.productVersionId)) || "";
        return { id: d.id, label: ver ? `${name} ${ver}` : name };
      });
    },
    enabled: !!deploymentId,
    staleTime: 60_000,
  });
}
