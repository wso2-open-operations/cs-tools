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
import { useLogger } from "@/hooks/useLogger";
import { mockProjects } from "@/models/mockData";
import { ApiQueryKeys } from "@/constants/apiQueryKeys";
import type { SearchProjectsRequest } from "@/models/requests";
import type { SearchProjectsResponse } from "@/models/responses";

/**
 * Custom hook to search projects.
 * This hook uses an infinite query to fetch projects in pages.
 *
 * @param {SearchProjectsRequest} searchData - The search and pagination parameters.
 * @param {boolean} fetchAll - If true, treats this as a shared "all projects" query.
 * @returns {UseInfiniteQueryResult<InfiniteData<SearchProjectsResponse>, Error>} The infinite query result object.
 */
export default function useSearchProjects(
  searchData: SearchProjectsRequest = {},
  fetchAll: boolean = false,
): UseInfiniteQueryResult<InfiniteData<SearchProjectsResponse>, Error> {
  const logger = useLogger();
  const limit = fetchAll ? 100 : searchData.pagination?.limit || 10;

  // A stable key for the "all projects" query ensures cache sharing
  const queryKey = fetchAll
    ? [ApiQueryKeys.PROJECTS, "all"]
    : [ApiQueryKeys.PROJECTS, searchData];

  return useInfiniteQuery<SearchProjectsResponse, Error>({
    getPreviousPageParam: (firstPage) => {
      const prevOffset = firstPage.offset - limit;
      return prevOffset >= 0 ? prevOffset : undefined;
    },
    getNextPageParam: (lastPage) => {
      const nextOffset = lastPage.offset + limit;
      return nextOffset < lastPage.totalRecords ? nextOffset : undefined;
    },
    initialPageParam: 0,
    queryFn: async ({ pageParam = 0 }): Promise<SearchProjectsResponse> => {
      logger.debug(
        `Fetching projects... offset: ${pageParam}, limit: ${limit}, fetchAll: ${fetchAll}`,
      );

      // Simulate a network delay
      await new Promise((resolve) => setTimeout(resolve, 800));

      const offset = typeof pageParam === "number" ? pageParam : 0;

      const results: SearchProjectsResponse = {
        limit,
        offset,
        projects: mockProjects.slice(offset, offset + limit).map((project) => ({
          createdOn: project.createdOn,
          description: project.description,
          id: project.id,
          key: project.key,
          name: project.name,
        })),
        totalRecords: mockProjects.length,
      };

      logger.debug("Projects fetched successfully", results);

      return results;
    },
    queryKey,
    staleTime: Infinity,
  });
}
