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

import { keepPreviousData, useQuery, type UseQueryResult } from "@tanstack/react-query";
import { ApiQueryKeys } from "@constants/apiConstants";
import { useBackendApi } from "@api/backend/client";
import type {
  BeSearchProductVulnerabilitiesPayload,
  BeSearchProductVulnerabilitiesResponse,
} from "@api/backend/types";

/**
 * Search product vulnerabilities via `POST /products/vulnerabilities/search`
 * (ServiceNow data source only). Returns the raw paged response so the listing
 * can drive server-side pagination from `total`. `keepPreviousData` keeps the
 * table populated while the next page/filter loads.
 */
export function useSearchProductVulnerabilities(
  payload: BeSearchProductVulnerabilitiesPayload,
): UseQueryResult<BeSearchProductVulnerabilitiesResponse, Error> {
  const api = useBackendApi();

  return useQuery<BeSearchProductVulnerabilitiesResponse, Error>({
    queryKey: [ApiQueryKeys.PRODUCT_VULNERABILITIES_SEARCH, payload],
    queryFn: (): Promise<BeSearchProductVulnerabilitiesResponse> =>
      api.post<BeSearchProductVulnerabilitiesPayload, BeSearchProductVulnerabilitiesResponse>(
        "/products/vulnerabilities/search",
        payload,
      ),
    placeholderData: keepPreviousData,
    staleTime: 30_000,
  });
}
