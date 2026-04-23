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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

import {
  type InfiniteData,
  useInfiniteQuery,
  useQuery,
  type UseInfiniteQueryResult,
  type UseQueryResult,
} from "@tanstack/react-query";
import { useAsgardeo } from "@asgardeo/react";
import { useAuthApiClient } from "@/hooks/useAuthApiClient";
import { useLogger } from "@hooks/useLogger";
import { ApiQueryKeys } from "@constants/apiConstants";
import { addApiHeaders } from "@utils/apiUtils";
import type {
  UsePostDeploymentProductsSearchAllOptions,
  UsePostDeploymentProductsSearchInfiniteOptions,
} from "@features/project-details/types/projectDetailsApi";
import type { DeployedProductSearchRequest } from "@features/project-details/types/deployments";
import type {
  DeploymentProductItem,
  DeployedProductsResponsePayload,
  DeployedProductsResponse,
} from "@features/project-details/types/deployments";
import { isDeployedProductsResponse } from "@features/project-details/utils/deployments";

const DEFAULT_PAGE_SIZE = 10;

export type FetchFn = (url: string, init?: RequestInit) => Promise<Response>;

function mergeAuthHeadersIntoFetchInit(
  idToken: string,
  initHeaders?: HeadersInit,
): Headers {
  const merged = new Headers(initHeaders ?? undefined);
  for (const [key, value] of Object.entries(addApiHeaders(idToken))) {
    merged.set(key, value as string);
  }
  return merged;
}

/**
 * Returns a fetch-compatible function that applies Bearer + x-user-id-token while
 * preserving caller headers (e.g. Content-Type for JSON POST body binding).
 * Use when `useAuthApiClient` cannot be used (e.g. token from `getIdToken()` inside useQueries).
 *
 * @param {string} idToken - Asgardeo ID token.
 * @returns {FetchFn} Wrapped global fetch.
 */
export function createFetchWithMergedAuthHeaders(idToken: string): FetchFn {
  return (url, init) =>
    fetch(url, {
      ...init,
      headers: mergeAuthHeadersIntoFetchInit(idToken, init?.headers),
    });
}

/**
 * Builds a JSON body that matches backend DeployedProductSearchPayload only
 * (pagination + optional filters.consumption). Avoids spreading unknown keys.
 */
function buildDeployedProductSearchPayload(
  request: DeployedProductSearchRequest | undefined,
  offset: number,
  limit: number,
): DeployedProductSearchRequest {
  const payload: DeployedProductSearchRequest = {
    pagination: { offset, limit },
  };
  const consumption = request?.filters?.consumption;
  const productCategories = request?.filters?.productCategories;
  const hasConsumption =
    !!consumption &&
    (consumption.include !== undefined ||
      (consumption.startDate != null && consumption.startDate !== "") ||
      (consumption.endDate != null && consumption.endDate !== ""));
  if (hasConsumption || productCategories?.length) {
    payload.filters = {
      ...(hasConsumption
        ? {
            consumption: {
              ...(consumption!.include !== undefined
                ? { include: consumption!.include }
                : {}),
              ...(consumption!.startDate
                ? { startDate: consumption!.startDate }
                : {}),
              ...(consumption!.endDate
                ? { endDate: consumption!.endDate }
                : {}),
            },
          }
        : {}),
      ...(productCategories?.length ? { productCategories } : {}),
    };
  }
  return payload;
}

async function postDeploymentProductsSearchPage(params: {
  deploymentId: string;
  request: DeployedProductSearchRequest | undefined;
  offset: number;
  limit: number;
  fetchFn: FetchFn;
  logger?: ReturnType<typeof useLogger>;
}): Promise<DeployedProductsResponsePayload> {
  const { deploymentId, request, offset, limit, fetchFn, logger } = params;

  const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;
  if (!baseUrl) {
    throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
  }

  const requestUrl = `${baseUrl}/deployments/${deploymentId}/products/search`;
  const payload = buildDeployedProductSearchPayload(request, offset, limit);

  const response = await fetchFn(requestUrl, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });

  logger?.debug?.(
    `[usePostDeploymentProductsSearch] Response status: ${response.status}`,
  );

  if (!response.ok) {
    throw new Error(
      `Error searching deployment products: ${response.statusText}`,
    );
  }

  return (await response.json()) as DeployedProductsResponsePayload;
}

function normalizeProductsPayload(
  payload: DeployedProductsResponsePayload,
): DeployedProductsResponse {
  if (Array.isArray(payload)) {
    return {
      deployedProducts: payload,
      totalRecords: payload.length,
      offset: 0,
      limit: payload.length,
    };
  }
  return payload;
}

export async function fetchDeploymentProductsAll(params: {
  deploymentId: string;
  request?: DeployedProductSearchRequest;
  pageSize?: number;
  fetchFn: FetchFn;
  logger?: ReturnType<typeof useLogger>;
}): Promise<DeploymentProductItem[]> {
  const {
    deploymentId,
    request,
    pageSize = DEFAULT_PAGE_SIZE,
    fetchFn,
    logger,
  } = params;

  const results: DeploymentProductItem[] = [];
  let offset = 0;

  while (true) {
    const payload = await postDeploymentProductsSearchPage({
      deploymentId,
      request,
      offset,
      limit: pageSize,
      fetchFn,
      logger,
    });
    const page = normalizeProductsPayload(payload);
    const batch = page.deployedProducts ?? [];
    results.push(...batch);

    if (batch.length === 0) {
      break;
    }

    const limit = page.limit ?? pageSize;
    const nextOffset = (page.offset ?? offset) + limit;
    const total = page.totalRecords;

    if (typeof total === "number" && !Number.isNaN(total)) {
      if (nextOffset >= total) {
        break;
      }
      offset = nextOffset;
      continue;
    }

    if (batch.length < limit) {
      break;
    }
    offset = nextOffset;
  }

  return results;
}

/**
 * Search products of a deployment with server pagination.
 * Use when the UI should load 10 first, then load more on scroll/pagination.
 *
 * @param {string} deploymentId - Deployment ID.
 * @param {UsePostDeploymentProductsSearchInfiniteOptions} [options] - request/pageSize/enabled.
 * @returns {UseInfiniteQueryResult<InfiniteData<DeployedProductsResponsePayload>, Error>} Infinite query result.
 */
export function usePostDeploymentProductsSearchInfinite(
  deploymentId: string,
  options?: UsePostDeploymentProductsSearchInfiniteOptions,
): UseInfiniteQueryResult<
  InfiniteData<DeployedProductsResponsePayload>,
  Error
> {
  const logger = useLogger();
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();
  const {
    request,
    pageSize = DEFAULT_PAGE_SIZE,
    enabled = true,
  } = options ?? {};

  return useInfiniteQuery<DeployedProductsResponsePayload, Error>({
    queryKey: [
      ApiQueryKeys.DEPLOYMENT_PRODUCTS,
      deploymentId,
      "search",
      request,
      pageSize,
    ],
    initialPageParam: 0,
    queryFn: ({ pageParam }) =>
      postDeploymentProductsSearchPage({
        deploymentId,
        request,
        offset: Number(pageParam) || 0,
        limit: pageSize,
        fetchFn: authFetch,
        logger,
      }),
    getNextPageParam: (lastPayload) => {
      const lastPage = normalizeProductsPayload(lastPayload);
      const offset = lastPage.offset ?? 0;
      const limit = lastPage.limit ?? pageSize;
      const nextOffset = offset + limit;
      const items = lastPage.deployedProducts ?? [];
      const total = lastPage.totalRecords;
      if (typeof total === "number" && !Number.isNaN(total)) {
        return nextOffset < total ? nextOffset : undefined;
      }
      if (items.length === limit) {
        return nextOffset;
      }
      return undefined;
    },
    enabled: enabled && !!deploymentId && isSignedIn && !isAuthLoading,
    staleTime: 0,
  });
}

/**
 * Search products of a deployment and automatically fetch all pages until the end.
 * Use for classification/background logic where the full list is required.
 *
 * @param {string} deploymentId - Deployment ID.
 * @param {UsePostDeploymentProductsSearchAllOptions} [options] - request/pageSize/enabled.
 * @returns {UseQueryResult<DeploymentProductItem[], Error>} Full products list.
 */
export function usePostDeploymentProductsSearchAll(
  deploymentId: string,
  options?: UsePostDeploymentProductsSearchAllOptions,
): UseQueryResult<DeploymentProductItem[], Error> {
  const logger = useLogger();
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();
  const {
    request,
    pageSize = DEFAULT_PAGE_SIZE,
    enabled = true,
  } = options ?? {};

  return useQuery<DeploymentProductItem[], Error>({
    queryKey: [
      ApiQueryKeys.DEPLOYMENT_PRODUCTS,
      deploymentId,
      "search-all",
      request,
      pageSize,
    ],
    queryFn: () =>
      fetchDeploymentProductsAll({
        deploymentId,
        request,
        pageSize,
        fetchFn: authFetch,
        logger,
      }),
    enabled: enabled && !!deploymentId && isSignedIn && !isAuthLoading,
    staleTime: 0,
  });
}

export function extractDeploymentProducts(
  payload: DeployedProductsResponsePayload | undefined,
): DeploymentProductItem[] {
  if (!payload) return [];
  if (Array.isArray(payload)) return payload;
  if (isDeployedProductsResponse(payload))
    return payload.deployedProducts ?? [];
  return [];
}
