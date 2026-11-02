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
  | {
      createdByMe?: boolean;
      statusIds?: number[];
      severityIds?: number[];
      engagementTypeKeys?: number[];
    }
  | undefined {
  if (!postData) return undefined;
  try {
    const body = JSON.parse(postData) as {
      filters?: {
        createdByMe?: boolean;
        statusIds?: number[];
        severityIds?: number[];
        engagementTypeKeys?: number[];
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
 * Starts waiting for an engagement search filtered to one or more types.
 *
 * Call this **before** choosing the type, then await it after.
 *
 * The chosen type travels as numeric ids in `filters.engagementTypeKeys` — a
 * breakdown entry can map to SEVERAL ids, since `engagementTypeOptions` groups
 * every backend type sharing a display name into one comma-joined option. So
 * this asserts the request carries the ids the UI offered, not a single id.
 *
 * Matching on the ids rather than the label is what proves the filter reached
 * the backend rather than only ticking a checkbox in the menu.
 *
 * @param page - Test page.
 * @param typeIds - Ids the request must carry, all of them.
 * @returns Promise for the matching search response.
 */
export function caseSearchWithEngagementTypes(
  page: Page,
  typeIds: number[],
): Promise<Response> {
  return page.waitForResponse(
    (response) => {
      if (!isCaseSearch(response)) return false;
      const keys = searchFilters(
        response.request().postData(),
      )?.engagementTypeKeys;
      return (
        !!keys && typeIds.every((id) => keys.includes(id))
      );
    },
    { timeout: SEARCH_TIMEOUT_MS },
  );
}

/**
 * Starts waiting for an engagement search carrying no type filter.
 *
 * Call this **before** clearing, then await it after — the initial unfiltered
 * load matches this predicate too, so arming it beforehand is what ties the
 * wait to the clearing rather than to the page load.
 *
 * @param page - Test page.
 * @returns Promise for the matching search response.
 */
export function caseSearchWithoutEngagementTypes(
  page: Page,
): Promise<Response> {
  return page.waitForResponse(
    (response) => {
      if (!isCaseSearch(response)) return false;
      const keys = searchFilters(
        response.request().postData(),
      )?.engagementTypeKeys;
      return !keys || keys.length === 0;
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

/** Whether a response came from a POST to the conversations search. */
function isConversationSearch(response: Response): boolean {
  return (
    response.request().method() === "POST" &&
    new URL(response.url()).pathname.endsWith("/conversations/search")
  );
}

/**
 * Starts waiting for a conversations search matching the given criteria.
 *
 * Call this **before** the action, then await it after.
 *
 * The conversations list posts the same `filters` / `sortBy` shape as the cases
 * search, to its own endpoint — so the search term, the state filter and the sort
 * are all observable on the wire, which is the only place they are unambiguous:
 * the list's heading reports the view, not the query.
 *
 * Only the fields given are matched. `stateKeys: false` matches a request that
 * carries none, which is how a cleared filter is recognised.
 *
 * Note the field is `stateKeys` — not `stateIds`, as the cases search would
 * suggest. Verified against the request the page builds.
 *
 * @param page - Test page.
 * @param match - What the request must carry.
 * @returns Promise for the matching search response.
 */
export function conversationSearchWith(
  page: Page,
  match: {
    searchQuery?: string;
    createdByMe?: boolean;
    stateKeys?: boolean;
    sortField?: string;
    sortOrder?: string;
  },
): Promise<Response> {
  return page.waitForResponse(
    (response) => {
      if (!isConversationSearch(response)) return false;
      const postData = response.request().postData();
      if (!postData) return false;
      try {
        const body = JSON.parse(postData) as {
          filters?: {
            searchQuery?: string;
            createdByMe?: boolean;
            stateKeys?: number[];
          };
          sortBy?: { field?: string; order?: string };
        };
        const filters = body.filters ?? {};

        if (
          match.searchQuery !== undefined &&
          filters.searchQuery !== match.searchQuery
        ) {
          return false;
        }
        if (
          match.createdByMe !== undefined &&
          Boolean(filters.createdByMe) !== match.createdByMe
        ) {
          return false;
        }
        if (match.stateKeys !== undefined) {
          const hasStates = !!filters.stateKeys && filters.stateKeys.length > 0;
          if (hasStates !== match.stateKeys) return false;
        }
        if (
          match.sortField !== undefined &&
          body.sortBy?.field !== match.sortField
        ) {
          return false;
        }
        if (
          match.sortOrder !== undefined &&
          body.sortBy?.order?.toLowerCase() !== match.sortOrder.toLowerCase()
        ) {
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
 * The `filters` block of a `/cases/search` request body, as the dashboard's
 * Outstanding Support Cases table builds it (`caseSearchRequest` in
 * CasesTable.tsx). Only the fields the dashboard specs read are typed.
 */
export interface CaseSearchFilters {
  caseTypes?: string[];
  statusIds?: number[];
  severityIds?: number[];
  issueIds?: number[];
  deploymentIds?: string[];
  createdBy?: string[];
  createdByMe?: boolean;
  /** Engagement searches only — numeric engagement-type ids. */
  engagementTypeKeys?: number[];
}

/** One case in a `/cases/search` response — the fields the specs compare. */
export interface CaseSearchResultItem {
  id: string;
  number: string;
  createdBy?: string | null;
  status?: { id: string; label: string } | null;
  issueType?: { id: string; label: string } | null;
  severity?: { id: string; label: string } | null;
  engagementType?: { id: string; label: string } | null;
}

/** A `/cases/search` response body. */
export interface CaseSearchResult {
  cases: CaseSearchResultItem[];
  totalRecords: number;
}

/**
 * Reads the `filters` block out of a case search's request.
 *
 * @param response - A `/cases/search` response.
 * @returns The request's filters, or undefined when the body is unparseable.
 */
export function caseSearchFiltersOf(
  response: Response,
): CaseSearchFilters | undefined {
  const postData = response.request().postData();
  if (!postData) return undefined;
  try {
    const body = JSON.parse(postData) as { filters?: CaseSearchFilters };
    return body.filters ?? {};
  } catch {
    return undefined;
  }
}

/**
 * Starts waiting for a search made by the dashboard's Outstanding Support Cases
 * table, matching a caller-supplied condition on its filters.
 *
 * Call this **before** the action, then await it after.
 *
 * Narrowed to `caseTypes: ["default_case"]` without `createdByMe`: the
 * dashboard's other searches (the engagements breakdown among them) hit the
 * same endpoint with other case types, and My Cases sets `createdByMe`, so
 * neither can resolve a wait meant for the All Cases table.
 *
 * @param page - Test page.
 * @param match - Condition the request's filters must satisfy.
 * @returns Promise for the matching search response.
 */
export function dashboardCasesSearch(
  page: Page,
  match: (filters: CaseSearchFilters) => boolean,
): Promise<Response> {
  return page.waitForResponse(
    (response) => {
      const filters = dashboardTableFiltersOf(response);
      return !!filters && match(filters);
    },
    { timeout: SEARCH_TIMEOUT_MS },
  );
}

/**
 * The filters of a search made by the dashboard's All Cases table, or
 * undefined for any other response. See {@link dashboardCasesSearch} for how
 * the table's searches are told apart from the page's others.
 */
function dashboardTableFiltersOf(
  response: Response,
): CaseSearchFilters | undefined {
  if (!isCaseSearch(response)) return undefined;
  const filters = caseSearchFiltersOf(response);
  if (!filters || filters.createdByMe) return undefined;
  if (
    filters.caseTypes?.length !== 1 ||
    filters.caseTypes[0] !== "default_case"
  ) {
    return undefined;
  }
  return filters;
}

/**
 * Records every search the dashboard's All Cases table makes from now on.
 *
 * Needed for the table's *first* load, which cannot be waited on with a
 * predicate armed in advance: the table searches once with the hard-coded
 * fallback statuses (`CASE_TABLE_OUTSTANDING_STATUS_IDS`) before
 * `GET /projects/{id}/filters` has answered, then again with the metadata's
 * non-Closed states. When the two sets coincide the query key does not change
 * and there is no second request — so a waiter armed for "the default set"
 * could have already missed its only match by the time the default set is
 * known. Recording from before navigation and picking afterwards has no such
 * race.
 *
 * Call this **before** navigating.
 *
 * @param page - Test page.
 * @returns A live array the recorded responses are appended to.
 */
export function recordDashboardCasesSearches(page: Page): Response[] {
  const recorded: Response[] = [];
  page.on("response", (response) => {
    if (dashboardTableFiltersOf(response)) recorded.push(response);
  });
  return recorded;
}

/**
 * Whether two id lists hold the same ids, ignoring order.
 *
 * @param actual - Ids the request carried, if any.
 * @param expected - Ids it should carry.
 * @returns True when both hold exactly the same set.
 */
export function sameIds<T extends string | number>(
  actual: readonly T[] | undefined,
  expected: readonly T[],
): boolean {
  if (!actual || actual.length !== expected.length) return false;
  const want = new Set(expected);
  return actual.every((id) => want.has(id));
}

/**
 * Whether a dashboard table search carries no user-chosen filter: no severity,
 * category, deployment or creator, and its statuses are the table's default
 * outstanding set rather than a selection.
 *
 * The dashboard never sends a search *without* `statusIds` — when none is
 * selected it sends every non-Closed state from `GET /projects/{id}/filters`
 * (`resolveCasesTableDefaultStatusIds`) — so "unfiltered" has to be recognised
 * by that default set rather than by the field's absence.
 *
 * @param filters - The request's filters.
 * @param defaultStatusIds - The project's non-Closed case-state ids.
 * @returns True for the table's unfiltered search.
 */
export function isUnfilteredDashboardSearch(
  filters: CaseSearchFilters,
  defaultStatusIds: readonly number[],
): boolean {
  return (
    sameIds(filters.statusIds, defaultStatusIds) &&
    !filters.severityIds?.length &&
    !filters.issueIds?.length &&
    !filters.deploymentIds?.length &&
    !filters.createdBy?.length
  );
}

/**
 * The `filters` block of the cases list's request when it is opened with
 * `?statusFilter=resolved` — the fields AllCasesPage's `caseSearchRequest` can
 * emit, so a stray one (a persisted search term, a created-date range) is
 * visible to the assertion rather than dropped by the type.
 */
export interface ResolvedCasesFilters extends CaseSearchFilters {
  closedStartDate?: string;
  closedEndDate?: string;
  searchQuery?: string;
  startCreatedDate?: string;
  endCreatedDate?: string;
  startUpdatedDate?: string;
  endUpdatedDate?: string;
}

/** A `/cases/search` request body as the cases list sends it. */
export interface CaseSearchRequestBody {
  filters?: ResolvedCasesFilters;
  pagination?: { offset?: number; limit?: number };
  sortBy?: { field?: string; order?: string };
}

/**
 * Reads a `/cases/search` request body.
 *
 * @param response - A `/cases/search` response.
 * @returns The parsed body, or undefined when absent or unparseable.
 */
export function caseSearchBodyOf(
  response: Response,
): CaseSearchRequestBody | undefined {
  const postData = response.request().postData();
  if (!postData) return undefined;
  try {
    return JSON.parse(postData) as CaseSearchRequestBody;
  } catch {
    return undefined;
  }
}

/**
 * Starts waiting for the first page of the "Resolved Cases (Last 30d)" list.
 *
 * Call this **before** clicking the card, then await it after.
 *
 * Matches a search carrying BOTH a closed-date window and a status filter.
 * AllCasesPage builds the window synchronously from the URL, but the Closed
 * status id arrives a render later — a `useEffect` looks it up in the filter
 * metadata and writes it into the filters — so the page can issue a first
 * search with the window and no status. Waiting for both is what ties this to
 * the request whose results the list settles on.
 *
 * @param page - Test page.
 * @returns Promise for the matching search response.
 */
export function resolvedCasesSearch(page: Page): Promise<Response> {
  return page.waitForResponse(
    (response) => {
      if (!isCaseSearch(response)) return false;
      const body = caseSearchBodyOf(response);
      const filters = body?.filters;
      return (
        !!filters?.closedStartDate &&
        !!filters.statusIds?.length &&
        (body?.pagination?.offset ?? 0) === 0
      );
    },
    { timeout: SEARCH_TIMEOUT_MS },
  );
}

/**
 * Reads a case's `closedOn` from `GET /cases/{id}`, as the signed-in user.
 *
 * The search response carries no closing date (the backend's `searchCases`
 * mapping omits it), so the only way to check *when* a listed case was closed
 * is its detail record. Read-only.
 *
 * The call reuses the auth headers of a request the app itself just made rather
 * than reading tokens out of storage, and goes through `page.request` so the
 * browser's CORS layer cannot turn a real status into an opaque failure.
 *
 * @param page - Test page.
 * @param searchResponse - A `/cases/search` response made by the app; its URL
 *   gives the backend base and its request gives the auth headers.
 * @param caseId - Case to read.
 * @returns The raw `closedOn` value (null when the case has none).
 */
export async function caseClosedOn(
  page: Page,
  searchResponse: Response,
  caseId: string,
): Promise<string | null> {
  const searchUrl = searchResponse.url();
  const baseUrl = searchUrl.slice(0, searchUrl.indexOf("/projects/"));
  const sent = await searchResponse.request().allHeaders();
  const headers: Record<string, string> = { Accept: "application/json" };
  for (const name of ["authorization", "x-user-id-token"]) {
    if (sent[name]) headers[name] = sent[name];
  }

  const response = await page.request.get(`${baseUrl}/cases/${caseId}`, {
    headers,
    timeout: SEARCH_TIMEOUT_MS,
  });
  if (!response.ok()) {
    throw new Error(
      `GET /cases/${caseId} failed: ${response.status()} ${response.statusText()}`,
    );
  }
  const body = (await response.json()) as { closedOn?: string | null };
  return body.closedOn ?? null;
}

/**
 * Parses a backend timestamp the way the app does
 * (`normalizeBackendTimestamp` in src/utils/dateTime.ts): an explicit zone is
 * honoured, an unzoned value is UTC.
 *
 * @param raw - Timestamp as the backend sent it.
 * @returns Epoch milliseconds, or NaN when unparseable.
 */
export function parseBackendUtc(raw: string): number {
  const trimmed = raw.trim();
  if (/(Z|[+-]\d{2}:?\d{2})$/.test(trimmed)) return Date.parse(trimmed);
  const parts =
    /^(\d{4})-(\d{1,2})-(\d{1,2})[ T](\d{1,2}):(\d{1,2}):(\d{1,2})(\.\d+)?$/.exec(
      trimmed,
    );
  if (!parts) return Number.NaN;
  const [, y, mo, d, h, mi, s, frac = ""] = parts;
  const pad = (v: string): string => v.padStart(2, "0");
  return Date.parse(
    `${y}-${pad(mo!)}-${pad(d!)}T${pad(h!)}:${pad(mi!)}:${pad(s!)}${frac}Z`,
  );
}

/**
 * Starts waiting for the Engagements list's search when it is opened from the
 * dashboard's Outstanding Engagements chart.
 *
 * Call this **before** clicking the chart, then await it after.
 *
 * The list can search twice on arrival: the outstanding `statusIds` are
 * derived from `GET /projects/{id}/filters`, so if that metadata is not cached
 * yet the first request carries the type but no states. Only the request
 * carrying both is the drill-down's real query, so both are required here.
 *
 * `engagementTypeKeys` is compared as a set against `typeIds`. An absent field
 * is read as empty, so passing `[]` matches a search that carries no type ids
 * — which is what the page sends for a legend entry the stats gave no backend
 * id (the legend then falls back to the category *name* as its id, and the
 * page's `Number()` split drops it).
 *
 * @param page - Test page.
 * @param typeIds - The engagement-type ids the request must carry, exactly.
 * @returns Promise for the matching search response.
 */
export function engagementChartSearch(
  page: Page,
  typeIds: readonly number[],
): Promise<Response> {
  return page.waitForResponse(
    (response) => {
      if (!isCaseSearch(response)) return false;
      const filters = caseSearchFiltersOf(response);
      return (
        !!filters &&
        filters.caseTypes?.length === 1 &&
        filters.caseTypes[0] === "engagement" &&
        (filters.statusIds?.length ?? 0) > 0 &&
        sameIds(filters.engagementTypeKeys ?? [], typeIds)
      );
    },
    { timeout: SEARCH_TIMEOUT_MS },
  );
}
