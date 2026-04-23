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
  type UseInfiniteQueryResult,
  type InfiniteData,
} from "@tanstack/react-query";
import { useAsgardeo } from "@asgardeo/react";
import { useAuthApiClient } from "@/hooks/useAuthApiClient";
import { ApiError } from "@utils/ApiError";
import { useLogger } from "@hooks/useLogger";
import { ApiQueryKeys } from "@constants/apiConstants";
import type {
  SearchProjectsResponse,
  ProjectListItem,
} from "@features/project-hub/types/projects";
import type { SearchProjectsRequest } from "@features/project-hub/types/projects";

interface UseInfiniteProjectsParams {
  searchQuery?: string;
  pageSize?: number;
  enabled?: boolean;
}

/**
 * Custom hook to fetch projects with infinite scroll/pagination support.
 *
 * @param {UseInfiniteProjectsParams} params - Search query and page size.
 * @returns {UseInfiniteQueryResult} The infinite query result object with projects data.
 */
export default function useInfiniteProjects({
  searchQuery,
  pageSize = 20,
  enabled = true,
}: UseInfiniteProjectsParams = {}): UseInfiniteQueryResult<
  InfiniteData<SearchProjectsResponse>,
  Error
> {
  const logger = useLogger();
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();

  const normalizedSearchQuery = searchQuery?.trim() || undefined;

  return useInfiniteQuery<SearchProjectsResponse, Error>({
    queryKey: [
      ApiQueryKeys.PROJECTS,
      "infinite",
      normalizedSearchQuery,
      pageSize,
    ],
    enabled: enabled && isSignedIn && !isAuthLoading,
    queryFn: async ({ pageParam = 0 }): Promise<SearchProjectsResponse> => {
      logger.debug(
        `[useInfiniteProjects] Fetching projects... offset: ${pageParam}, limit: ${pageSize}, searchQuery: ${normalizedSearchQuery || "none"}`,
      );

      try {
        const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;

        if (!baseUrl) {
          throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
        }

        const requestUrl = `${baseUrl}/projects/search`;
        const body: SearchProjectsRequest = {
          pagination: { offset: pageParam as number, limit: pageSize },
        };

        if (normalizedSearchQuery) {
          body.filters = { searchQuery: normalizedSearchQuery };
        }

        const response = await authFetch(requestUrl, {
          method: "POST",
          body: JSON.stringify(body),
        });

        logger.debug(
          `[useInfiniteProjects] Response status: ${response.status}`,
        );

        if (!response.ok) {
          let apiMessage: string | undefined;
          try {
            const errBody = await response.json();
            if (typeof errBody?.message === "string") {
              apiMessage = errBody.message;
            }
          } catch {
            // ignore – body may not be JSON
          }
          throw new ApiError(
            response.status,
            response.statusText,
            apiMessage ?? `Error fetching projects: ${response.statusText}`,
          );
        }

        const data: SearchProjectsResponse = await response.json();
        logger.debug("[useInfiniteProjects] Data received:", data);
        return data;
      } catch (error) {
        logger.error("[useInfiniteProjects] Error:", error);
        throw error;
      }
    },
    getNextPageParam: (lastPage, allPages) => {
      if (lastPage.projects.length === 0) {
        return undefined;
      }

      const totalFetched = allPages.reduce(
        (sum, page) => sum + page.projects.length,
        0,
      );

      // If we've fetched all available projects, return undefined to stop
      if (totalFetched >= lastPage.totalRecords) {
        return undefined;
      }

      // Otherwise, return the next offset
      return totalFetched;
    },
    initialPageParam: 0,
    staleTime: 0,
  });
}

/**
 * Helper function to flatten paginated project data into a single array.
 *
 * @param {InfiniteData<SearchProjectsResponse> | undefined} data - The infinite query data.
 * @returns {ProjectListItem[]} Flattened array of all projects.
 */
export function flattenProjectPages(
  data: InfiniteData<SearchProjectsResponse> | undefined,
): ProjectListItem[] {
  if (!data) return [];
  return data.pages.flatMap((page) => page.projects);
}

/**
 * Helper function to get total records count from infinite query data.
 *
 * @param {InfiniteData<SearchProjectsResponse> | undefined} data - The infinite query data.
 * @returns {number} Total number of records available.
 */
export function getTotalRecords(
  data: InfiniteData<SearchProjectsResponse> | undefined,
): number {
  if (!data || data.pages.length === 0) return 0;
  return data.pages[0].totalRecords;
}
