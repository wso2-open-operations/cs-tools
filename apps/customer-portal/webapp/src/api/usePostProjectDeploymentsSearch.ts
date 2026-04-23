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
import type { DeploymentSearchRequest } from "@features/project-details/types/deployments";
import type {
  ProjectDeploymentItem,
  ProjectDeploymentsListResponse,
} from "@features/project-details/types/deployments";

const DEFAULT_PAGE_SIZE = 10;

async function postDeploymentsSearchPage(params: {
  projectId: string;
  request: DeploymentSearchRequest | undefined;
  offset: number;
  limit: number;
  authFetch: ReturnType<typeof useAuthApiClient>;
  logger: ReturnType<typeof useLogger>;
}): Promise<ProjectDeploymentsListResponse> {
  const { projectId, request, offset, limit, authFetch, logger } = params;

  const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;
  if (!baseUrl) {
    throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
  }

  const requestUrl = `${baseUrl}/projects/${projectId}/deployments/search`;
  const payload: DeploymentSearchRequest = {
    ...(request ?? {}),
    pagination: {
      ...(request?.pagination ?? {}),
      offset,
      limit,
    },
  };

  const response = await authFetch(requestUrl, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });

  logger.debug(
    `[usePostProjectDeploymentsSearch] Response status: ${response.status}`,
  );

  if (!response.ok) {
    throw new Error(`Error searching deployments: ${response.statusText}`);
  }

  return (await response.json()) as ProjectDeploymentsListResponse;
}

export interface UsePostProjectDeploymentsSearchInfiniteOptions {
  request?: DeploymentSearchRequest;
  pageSize?: number;
  enabled?: boolean;
}

/**
 * Search deployments of a project with server pagination.
 * Use when the UI should load 10 first, then load more on scroll/pagination.
 *
 * @param {string} projectId - Project ID.
 * @param {UsePostProjectDeploymentsSearchInfiniteOptions} [options] - request/pageSize/enabled.
 * @returns {UseInfiniteQueryResult<InfiniteData<ProjectDeploymentsListResponse>, Error>} Infinite query result.
 */
export function usePostProjectDeploymentsSearchInfinite(
  projectId: string,
  options?: UsePostProjectDeploymentsSearchInfiniteOptions,
): UseInfiniteQueryResult<InfiniteData<ProjectDeploymentsListResponse>, Error> {
  const logger = useLogger();
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();
  const {
    request,
    pageSize = DEFAULT_PAGE_SIZE,
    enabled = true,
  } = options ?? {};

  return useInfiniteQuery<ProjectDeploymentsListResponse, Error>({
    queryKey: [
      ApiQueryKeys.DEPLOYMENTS,
      projectId,
      "search",
      request,
      pageSize,
    ],
    initialPageParam: 0,
    queryFn: ({ pageParam }) =>
      postDeploymentsSearchPage({
        projectId,
        request,
        offset: Number(pageParam) || 0,
        limit: pageSize,
        authFetch,
        logger,
      }),
    getNextPageParam: (lastPage) => {
      const offset = lastPage.offset ?? 0;
      const limit = lastPage.limit ?? pageSize;
      const nextOffset = offset + limit;
      const items = lastPage.deployments ?? [];
      const total = lastPage.totalRecords;
      if (typeof total === "number" && !Number.isNaN(total)) {
        return nextOffset < total ? nextOffset : undefined;
      }
      if (items.length === limit) {
        return nextOffset;
      }
      return undefined;
    },
    enabled: enabled && !!projectId && isSignedIn && !isAuthLoading,
    staleTime: 0,
  });
}

export interface UsePostProjectDeploymentsSearchAllOptions {
  request?: DeploymentSearchRequest;
  pageSize?: number;
  enabled?: boolean;
}

/**
 * Search deployments and automatically fetch all pages until the end.
 * Use for classification/background logic where the full list is required.
 *
 * @param {string} projectId - Project ID.
 * @param {UsePostProjectDeploymentsSearchAllOptions} [options] - request/pageSize/enabled.
 * @returns {UseQueryResult<ProjectDeploymentItem[], Error>} Full deployments list.
 */
export function usePostProjectDeploymentsSearchAll(
  projectId: string,
  options?: UsePostProjectDeploymentsSearchAllOptions,
): UseQueryResult<ProjectDeploymentItem[], Error> {
  const logger = useLogger();
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();
  const {
    request,
    pageSize = DEFAULT_PAGE_SIZE,
    enabled = true,
  } = options ?? {};

  return useQuery<ProjectDeploymentItem[], Error>({
    queryKey: [
      ApiQueryKeys.DEPLOYMENTS,
      projectId,
      "search-all",
      request,
      pageSize,
    ],
    queryFn: async () => {
      const results: ProjectDeploymentItem[] = [];
      let offset = 0;

      while (true) {
        const page = await postDeploymentsSearchPage({
          projectId,
          request,
          offset,
          limit: pageSize,
          authFetch,
          logger,
        });
        const items = page.deployments ?? [];
        results.push(...items);

        if (items.length === 0) {
          break;
        }

        const pageLimit = page.limit ?? pageSize;
        const nextOffset = (page.offset ?? offset) + pageLimit;
        const total = page.totalRecords;

        if (typeof total === "number" && !Number.isNaN(total)) {
          if (nextOffset >= total) {
            break;
          }
          offset = nextOffset;
          continue;
        }

        if (items.length < pageLimit) {
          break;
        }
        offset = nextOffset;
      }

      return results;
    },
    enabled: enabled && !!projectId && isSignedIn && !isAuthLoading,
    staleTime: 0,
  });
}
