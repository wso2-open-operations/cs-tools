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
  useQuery,
  useInfiniteQuery,
  type UseQueryResult,
  type UseInfiniteQueryResult,
  type InfiniteData,
} from "@tanstack/react-query";
import { useAsgardeo } from "@asgardeo/react";
import { useAuthApiClient } from "@api/useAuthApiClient";
import { useLogger } from "@hooks/useLogger";
import { ApiQueryKeys } from "@constants/apiConstants";
import type { ChangeRequestSearchRequest } from "@models/requests";
import type { ChangeRequestSearchResponse } from "@models/responses";

export interface UseGetChangeRequestsOptions {
  enabled?: boolean;
}

/**
 * Custom hook to search change requests for a specific project.
 *
 * @param {string} projectId - The ID of the project to search change requests for.
 * @param {Omit<ChangeRequestSearchRequest, 'pagination'>} baseRequest - The search parameters excluding pagination.
 * @param {number} offset - Pagination offset.
 * @param {number} limit - Page size.
 * @param {UseGetChangeRequestsOptions} options - Optional query options.
 * @returns {UseQueryResult<ChangeRequestSearchResponse, Error>} The query result object.
 */
export default function useGetChangeRequests(
  projectId: string,
  baseRequest: Omit<ChangeRequestSearchRequest, "pagination">,
  offset: number,
  limit: number,
  options?: UseGetChangeRequestsOptions,
): UseQueryResult<ChangeRequestSearchResponse, Error> {
  const logger = useLogger();
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();

  return useQuery<ChangeRequestSearchResponse, Error>({
    queryKey: [
      ApiQueryKeys.CHANGE_REQUESTS,
      projectId,
      baseRequest,
      offset,
      limit,
    ],
    queryFn: async (): Promise<ChangeRequestSearchResponse> => {
      const requestBody: ChangeRequestSearchRequest = {
        ...baseRequest,
        pagination: {
          offset,
          limit,
        },
      };

      logger.debug(
        `[useGetChangeRequests] Fetching change requests for project: ${projectId}`,
        requestBody,
      );

      try {
        const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;

        if (!baseUrl) {
          throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
        }

        const requestUrl = `${baseUrl}/projects/${projectId}/change-requests/search`;

        const response = await authFetch(requestUrl, {
          method: "POST",

          body: JSON.stringify(requestBody),
        });

        logger.debug(
          `[useGetChangeRequests] Response status: ${response.status}`,
        );

        if (!response.ok) {
          throw new Error(
            `Error fetching change requests: ${response.statusText}`,
          );
        }

        const data: ChangeRequestSearchResponse = await response.json();
        logger.debug("[useGetChangeRequests] Data received:", data);
        return data;
      } catch (error) {
        logger.error("[useGetChangeRequests] Error:", error);
        throw error;
      }
    },
    enabled:
      (options?.enabled ?? true) &&
      !!projectId &&
      isSignedIn &&
      !isAuthLoading &&
      offset >= 0 &&
      limit > 0,
    staleTime: 5 * 60 * 1000,
    refetchOnWindowFocus: false,
  });
}

/**
 * Custom hook to fetch all change requests using infinite query (for calendar view).
 * Fetches data in batches of 10 items per call.
 *
 * @param {string} projectId - The ID of the project to search change requests for.
 * @param {Omit<ChangeRequestSearchRequest, 'pagination'>} baseRequest - The search parameters excluding pagination.
 * @param {UseGetChangeRequestsOptions} options - Optional query options.
 * @returns {UseInfiniteQueryResult<InfiniteData<ChangeRequestSearchResponse>, Error>} The infinite query result object.
 */
export function useGetChangeRequestsInfinite(
  projectId: string,
  baseRequest: Omit<ChangeRequestSearchRequest, "pagination">,
  options?: UseGetChangeRequestsOptions,
): UseInfiniteQueryResult<InfiniteData<ChangeRequestSearchResponse>, Error> {
  const logger = useLogger();
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();
  const pageSize = 10;

  return useInfiniteQuery<ChangeRequestSearchResponse, Error>({
    queryKey: [
      ApiQueryKeys.CHANGE_REQUESTS,
      "infinite",
      projectId,
      baseRequest,
    ],
    queryFn: async ({
      pageParam = 0,
    }): Promise<ChangeRequestSearchResponse> => {
      const requestBody: ChangeRequestSearchRequest = {
        ...baseRequest,
        pagination: {
          offset: pageParam as number,
          limit: pageSize,
        },
      };

      logger.debug(
        `[useGetChangeRequestsInfinite] Fetching change requests for project: ${projectId}, offset: ${pageParam}`,
        requestBody,
      );

      try {
        const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;

        if (!baseUrl) {
          throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
        }

        const requestUrl = `${baseUrl}/projects/${projectId}/change-requests/search`;

        const response = await authFetch(requestUrl, {
          method: "POST",

          body: JSON.stringify(requestBody),
        });

        logger.debug(
          `[useGetChangeRequestsInfinite] Response status: ${response.status}`,
        );

        if (!response.ok) {
          throw new Error(
            `Error fetching change requests: ${response.statusText}`,
          );
        }

        const data: ChangeRequestSearchResponse = await response.json();
        logger.debug("[useGetChangeRequestsInfinite] Data received:", data);
        return data;
      } catch (error) {
        logger.error("[useGetChangeRequestsInfinite] Error:", error);
        throw error;
      }
    },
    getNextPageParam: (lastPage) => {
      const currentOffset = lastPage.offset || 0;
      const fetchedCount = lastPage.changeRequests?.length || 0;
      const totalRecords = lastPage.totalRecords || 0;

      // If we've fetched all records, return undefined to stop pagination
      if (currentOffset + fetchedCount >= totalRecords) {
        return undefined;
      }

      // Return next offset
      return currentOffset + pageSize;
    },
    initialPageParam: 0,
    enabled:
      (options?.enabled ?? true) && !!projectId && isSignedIn && !isAuthLoading,
    staleTime: 5 * 60 * 1000,
    refetchOnWindowFocus: false,
  });
}
