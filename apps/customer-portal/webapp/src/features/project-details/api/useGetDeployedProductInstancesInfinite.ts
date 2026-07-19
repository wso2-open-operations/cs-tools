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
  useInfiniteQuery,
  type InfiniteData,
  type UseInfiniteQueryResult,
} from "@tanstack/react-query";
import { useAsgardeo } from "@asgardeo/react";
import { useAuthApiClient } from "@/hooks/useAuthApiClient";
import { ApiQueryKeys } from "@constants/apiConstants";
import type {
  DateRangeFilter,
  InstancesResponse,
} from "@features/project-details/types/usage";

const PAGE_SIZE = 10;

/**
 * Infinite query for a deployed product's instances, loading 10 at a time.
 * Call `fetchNextPage()` (e.g. via an IntersectionObserver sentinel) to load more.
 */
export default function useGetDeployedProductInstancesInfinite(
  deployedProductId: string | undefined,
  dateRange: DateRangeFilter,
): UseInfiniteQueryResult<InfiniteData<InstancesResponse>, Error> {
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();

  return useInfiniteQuery<InstancesResponse, Error, InfiniteData<InstancesResponse>>({
    queryKey: [
      ApiQueryKeys.DEPLOYED_PRODUCT_INSTANCES_SEARCH,
      deployedProductId,
      dateRange,
      "infinite",
    ],
    initialPageParam: 0,
    getNextPageParam: (lastPage) => {
      const nextOffset = lastPage.offset + lastPage.limit;
      return nextOffset < lastPage.totalRecords ? nextOffset : undefined;
    },
    queryFn: async ({ pageParam }): Promise<InstancesResponse> => {
      const offset = pageParam as number;
      const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL ?? "";
      const response = await authFetch(
        `${baseUrl}/deployments/products/${deployedProductId}/instances/search`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            filters: dateRange,
            pagination: { offset, limit: PAGE_SIZE },
          }),
        },
      );
      if (!response.ok) {
        throw new Error(
          `Failed to search deployed product instances: ${response.status}`,
        );
      }
      return response.json() as Promise<InstancesResponse>;
    },
    enabled: !!deployedProductId && isSignedIn && !isAuthLoading,
    staleTime: Infinity,
    refetchOnWindowFocus: false,
    refetchOnReconnect: false,
  });
}
