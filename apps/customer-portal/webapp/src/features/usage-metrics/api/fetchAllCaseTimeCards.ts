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

import type {
  CaseTimeCard,
  CaseTimeCardSearchResponse,
  TimeCardSearchRequest,
} from "@features/usage-metrics/types/timeTracking";
import type { AuthFetchFn } from "@features/support/api/fetchProjectCaseSearchResults";

const PAGE_SIZE = 100;

/**
 * Fetches all pages of case time cards for a project, used for CSV/PDF export.
 *
 * @param authFetch - Authenticated fetch from `useAuthApiClient`.
 * @param projectId - Project ID.
 * @param filters - Optional date range and state filters.
 * @returns All case time cards matching the filters.
 */
export async function fetchAllCaseTimeCards(
  authFetch: AuthFetchFn,
  projectId: string,
  filters: TimeCardSearchRequest["filters"],
): Promise<CaseTimeCard[]> {
  const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;
  if (!baseUrl) {
    throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
  }

  const requestUrl = `${baseUrl}/projects/${projectId}/cases/time-cards/search`;
  const allCards: CaseTimeCard[] = [];
  let offset = 0;

  for (;;) {
    const body: TimeCardSearchRequest = {
      filters,
      pagination: { limit: PAGE_SIZE, offset },
    };

    const response = await authFetch(requestUrl, {
      method: "POST",
      body: JSON.stringify(body),
    });

    if (!response.ok) {
      const errorBody = await response.text().catch(() => "");
      throw new Error(
        errorBody.trim() ||
          `Error fetching time cards (${response.status} ${response.statusText})`,
      );
    }

    const data: CaseTimeCardSearchResponse = await response.json();
    const page = data.caseTimeCards ?? [];
    allCards.push(...page);

    const total = Number.isFinite(data.totalRecords) ? data.totalRecords : allCards.length;
    const responseLimit = Number.isFinite(data.limit) && data.limit > 0 ? data.limit : PAGE_SIZE;
    const nextOffset = (Number.isFinite(data.offset) ? data.offset : offset) + responseLimit;

    if (page.length === 0 || allCards.length >= total || nextOffset <= offset) {
      break;
    }
    offset = nextOffset;
  }

  return allCards;
}
