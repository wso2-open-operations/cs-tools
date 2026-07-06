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

import { CASE_SEARCH_RESULTS_PAGE_SIZE } from "@features/support/constants/supportConstants";
import type {
  CaseListItem,
  CaseSearchRequest,
  CaseSearchResponse,
} from "@features/support/types/cases";

export type AuthFetchFn = (
  input: RequestInfo | URL,
  init?: RequestInit,
) => Promise<Response>;

/**
 * Loads every page of case search results for the current filters, search text, and sort.
 * Used by Download Results so the CSV matches the full refined list, not only the visible page.
 *
 * @param authFetch - Authenticated fetch from `useAuthApiClient`.
 * @param projectId - Project id.
 * @param searchRequest - Case search body without pagination (filters + sort).
 * @param pageSize - Rows per API request.
 * @returns All cases matching the search request.
 */
export async function fetchProjectCaseSearchResults(
  authFetch: AuthFetchFn,
  projectId: string,
  searchRequest: Omit<CaseSearchRequest, "pagination">,
  pageSize: number = CASE_SEARCH_RESULTS_PAGE_SIZE,
): Promise<CaseListItem[]> {
  const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;
  if (!baseUrl) {
    throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
  }

  const limit = Math.min(
    CASE_SEARCH_RESULTS_PAGE_SIZE,
    Math.max(1, Math.floor(pageSize)),
  );
  const requestUrl = `${baseUrl}/projects/${projectId}/cases/search`;
  const matchingCases: CaseListItem[] = [];
  let offset = 0;

  for (;;) {
    const requestBody: CaseSearchRequest = {
      ...searchRequest,
      pagination: { offset, limit },
    };

    const response = await authFetch(requestUrl, {
      method: "POST",
      body: JSON.stringify(requestBody),
    });

    if (!response.ok) {
      const errorBody = await response.text().catch(() => "");
      throw new Error(
        errorBody.trim() ||
          `Error fetching case search results (${response.status} ${response.statusText})`,
      );
    }

    const data: CaseSearchResponse = await response.json();
    const pageCases = data.cases ?? [];
    matchingCases.push(...pageCases);

    const totalRecords = Number.isFinite(data.totalRecords)
      ? data.totalRecords
      : matchingCases.length;
    const pageCount = pageCases.length;
    const responseLimit =
      Number.isFinite(data.limit) && data.limit > 0 ? data.limit : limit;
    const nextOffset =
      (Number.isFinite(data.offset) ? data.offset : offset) + responseLimit;

    if (
      pageCount === 0 ||
      matchingCases.length >= totalRecords ||
      nextOffset <= offset
    ) {
      break;
    }
    offset = nextOffset;
  }

  return matchingCases;
}
