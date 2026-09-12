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

//
// Waiting on a list's search results.
//
// Every searchable list in the portal — the cases list and the Security Report
// Analysis list included — is backed by the same `POST /projects/{id}/cases/search`
// endpoint, so one helper serves them all.
//

import { type Page, type Response } from "../fixtures/test";

/** How long to allow for a debounced search to fire and return. */
const SEARCH_TIMEOUT_MS = 60_000;

/**
 * Reads the search term out of a `/cases/search` request body.
 *
 * @param postData - Raw request body, or null when there is none.
 * @returns The search term, or undefined if absent or unparseable.
 */
function searchQueryOf(postData: string | null): string | undefined {
  if (!postData) return undefined;
  try {
    // The term travels inside `filters`, alongside caseTypes — verified against a
    // live request. It is NOT at the root of the body.
    const body = JSON.parse(postData) as {
      filters?: { searchQuery?: string };
    };
    return body.filters?.searchQuery;
  } catch {
    return undefined;
  }
}

/** Whether a `/cases/search` response came from a POST to that endpoint. */
function isCaseSearch(response: Response): boolean {
  return (
    response.request().method() === "POST" &&
    new URL(response.url()).pathname.endsWith("/cases/search")
  );
}

/** Parses a request body, or returns undefined when it is absent/unparseable. */
function searchFilters(
  postData: string | null,
):
  | { createdByMe?: boolean; statusIds?: number[]; severityIds?: number[] }
  | undefined {
  if (!postData) return undefined;
  try {
    const body = JSON.parse(postData) as {
      filters?: {
        createdByMe?: boolean;
        statusIds?: number[];
        severityIds?: number[];
      };
    };
    return body.filters ?? {};
  } catch {
    return undefined;
  }
}

/**
 * Starts waiting for a case search asking for a specific page of results.
 *
 * Call this **before** operating the pagination control, then await it after.
 *
 * Page size and position travel as `pagination.limit` and `pagination.offset`,
 * so matching on them ties the wait to the change under test rather than to any
 * other refetch the table happens to make. Only the fields given are matched.
 *
 * @param page - Test page.
 * @param match - The `limit` and/or `offset` the request must carry.
 * @returns Promise for the matching search response.
 */
export function caseSearchWithPagination(
  page: Page,
  match: { limit?: number; offset?: number },
): Promise<Response> {
  return page.waitForResponse(
    (response) => {
      if (!isCaseSearch(response)) return false;
      const postData = response.request().postData();
      if (!postData) return false;
      try {
        const body = JSON.parse(postData) as {
          pagination?: { limit?: number; offset?: number };
        };
        const pagination = body.pagination;
        if (!pagination) return false;
        if (match.limit !== undefined && pagination.limit !== match.limit) {
          return false;
        }
        if (match.offset !== undefined && pagination.offset !== match.offset) {
          return false;
        }
        return true;
      } catch {
        return false;
      }
    },
    { timeout: SEARCH_TIMEOUT_MS },
  );
}

/**
 * Starts waiting for the search response of the unfiltered "all cases" list.
 *
 * Call this **before** the navigation that opens the list, then await it after.
 *
 * Identified by what the body does *not* carry, since an unfiltered list sends
 * only `caseTypes` (verified live). Both exclusions are load-bearing:
 *
 * - no `createdByMe`, which is what separates it from My Cases;
 * - no `statusIds`, which is what separates it from the Outstanding Cases card
 *   on the Support Center page — that card searches the same endpoint with the
 *   open-case states, and its request would otherwise satisfy this predicate and
 *   resolve the wait before the list had even been opened.
 *
 * @param page - Test page.
 * @returns Promise for the matching search response.
 */
export function allCasesSearchResponse(page: Page): Promise<Response> {
  return page.waitForResponse(
    (response) => {
      if (!isCaseSearch(response)) return false;
      const filters = searchFilters(response.request().postData());
      if (!filters) return false;
      return (
        filters.createdByMe === undefined && filters.statusIds === undefined
      );
    },
    { timeout: SEARCH_TIMEOUT_MS },
  );
}

/**
 * Starts waiting for the search response of the "my cases" list.
 *
 * Call this **before** the navigation that opens the list, then await it after.
 *
 * `createdByMe` is what actually narrows the list — the heading merely reports
 * it — so matching the flag in the request body is both the wait and the
 * contract check. It travels inside `filters`, and the builder emits it on every
 * branch (buildDashboardCaseSearchFilters).
 *
 * @param page - Test page.
 * @returns Promise for the matching search response.
 */
export function myCasesSearchResponse(page: Page): Promise<Response> {
  return page.waitForResponse(
    (response) => {
      if (!isCaseSearch(response)) return false;
      return searchFilters(response.request().postData())?.createdByMe === true;
    },
    { timeout: SEARCH_TIMEOUT_MS },
  );
}

/**
 * Starts waiting for the search response produced by a specific term.
 *
 * Call this **before** typing, then await the returned promise afterwards.
 *
 * Matching the request's own term is what makes this reliable. Waiting for
 * `networkidle` instead is too weak: it can resolve before the debounced request
 * has even been issued, leaving a caller to read stale rows and conclude a record
 * does not exist — which, for the specs that create on absence, means a permanent
 * duplicate that cannot be deleted.
 *
 * @param page - Test page.
 * @param searchText - The term about to be typed into the list's search box.
 * @returns Promise for the matching search response.
 */
export function caseSearchResponse(
  page: Page,
  searchText: string,
): Promise<Response> {
  return page.waitForResponse(
    (response) => {
      const request = response.request();
      if (request.method() !== "POST") return false;
      if (!new URL(response.url()).pathname.endsWith("/cases/search")) {
        return false;
      }
      return searchQueryOf(request.postData()) === searchText;
    },
    { timeout: SEARCH_TIMEOUT_MS },
  );
}

/**
 * Starts waiting for a case search filtered to a specific severity.
 *
 * Call this **before** choosing the severity, then await it after.
 *
 * The chosen severity travels as a numeric id inside `filters.severityIds`, so
 * matching it is what proves the filter reached the backend rather than only
 * highlighting an option in the menu.
 *
 * @param page - Test page.
 * @param severityId - Severity id the request must carry.
 * @returns Promise for the matching search response.
 */
export function caseSearchWithSeverity(
  page: Page,
  severityId: number,
): Promise<Response> {
  return page.waitForResponse(
    (response) => {
      if (!isCaseSearch(response)) return false;
      const severityIds = searchFilters(
        response.request().postData(),
      )?.severityIds;
      return !!severityIds && severityIds.includes(severityId);
    },
    { timeout: SEARCH_TIMEOUT_MS },
  );
}

/**
 * Starts waiting for a case search carrying no severity filter.
 *
 * Call this **before** clearing the filter, then await it after — the initial
 * unfiltered load matches this predicate too, so arming it beforehand is what
 * ties the wait to the clearing rather than to the page load.
 *
 * @param page - Test page.
 * @returns Promise for the matching search response.
 */
export function caseSearchWithoutSeverity(page: Page): Promise<Response> {
  return page.waitForResponse(
    (response) => {
      if (!isCaseSearch(response)) return false;
      const filters = searchFilters(response.request().postData());
      if (!filters) return false;
      return filters.severityIds === undefined;
    },
    { timeout: SEARCH_TIMEOUT_MS },
  );
}

/**
 * Starts waiting for a case search carrying — or not carrying — a status filter.
 *
 * Call this **before** applying or clearing the filter, then await it after: the
 * "no filter" form also describes the page's own first load, so arming it
 * beforehand is what ties the wait to the change under test.
 *
 * @param page - Test page.
 * @param filtered - True to match a request with statuses, false for one without.
 * @returns Promise for the matching search response.
 */
export function caseSearchWithStatusFilter(
  page: Page,
  filtered: boolean,
): Promise<Response> {
  return page.waitForResponse(
    (response) => {
      if (!isCaseSearch(response)) return false;
      const statusIds = searchFilters(
        response.request().postData(),
      )?.statusIds;
      const hasStatuses = !!statusIds && statusIds.length > 0;
      return hasStatuses === filtered;
    },
    { timeout: SEARCH_TIMEOUT_MS },
  );
}

/**
 * Starts waiting for a case search sorted a particular way.
 *
 * `sortBy` sits at the root of the body rather than inside `filters`, so it is
 * read separately from the filter helpers above.
 *
 * @param page - Test page.
 * @param field - Sort field the request must carry, e.g. "state".
 * @param order - Sort order, e.g. "asc".
 * @returns Promise for the matching search response.
 */
export function caseSearchWithSort(
  page: Page,
  field: string,
  order: string,
): Promise<Response> {
  return page.waitForResponse(
    (response) => {
      if (!isCaseSearch(response)) return false;
      const postData = response.request().postData();
      if (!postData) return false;
      try {
        const body = JSON.parse(postData) as {
          sortBy?: { field?: string; order?: string };
        };
        return (
          body.sortBy?.field === field &&
          body.sortBy?.order?.toLowerCase() === order.toLowerCase()
        );
      } catch {
        return false;
      }
    },
    { timeout: SEARCH_TIMEOUT_MS },
  );
}
