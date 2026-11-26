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
 * A code dashboard's team schedule calls (`portal.searchRota`,
 * `portal.searchAbsences`): the Team Schedule page's own searches, `POST
 * /team-schedule/assignments/search` and `/absences/search`. Both read a
 * window of rota dates (the endpoint needs both ends, at most 70 days apart)
 * and answer every match at once, with no paging. People are passed on by
 * name only, and the free-text notes not at all.
 */

/** The longest window the schedule searches take, in days between the two ends. */
const SCHEDULE_MAX_WINDOW_DAYS = 70;

const PLAIN_DATE = /^\d{4}-\d{2}-\d{2}$/;

/** Body of both schedule searches. */
export interface ScheduleSearchRequest {
  from: string;
  to: string;
  teamKeys?: string[];
  userEmail?: string;
}

/** A window end as a date: a plain date as is, a token as the viewer's local date, or throws. */
function windowDay(fn: string, key: string, value: unknown): string {
  const day =
    typeof value !== "string" ? undefined : PLAIN_DATE.test(value) ? value : resolveRelativeDatePlaceholderToDate(value, new Date());
  if (day === undefined) throw new Error(`${fn}: ${key} must be a date (YYYY-MM-DD) or a date token`);
  return day;
}

/** Whole days from one `YYYY-MM-DD` date to another. */
function daysBetween(from: string, to: string): number {
  return Math.round((Date.parse(`${to}T00:00:00Z`) - Date.parse(`${from}T00:00:00Z`)) / 86_400_000);
}

/**
 * Turns `portal.searchRota(args)` or `portal.searchAbsences(args)` into the
 * search body, or throws (the code sees the message). Args: `startDate` and
 * `endDate` (both needed, at most 70 days apart), `teams` (team keys) and
 * `mine` (the viewer's own, by their email: no other person can be named).
 */
export function buildScheduleSearchRequest(
  fn: string,
  args: unknown,
  userEmail: string | undefined,
): ScheduleSearchRequest {
  const all = (args ?? {}) as Record<string, unknown>;
  for (const key of Object.keys(all)) {
    if (!["startDate", "endDate", "teams", "mine"].includes(key)) throw new Error(`${fn}: unknown argument "${key}"`);
  }
  if (all.startDate === undefined || all.endDate === undefined) {
    throw new Error(`${fn}: startDate and endDate are both needed`);
  }
  const from = windowDay(fn, "startDate", all.startDate);
  const to = windowDay(fn, "endDate", all.endDate);
  const days = daysBetween(from, to);
  if (days < 0) throw new Error(`${fn}: endDate must not be before startDate`);
  if (days > SCHEDULE_MAX_WINDOW_DAYS) {
    throw new Error(`${fn}: the window must be at most ${SCHEDULE_MAX_WINDOW_DAYS} days`);
  }
  const body: ScheduleSearchRequest = { from, to };
  if (all.teams !== undefined) {
    const { teams } = all;
    if (!Array.isArray(teams) || teams.length === 0 || !teams.every((t) => typeof t === "string" && t.trim() !== "")) {
      throw new Error(`${fn}: teams must be a non-empty list of team keys`);
    }
    body.teamKeys = teams.map((t: string) => t.trim().toLowerCase());
  }
  if (all.mine !== undefined && typeof all.mine !== "boolean") throw new Error(`${fn}: mine must be true or false`);
  if (all.mine) {
    if (!userEmail) throw new Error(`${fn}: the signed-in user is not loaded`);
    body.userEmail = userEmail;
  }
  return body;
}

/** An engineer as the schedule searches return one (the fields read here). */
interface ScheduleEngineerView {
  name?: string | null;
  isLead?: boolean;
}

/** A rota assignment as `/team-schedule/assignments/search` returns it (the fields read here). */
interface ScheduleAssignmentView {
  engineer?: ScheduleEngineerView | null;
  teamKey?: string | null;
  shiftCode?: string | null;
  rotaDate?: string | null;
  startsAt?: string | null;
  endsAt?: string | null;
  isOnCall?: boolean;
}

/** One shift as `portal.searchRota` returns it. */
export interface CodeDashboardRotaShift {
  date: string | null;
  engineer: string | null;
  lead: boolean;
  team: string | null;
  shift: string | null;
  startsAt: string | null;
  endsAt: string | null;
  onCall: boolean;
}

/** Reshapes a rota search into what `portal.searchRota` returns. */
export function toSearchRotaResult(res: { assignments?: ScheduleAssignmentView[] | null; count?: number }): {
  total: number;
  items: CodeDashboardRotaShift[];
} {
  const items = (res.assignments ?? []).map(
    (a): CodeDashboardRotaShift => ({
      date: a.rotaDate ?? null,
      engineer: a.engineer?.name ?? null,
      lead: a.engineer?.isLead === true,
      team: a.teamKey ?? null,
      shift: a.shiftCode ?? null,
      startsAt: a.startsAt ?? null,
      endsAt: a.endsAt ?? null,
      onCall: a.isOnCall === true,
    }),
  );
  return { total: res.count ?? items.length, items };
}

/** An absence as `/team-schedule/absences/search` returns it (the fields read here). */
interface ScheduleAbsenceView {
  engineer?: ScheduleEngineerView | null;
  teamKey?: string | null;
  kindCode?: string | null;
  startsOn?: string | null;
  endsOn?: string | null;
  allocatedTo?: string | null;
}

/** One absence as `portal.searchAbsences` returns it: `endsOn` null runs until further notice. */
export interface CodeDashboardAbsence {
  engineer: string | null;
  team: string | null;
  kind: string | null;
  startsOn: string | null;
  endsOn: string | null;
  allocatedTo: string | null;
}

/** Reshapes an absence search into what `portal.searchAbsences` returns. */
export function toSearchAbsencesResult(res: { absences?: ScheduleAbsenceView[] | null; count?: number }): {
  total: number;
  items: CodeDashboardAbsence[];
} {
  const items = (res.absences ?? []).map(
    (a): CodeDashboardAbsence => ({
      engineer: a.engineer?.name ?? null,
      team: a.teamKey ?? null,
      kind: a.kindCode ?? null,
      startsOn: a.startsOn ?? null,
      endsOn: a.endsOn ?? null,
      allocatedTo: a.allocatedTo ?? null,
    }),
  );
  return { total: res.count ?? items.length, items };
}
