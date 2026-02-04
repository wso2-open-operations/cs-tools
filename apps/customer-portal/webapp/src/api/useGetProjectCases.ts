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
import { useLogger } from "@/hooks/useLogger";
import { mockCases } from "@/models/mockData";
import { ApiQueryKeys, API_MOCK_DELAY } from "@/constants/apiConstants";
import type { CaseSearchRequest } from "@/models/requests";
import type { CaseSearchResponse } from "@/models/responses";

/**
 * Custom hook to search cases for a specific project.
 *
 * @param {string} projectId - The ID of the project to search cases for.
 * @param {CaseSearchRequest} requestBody - The search parameters including filters, pagination, and sorting.
 * @returns {UseQueryResult<CaseSearchResponse, Error>} The query result object.
 */
export default function useGetProjectCases(
  projectId: string,
  requestBody: CaseSearchRequest,
): UseQueryResult<CaseSearchResponse, Error> {
  const logger = useLogger();

  return useQuery<CaseSearchResponse, Error>({
    queryKey: [ApiQueryKeys.PROJECT_CASES, projectId, requestBody],
    queryFn: async (): Promise<CaseSearchResponse> => {
      logger.debug(
        `Fetching cases for project: ${projectId} with params:`,
        requestBody,
      );

      await new Promise((resolve) => setTimeout(resolve, API_MOCK_DELAY));

      // TODO: Filter logic need be implemented here

      const { offset = 0, limit = 10 } = requestBody.pagination;
      const filteredCases = mockCases.filter(
        (cases) => cases.project.id === projectId || projectId === "all",
      );

      // slice for pagination
      const pagedCases = filteredCases.slice(offset, offset + limit);

      const response: CaseSearchResponse = {
        cases: pagedCases.length > 0 ? pagedCases : mockCases.slice(0, limit),
        totalRecords:
          filteredCases.length > 0 ? filteredCases.length : mockCases.length,
        offset,
        limit,
      };

      logger.debug("Cases fetched successfully", response);
      return response;
    },
    enabled: !!projectId,
  });
}
