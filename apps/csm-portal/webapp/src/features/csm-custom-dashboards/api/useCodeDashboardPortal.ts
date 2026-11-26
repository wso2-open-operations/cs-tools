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

import { useMemo } from "react";
import { useBackendApi } from "@api/backend/client";
import type {
  BeCaseSearchResponse,
  BeCaseSearchView,
  BeCaseSortField,
  BeCaseVolumeTrendResponse,
} from "@api/backend/types";
import { BE_MAX_PAGE_LIMIT } from "@constants/apiConstants";
import { useCurrentUser } from "@context/current-user/CurrentUserContext";
import {
  buildCaseResolutionTimeRequest,
  toCaseResolutionTimeResult,
  type CaseResolutionTimeRequest,
  type CaseResolutionTimeResponse,
} from "@features/csm-custom-dashboards/api/codeDashboardResolutionTime";
import {
  buildScheduleSearchRequest,
  toSearchAbsencesResult,
  toSearchRotaResult,
  type ScheduleSearchRequest,
} from "@features/csm-custom-dashboards/api/codeDashboardTeamSchedule";
import {
  buildSearchCustomerHealthRequest,
  createNameLookup,
  toSearchCustomerHealthResult,
  type CustomerHealthSummaryRequest,
} from "@features/csm-custom-dashboards/api/codeDashboardCustomerHealth";
import {
  buildSearchAccountsRequest,
  buildSearchProjectsRequest,
  toSearchAccountsResult,
  toSearchProjectsResult,
  type AccountSearchRequest,
  type ProjectSearchRequest,
} from "@features/csm-custom-dashboards/api/codeDashboardCustomers";
import {
  buildCountPlgRequest,
  buildPlgSummaryPath,
  buildPlgTrendRequest,
  buildSearchPlgRequest,
  createPlaybookCodeChecker,
  toCountPlgResult,
  toPlgSummaryResult,
  toPlgTrendResult,
  toSearchPlgResult,
  type PlgCountResponse,
  type PlgSummaryResponse,
  type PlgTrendRequest,
  type PlgTrendResponse,
} from "@features/csm-custom-dashboards/api/codeDashboardPlg";
import { buildTrendWindow, type TrendWindow } from "@features/csm-custom-dashboards/api/codeDashboardTrendWindow";
import {
  buildCountFeedbackRequest,
  buildSearchFeedbackRequest,
  toCountFeedbackResult,
  toSearchFeedbackResult,
  type FeedbackAggregateRequest,
  type FeedbackAggregateResponse,
  type FeedbackSearchRequest,
} from "@features/csm-custom-dashboards/api/codeDashboardFeedback";
import {
  buildCountOperationsRequest,
  buildSearchOperationsRequest,
  toCountOperationsResult,
  toSearchOperationsResult,
  type OperationsAggregateRequest,
  type OperationsAggregateResponse,
  type OperationsSearchRequest,
} from "@features/csm-custom-dashboards/api/codeDashboardOperations";
import {
  buildCountTimeCardsRequest,
  buildSearchTimeCardsRequest,
  createTeamListResolver,
  toCountTimeCardsResult,
  toSearchTimeCardsResult,
  type TimeCardAggregateRequest,
  type TimeCardAggregateResponse,
  type TimeCardSearchRequest,
  type TimeCardSearchResponse,
} from "@features/csm-custom-dashboards/api/codeDashboardTimeCards";

/** A fixed portal API a code dashboard may call: takes the call's args, returns its result. */
export type CodeDashboardPortalApi = (args: unknown) => unknown;

/**
 * The fields a code dashboard may group cases by: entity-service's aggregate
 * allow-list. `productVersion` also needs a product filter naming one product
 * (`namesOneProduct`).
 */
const CASE_GROUP_BY_FIELDS = ["account", "state", "severity", "type", "product", "productVersion", "project", "region"] as const;
type CaseGroupByField = (typeof CASE_GROUP_BY_FIELDS)[number];

/** Body of POST /cases/aggregate. */
interface CaseAggregateRequest {
  filters: { filters: unknown[] };
  groupBy: CaseGroupByField;
  maxGroups?: number;
}

/** Response of POST /cases/aggregate (backend `AggregateResponse`). */
interface CaseAggregateResponse {
  groups?: { key: string; label: string; count: number }[];
  othersCount?: number;
  totalRecords?: number;
}

/** What `portal.countCases` returns to the code: `groups` only when it asked for a `groupBy`. */
export interface CountCasesResult {
  total: number;
  groups?: { label: string; count: number }[];
}

/**
 * Turns `portal.countCases(args)` into a `POST /cases/aggregate` body, or
 * throws (the code sees the message) when the args are not `{filters?, groupBy?, maxGroups?}`.
 * The endpoint always needs a `groupBy`; a plain count groups by `state`, which every case has,
 * and keeps only the total.
 */
export function buildCountCasesRequest(args: unknown): { body: CaseAggregateRequest; grouped: boolean } {
  const { filters, groupBy, maxGroups } = (args ?? {}) as { filters?: unknown; groupBy?: unknown; maxGroups?: unknown };
  if (filters !== undefined && !Array.isArray(filters)) {
    throw new Error("countCases: filters must be an array of {field, op, values}");
  }
  if (groupBy !== undefined && !CASE_GROUP_BY_FIELDS.includes(groupBy as CaseGroupByField)) {
    throw new Error(`countCases: groupBy must be one of ${CASE_GROUP_BY_FIELDS.join(", ")}`);
  }
  if (maxGroups !== undefined && !(Number.isInteger(maxGroups) && (maxGroups as number) > 0)) {
    throw new Error("countCases: maxGroups must be a positive integer");
  }
  if (groupBy === "productVersion" && !namesOneProduct(filters ?? [])) {
    throw new Error('countCases: groupBy productVersion needs a product filter (op "in") naming exactly one product');
  }
  return {
    body: {
      filters: { filters: filters ?? [] },
      groupBy: (groupBy as CaseGroupByField | undefined) ?? "state",
      ...(maxGroups !== undefined ? { maxGroups: maxGroups as number } : {}),
    },
    grouped: groupBy !== undefined,
  };
}

/**
 * Whether a case filter list keeps exactly one product (its `product` `in`
 * filters name one product between them): a version means something only
 * within one product.
 */
function namesOneProduct(filters: unknown[]): boolean {
  const products = new Set<unknown>();
  for (const f of filters) {
    const { field, op, values } = (f ?? {}) as { field?: unknown; op?: unknown; values?: unknown };
    if (field === "product" && op === "in" && Array.isArray(values)) values.forEach((v) => products.add(v));
  }
  return products.size === 1;
}

/** Reshapes a `POST /cases/aggregate` response into what `portal.countCases` returns. */
export function toCountCasesResult(res: CaseAggregateResponse, grouped: boolean): CountCasesResult {
  const total = res.totalRecords ?? 0;
  if (!grouped) return { total };
  const groups = (res.groups ?? []).map(({ label, count }) => ({ label, count }));
  if (res.othersCount) groups.push({ label: "Others", count: res.othersCount });
  return { total, groups };
}

/** The fields `POST /cases/search` can sort by (`BeCaseSortField`). */
const CASE_SORT_FIELDS: readonly BeCaseSortField[] = ["createdOn", "updatedOn", "severity", "state", "assignee"];

/** How many cases `portal.searchCases` returns when the code does not say. */
const SEARCH_CASES_DEFAULT_LIMIT = 10;

/** Body of POST /cases/search, as `portal.searchCases` sends it: always the first page. */
interface CaseSearchRequest {
  filters: { filters: unknown[] };
  sortBy?: { field: BeCaseSortField; order: "asc" | "desc" };
  pagination: { offset: 0; limit: number };
}

/** One case as `portal.searchCases` returns it: plain values, related records by name. */
export interface CodeDashboardCase {
  id: string;
  number: string | null;
  subject: string | null;
  type: string | null;
  severity: string | null;
  state: string | null;
  escalationLevel: string | null;
  createdOn: string | null;
  updatedOn: string | null;
  /** When it was resolved, and closed; `null` while it is not (a closed case may have no resolved time). */
  resolvedOn: string | null;
  closedOn: string | null;
  account: string | null;
  project: string | null;
  assignee: string | null;
}

/** What `portal.searchCases` returns to the code. */
export interface SearchCasesResult {
  total: number;
  items: CodeDashboardCase[];
}

/**
 * Turns `portal.searchCases(args)` into a `POST /cases/search` body, or throws
 * (the code sees the message) when the args are not
 * `{filters?, sortBy?: {field, order?}, limit?}`. `limit` is 1 to the backend's
 * page cap (50) and defaults to 10; only the first page is read.
 */
export function buildSearchCasesRequest(args: unknown): CaseSearchRequest {
  const { filters, sortBy, limit } = (args ?? {}) as { filters?: unknown; sortBy?: unknown; limit?: unknown };
  if (filters !== undefined && !Array.isArray(filters)) {
    throw new Error("searchCases: filters must be an array of {field, op, values}");
  }
  if (limit !== undefined && !(Number.isInteger(limit) && (limit as number) >= 1 && (limit as number) <= BE_MAX_PAGE_LIMIT)) {
    throw new Error(`searchCases: limit must be an integer from 1 to ${BE_MAX_PAGE_LIMIT}`);
  }
  let sort: CaseSearchRequest["sortBy"];
  if (sortBy !== undefined) {
    const { field, order } = (sortBy ?? {}) as { field?: unknown; order?: unknown };
    if (!CASE_SORT_FIELDS.includes(field as BeCaseSortField)) {
      throw new Error(`searchCases: sortBy.field must be one of ${CASE_SORT_FIELDS.join(", ")}`);
    }
    if (order !== undefined && order !== "asc" && order !== "desc") {
      throw new Error('searchCases: sortBy.order must be "asc" or "desc"');
    }
    sort = { field: field as BeCaseSortField, order: (order as "asc" | "desc" | undefined) ?? "desc" };
  }
  return {
    filters: { filters: filters ?? [] },
    ...(sort ? { sortBy: sort } : {}),
    pagination: { offset: 0, limit: (limit as number | undefined) ?? SEARCH_CASES_DEFAULT_LIMIT },
  };
}

/**
 * Reshapes a `POST /cases/search` response into what `portal.searchCases`
 * returns. Only the listed fields are passed on (no description or other
 * free-text bodies), so the code gets what a list or table needs and nothing more.
 */
export function toSearchCasesResult(res: Partial<BeCaseSearchResponse>): SearchCasesResult {
  const items = (res.cases ?? []).map(
    (c: BeCaseSearchView): CodeDashboardCase => ({
      id: c.id,
      number: c.number ?? null,
      subject: c.subject ?? null,
      type: c.type ?? null,
      severity: c.severity ?? null,
      state: c.state ?? null,
      escalationLevel: c.escalationLevel ?? null,
      createdOn: c.createdOn ?? null,
      updatedOn: c.updatedOn ?? null,
      resolvedOn: c.resolvedOn ?? null,
      closedOn: c.closedOn ?? null,
      account: c.account?.name ?? null,
      project: c.project?.name ?? null,
      assignee: c.assignedEngineer?.name ?? null,
    }),
  );
  return { total: res.total ?? items.length, items };
}

/** Body of POST /cases/volume-trend: a trailing `days` window or a fixed one. */
type CaseTrendRequest = { filters: { filters: unknown[] } } & TrendWindow;

/** One period of `portal.caseTrend`: cases opened (created) and closed in it. */
export interface CaseTrendPoint {
  date: string;
  endDate?: string;
  opened: number;
  closed: number;
}

/**
 * Turns `portal.caseTrend(args)` into a `POST /cases/volume-trend` body, or
 * throws (the code sees the message) when the args are not
 * `{filters?, days?}` or `{filters?, startDate, endDate, bucket?}`
 * (`buildTrendWindow`).
 */
export function buildCaseTrendRequest(args: unknown): CaseTrendRequest {
  const { filters, days, startDate, endDate, bucket } = (args ?? {}) as {
    filters?: unknown;
    days?: unknown;
    startDate?: unknown;
    endDate?: unknown;
    bucket?: unknown;
  };
  if (filters !== undefined && !Array.isArray(filters)) {
    throw new Error("caseTrend: filters must be an array of {field, op, values}");
  }
  return {
    filters: { filters: (filters as unknown[] | undefined) ?? [] },
    ...buildTrendWindow("caseTrend", { days, startDate, endDate, bucket }),
  };
}

/**
 * Reshapes a `POST /cases/volume-trend` response into what `portal.caseTrend`
 * returns: one point per period, oldest first, with both counts. The two
 * series share their dates (the backend's contract); they are matched by
 * label, so their order does not matter.
 */
export function toCaseTrendResult(res: BeCaseVolumeTrendResponse): { points: CaseTrendPoint[] } {
  const series = (label: string) => res.series?.find((s) => s.label === label)?.points ?? [];
  const closed = new Map(series("Closed").map((p) => [p.date, p.count]));
  const points = series("Opened").map((p) => ({
    date: p.date,
    ...(p.endDate ? { endDate: p.endDate } : {}),
    opened: p.count,
    closed: closed.get(p.date) ?? 0,
  }));
  return { points };
}

/**
 * The case filter fields a code dashboard writes as names, as the user wrote
 * them (a product, version, project, customer, CRE/SRE team or region). The
 * case search takes ids or exact names instead (and counts nothing, or
 * everything, for a region no account has), and code has no step after
 * generation to swap or check them, so they are resolved when the call runs.
 */
const NAMED_FILTER_FIELDS = new Set(["product", "productVersion", "project", "account", "creTeam", "sreTeam", "region"]);

/** Whether a case filter list names a product, version, project, customer, team or region. */
export function hasNamedFilter(filters: unknown[]): boolean {
  return filters.some((f) => typeof f === "object" && f !== null && NAMED_FILTER_FIELDS.has((f as { field?: unknown }).field as string));
}

/** Body and reply of `POST /dashboards/resolved-filters`. */
interface ResolvedFiltersBody {
  filters: unknown[];
}

/**
 * Returns a function turning a case filter list into the one the case search
 * takes: a list naming nothing is returned as it is; any other is sent to
 * `POST /dashboards/resolved-filters` (the backend's own lookups, the JSON
 * approach's rules), once per distinct list for the resolver's life, so one
 * frame's repeated calls look a name up once. A failed lookup is not kept,
 * and its error (with the backend's reason, e.g. a project that matches
 * nothing) reaches the code.
 */
export function createNamedFilterResolver(
  post: (body: ResolvedFiltersBody) => Promise<ResolvedFiltersBody>,
): (filters: unknown[]) => Promise<unknown[]> {
  const cache = new Map<string, Promise<unknown[]>>();
  return (filters) => {
    if (!hasNamedFilter(filters)) return Promise.resolve(filters);
    const key = JSON.stringify(filters);
    let resolved = cache.get(key);
    if (!resolved) {
      resolved = post({ filters }).then((res) => res.filters);
      cache.set(key, resolved);
      resolved.catch(() => cache.delete(key));
    }
    return resolved;
  };
}

/**
 * The fixed `portal.*` APIs CodeDashboardFrame answers.
 * `currentUser` is the signed-in user; `countCases` calls `POST /cases/aggregate`,
 * `searchCases` `POST /cases/search`, `caseTrend` `POST /cases/volume-trend`,
 * `caseResolutionTime` `POST /cases/resolution-time` (`codeDashboardResolutionTime.ts`),
 * and `countTimeCards` / `searchTimeCards` `POST /time-cards/aggregate` /
 * `/search` (`codeDashboardTimeCards.ts`), and `countOperations` /
 * `searchOperations` an incident's, change request's or problem's own
 * `/aggregate` / `/search` (`codeDashboardOperations.ts`), and
 * `searchAccounts` / `searchProjects` `POST /accounts/search` /
 * `/projects/search` (`codeDashboardCustomers.ts`), and
 * `searchCustomerHealth` `POST /customer-health/summary`, its product and
 * team checked against `GET /products` / `GET /abt-teams`
 * (`codeDashboardCustomerHealth.ts`), and `countFeedback` /
 * `searchFeedback` `POST /cases/feedback/aggregate` / `/search`
 * (`codeDashboardFeedback.ts`), and `countPlg` / `searchPlg` / `plgTrend` the
 * PLG organisation and registration searches, aggregates and sign-up trend
 * (`codeDashboardPlg.ts`), and `plgSummary` `GET /plg/analytics/dashboard`
 * (the PLG page's own figures, `codeDashboardPlg.ts`), and `searchRota` /
 * `searchAbsences` `POST /team-schedule/assignments/search` / `/absences/search`
 * (`codeDashboardTeamSchedule.ts`), all with the
 * viewer's own login, so
 * the backend's normal permissions apply; a filter list naming a product,
 * version, project, customer, team or region is resolved first
 * (`createNamedFilterResolver`). Any name not here is refused by the frame.
 */
export function useCodeDashboardPortal(): Record<string, CodeDashboardPortalApi> {
  const api = useBackendApi();
  const { user } = useCurrentUser();

  return useMemo(() => {
    const resolveNames = createNamedFilterResolver((body) =>
      api.post<ResolvedFiltersBody, ResolvedFiltersBody>("/dashboards/resolved-filters", body),
    );
    const productName = createNameLookup(() => api.get<string[]>("/products"));
    const teamName = createNameLookup(() => api.get<string[]>("/abt-teams"));
    const checkPlaybookCodes = createPlaybookCodeChecker((body) =>
      api.post<typeof body, unknown>("/dashboards/resolved-filters", body),
    );
    const resolveTeams = createTeamListResolver((body) =>
      api.post<{ teamIds: string[] }, { teamIds?: string[] }>("/dashboards/resolved-filters", body),
    );
    return {
      currentUser: () => {
        if (!user?.id) throw new Error("currentUser: the signed-in user is not loaded");
        return { id: user.id, name: [user.firstName, user.lastName].filter(Boolean).join(" "), email: user.email };
      },
      countCases: async (args: unknown) => {
        const { body, grouped } = buildCountCasesRequest(args);
        body.filters.filters = await resolveNames(body.filters.filters);
        const res = await api.post<CaseAggregateRequest, CaseAggregateResponse>("/cases/aggregate", body);
        return toCountCasesResult(res, grouped);
      },
      caseResolutionTime: async (args: unknown) => {
        const { body, grouped } = buildCaseResolutionTimeRequest(args);
        body.filters.filters = await resolveNames(body.filters.filters);
        const res = await api.post<CaseResolutionTimeRequest, CaseResolutionTimeResponse>("/cases/resolution-time", body);
        return toCaseResolutionTimeResult(res, grouped);
      },
      searchCases: async (args: unknown) => {
        const body = buildSearchCasesRequest(args);
        body.filters.filters = await resolveNames(body.filters.filters);
        const res = await api.post<CaseSearchRequest, BeCaseSearchResponse>("/cases/search", body);
        return toSearchCasesResult(res);
      },
      caseTrend: async (args: unknown) => {
        const body = buildCaseTrendRequest(args);
        body.filters.filters = await resolveNames(body.filters.filters);
        const res = await api.post<CaseTrendRequest, BeCaseVolumeTrendResponse>("/cases/volume-trend", body);
        return toCaseTrendResult(res);
      },
      countTimeCards: async (args: unknown) => {
        const { body, grouped } = buildCountTimeCardsRequest(args, user?.id);
        if (body.filters.teamIds) body.filters.teamIds = await resolveTeams(body.filters.teamIds);
        const res = await api.post<TimeCardAggregateRequest, TimeCardAggregateResponse>("/time-cards/aggregate", body);
        return toCountTimeCardsResult(res, grouped);
      },
      searchTimeCards: async (args: unknown) => {
        const body = buildSearchTimeCardsRequest(args, user?.id);
        if (body.filters.teamIds) body.filters.teamIds = await resolveTeams(body.filters.teamIds);
        const res = await api.post<TimeCardSearchRequest, TimeCardSearchResponse>("/time-cards/search", body);
        return toSearchTimeCardsResult(res);
      },
      countOperations: async (args: unknown) => {
        const { path, body, grouped } = buildCountOperationsRequest(args, user?.id);
        const res = await api.post<OperationsAggregateRequest, OperationsAggregateResponse>(path, body);
        return toCountOperationsResult(res, grouped);
      },
      searchOperations: async (args: unknown) => {
        const { path, itemsKey, body } = buildSearchOperationsRequest(args, user?.id);
        const res = await api.post<OperationsSearchRequest, Record<string, unknown>>(path, body);
        return toSearchOperationsResult(res, itemsKey);
      },
      searchAccounts: async (args: unknown) => {
        const body = buildSearchAccountsRequest(args);
        const res = await api.post<AccountSearchRequest, Parameters<typeof toSearchAccountsResult>[0]>("/accounts/search", body);
        return toSearchAccountsResult(res);
      },
      searchProjects: async (args: unknown) => {
        const body = buildSearchProjectsRequest(args);
        const res = await api.post<ProjectSearchRequest, Parameters<typeof toSearchProjectsResult>[0]>("/projects/search", body);
        return toSearchProjectsResult(res);
      },
      searchCustomerHealth: async (args: unknown) => {
        const body = buildSearchCustomerHealthRequest(args, user?.email);
        if (body.product) body.product = await productName("searchCustomerHealth", "product", body.product);
        if (body.abtTeam) body.abtTeam = await teamName("searchCustomerHealth", "team", body.abtTeam);
        const res = await api.post<CustomerHealthSummaryRequest, Parameters<typeof toSearchCustomerHealthResult>[0]>(
          "/customer-health/summary",
          body,
        );
        return toSearchCustomerHealthResult(res);
      },
      plgSummary: async (args: unknown) => {
        const res = await api.get<PlgSummaryResponse>(buildPlgSummaryPath(args));
        return toPlgSummaryResult(res ?? {});
      },
      searchRota: async (args: unknown) => {
        const body = buildScheduleSearchRequest("searchRota", args, user?.email);
        const res = await api.post<ScheduleSearchRequest, Parameters<typeof toSearchRotaResult>[0]>(
          "/team-schedule/assignments/search",
          body,
        );
        return toSearchRotaResult(res);
      },
      searchAbsences: async (args: unknown) => {
        const body = buildScheduleSearchRequest("searchAbsences", args, user?.email);
        const res = await api.post<ScheduleSearchRequest, Parameters<typeof toSearchAbsencesResult>[0]>(
          "/team-schedule/absences/search",
          body,
        );
        return toSearchAbsencesResult(res);
      },
      countFeedback: async (args: unknown) => {
        const { body, grouped } = buildCountFeedbackRequest(args);
        const res = await api.post<FeedbackAggregateRequest, FeedbackAggregateResponse>("/cases/feedback/aggregate", body);
        return toCountFeedbackResult(res, grouped);
      },
      searchFeedback: async (args: unknown) => {
        const body = buildSearchFeedbackRequest(args);
        const res = await api.post<FeedbackSearchRequest, Parameters<typeof toSearchFeedbackResult>[0]>("/cases/feedback/search", body);
        return toSearchFeedbackResult(res);
      },
      countPlg: async (args: unknown) => {
        const { path, body, grouped } = buildCountPlgRequest(args, user?.id);
        await checkPlaybookCodes(body.filters as Record<string, unknown>);
        const res = await api.post<Record<string, unknown>, PlgCountResponse>(path, body);
        return toCountPlgResult(res, grouped);
      },
      searchPlg: async (args: unknown) => {
        const { path, itemsKey, body } = buildSearchPlgRequest(args, user?.id);
        await checkPlaybookCodes(body.filters as Record<string, unknown>);
        const res = await api.post<Record<string, unknown>, Record<string, unknown>>(path, body);
        return toSearchPlgResult(res, itemsKey);
      },
      plgTrend: async (args: unknown) => {
        const body = buildPlgTrendRequest(args, user?.id);
        await checkPlaybookCodes(body.filters);
        const res = await api.post<PlgTrendRequest, PlgTrendResponse>("/plg/registrations/volume-trend", body);
        return toPlgTrendResult(res);
      },
    };
  }, [api, user]);
}
