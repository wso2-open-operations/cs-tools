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

import { resolveRelativeDatePlaceholderToDate } from "@utils/resolveRelativeDatePlaceholder";

/**
 * A code dashboard's customer calls (`portal.searchAccounts`,
 * `portal.searchProjects`): the args are checked and turned into the flat
 * bodies `POST /accounts/search` and `/projects/search` take, as an account
 * or project widget's are. Their values are case-sensitive, the project
 * dates take plain dates only, and a bad body is a 400 that gives no reason,
 * so every arg is checked here and date tokens are resolved first.
 */

const ACCOUNT_LIFE_CYCLES = ["Customer", "Partner", "Prospect", "Lost Customer", "Lost Prospect"] as const;
const ACCOUNT_RISKS = [
  "hasNoGoLive",
  "hasGoneLive",
  "hasRecentCases",
  "noSupportCases6mo",
  "hasEolProduct",
  "hasAbandonedMigrations",
  "hasMigrationDelays",
  "hasRecentEscalations",
] as const;
const ACCOUNT_HEALTH_STATUSES = ["at_risk", "healthy", "to_be_reviewed"] as const;

const PROJECT_ONBOARDING_STATUSES = ["Not-Started", "In-Progress", "Completed", "Not-Applicable"] as const;
const PROJECT_CLOSURE_STATES = ["Open", "Suspended", "Restricted"] as const;

/** The project dates a search can filter (`<date>From` / `<date>To`) and sort by. */
const PROJECT_DATES = [
  "endDate",
  "onboardingStartDate",
  "goLivePlanDate",
  "goLiveDate",
  "onboardingExpiryDate",
  "productionReadinessCompletionDate",
] as const;
const PROJECT_SORT_DATES: readonly string[] = PROJECT_DATES.filter((d) => d !== "onboardingStartDate");

/** The page both searches read when the code does not say, and its cap. */
const SEARCH_CUSTOMERS_DEFAULT_LIMIT = 10;
const SEARCH_CUSTOMERS_MAX_LIMIT = 50;

const PLAIN_DATE = /^\d{4}-\d{2}-\d{2}$/;

/** Refuses an arg name not in `known`, so a misspelt one is not silently ignored. */
function checkArgNames(fn: string, args: Record<string, unknown>, known: readonly string[]): void {
  for (const key of Object.keys(args)) {
    if (!known.includes(key)) throw new Error(`${fn}: unknown argument "${key}"`);
  }
}

/** `value` when it is one of `allowed`, or throws. */
function oneOf(fn: string, key: string, value: unknown, allowed: readonly string[]): string {
  if (typeof value !== "string" || !allowed.includes(value)) {
    throw new Error(`${fn}: ${key} must be one of ${allowed.join(", ")}`);
  }
  return value;
}

/** `value` when it is a non-empty list of `allowed`, or throws. */
function listOf(fn: string, key: string, value: unknown, allowed: readonly string[]): string[] {
  if (!Array.isArray(value) || value.length === 0 || !value.every((v) => allowed.includes(v))) {
    throw new Error(`${fn}: ${key} must be a non-empty list of ${allowed.join(", ")}`);
  }
  return value as string[];
}

/** The page size an args object asks for (1 to 50, default 10), or throws. */
function pageLimit(fn: string, limit: unknown): number {
  if (limit === undefined) return SEARCH_CUSTOMERS_DEFAULT_LIMIT;
  if (!(Number.isInteger(limit) && (limit as number) >= 1 && (limit as number) <= SEARCH_CUSTOMERS_MAX_LIMIT)) {
    throw new Error(`${fn}: limit must be an integer from 1 to ${SEARCH_CUSTOMERS_MAX_LIMIT}`);
  }
  return limit as number;
}

/** Body of POST /accounts/search. */
export interface AccountSearchRequest {
  filters: Record<string, unknown>;
  pagination: { offset: 0; limit: number };
}

/**
 * Turns `portal.searchAccounts(args)` into a `POST /accounts/search` body, or
 * throws (the code sees the message). Args: `lifeCycle`, `team` (a team key
 * or name: the account search matches the account's CRE or SRE team by
 * name, any case, so no lookup is needed), `risks` (any of them),
 * `healthStatus`, `hasProject`, `active` and `limit`. The search has no sort.
 */
export function buildSearchAccountsRequest(args: unknown): AccountSearchRequest {
  const fn = "searchAccounts";
  const all = (args ?? {}) as Record<string, unknown>;
  checkArgNames(fn, all, ["lifeCycle", "team", "risks", "healthStatus", "hasProject", "active", "limit"]);
  const filters: Record<string, unknown> = {};
  if (all.lifeCycle !== undefined) filters.lifeCycle = oneOf(fn, "lifeCycle", all.lifeCycle, ACCOUNT_LIFE_CYCLES);
  if (all.team !== undefined) {
    if (typeof all.team !== "string" || all.team === "") throw new Error(`${fn}: team must be one team key`);
    filters.abtTeam = all.team;
  }
  if (all.risks !== undefined) filters.risks = listOf(fn, "risks", all.risks, ACCOUNT_RISKS);
  if (all.healthStatus !== undefined) {
    filters.healthStatus = oneOf(fn, "healthStatus", all.healthStatus, ACCOUNT_HEALTH_STATUSES);
  }
  for (const key of ["hasProject", "active"] as const) {
    if (all[key] === undefined) continue;
    if (typeof all[key] !== "boolean") throw new Error(`${fn}: ${key} must be true or false`);
    filters[key] = all[key];
  }
  return { filters, pagination: { offset: 0, limit: pageLimit(fn, all.limit) } };
}

/** An account as `POST /accounts/search` returns it (the fields read here). */
interface AccountSearchView {
  id: string;
  name?: string | null;
  number?: string | null;
  supportTier?: string | null;
  region?: string | null;
  country?: string | null;
  creTeam?: { name?: string | null } | null;
  sreTeam?: { name?: string | null } | null;
  accountManager?: { name?: string | null } | null;
}

/** One account as `portal.searchAccounts` returns it: plain values, people and teams by name. */
export interface CodeDashboardAccount {
  id: string;
  name: string | null;
  number: string | null;
  tier: string | null;
  region: string | null;
  country: string | null;
  creTeam: string | null;
  sreTeam: string | null;
  accountManager: string | null;
}

/** Reshapes a `POST /accounts/search` response into what `portal.searchAccounts` returns. */
export function toSearchAccountsResult(res: { accounts?: AccountSearchView[] | null; total?: number }): {
  total: number;
  items: CodeDashboardAccount[];
} {
  const items = (res.accounts ?? []).map(
    (a): CodeDashboardAccount => ({
      id: a.id,
      name: a.name ?? null,
      number: a.number ?? null,
      tier: a.supportTier ?? null,
      region: a.region ?? null,
      country: a.country ?? null,
      creTeam: a.creTeam?.name ?? null,
      sreTeam: a.sreTeam?.name ?? null,
      accountManager: a.accountManager?.name ?? null,
    }),
  );
  return { total: res.total ?? items.length, items };
}

/** Body of POST /projects/search: flat filters beside the page and the sort. */
export type ProjectSearchRequest = Record<string, unknown> & { pagination: { offset: 0; limit: number } };

/**
 * Turns `portal.searchProjects(args)` into a `POST /projects/search` body, or
 * throws. Args: `onboardingStatus`, `closureStatus`, `closureStates`,
 * `excludeClosureStates`, the `<date>From` / `<date>To` pairs (one date
 * each; tokens resolved here, as a project widget's are), `sortBy: {field,
 * order?}` (a project date, default `"asc"`: soonest first) and `limit`.
 */
export function buildSearchProjectsRequest(args: unknown): ProjectSearchRequest {
  const fn = "searchProjects";
  const all = (args ?? {}) as Record<string, unknown>;
  const dateKeys = PROJECT_DATES.flatMap((d) => [`${d}From`, `${d}To`]);
  checkArgNames(fn, all, ["onboardingStatus", "closureStatus", "closureStates", "excludeClosureStates", ...dateKeys, "sortBy", "limit"]);
  const body: Record<string, unknown> = {};
  if (all.onboardingStatus !== undefined) {
    body.onboardingStatus = listOf(fn, "onboardingStatus", all.onboardingStatus, PROJECT_ONBOARDING_STATUSES);
  }
  if (all.closureStatus !== undefined) body.closureStatus = oneOf(fn, "closureStatus", all.closureStatus, PROJECT_CLOSURE_STATES);
  for (const key of ["closureStates", "excludeClosureStates"] as const) {
    if (all[key] !== undefined) body[key] = listOf(fn, key, all[key], PROJECT_CLOSURE_STATES);
  }
  for (const key of dateKeys) {
    const value = all[key];
    if (value === undefined) continue;
    // A token becomes the viewer's local date, as a project widget's does.
    const resolved =
      typeof value !== "string" || PLAIN_DATE.test(value) ? value : resolveRelativeDatePlaceholderToDate(value, new Date());
    if (typeof resolved !== "string" || !PLAIN_DATE.test(resolved)) {
      throw new Error(`${fn}: ${key} must be a date (YYYY-MM-DD) or a date token`);
    }
    body[key] = resolved;
  }
  if (all.sortBy !== undefined) {
    const { field, order } = (all.sortBy ?? {}) as { field?: unknown; order?: unknown };
    if (!PROJECT_SORT_DATES.includes(field as string)) {
      throw new Error(`${fn}: sortBy.field must be one of ${PROJECT_SORT_DATES.join(", ")}`);
    }
    if (order !== undefined && order !== "asc" && order !== "desc") {
      throw new Error(`${fn}: sortBy.order must be "asc" or "desc"`);
    }
    body.sortBy = field;
    body.sortOrder = order ?? "asc";
  }
  return { ...body, pagination: { offset: 0, limit: pageLimit(fn, all.limit) } };
}

/** A project as `POST /projects/search` returns it (the fields read here). */
interface ProjectSearchView {
  id: string;
  key?: string | null;
  name?: string | null;
  account?: { name?: string | null } | null;
  onboardingStatus?: string | null;
  closureState?: string | null;
  startDate?: string | null;
  endDate?: string | null;
  goLiveDate?: string | null;
  activeCasesCount?: number | null;
}

/** One project as `portal.searchProjects` returns it: plain values, the account by name. */
export interface CodeDashboardProject {
  id: string;
  key: string | null;
  name: string | null;
  account: string | null;
  onboardingStatus: string | null;
  closureState: string | null;
  startDate: string | null;
  endDate: string | null;
  goLiveDate: string | null;
  activeCases: number;
}

/** Reshapes a `POST /projects/search` response into what `portal.searchProjects` returns. */
export function toSearchProjectsResult(res: { projects?: ProjectSearchView[] | null; total?: number }): {
  total: number;
  items: CodeDashboardProject[];
} {
  const items = (res.projects ?? []).map(
    (p): CodeDashboardProject => ({
      id: p.id,
      key: p.key ?? null,
      name: p.name ?? null,
      account: p.account?.name ?? null,
      onboardingStatus: p.onboardingStatus ?? null,
      closureState: p.closureState ?? null,
      startDate: p.startDate ?? null,
      endDate: p.endDate ?? null,
      goLiveDate: p.goLiveDate ?? null,
      activeCases: p.activeCasesCount ?? 0,
    }),
  );
  return { total: res.total ?? items.length, items };
}
