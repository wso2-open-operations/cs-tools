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

import { buildTrendWindow, type TrendWindow } from "@features/csm-custom-dashboards/api/codeDashboardTrendWindow";
import { resolveRelativeDatePlaceholderToUtcDate } from "@utils/resolveRelativeDatePlaceholder";

/**
 * A code dashboard's PLG calls (`portal.countPlg`, `portal.searchPlg`,
 * `portal.plgTrend`) on the PLG organisation and registration searches,
 * aggregates and the registration volume trend, and `portal.plgSummary` on
 * the PLG page's own figures, built as a PLG widget's
 * bodies are (`plgSearchFilters`): flat filters, the sign-up date bounds as
 * UTC instants (a plain date or a token is a 400), `unacknowledgedOnly`
 * beside the filters. Values are checked here first. The searches' sign-up
 * emails and names are never handed to the code.
 */

const PLG_KINDS = { organization: "organizations", registration: "registrations" } as const;
type PlgKind = keyof typeof PLG_KINDS;

const PLG_LISTS: Record<string, readonly string[]> = {
  productCodes: ["IAM", "API_PLATFORM", "INTEGRATION_PLATFORM", "AGENT_PLATFORM", "ENG_PLATFORM"],
  lifecycleStages: ["REGISTRATION", "PLG_CS_ELIGIBLE", "FIRST_VALUE_ACHIEVED", "ACTIVATED", "COMMERCIAL", "ABANDONED"],
  subscriptionTiers: ["FREE", "TRIAL", "TRIAL_EXTENDED", "PAYG"],
};

/** The groupings each kind's aggregate takes (`playbookAnswer` needs a `playbookTask`). */
const PLG_GROUP_BY: Record<PlgKind, readonly string[]> = {
  organization: ["owner", "country"],
  registration: ["country", "playbookAnswer"],
};

/** The page `portal.searchPlg` reads when the code does not say, and its cap. */
const SEARCH_PLG_DEFAULT_LIMIT = 10;
const SEARCH_PLG_MAX_LIMIT = 50;

const PLAIN_DATE = /^\d{4}-\d{2}-\d{2}$/;
const EMAIL = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

/** The flat filters the PLG searches, aggregates and trend take. */
export type PlgFilters = Record<string, unknown>;

/** What every PLG call sends: the filters and, for registrations, `unacknowledgedOnly`. */
interface PlgQuery {
  filters: PlgFilters;
  unacknowledgedOnly?: true;
}

/** `kind` as a PLG kind, or throws. */
function plgKind(fn: string, kind: unknown): PlgKind {
  if (kind !== "organization" && kind !== "registration") {
    throw new Error(`${fn}: kind must be "organization" or "registration"`);
  }
  return kind;
}

/**
 * Checks the filter args every PLG call shares and returns the query.
 * `mine` is the viewer's own organisations (`ownerIds`), the only owner id a
 * code dashboard can name; `ownerEmails` are the addresses the request gave.
 * `registeredFrom` / `registeredTo` (dates or tokens, read as UTC days) become
 * the start and end instants of those days. `playbookAnswers` and
 * `playbookUnanswered` need a `playbookTask` (the PLG searches' own rule);
 * the codes come from the prompt's PLG playbook answers section. Any other
 * arg not in `extra` is refused.
 */
function buildPlgQuery(
  fn: string,
  kind: PlgKind,
  args: Record<string, unknown>,
  extra: readonly string[],
  userId: string | undefined,
): PlgQuery {
  const known = [
    ...Object.keys(PLG_LISTS),
    "registeredFrom",
    "registeredTo",
    "unowned",
    "mine",
    "ownerEmails",
    "countryNames",
    "countryUnknown",
    "playbookTask",
    "playbookAnswers",
    "playbookUnanswered",
    ...(kind === "registration" ? ["unacknowledgedOnly"] : []),
    ...extra,
  ];
  for (const key of Object.keys(args)) {
    if (!known.includes(key)) throw new Error(`${fn}: a ${kind} call takes no "${key}"`);
  }
  const filters: PlgFilters = {};
  for (const [key, allowed] of Object.entries(PLG_LISTS)) {
    const value = args[key];
    if (value === undefined) continue;
    if (!Array.isArray(value) || value.length === 0 || !value.every((v) => allowed.includes(v))) {
      throw new Error(`${fn}: ${key} must be a non-empty list of ${allowed.join(", ")}`);
    }
    filters[key] = value;
  }
  for (const [key, time] of [
    ["registeredFrom", "T00:00:00Z"],
    ["registeredTo", "T23:59:59.999999Z"],
  ] as const) {
    const value = args[key];
    if (value === undefined) continue;
    const day = typeof value !== "string" || PLAIN_DATE.test(value) ? value : resolveRelativeDatePlaceholderToUtcDate(value, new Date());
    if (typeof day !== "string" || !PLAIN_DATE.test(day)) {
      throw new Error(`${fn}: ${key} must be a date (YYYY-MM-DD) or a date token`);
    }
    filters[key] = `${day}${time}`;
  }
  for (const key of ["unowned", "countryUnknown", "mine", "unacknowledgedOnly", "playbookUnanswered"] as const) {
    if (args[key] !== undefined && typeof args[key] !== "boolean") throw new Error(`${fn}: ${key} must be true or false`);
  }
  if (args.unowned) filters.unowned = true;
  if (args.mine) {
    if (!userId) throw new Error(`${fn}: the signed-in user is not loaded`);
    filters.ownerIds = [userId];
  }
  if (args.ownerEmails !== undefined) {
    const emails = args.ownerEmails;
    if (!Array.isArray(emails) || emails.length === 0 || !emails.every((e) => typeof e === "string" && EMAIL.test(e))) {
      throw new Error(`${fn}: ownerEmails must be a non-empty list of email addresses`);
    }
    filters.ownerEmails = emails;
  }
  if (args.unowned && (args.mine || filters.ownerEmails)) {
    throw new Error(`${fn}: unowned cannot go with mine or ownerEmails`);
  }
  if (args.countryNames !== undefined) {
    const names = args.countryNames;
    if (!Array.isArray(names) || names.length === 0 || !names.every((n) => typeof n === "string" && n.trim() !== "")) {
      throw new Error(`${fn}: countryNames must be a non-empty list of country names`);
    }
    filters.countryNames = names;
  }
  if (args.countryUnknown) {
    if (filters.countryNames) throw new Error(`${fn}: countryUnknown cannot go with countryNames`);
    filters.countryUnknown = true;
  }
  if (args.playbookTask !== undefined) {
    if (typeof args.playbookTask !== "string" || args.playbookTask.trim() === "") {
      throw new Error(`${fn}: playbookTask must be one task code`);
    }
    filters.playbookTask = args.playbookTask;
  }
  if (args.playbookAnswers !== undefined) {
    const answers = args.playbookAnswers;
    if (!Array.isArray(answers) || answers.length === 0 || !answers.every((a) => typeof a === "string" && a.trim() !== "")) {
      throw new Error(`${fn}: playbookAnswers must be a non-empty list of answer codes`);
    }
    filters.playbookAnswers = answers;
  }
  if (args.playbookUnanswered) {
    if (filters.playbookAnswers) throw new Error(`${fn}: playbookUnanswered cannot go with playbookAnswers`);
    filters.playbookUnanswered = true;
  }
  if ((filters.playbookAnswers || filters.playbookUnanswered) && !filters.playbookTask) {
    throw new Error(`${fn}: playbookAnswers and playbookUnanswered need a playbookTask`);
  }
  return { filters, ...(args.unacknowledgedOnly ? { unacknowledgedOnly: true as const } : {}) };
}

/**
 * Turns `portal.countPlg(args)` into the path and body to call, or throws.
 * Without `groupBy` the count is the kind's search `total` (one record
 * read); with one it is the kind's aggregate: `owner` (organisations only)
 * or `country`, with `maxGroups`.
 */
export function buildCountPlgRequest(
  args: unknown,
  userId: string | undefined,
): { path: string; body: Record<string, unknown>; grouped: boolean } {
  const fn = "countPlg";
  const all = (args ?? {}) as Record<string, unknown>;
  const kind = plgKind(fn, all.kind);
  const { kind: _kind, groupBy, maxGroups, ...rest } = all;
  const query = buildPlgQuery(fn, kind, rest, [], userId);
  if (groupBy === undefined) {
    if (maxGroups !== undefined) throw new Error(`${fn}: maxGroups goes only with groupBy`);
    return { path: `/plg/${PLG_KINDS[kind]}/search`, body: { ...query, pagination: { offset: 0, limit: 1 } }, grouped: false };
  }
  if (!PLG_GROUP_BY[kind].includes(groupBy as string)) {
    throw new Error(`${fn}: a ${kind} groupBy must be ${PLG_GROUP_BY[kind].map((g) => `"${g}"`).join(" or ")}`);
  }
  if (groupBy === "playbookAnswer" && !query.filters.playbookTask) {
    throw new Error(`${fn}: groupBy playbookAnswer needs a playbookTask`);
  }
  if (maxGroups !== undefined && !(Number.isInteger(maxGroups) && (maxGroups as number) > 0)) {
    throw new Error(`${fn}: maxGroups must be a positive integer`);
  }
  return {
    path: `/plg/${PLG_KINDS[kind]}/aggregate`,
    body: { ...query, groupBy, ...(maxGroups !== undefined ? { maxGroups } : {}) },
    grouped: true,
  };
}

/** Response of a PLG aggregate, or of a PLG search read for its total. */
export interface PlgCountResponse {
  groups?: { label: string; count: number }[] | null;
  othersCount?: number;
  totalRecords?: number;
  total?: number;
}

/** Reshapes a PLG search (total only) or aggregate response into what `portal.countPlg` returns. */
export function toCountPlgResult(
  res: PlgCountResponse,
  grouped: boolean,
): { total: number; groups?: { label: string; count: number }[] } {
  if (!grouped) return { total: res.total ?? 0 };
  const groups = (res.groups ?? []).map(({ label, count }) => ({ label, count }));
  if (res.othersCount) groups.push({ label: "Others", count: res.othersCount });
  return { total: res.totalRecords ?? 0, groups };
}

/**
 * Turns `portal.searchPlg(args)` into the kind's search path and body, or
 * throws. Args: the filters, `sortBy: {field, order?}` (organisations only:
 * `name` or `registeredOn`; registrations are newest first) and `limit`
 * (1 to 50, default 10).
 */
export function buildSearchPlgRequest(
  args: unknown,
  userId: string | undefined,
): { path: string; itemsKey: string; body: Record<string, unknown> } {
  const fn = "searchPlg";
  const all = (args ?? {}) as Record<string, unknown>;
  const kind = plgKind(fn, all.kind);
  const { kind: _kind, sortBy, limit, ...rest } = all;
  const query = buildPlgQuery(fn, kind, rest, [], userId);
  if (limit !== undefined && !(Number.isInteger(limit) && (limit as number) >= 1 && (limit as number) <= SEARCH_PLG_MAX_LIMIT)) {
    throw new Error(`${fn}: limit must be an integer from 1 to ${SEARCH_PLG_MAX_LIMIT}`);
  }
  const body: Record<string, unknown> = {
    ...query,
    pagination: { offset: 0, limit: (limit as number | undefined) ?? SEARCH_PLG_DEFAULT_LIMIT },
  };
  if (sortBy !== undefined) {
    if (kind === "registration") throw new Error(`${fn}: a registration list takes no sortBy (newest first)`);
    const { field, order } = (sortBy ?? {}) as { field?: unknown; order?: unknown };
    if (field !== "name" && field !== "registeredOn") throw new Error(`${fn}: sortBy.field must be "name" or "registeredOn"`);
    if (order !== undefined && order !== "asc" && order !== "desc") throw new Error(`${fn}: sortBy.order must be "asc" or "desc"`);
    body.sortBy = field;
    body.sortOrder = order ?? (field === "name" ? "asc" : "desc");
  }
  return { path: `/plg/${PLG_KINDS[kind]}/search`, itemsKey: PLG_KINDS[kind], body };
}

/** A product and its stage on a PLG record. */
interface PlgProductView {
  product?: { code?: string | null; name?: string | null } | null;
  lifecycleStage?: string | null;
  stageName?: string | null;
}

/** A PLG organisation or registration as its search returns it (the fields read here). */
interface PlgSearchView extends PlgProductView {
  id?: string;
  orgPlatformId?: string;
  organizationName?: string | null;
  registeredOn?: string | null;
  owner?: { name?: string | null } | null;
  platformStages?: PlgProductView[] | null;
}

/** One PLG organisation or registration as `portal.searchPlg` returns it: no sign-up email or name. */
export interface CodeDashboardPlgRecord {
  id: string;
  organization: string | null;
  registeredOn: string | null;
  owner: string | null;
  products: { code: string | null; name: string | null; stage: string | null }[];
}

/** Reshapes a PLG search response into what `portal.searchPlg` returns. */
export function toSearchPlgResult(
  res: Record<string, unknown>,
  itemsKey: string,
): { total: number; items: CodeDashboardPlgRecord[] } {
  const product = (p: PlgProductView) => ({
    code: p.product?.code ?? null,
    name: p.product?.name ?? null,
    stage: p.stageName ?? p.lifecycleStage ?? null,
  });
  const records = (res[itemsKey] as PlgSearchView[] | null | undefined) ?? [];
  const items = records.map(
    (r): CodeDashboardPlgRecord => ({
      id: r.id ?? r.orgPlatformId ?? "",
      organization: r.organizationName ?? null,
      registeredOn: r.registeredOn ?? null,
      owner: r.owner?.name ?? null,
      products: r.platformStages ? r.platformStages.map(product) : r.product ? [product(r)] : [],
    }),
  );
  return { total: typeof res.total === "number" ? res.total : items.length, items };
}

/** Body of POST /plg/registrations/volume-trend. */
export type PlgTrendRequest = PlgQuery & TrendWindow;

/**
 * Turns `portal.plgTrend(args)` into a `POST /plg/registrations/volume-trend`
 * body, or throws: registrations (sign-ups) per period, with the
 * registration filters and the window of `buildTrendWindow`.
 */
export function buildPlgTrendRequest(args: unknown, userId: string | undefined): PlgTrendRequest {
  const fn = "plgTrend";
  const { days, startDate, endDate, bucket, ...rest } = (args ?? {}) as Record<string, unknown>;
  return { ...buildPlgQuery(fn, "registration", rest, [], userId), ...buildTrendWindow(fn, { days, startDate, endDate, bucket }) };
}

/** Response of POST /plg/registrations/volume-trend: one "Registered" series. */
export interface PlgTrendResponse {
  series?: { label: string; points: { date: string; endDate?: string; count: number }[] }[] | null;
}

/** Reshapes the trend into what `portal.plgTrend` returns: one point per period, oldest first. */
export function toPlgTrendResult(res: PlgTrendResponse): { points: { date: string; endDate?: string; count: number }[] } {
  const points = (res.series?.[0]?.points ?? []).map((p) => ({
    date: p.date,
    ...(p.endDate ? { endDate: p.endDate } : {}),
    count: p.count,
  }));
  return { points };
}

/**
 * Turns `portal.plgSummary(args)` into the `GET /plg/analytics/dashboard`
 * path (the PLG page's own figures), or throws. Args: `startDate` /
 * `endDate`, a date or a token each (tokens read as UTC days). The endpoint
 * reads both as dates and counts up to the start of `to`, so `to` is the day
 * after `endDate`, or that day's sign-ups would be left out. With neither,
 * the endpoint reads the last 180 days.
 */
export function buildPlgSummaryPath(args: unknown): string {
  const fn = "plgSummary";
  const all = (args ?? {}) as Record<string, unknown>;
  for (const key of Object.keys(all)) {
    if (key !== "startDate" && key !== "endDate") throw new Error(`${fn}: unknown argument "${key}"`);
  }
  const days: Partial<Record<"startDate" | "endDate", string>> = {};
  for (const key of ["startDate", "endDate"] as const) {
    const value = all[key];
    if (value === undefined) continue;
    // A token resolves to a YYYY-MM-DD date, anything else to undefined.
    const day =
      typeof value !== "string" ? undefined : PLAIN_DATE.test(value) ? value : resolveRelativeDatePlaceholderToUtcDate(value, new Date());
    if (day === undefined) throw new Error(`${fn}: ${key} must be a date (YYYY-MM-DD) or a date token`);
    days[key] = day;
  }
  if (days.startDate && days.endDate && days.endDate < days.startDate) {
    throw new Error(`${fn}: endDate must not be before startDate`);
  }
  const query = new URLSearchParams();
  if (days.startDate) query.set("from", days.startDate);
  if (days.endDate) query.set("to", nextUtcDay(days.endDate));
  const text = query.toString();
  return `/plg/analytics/dashboard${text ? `?${text}` : ""}`;
}

/** The day after a `YYYY-MM-DD` date, as `YYYY-MM-DD` (UTC). */
function nextUtcDay(day: string): string {
  const next = new Date(`${day}T00:00:00Z`);
  next.setUTCDate(next.getUTCDate() + 1);
  return next.toISOString().slice(0, 10);
}

/** Response of GET /plg/analytics/dashboard (the fields read here). */
export interface PlgSummaryResponse {
  summary?: {
    totalOrganizations?: number;
    totalRegistrations?: number;
    newRegistrations?: number;
    pairingsNeedingAttention?: number;
    trialsEndingSoon?: number;
    unresolvedIngestFailures?: number;
  } | null;
  registrationsByProduct?: { label: string; count: number }[] | null;
  registrationsByLifecycleStage?: { label: string; count: number }[] | null;
  subscriptionMix?: { label: string; count: number }[] | null;
  registrationsOverTime?: { buckets?: string[] | null; series?: { label: string; counts: number[] }[] | null } | null;
}

/** What `portal.plgSummary` returns: the window's counts and breakdowns, and the backlog counts. */
export interface PlgSummaryResult {
  organizations: number;
  registrations: number;
  unacknowledged: number;
  needingAttention: number;
  trialsEndingSoon: number;
  ingestFailures: number;
  byProduct: { label: string; count: number }[];
  byStage: { label: string; count: number }[];
  byTier: { label: string; count: number }[];
  weekly: { weeks: string[]; series: { label: string; counts: number[] }[] };
}

/** Reshapes the PLG page's figures into what `portal.plgSummary` returns. */
export function toPlgSummaryResult(res: PlgSummaryResponse): PlgSummaryResult {
  const s = res.summary ?? {};
  const pairs = (list: { label: string; count: number }[] | null | undefined) =>
    (list ?? []).map(({ label, count }) => ({ label, count }));
  return {
    organizations: s.totalOrganizations ?? 0,
    registrations: s.totalRegistrations ?? 0,
    unacknowledged: s.newRegistrations ?? 0,
    needingAttention: s.pairingsNeedingAttention ?? 0,
    trialsEndingSoon: s.trialsEndingSoon ?? 0,
    ingestFailures: s.unresolvedIngestFailures ?? 0,
    byProduct: pairs(res.registrationsByProduct),
    byStage: pairs(res.registrationsByLifecycleStage),
    byTier: pairs(res.subscriptionMix),
    weekly: {
      weeks: res.registrationsOverTime?.buckets ?? [],
      series: (res.registrationsOverTime?.series ?? []).map(({ label, counts }) => ({ label, counts })),
    },
  };
}

/** Body of the playbook code check on `POST /dashboards/resolved-filters`. */
interface PlaybookCodes {
  playbookTask: string;
  playbookAnswers?: string[];
}

/**
 * Returns a function checking a PLG call's playbook task and answer codes
 * through `POST /dashboards/resolved-filters` (the generate path's own
 * check, on the playbooks as they are), once per distinct pair for the
 * checker's life. The PLG searches count nothing for a code that does not
 * exist, so the call is not made until its codes pass; a failed check (with
 * the backend's reason, e.g. the known tasks) reaches the code, and is not
 * kept. Filters without a `playbookTask` are not checked.
 */
export function createPlaybookCodeChecker(post: (body: PlaybookCodes) => Promise<unknown>): (filters: PlgFilters) => Promise<void> {
  const cache = new Map<string, Promise<void>>();
  return (filters) => {
    if (typeof filters.playbookTask !== "string") return Promise.resolve();
    const body: PlaybookCodes = {
      playbookTask: filters.playbookTask,
      ...(Array.isArray(filters.playbookAnswers) ? { playbookAnswers: filters.playbookAnswers as string[] } : {}),
    };
    const key = JSON.stringify(body);
    let checked = cache.get(key);
    if (!checked) {
      checked = post(body).then(() => undefined);
      cache.set(key, checked);
      checked.catch(() => cache.delete(key));
    }
    return checked;
  };
}
