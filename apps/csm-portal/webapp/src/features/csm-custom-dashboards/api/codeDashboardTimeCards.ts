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
 * A code dashboard's time card calls (`portal.countTimeCards`,
 * `portal.searchTimeCards`): the args are checked and turned into the flat
 * filters `POST /time-cards/aggregate` and `/search` take. Those endpoints
 * take plain dates only and answer a bad body with a 400 that gives no
 * reason, so date tokens are resolved here, as a time card widget's are, and
 * every rule the backend would refuse silently is checked first.
 */

/** The time card states the endpoints take. */
const TIME_CARD_STATES = ["pending", "submitted", "approved", "rejected", "processed", "recalled"] as const;

/** The fields `portal.countTimeCards` may group by (`team` only with `teams`). */
const TIME_CARD_GROUP_BY_FIELDS = ["state", "user", "project", "day", "team"] as const;
type TimeCardGroupByField = (typeof TIME_CARD_GROUP_BY_FIELDS)[number];

/** The most teams one call takes (the time card search's own cap). */
const TIME_CARD_MAX_TEAMS = 100;

/** The longest window a `day` grouping takes (the backend's). */
const TIME_CARD_MAX_DAYS = 31;

/** The fields `portal.searchTimeCards` can sort by, its default page and its cap. */
const TIME_CARD_SORT_FIELDS = ["workDate", "updatedOn"] as const;
const SEARCH_TIME_CARDS_DEFAULT_LIMIT = 10;
const SEARCH_TIME_CARDS_MAX_LIMIT = 50;

const PLAIN_DATE = /^\d{4}-\d{2}-\d{2}$/;

/** The flat filters both endpoints take. */
export interface TimeCardFilters {
  states?: string[];
  userIds?: string[];
  teamIds?: string[];
  startDate?: string;
  endDate?: string;
}

/**
 * Checks the filter args both calls share (`states`, `mine`, `teams`,
 * `startDate`, `endDate`) and returns the flat filters, dates resolved.
 * `mine` is the viewer's own cards: the only person a code dashboard can
 * name, so no user id is taken from the code. `teams` are team keys or
 * names, sent as `teamIds` once the hook has looked them up
 * (`createTeamListResolver`). Any other arg name not in `extra` is refused,
 * so a misspelt or case-search style arg is not silently ignored.
 */
function buildTimeCardFilters(
  fn: string,
  args: Record<string, unknown>,
  extra: readonly string[],
  userId: string | undefined,
): TimeCardFilters {
  for (const key of Object.keys(args)) {
    if (!["states", "mine", "teams", "startDate", "endDate", ...extra].includes(key)) {
      throw new Error(`${fn}: unknown argument "${key}"`);
    }
  }
  const { states, mine, teams, startDate, endDate } = args;
  const filters: TimeCardFilters = {};
  if (states !== undefined) {
    if (
      !Array.isArray(states) ||
      states.length === 0 ||
      !states.every((s) => (TIME_CARD_STATES as readonly unknown[]).includes(s))
    ) {
      throw new Error(`${fn}: states must be a non-empty list of ${TIME_CARD_STATES.join(", ")}`);
    }
    filters.states = states as string[];
  }
  if (mine !== undefined && typeof mine !== "boolean") throw new Error(`${fn}: mine must be true or false`);
  if (mine) {
    if (!userId) throw new Error(`${fn}: the signed-in user is not loaded`);
    filters.userIds = [userId];
  }
  if (teams !== undefined) {
    if (
      !Array.isArray(teams) ||
      teams.length === 0 ||
      teams.length > TIME_CARD_MAX_TEAMS ||
      !teams.every((t) => typeof t === "string" && t !== "")
    ) {
      throw new Error(`${fn}: teams must be a list of 1 to ${TIME_CARD_MAX_TEAMS} team keys`);
    }
    filters.teamIds = teams as string[];
  }
  for (const [key, value] of [
    ["startDate", startDate],
    ["endDate", endDate],
  ] as const) {
    if (value === undefined) continue;
    if (typeof value !== "string") throw new Error(`${fn}: ${key} must be a date (YYYY-MM-DD) or a date token`);
    // A token becomes the viewer's local date, as a time card widget's does.
    const resolved = PLAIN_DATE.test(value) ? value : resolveRelativeDatePlaceholderToDate(value, new Date());
    if (typeof resolved !== "string" || !PLAIN_DATE.test(resolved)) {
      throw new Error(`${fn}: ${key} must be a date (YYYY-MM-DD) or a date token`);
    }
    filters[key] = resolved;
  }
  return filters;
}

/** Days from `start` to `end`, both `YYYY-MM-DD` and counted inclusively. */
function daysInWindow(start: string, end: string): number {
  return Math.round((Date.parse(`${end}T00:00:00Z`) - Date.parse(`${start}T00:00:00Z`)) / 86_400_000) + 1;
}

/** Body of POST /time-cards/aggregate. */
export interface TimeCardAggregateRequest {
  filters: TimeCardFilters;
  groupBy: TimeCardGroupByField;
  measure?: "count" | "minutes";
  maxGroups?: number;
}

/** Response of POST /time-cards/aggregate (backend `AggregateResponse` with minutes). */
export interface TimeCardAggregateResponse {
  groups?: { key: string; label: string; count: number; totalMinutes?: number }[];
  othersCount?: number;
  othersMinutes?: number;
  totalRecords?: number;
  totalMinutes?: number;
}

/** What `portal.countTimeCards` returns: cards and hours, per group when it asked for one. */
export interface CountTimeCardsResult {
  total: number;
  hours: number;
  groups?: { label: string; count: number; hours: number }[];
}

/**
 * Turns `portal.countTimeCards(args)` into a `POST /time-cards/aggregate`
 * body, or throws (the code sees the message). Args: the shared filters,
 * `groupBy` (`state`, `user`, `project`, `day`, or `team`: one group per
 * team in `teams`), `measure` (`"count"` or
 * `"hours"`: which the top `maxGroups` groups are kept by) and `maxGroups`.
 * `day` needs both dates, at most 31 days apart, and takes no `maxGroups`.
 * The endpoint always needs a `groupBy`; a plain count groups by `state`,
 * which every card has, and keeps only the totals.
 */
export function buildCountTimeCardsRequest(
  args: unknown,
  userId: string | undefined,
): { body: TimeCardAggregateRequest; grouped: boolean } {
  const all = (args ?? {}) as Record<string, unknown>;
  const filters = buildTimeCardFilters("countTimeCards", all, ["groupBy", "measure", "maxGroups"], userId);
  const { groupBy, measure, maxGroups } = all;
  if (groupBy !== undefined && !TIME_CARD_GROUP_BY_FIELDS.includes(groupBy as TimeCardGroupByField)) {
    throw new Error(`countTimeCards: groupBy must be one of ${TIME_CARD_GROUP_BY_FIELDS.join(", ")}`);
  }
  if (measure !== undefined && measure !== "count" && measure !== "hours") {
    throw new Error('countTimeCards: measure must be "count" or "hours"');
  }
  if (maxGroups !== undefined) {
    if (groupBy === undefined || groupBy === "day") {
      throw new Error("countTimeCards: maxGroups goes only with a groupBy other than day");
    }
    if (!(Number.isInteger(maxGroups) && (maxGroups as number) > 0)) {
      throw new Error("countTimeCards: maxGroups must be a positive integer");
    }
  }
  if (groupBy === "team" && !filters.teamIds) {
    throw new Error("countTimeCards: groupBy team needs teams (the teams to show)");
  }
  if (groupBy === "day") {
    if (!filters.startDate || !filters.endDate) {
      throw new Error("countTimeCards: groupBy day needs both startDate and endDate");
    }
    const days = daysInWindow(filters.startDate, filters.endDate);
    if (days < 1 || days > TIME_CARD_MAX_DAYS) {
      throw new Error(`countTimeCards: groupBy day takes a window of 1 to ${TIME_CARD_MAX_DAYS} days`);
    }
  }
  return {
    body: {
      filters,
      groupBy: (groupBy as TimeCardGroupByField | undefined) ?? "state",
      ...(measure !== undefined ? { measure: measure === "hours" ? "minutes" : "count" } : {}),
      ...(maxGroups !== undefined ? { maxGroups: maxGroups as number } : {}),
    },
    grouped: groupBy !== undefined,
  };
}

/** Minutes as hours, to two decimals. */
function hours(minutes: number | undefined): number {
  return Math.round(((minutes ?? 0) / 60) * 100) / 100;
}

/** Reshapes a `POST /time-cards/aggregate` response into what `portal.countTimeCards` returns. */
export function toCountTimeCardsResult(res: TimeCardAggregateResponse, grouped: boolean): CountTimeCardsResult {
  const totals = { total: res.totalRecords ?? 0, hours: hours(res.totalMinutes) };
  if (!grouped) return totals;
  const groups = (res.groups ?? []).map((g) => ({ label: g.label, count: g.count, hours: hours(g.totalMinutes) }));
  if (res.othersCount) groups.push({ label: "Others", count: res.othersCount, hours: hours(res.othersMinutes) });
  return { ...totals, groups };
}

/** Body of POST /time-cards/search, as `portal.searchTimeCards` sends it: always the first page. */
export interface TimeCardSearchRequest {
  filters: TimeCardFilters;
  sortBy?: { field: (typeof TIME_CARD_SORT_FIELDS)[number]; order: "asc" | "desc" };
  pagination: { offset: 0; limit: number };
}

/** A time card as `POST /time-cards/search` returns it (the fields read here). */
interface TimeCardSearchView {
  id: string;
  workDate?: string | null;
  state?: string | null;
  totalTime?: number | null;
  user?: { name?: string | null } | null;
  project?: { name?: string | null } | null;
  case?: { number?: string | null } | null;
}

/** Response of POST /time-cards/search. */
export interface TimeCardSearchResponse {
  timeCards?: TimeCardSearchView[];
  total?: number;
}

/** One time card as `portal.searchTimeCards` returns it: plain values, related records by name. */
export interface CodeDashboardTimeCard {
  id: string;
  workDate: string | null;
  state: string | null;
  hours: number;
  user: string | null;
  project: string | null;
  caseNumber: string | null;
}

/**
 * Turns `portal.searchTimeCards(args)` into a `POST /time-cards/search`
 * body, or throws. Args: the shared filters, `sortBy: {field, order?}`
 * (`workDate` or `updatedOn`, default `"desc"`) and `limit` (1 to 50,
 * default 10); only the first page is read.
 */
export function buildSearchTimeCardsRequest(args: unknown, userId: string | undefined): TimeCardSearchRequest {
  const all = (args ?? {}) as Record<string, unknown>;
  const filters = buildTimeCardFilters("searchTimeCards", all, ["sortBy", "limit"], userId);
  const { sortBy, limit } = all;
  if (
    limit !== undefined &&
    !(Number.isInteger(limit) && (limit as number) >= 1 && (limit as number) <= SEARCH_TIME_CARDS_MAX_LIMIT)
  ) {
    throw new Error(`searchTimeCards: limit must be an integer from 1 to ${SEARCH_TIME_CARDS_MAX_LIMIT}`);
  }
  let sort: TimeCardSearchRequest["sortBy"];
  if (sortBy !== undefined) {
    const { field, order } = (sortBy ?? {}) as { field?: unknown; order?: unknown };
    if (!(TIME_CARD_SORT_FIELDS as readonly unknown[]).includes(field)) {
      throw new Error(`searchTimeCards: sortBy.field must be one of ${TIME_CARD_SORT_FIELDS.join(", ")}`);
    }
    if (order !== undefined && order !== "asc" && order !== "desc") {
      throw new Error('searchTimeCards: sortBy.order must be "asc" or "desc"');
    }
    sort = { field: field as (typeof TIME_CARD_SORT_FIELDS)[number], order: (order as "asc" | "desc" | undefined) ?? "desc" };
  }
  return {
    filters,
    ...(sort ? { sortBy: sort } : {}),
    pagination: { offset: 0, limit: (limit as number | undefined) ?? SEARCH_TIME_CARDS_DEFAULT_LIMIT },
  };
}

/** Reshapes a `POST /time-cards/search` response into what `portal.searchTimeCards` returns. */
export function toSearchTimeCardsResult(res: TimeCardSearchResponse): { total: number; items: CodeDashboardTimeCard[] } {
  const items = (res.timeCards ?? []).map(
    (t): CodeDashboardTimeCard => ({
      id: t.id,
      workDate: t.workDate ?? null,
      state: t.state ?? null,
      hours: hours(t.totalTime ?? 0),
      user: t.user?.name ?? null,
      project: t.project?.name ?? null,
      caseNumber: t.case?.number ?? null,
    }),
  );
  return { total: res.total ?? items.length, items };
}

/**
 * Returns a function turning a time card call's team keys or names into the
 * teams' group ids, through `POST /dashboards/resolved-filters` (the
 * backend's registry, the JSON approach's rules), once per distinct list for
 * the resolver's life. A failed lookup is not kept, and its error (with the
 * backend's reason, e.g. a team that does not exist) reaches the code.
 */
export function createTeamListResolver(
  post: (body: { teamIds: string[] }) => Promise<{ teamIds?: string[] }>,
): (teams: string[]) => Promise<string[]> {
  const cache = new Map<string, Promise<string[]>>();
  return (teams) => {
    const key = JSON.stringify(teams);
    let resolved = cache.get(key);
    if (!resolved) {
      resolved = post({ teamIds: teams }).then((res) => res.teamIds ?? []);
      cache.set(key, resolved);
      resolved.catch(() => cache.delete(key));
    }
    return resolved;
  };
}
