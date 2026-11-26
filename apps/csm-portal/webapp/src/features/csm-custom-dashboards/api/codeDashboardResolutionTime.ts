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

import { resolveRelativeDatePlaceholderToUtcDate } from "@utils/resolveRelativeDatePlaceholder";

/**
 * A code dashboard's case resolution time call (`portal.caseResolutionTime`):
 * `POST /cases/resolution-time`, the cases resolved in a window and their
 * mean business time to resolve (MTTR). The backend passes no reason on for a
 * refused body, so the window and grouping are checked here first; the window
 * is sent as written (dates or tokens), for the entity service resolves
 * tokens the same way (UTC days).
 */

/** The fields a resolution time may be grouped by (the case aggregate's, less state and productVersion). */
const RESOLUTION_GROUP_BY = ["severity", "type", "account", "product", "project", "region"] as const;

/** The longest window, in months, as the entity service allows. */
const RESOLUTION_MAX_MONTHS = 24;

const PLAIN_DATE = /^\d{4}-\d{2}-\d{2}$/;

/** Body of POST /cases/resolution-time. */
export interface CaseResolutionTimeRequest {
  filters: { filters: unknown[] };
  resolvedFrom?: string;
  resolvedTo?: string;
  groupBy?: string;
  maxGroups?: number;
}

/** A window end as a UTC day: a plain date as is, a token resolved as the entity service will, or throws. */
function windowDay(key: string, value: unknown): string {
  const day =
    typeof value !== "string" ? undefined : PLAIN_DATE.test(value) ? value : resolveRelativeDatePlaceholderToUtcDate(value, new Date());
  if (day === undefined) throw new Error(`caseResolutionTime: ${key} must be a date (YYYY-MM-DD) or a date token`);
  return day;
}

/**
 * Turns `portal.caseResolutionTime(args)` into a `POST /cases/resolution-time`
 * body, or throws (the code sees the message). Args: `filters` (the case
 * filters, names resolved by the caller as for `countCases`), `startDate` and
 * `endDate` (when the cases were resolved: both or neither, at most 24 months
 * apart; neither is the last 12 months), `groupBy` and `maxGroups`.
 */
export function buildCaseResolutionTimeRequest(args: unknown): { body: CaseResolutionTimeRequest; grouped: boolean } {
  const all = (args ?? {}) as Record<string, unknown>;
  for (const key of Object.keys(all)) {
    if (!["filters", "startDate", "endDate", "groupBy", "maxGroups"].includes(key)) {
      throw new Error(`caseResolutionTime: unknown argument "${key}"`);
    }
  }
  const { filters, startDate, endDate, groupBy, maxGroups } = all;
  if (filters !== undefined && !Array.isArray(filters)) {
    throw new Error("caseResolutionTime: filters must be an array of {field, op, values}");
  }
  const body: CaseResolutionTimeRequest = { filters: { filters: (filters as unknown[] | undefined) ?? [] } };
  if ((startDate === undefined) !== (endDate === undefined)) {
    throw new Error("caseResolutionTime: startDate and endDate go together");
  }
  if (startDate !== undefined) {
    const from = windowDay("startDate", startDate);
    const to = windowDay("endDate", endDate);
    if (to < from) throw new Error("caseResolutionTime: endDate must not be before startDate");
    const limit = new Date(`${from}T00:00:00Z`);
    limit.setUTCMonth(limit.getUTCMonth() + RESOLUTION_MAX_MONTHS);
    if (to >= limit.toISOString().slice(0, 10)) {
      throw new Error(`caseResolutionTime: the window covers at most ${RESOLUTION_MAX_MONTHS} months`);
    }
    body.resolvedFrom = startDate as string;
    body.resolvedTo = endDate as string;
  }
  if (groupBy !== undefined) {
    if (!RESOLUTION_GROUP_BY.includes(groupBy as (typeof RESOLUTION_GROUP_BY)[number])) {
      throw new Error(`caseResolutionTime: groupBy must be one of ${RESOLUTION_GROUP_BY.join(", ")}`);
    }
    body.groupBy = groupBy as string;
  }
  if (maxGroups !== undefined) {
    if (groupBy === undefined || !(Number.isInteger(maxGroups) && (maxGroups as number) > 0)) {
      throw new Error("caseResolutionTime: maxGroups must be a positive integer, with groupBy");
    }
    body.maxGroups = maxGroups as number;
  }
  return { body, grouped: groupBy !== undefined };
}

/** One set of resolved cases as the endpoint returns it. */
interface ResolutionTimeBucketView {
  key?: string;
  label?: string;
  count?: number;
  measured?: number;
  averageHours?: number | null;
}

/** Response of POST /cases/resolution-time. */
export interface CaseResolutionTimeResponse {
  resolvedFrom?: string;
  resolvedTo?: string;
  total?: ResolutionTimeBucketView | null;
  groups?: ResolutionTimeBucketView[] | null;
  others?: ResolutionTimeBucketView | null;
}

/** One set of resolved cases as `portal.caseResolutionTime` returns it. */
export interface CodeDashboardResolutionTime {
  count: number;
  measured: number;
  averageHours: number | null;
}

/** What `portal.caseResolutionTime` returns: `groups` only when it asked for a `groupBy`. */
export interface CaseResolutionTimeResult extends CodeDashboardResolutionTime {
  startDate: string | null;
  endDate: string | null;
  groups?: (CodeDashboardResolutionTime & { label: string })[];
}

function bucket(b: ResolutionTimeBucketView | null | undefined): CodeDashboardResolutionTime {
  return { count: b?.count ?? 0, measured: b?.measured ?? 0, averageHours: b?.averageHours ?? null };
}

/** Reshapes the endpoint's answer: the total's figures at the top, the groups (and "Others") as labelled sets. */
export function toCaseResolutionTimeResult(res: CaseResolutionTimeResponse, grouped: boolean): CaseResolutionTimeResult {
  const result: CaseResolutionTimeResult = {
    startDate: res.resolvedFrom ?? null,
    endDate: res.resolvedTo ?? null,
    ...bucket(res.total),
  };
  if (!grouped) return result;
  const groups = (res.groups ?? []).map((g) => ({ label: g.label || g.key || "", ...bucket(g) }));
  if (res.others && (res.others.count ?? 0) > 0) groups.push({ label: "Others", ...bucket(res.others) });
  return { ...result, groups };
}
