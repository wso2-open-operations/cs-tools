// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License. You may obtain a copy of the License at
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
import { useAsgardeo } from "@asgardeo/react";
import { useAuthApiClient } from "@api/useAuthApiClient";
import { useLogger } from "@hooks/useLogger";
import { ApiQueryKeys } from "@constants/apiConstants";
import type { CaseSearchRequest } from "@/types/cases";
import type { CaseSearchResponse } from "@/types/cases";

export interface UseGetProjectCasesPageOptions {
  enabled?: boolean;
}

/**
 * Fetches a single page of cases (fetch-on-demand).
 * Use for dashboard Outstanding Engagements: load 10 first, fetch more only when user navigates.
 *
 * @param projectId - Project ID.
 * @param baseRequest - Search filters (no pagination).
 * @param offset - Pagination offset.
 * @param limit - Page size.
 * @param options - Optional { enabled } to control query execution.
 * @returns Query result with cases for the requested page.
 */
export function useGetProjectCasesPage(
  projectId: string,
  baseRequest: Omit<CaseSearchRequest, "pagination">,
  offset: number,
  limit: number,
  options?: UseGetProjectCasesPageOptions,
): UseQueryResult<CaseSearchResponse, Error> {
  const logger = useLogger();
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();

  return useQuery<CaseSearchResponse, Error>({
    queryKey: [
      ApiQueryKeys.PROJECT_CASES,
      "page",
      projectId,
      baseRequest,
      offset,
      limit,
    ],
    queryFn: async (): Promise<CaseSearchResponse> => {
      const requestBody: CaseSearchRequest = {
        ...baseRequest,
        pagination: { offset, limit },
      };
      logger.debug(
        `[useGetProjectCasesPage] Fetching cases for project: ${projectId}, offset: ${offset}, limit: ${limit}`,
      );
      const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;
      if (!baseUrl) {
        throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
      }
      const response = await authFetch(
        `${baseUrl}/projects/${projectId}/cases/search`,
        {
          method: "POST",

          body: JSON.stringify(requestBody),
        },
      );
      if (!response.ok) {
        throw new Error(`Error fetching project cases: ${response.statusText}`);
      }
      return response.json();
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
    refetchOnReconnect: false,
  });
}
