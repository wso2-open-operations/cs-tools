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
import { useAuthApiClient } from "@hooks/useAuthApiClient";
import { useLogger } from "@hooks/useLogger";
import { ApiQueryKeys } from "@constants/apiConstants";
import { searchMockCsm } from "@features/csm-search/api/mocks/searchMocks";
import type { CsmSearchResults } from "@features/csm-search/types/csmSearch";

const MOCK_LATENCY_MS = 80;
const MIN_QUERY_LENGTH = 2;

const EMPTY_RESULTS: CsmSearchResults = {
  query: "",
  cases: [],
  projects: [],
  accounts: [],
};

/**
 * Global search across cases, projects and accounts.
 *
 * Disabled when the query is shorter than {@link MIN_QUERY_LENGTH} characters
 * so the dropdown isn't flooded by single-letter matches.
 */
export function useGetCsmSearch(
  query: string,
): UseQueryResult<CsmSearchResults, Error> {
  const logger = useLogger();
  const authFetch = useAuthApiClient();
  const trimmed = query.trim();
  const enabled =
    trimmed.length >= MIN_QUERY_LENGTH;

  return useQuery<CsmSearchResults, Error>({
    queryKey: [ApiQueryKeys.CSM_SEARCH, trimmed],
    queryFn: async (): Promise<CsmSearchResults> => {
      if (window.config?.CSM_PORTAL_USE_MOCKS) {
        logger.debug(`[useGetCsmSearch] mock search for "${trimmed}"`);
        await new Promise((r) => setTimeout(r, MOCK_LATENCY_MS));
        return searchMockCsm(trimmed);
      }
      const baseUrl = window.config?.CSM_PORTAL_BACKEND_BASE_URL;
      if (!baseUrl) {
        throw new Error("CSM_PORTAL_BACKEND_BASE_URL is not configured");
      }
      const url = `${baseUrl}/csm/search?q=${encodeURIComponent(trimmed)}`;
      const response = await authFetch(url, { method: "GET" });
      if (!response.ok) {
        throw new Error(`Search failed: ${response.statusText}`);
      }
      return (await response.json()) as CsmSearchResults;
    },
    enabled,
    placeholderData: EMPTY_RESULTS,
    staleTime: 5_000,
  });
}

export { MIN_QUERY_LENGTH };
