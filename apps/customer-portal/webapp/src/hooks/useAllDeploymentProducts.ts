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

import { useMemo } from "react";
import { useQueries } from "@tanstack/react-query";
import { useAsgardeo } from "@asgardeo/react";
import { ApiQueryKeys } from "@constants/apiConstants";
import {
  createFetchWithMergedAuthHeaders,
  fetchDeploymentProductsAll,
} from "@api/usePostDeploymentProductsSearch";
import type { DeploymentProductItem } from "@models/responses";

interface DeploymentForProducts {
  id: string;
  name?: string;
  type?: { id?: string; label?: string };
}

/**
 * Fetches products for all deployments and returns a map of deploymentId -> products.
 *
 * @param {DeploymentForProducts[] | undefined} projectDeployments - Project deployments.
 * @returns {{ productsByDeploymentId: Record<string, DeploymentProductItem[]>; isLoading: boolean }} Map and loading state.
 */
export function useAllDeploymentProducts(
  projectDeployments: DeploymentForProducts[] | undefined,
): {
  productsByDeploymentId: Record<string, DeploymentProductItem[]>;
  isLoading: boolean;
} {
  const { getIdToken } = useAsgardeo();
  const deploymentList = Array.isArray(projectDeployments)
    ? projectDeployments
    : [];
  const deploymentIds = deploymentList.map((d) => d.id).filter(Boolean);

  const results = useQueries({
    queries: deploymentIds.map((deploymentId) => ({
      queryKey: [ApiQueryKeys.DEPLOYMENT_PRODUCTS, deploymentId, "search-all"],
      queryFn: async () => {
        const token = await getIdToken();
        if (!token) {
          throw new Error("Unable to retrieve ID token");
        }
        return fetchDeploymentProductsAll({
          deploymentId,
          pageSize: 10,
          fetchFn: createFetchWithMergedAuthHeaders(token),
        });
      },
      enabled: !!deploymentId,
      staleTime: 5 * 60 * 1000,
    })),
  });

  const productsByDeploymentId = useMemo(() => {
    const map: Record<string, DeploymentProductItem[]> = {};
    deploymentIds.forEach((id, i) => {
      map[id] = (results[i]?.data as DeploymentProductItem[] | undefined) ?? [];
    });
    return map;
  }, [results, deploymentIds]);

  const isLoading = results.some((r) => r.isLoading);

  return { productsByDeploymentId, isLoading };
}
