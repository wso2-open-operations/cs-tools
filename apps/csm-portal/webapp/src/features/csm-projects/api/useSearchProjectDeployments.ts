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
import { isMockMode, useBackendApi } from "@api/backend/client";
import type {
  BeDeployedProductSearchPayload,
  BeDeployedProductSearchResponse,
  BeDeploymentSearchPayload,
  BeDeploymentSearchResponse,
  BeProductSearchPayload,
  BeProductSearchResponse,
  BeProductVersionSearchPayload,
  BeProductVersionSearchResponse,
} from "@api/backend/types";
import { getMockDeploymentsForProject } from "@features/csm-projects/api/mocks/deploymentsMocks";
import type {
  CsmDeploymentEnvironment,
  CsmDeploymentRow,
} from "@features/csm-projects/types/csmProjects";

const DEPLOYMENTS_PAGE_LIMIT = 100;
const PRODUCTS_PAGE_LIMIT = 100;
const VERSIONS_PAGE_LIMIT = 100;

function uiEnvironmentFromBe(
  type: string | undefined,
): CsmDeploymentEnvironment {
  switch (type) {
    case "primary_production":
      return "prod";
    case "staging":
    case "qa":
    case "stress":
    case "uat":
    case "development":
      return type === "development" ? "dev" : (type as CsmDeploymentEnvironment);
    default:
      return "prod";
  }
}

/**
 * Composite hook that returns the deployments belonging to a project in the
 * UI's `CsmDeploymentRow` shape. In LIVE mode it fans out across the
 * backend's split deployment / product / version endpoints to assemble the
 * row; in MOCK mode it falls back to the seeded data in
 * `deploymentsMocks.ts` so the UI keeps rendering without a backend.
 *
 * Fan-out plan (LIVE):
 *   1. `POST /projects/{id}/deployments/search` → deployments
 *   2. `POST /products/search` (limit 100) → productId → product name map
 *   3. For each deployment: `POST /deployments/{id}/products/search`
 *      → deployed products carrying productId + productVersionId
 *   4. `POST /products/{productId}/versions/search` per unique productId
 *      → versionId → version label map
 *   5. Merge maps onto each `BeDeployedProduct` to build UI products.
 */
export function useGetProjectDeploymentsResolved(
  projectId: string | undefined,
): UseQueryResult<CsmDeploymentRow[], Error> {
  const api = useBackendApi();

  return useQuery<CsmDeploymentRow[], Error>({
    queryKey: ["csm.projectDeployments.resolved", projectId ?? ""],
    queryFn: async (): Promise<CsmDeploymentRow[]> => {
      if (!projectId) return [];
      if (isMockMode()) {
        return getMockDeploymentsForProject(projectId);
      }

      // 1. Deployments under the project.
      const depResponse = await api.post<
        BeDeploymentSearchPayload,
        BeDeploymentSearchResponse
      >(`/projects/${encodeURIComponent(projectId)}/deployments/search`, {
        pagination: { offset: 0, limit: DEPLOYMENTS_PAGE_LIMIT },
      });
      const deployments = depResponse.deployments ?? [];
      if (deployments.length === 0) return [];

      // 2. Catalog: product name lookup. The BE doesn't have a get-by-id, so
      // we load the first page (limit 100) which is sufficient for current
      // catalog sizes and cache the map.
      const productsResponse = await api.post<
        BeProductSearchPayload,
        BeProductSearchResponse
      >("/products/search", {
        pagination: { offset: 0, limit: PRODUCTS_PAGE_LIMIT },
      });
      const productNameById = new Map<string, string>(
        (productsResponse.products ?? []).map((p) => [p.id, p.name ?? p.id]),
      );

      // 3. Deployed products per deployment, in parallel.
      const perDeployment = await Promise.all(
        deployments.map((d) =>
          api.post<
            BeDeployedProductSearchPayload,
            BeDeployedProductSearchResponse
          >(`/deployments/${encodeURIComponent(d.id)}/products/search`, {
            pagination: { offset: 0, limit: PRODUCTS_PAGE_LIMIT },
          }),
        ),
      );

      // 4. Resolve version labels for every product seen.
      const uniqueProductIds = Array.from(
        new Set(
          perDeployment.flatMap((r) =>
            (r.deployedProducts ?? [])
              .map((dp) => dp.productId)
              .filter((id): id is string => !!id),
          ),
        ),
      );
      const versionEntries = await Promise.all(
        uniqueProductIds.map(async (productId) => {
          const vr = await api.post<
            BeProductVersionSearchPayload,
            BeProductVersionSearchResponse
          >(`/products/${encodeURIComponent(productId)}/versions/search`, {
            pagination: { offset: 0, limit: VERSIONS_PAGE_LIMIT },
          });
          return (vr.productVersions ?? []).map(
            (v) =>
              [
                v.id,
                {
                  version: v.version ?? v.id,
                  supportStatus: v.currentSupportStatus ?? "available",
                },
              ] as const,
          );
        }),
      );
      const versionById = new Map(versionEntries.flat());

      // 5. Build UI rows.
      const rows: CsmDeploymentRow[] = deployments.map((d, i) => {
        const deployedProducts = perDeployment[i]?.deployedProducts ?? [];
        return {
          id: d.id,
          projectId: d.projectId ?? projectId,
          name: d.name ?? d.id,
          environment: uiEnvironmentFromBe(d.type),
          region: "—",
          products: deployedProducts.map((dp) => {
            const versionInfo = dp.productVersionId
              ? versionById.get(dp.productVersionId)
              : undefined;
            return {
              product:
                (dp.productId && productNameById.get(dp.productId)) ?? "—",
              version: versionInfo?.version ?? "—",
              updateLevel: "—",
              supportStatus: versionInfo?.supportStatus ?? "available",
            };
          }),
          lastUpdatedAt: d.updatedAt ?? d.createdAt ?? "",
        };
      });

      return rows;
    },
    enabled: !!projectId,
    staleTime: 30_000,
  });
}
