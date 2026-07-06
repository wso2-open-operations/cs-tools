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
import { useAsgardeo } from "@asgardeo/react";
import { useAuthApiClient } from "@/hooks/useAuthApiClient";
import { useLogger } from "@hooks/useLogger";
import { ApiQueryKeys } from "@constants/apiConstants";
import type {
  GlobalSearchPayload,
  GlobalSearchResponse,
} from "@features/project-hub/types/globalSearch";

export interface UseGetGlobalSearchOptions {
  enabled?: boolean;
}

/**
 * Executes a unified global search across projects and cases via POST /search.
 * Both result sets and their totals are returned in a single response.
 *
 * Use projectsPagination / casesPagination to control which results are
 * returned for each type independently. Omit filters.types to receive both.
 */
export function useGetGlobalSearch(
  payload: GlobalSearchPayload,
  options?: UseGetGlobalSearchOptions,
): UseQueryResult<GlobalSearchResponse, Error> {
  const logger = useLogger();
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();

  return useQuery<GlobalSearchResponse, Error>({
    queryKey: [ApiQueryKeys.GLOBAL_SEARCH, payload],
    queryFn: async ({ signal }): Promise<GlobalSearchResponse> => {
      logger.debug("[useGetGlobalSearch] Executing global search", payload);
      const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;
      if (!baseUrl) {
        throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
      }
      const response = await authFetch(`${baseUrl}/search`, {
        method: "POST",
        signal,
        body: JSON.stringify(payload),
      });
      if (!response.ok) {
        throw new Error(`Global search failed: ${response.statusText}`);
      }
      return response.json();
    },
    enabled: (options?.enabled ?? true) && isSignedIn && !isAuthLoading,
    staleTime: 0,
  });
}
