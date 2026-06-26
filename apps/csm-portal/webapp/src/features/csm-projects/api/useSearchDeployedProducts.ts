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
  BeDeployedProduct,
  BeDeployedProductSearchPayload,
  BeDeployedProductSearchResponse,
} from "@api/backend/types";

const PAGE_LIMIT = BE_MAX_PAGE_LIMIT;

/**
 * Full deployed-product records for a deployment, via
 * `POST /deployments/{id}/products/search`. Each record embeds the resolved
 * `product` and `version`, plus the SN-only `cores`/`tps`/`category`, so the
 * details panel can render them without secondary lookups.
 *
 * Distinct from {@link useDeployedProductOptions}, which keys the same endpoint
 * but returns a label-only shape for the case-create selector — hence the
 * `"records"` query-key suffix so the two caches don't collide. The query is
 * disabled until a deployment id is provided (the panel only mounts when its
 * row is expanded, so the request is naturally lazy).
 */
export function useSearchDeployedProducts(
  deploymentId: string | undefined,
): UseQueryResult<BeDeployedProduct[], Error> {
  const api = useBackendApi();

  return useQuery<BeDeployedProduct[], Error>({
    queryKey: [ApiQueryKeys.DEPLOYMENT_PRODUCTS, deploymentId ?? "", "records"],
    queryFn: async (): Promise<BeDeployedProduct[]> => {
      const all: BeDeployedProduct[] = [];
      for (let offset = 0; ; offset += PAGE_LIMIT) {
        const res = await api.post<
          BeDeployedProductSearchPayload,
          BeDeployedProductSearchResponse
        >(
          `/deployments/${encodeURIComponent(deploymentId as string)}/products/search`,
          { pagination: { offset, limit: PAGE_LIMIT } },
        );
        const page = res.deployedProducts ?? [];
        all.push(...page);
        if (page.length < PAGE_LIMIT) break;
      }
      return all;
    },
    enabled: !!deploymentId,
    staleTime: 60_000,
  });
}
