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

import { CHANGE_REQUEST_SEARCH_RESULTS_PAGE_SIZE } from "@features/operations/constants/operationsConstants";
import type { AuthFetchFn } from "@features/support/api/fetchProjectCaseSearchResults";
import type {
  ChangeRequestItem,
  ChangeRequestSearchRequest,
  ChangeRequestSearchResponse,
} from "@features/operations/types/changeRequests";

/**
 * Loads every page of change-request search results for the current filters and search text.
 *
 * @param authFetch - Authenticated fetch from `useAuthApiClient`.
 * @param projectId - Project id.
 * @param searchRequest - Search body without pagination.
 * @param pageSize - Rows per API request.
 * @returns All change requests matching the search request.
 */
export async function fetchChangeRequestSearchResults(
  authFetch: AuthFetchFn,
  projectId: string,
  searchRequest: Omit<ChangeRequestSearchRequest, "pagination">,
  pageSize: number = CHANGE_REQUEST_SEARCH_RESULTS_PAGE_SIZE,
): Promise<ChangeRequestItem[]> {
  const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;
  if (!baseUrl) {
    throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
  }

  const limit = Math.min(
    CHANGE_REQUEST_SEARCH_RESULTS_PAGE_SIZE,
    Math.max(1, Math.floor(pageSize)),
  );
  const requestUrl = `${baseUrl}/projects/${projectId}/change-requests/search`;
  const matchingItems: ChangeRequestItem[] = [];
  let offset = 0;

  for (;;) {
    const requestBody: ChangeRequestSearchRequest = {
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
          `Error fetching change request search results (${response.status} ${response.statusText})`,
      );
    }

    const data: ChangeRequestSearchResponse = await response.json();
    const pageItems = data.changeRequests ?? [];
    matchingItems.push(...pageItems);

    const totalRecords = Number.isFinite(data.totalRecords)
      ? data.totalRecords
      : matchingItems.length;
    const pageCount = pageItems.length;
    const responseLimit =
      Number.isFinite(data.limit) && data.limit > 0 ? data.limit : limit;
    const nextOffset =
      (Number.isFinite(data.offset) ? data.offset : offset) + responseLimit;

    if (
      pageCount === 0 ||
      matchingItems.length >= totalRecords ||
      nextOffset <= offset
    ) {
      break;
    }
    offset = nextOffset;
  }

  return matchingItems;
}
