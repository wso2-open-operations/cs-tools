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

import { infiniteQueryOptions, queryOptions } from "@tanstack/react-query";
import {
  CASES_ENDPOINT,
  CASES_SEARCH_ENDPOINT,
  CASE_COMMENTS_ENDPOINT,
  CASE_COMMENTS_SEARCH_ENDPOINT,
  CASE_DETAILS_ENDPOINT,
} from "@config/endpoints";
import type {
  CaseCommentCreatePayloadDto,
  CaseCommentCreateResponseDto,
  CaseCommentSearchResponseDto,
  CaseCreatePayloadDto,
  CaseCreateResponseDto,
  CasePatchPayloadDto,
  CaseSearchFiltersDto,
  CaseSearchPayloadDto,
  CaseSearchResponseDto,
  CaseSeverity,
  CaseState,
  CaseType,
  CaseViewDto,
  CreatedCaseDto,
  SecurityReportCreatePayloadDto,
  ServiceRequestCreatePayloadDto,
  UpdateCaseResponseDto,
} from "@src/types";
import { toCaseDetail, toCaseSummary, toComment, type CaseDetail, type CaseSummary, type Comment } from "@src/types";
import apiClient from "./apiClient";

export interface CaseSearchResult {
  items: CaseSummary[];
  total: number;
  limit: number;
  offset: number;
  hasMore: boolean;
}

interface CaseFieldFilter {
  field: string;
  op: "eq" | "in" | "notIn" | "isEmpty" | "isNotEmpty" | "gte" | "lte";
  values?: string[];
}

// The literal `values` entry a createdBy+eq filter must carry to mean "the authenticated
// caller" — mirrors entity-service's case_filters.go currentUserFilterPlaceholder.
const CURRENT_USER_FILTER_PLACEHOLDER = "__current_user_email__";

// /cases/search's wire contract is the generic field/op/values filter array (see
// entity-service's case_filters.go ParseCaseFieldFilters), not the named-field shape
// CaseSearchFiltersDto's callers build for convenience — mirrors the webapp's
// buildCaseSearchFilters (caseSearchPayload.ts). This is the one place that translates between
// them, so every call site below can keep building the convenience shape.
function toWireFilters(filters: CaseSearchFiltersDto = {}): { searchQuery?: string; filters?: CaseFieldFilter[] } {
  const fieldFilters: CaseFieldFilter[] = [];
  if (filters.types?.length) fieldFilters.push({ field: "type", op: "in", values: filters.types });
  if (filters.states?.length) fieldFilters.push({ field: "state", op: "in", values: filters.states });
  if (filters.severities?.length) fieldFilters.push({ field: "severity", op: "in", values: filters.severities });
  if (filters.workStates?.length) fieldFilters.push({ field: "workState", op: "in", values: filters.workStates });
  if (filters.issueTypes?.length) fieldFilters.push({ field: "issueType", op: "in", values: filters.issueTypes });
  if (filters.engagementTypes?.length)
    fieldFilters.push({ field: "engagementType", op: "in", values: filters.engagementTypes });
  if (filters.projectIds?.length) fieldFilters.push({ field: "projectId", op: "in", values: filters.projectIds });
  if (filters.deploymentIds?.length)
    fieldFilters.push({ field: "deploymentId", op: "in", values: filters.deploymentIds });
  if (filters.assignedUserIds?.length)
    fieldFilters.push({ field: "assignedUserId", op: "in", values: filters.assignedUserIds });
  if (filters.productNames?.length) fieldFilters.push({ field: "product", op: "in", values: filters.productNames });
  if (filters.createdByMe)
    fieldFilters.push({ field: "createdBy", op: "eq", values: [CURRENT_USER_FILTER_PLACEHOLDER] });
  if (filters.parentId) fieldFilters.push({ field: "parentId", op: "eq", values: [filters.parentId] });

  return {
    ...(filters.searchQuery && { searchQuery: filters.searchQuery }),
    ...(fieldFilters.length > 0 && { filters: fieldFilters }),
  };
}

// The one place any caller — inside this module or elsewhere in the app — should hit
// `/cases/search` from: applies the CaseSearchFiltersDto -> wire-contract translation above, so
// no other module needs to know the endpoint expects the generic filter array. Exported (not just
// used internally) for announcements.ts/engagements.ts/securityReports.ts, which each scope the
// same endpoint to one case `type` and previously posted to it directly with the old shape.
export async function postCasesSearch(payload: CaseSearchPayloadDto = {}): Promise<CaseSearchResponseDto> {
  const { data } = await apiClient.post<CaseSearchResponseDto>(CASES_SEARCH_ENDPOINT, {
    ...payload,
    filters: toWireFilters(payload.filters),
  });
  return data;
}

// Exported (not just used internally) so the Home dashboard's composition query can fan out
// count-only searches (`pagination: { limit: 1 }`, read `.total`) without going through the
// `cases.all` query-options wrapper — mirrors the webapp's useCaseComposition.ts, which calls its
// api client directly for the same reason.
export const getAllCases = async (payload: CaseSearchPayloadDto = {}): Promise<CaseSearchResult> => {
  const data = await postCasesSearch(payload);
  const items = data.cases.map(toCaseSummary);
  return {
    items,
    total: data.total,
    limit: data.limit,
    offset: data.offset,
    // Some data sources omit hasMore from the search envelope (see adminUsers.ts's searchUsers,
    // which hits the same quirk on /users/search); derive it from offset/total when that happens
    // instead of treating a missing field as "no more pages".
    hasMore: data.hasMore ?? data.offset + items.length < data.total,
  };
};

const getCase = async (id: string): Promise<CaseDetail> => {
  const { data } = await apiClient.get<CaseViewDto>(CASE_DETAILS_ENDPOINT(id));
  return toCaseDetail(data);
};

/** A case the current user is actively working (in-progress + ongoing) — surfaced by the
 * single-active-case conflict dialog when resuming/starting work on a different case.
 * `id` is null when the case is only known by the number ServiceNow's own 409 named (the search
 * that would resolve its UUID for an automatic pause came back empty) — the dialog still shows
 * it and blocks the action, it just can't offer a one-tap auto-pause for that entry. */
export interface MyOngoingCase {
  id: string | null;
  label: string;
}

// SupportPage.tsx's search (the only other place this app calls /cases/search with a text query —
// via components/support/filters.ts's toCaseSearchFilters) always sends an explicit `types` list
// alongside `searchQuery`. The searches below omitted it and consistently returned 0 results even
// for a case ServiceNow itself just confirmed exists (searchTotal=0 for an exact number match) —
// omitting `types` most likely scopes the search to nothing rather than "all types". Always send
// the full list explicitly instead of assuming an omitted filter means unrestricted.
const ALL_CASE_TYPES: CaseType[] = [
  "case",
  "service_request",
  "security_report_analysis",
  "engagement",
  "announcement",
];

// Mirrors the webapp's useFindMyOngoingCases.ts exactly, including its dual-path design:
//   - myUserId known: filter server-side by assignedUserIds — the result set is tiny (an
//     engineer's own ongoing cases), so this fits one page.
//   - myUserId unavailable (e.g. /users/me omitted it): fall back to an unfiltered-by-assignee
//     search matched client-side by email instead, paging until results are exhausted. Without
//     this fallback, a missing id would silently skip the whole check and let the 409 through —
//     confirmed live: PATCH { workState: "ongoing" } 409s with "[SERVICENOW_ERROR] Cannot set
//     work state to Ongoing — the assigned engineer already has an Ongoing case: <number>".
//   - Either way, `workState` is re-checked case-insensitively client-side as a backstop: the
//     search response can echo the raw ServiceNow label ("Ongoing") rather than the lowercased
//     enum the detail endpoint returns.
const CASES_SEARCH_MAX_PAGES = 20;

const findMyOngoingCases = async (
  excludeCaseId: string,
  myUserId: string | null,
  myEmail: string | null,
): Promise<MyOngoingCase[]> => {
  if (!myUserId && !myEmail) return [];

  const matches: MyOngoingCase[] = [];
  for (let page = 0; page < CASES_SEARCH_MAX_PAGES; page += 1) {
    const result = await getAllCases({
      filters: {
        types: ALL_CASE_TYPES,
        states: ["work_in_progress"],
        workStates: ["ongoing"],
        ...(myUserId ? { assignedUserIds: [myUserId] } : {}),
      },
      pagination: { offset: page * CASES_PAGE_LIMIT, limit: CASES_PAGE_LIMIT },
    });
    for (const c of result.items) {
      if (c.id === excludeCaseId) continue;
      if (c.workState?.toLowerCase() !== "ongoing") continue;
      if (!myUserId && c.assignedEngineer?.email?.toLowerCase() !== myEmail?.toLowerCase()) continue;
      matches.push({ id: c.id, label: c.wso2Id || c.number || c.subject || c.id });
    }
    // Assignee-filtered path: this tiny set fits one page and breaks here.
    // Email fallback: keep paging until the search results are exhausted.
    if (result.items.length < CASES_PAGE_LIMIT || !result.hasMore) break;
  }
  return matches;
};

// The upstream ServiceNow instance enforces the single-ongoing-case rule itself (this message is
// tagged [SERVICENOW_ERROR] — it never appears anywhere in the portal's own Go backend), so
// findMyOngoingCases's proactive search is only ever a *prediction* of what ServiceNow will do,
// not the source of truth. If the prediction misses (assignee-id/email mismatch, timing, a case
// updated by someone else moments ago, etc.) the PATCH still 409s with this exact message — which
// conveniently already names the real conflicting case by number. Parse it out and resolve it to
// a MyOngoingCase via search, so the pause-conflict dialog can still recover instead of just
// surfacing a dead-end error.
const ONGOING_CONFLICT_MESSAGE_RE = /already has an Ongoing case:\s*([A-Za-z0-9-]+)/i;

export function parseOngoingConflictCaseNumber(message: string | undefined | null): string | null {
  if (!message) return null;
  return ONGOING_CONFLICT_MESSAGE_RE.exec(message)?.[1] ?? null;
}

// Temporary debug shape — total hit count + a peek at what the search actually returned, so a
// null match can be told apart from "search found nothing" vs "search found something that
// doesn't match on number/wso2Id the way expected."
export interface FindCaseByNumberDebug {
  match: MyOngoingCase | null;
  searchTotal: number;
  searchSample: Array<{ id: string; number: string; wso2Id: string }>;
}

const findCaseByNumberDebug = async (caseNumber: string): Promise<FindCaseByNumberDebug> => {
  const result = await getAllCases({
    filters: { types: ALL_CASE_TYPES, searchQuery: caseNumber },
    pagination: { limit: CASES_PAGE_LIMIT },
  });
  const match = result.items.find(
    (c) => c.number?.toLowerCase() === caseNumber.toLowerCase() || c.wso2Id?.toLowerCase() === caseNumber.toLowerCase(),
  );
  return {
    match: match ? { id: match.id, label: match.wso2Id || match.number || match.id } : null,
    searchTotal: result.total,
    searchSample: result.items.slice(0, 3).map((c) => ({ id: c.id, number: c.number, wso2Id: c.wso2Id })),
  };
};

// Always resolves to a MyOngoingCase — never null — so the caller can always show the
// pause-conflict dialog once ServiceNow's error names a conflicting case, even when the search
// that would find its UUID comes back empty (confirmed live: it does, for at least some cases).
// A null `id` just means that specific entry can't be auto-paused from the dialog.
const findCaseByNumber = async (caseNumber: string): Promise<MyOngoingCase> => {
  const { match } = await findCaseByNumberDebug(caseNumber);
  return match ?? { id: null, label: caseNumber };
};

// openapi.yaml declares this endpoint's pagination.limit maximum as 100, but the live upstream
// entity service actually rejects 100 with a 400 — confirmed by hitting it directly. The webapp's
// own BE_MAX_PAGE_LIMIT constant (apiConstants.ts) is 50, not 100 as the spec claims; match that
// real, working value instead of the doc.
const COMMENTS_PAGE_LIMIT = 50;

// Upper bound on pages fetched for one case's thread (50 per page), so a runaway thread cannot
// turn one query into an unbounded request loop.
const COMMENTS_MAX_PAGES = 40;

const getCaseComments = async (id: string): Promise<Comment[]> => {
  const comments: Comment[] = [];
  for (let page = 0; page < COMMENTS_MAX_PAGES; page += 1) {
    const { data } = await apiClient.post<CaseCommentSearchResponseDto>(CASE_COMMENTS_SEARCH_ENDPOINT(id), {
      pagination: { offset: page * COMMENTS_PAGE_LIMIT, limit: COMMENTS_PAGE_LIMIT },
    });
    comments.push(...data.comments.map(toComment));
    const hasMore = data.hasMore ?? data.offset + data.comments.length < data.total;
    if (!hasMore || data.comments.length === 0) break;
  }
  return comments;
};

const createCase = async (
  payload: CaseCreatePayloadDto | SecurityReportCreatePayloadDto | ServiceRequestCreatePayloadDto,
): Promise<CreatedCaseDto> => {
  const { data } = await apiClient.post<CaseCreateResponseDto>(CASES_ENDPOINT, payload);
  return data.case;
};

const patchCase = async (id: string, payload: CasePatchPayloadDto): Promise<UpdateCaseResponseDto> => {
  const { data } = await apiClient.patch<UpdateCaseResponseDto>(CASE_DETAILS_ENDPOINT(id), payload);
  return data;
};

// The create response is a thin ack (id/createdOn/createdBy only — see
// CaseCommentCreateResponseDto), not the full comment. Callers refetch the comments list to
// hydrate the new entry rather than rely on this response for rendering.
const postComment = async (id: string, payload: CaseCommentCreatePayloadDto): Promise<void> => {
  await apiClient.post<CaseCommentCreateResponseDto>(CASE_COMMENTS_ENDPOINT(id), payload);
};

const CASES_PAGE_LIMIT = 20;

export interface ChildCaseRow {
  id: string;
  number: string;
  subject: string;
  severity: CaseSeverity | null;
  state: CaseState;
  assigneeName: string | null;
}

const CHILD_CASES_LIMIT = 20;

// Child cases of `parentCaseId` — cases whose own `parentId` points at it (the hierarchical
// major-case/child-case relationship). Reuses the general cross-project search rather than a
// dedicated endpoint, mirroring the webapp's useSearchChildCases. Single page (20 cases) — a case
// with more children than that isn't expected, same assumption the webapp makes.
const searchChildCases = async (parentCaseId: string): Promise<{ cases: ChildCaseRow[]; total: number }> => {
  const result = await getAllCases({
    filters: { types: ALL_CASE_TYPES, parentId: parentCaseId },
    pagination: { offset: 0, limit: CHILD_CASES_LIMIT },
  });
  return {
    cases: result.items.map((c) => ({
      id: c.id,
      number: c.number,
      subject: c.subject,
      severity: c.severity,
      state: c.state,
      assigneeName: c.assignedEngineer?.name ?? null,
    })),
    total: result.total,
  };
};

// Don't fire LinkCaseDialog's search until the user has typed something searchable — mirrors the
// webapp's QUICK_CASE_MIN_QUERY_LEN.
export const QUICK_SEARCH_MIN_QUERY_LEN = 2;
const QUICK_SEARCH_LIMIT = 8;

// Free-text case lookup for LinkCaseDialog's search-and-pick — the same `/cases/search` the cases
// list itself uses. Explicitly requests every case sub-type (mirrors the webapp's
// useQuickCaseSearch): entity-service defaults `filters.types` to `["case"]` only when the caller
// omits it, which would otherwise hide service requests/security reports/etc. from this search.
const quickSearchCases = async (query: string): Promise<CaseSummary[]> => {
  const trimmed = query.trim();
  if (trimmed.length < QUICK_SEARCH_MIN_QUERY_LEN) return [];
  const result = await getAllCases({
    filters: { types: ALL_CASE_TYPES, searchQuery: trimmed },
    pagination: { offset: 0, limit: QUICK_SEARCH_LIMIT },
  });
  return result.items;
};

export const cases = {
  all: (payload: CaseSearchPayloadDto = {}) =>
    queryOptions({
      queryKey: ["cases", payload],
      queryFn: () => getAllCases(payload),
    }),

  // The full "View All" list page: same filters as the recent-5 view but paged via infinite
  // scroll, mirroring the webapp's useGetCsmCases.ts (updatedOn desc, page-by-offset).
  infinite: (filters: CaseSearchFiltersDto) =>
    infiniteQueryOptions({
      queryKey: ["cases", "infinite", filters],
      queryFn: ({ pageParam }) =>
        getAllCases({
          filters,
          sortBy: { field: "updatedOn", order: "desc" },
          pagination: { offset: pageParam, limit: CASES_PAGE_LIMIT },
        }),
      initialPageParam: 0,
      getNextPageParam: (lastPage) => (lastPage.hasMore ? lastPage.offset + lastPage.limit : undefined),
    }),

  get: (id: string) =>
    queryOptions({
      queryKey: ["case", id],
      queryFn: () => getCase(id),
    }),

  comments: (id: string) =>
    queryOptions({
      queryKey: ["case", id, "comments"],
      queryFn: () => getCaseComments(id),
    }),

  // Linked Items tab: child cases (own `parentId` pointing at this one) — shared by both the
  // Child cases card and Linked service requests' enrichment (same underlying relationship, see
  // that widget's own comment), so both queries dedupe against this one cache entry.
  searchChildren: (caseId: string | undefined) =>
    queryOptions({
      queryKey: ["cases", "children", caseId ?? ""],
      queryFn: () => (caseId ? searchChildCases(caseId) : Promise.resolve({ cases: [], total: 0 })),
      enabled: !!caseId,
      staleTime: 30_000,
    }),

  // Linked Items tab: LinkCaseDialog's search-and-pick.
  quickSearch: (query: string) =>
    queryOptions({
      queryKey: ["cases", "quick-search", query.trim()],
      queryFn: () => quickSearchCases(query),
      enabled: query.trim().length >= QUICK_SEARCH_MIN_QUERY_LEN,
      staleTime: 15_000,
    }),

  create: createCase,
  patch: patchCase,
  postComment,
  findMyOngoingCases,
  findCaseByNumber,
  findCaseByNumberDebug,
};
