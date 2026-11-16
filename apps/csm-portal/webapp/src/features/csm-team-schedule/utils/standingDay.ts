/*
 * Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import type { ScheduleAbsence, ScheduleAbsenceKind, ScheduleAssignment, ScheduleShift, ScheduleTeam } from "../types";
import { coverSideOf } from "./handover";
import { HALF_DAY_LEAVE_CODES, partsInZone } from "./rota";

/** The window a team's ordinary weekday is when the team names none. */
export const REGULAR_HOURS_CODE = "CRE_REGULAR";

/** The India region shift (IND). */
export const INDIA_HOURS_CODE = "CRE_REGULAR_IND";

/**
 * Which window somebody's ordinary weekday is on their team: the team's own
 * (Americas cover, the India region shift), or Regular hours when it names
 * none. One exception: a team whose ordinary day is the India region shift
 * is led by an engineer from the Sri Lankan teams on rotation, so its lead
 * keeps Regular hours. Only that team: the Americas team's leads work Americas cover like
 * the rest of it.
 */
export function ordinaryDayCode(teamDefault: string | undefined, isLead: boolean): string {
  if (isLead && teamDefault === INDIA_HOURS_CODE) return REGULAR_HOURS_CODE;
  return teamDefault ?? REGULAR_HOURS_CODE;
}

/** Roles that sit above the rota and are never on it (the roster leaves them out too). */
const MANAGEMENT_ROLES = new Set(["americas_team_lead", "cre_head", "cs_head"]);

/** Marks a row this module made up, so nothing mistakes it for a stored one. */
export const STANDING_DAY_SOURCE = "STANDING_DAY";

/**
 * The instant a wall-clock minute of a calendar day is in a time zone, as ISO.
 * A minute past 1440 runs into the next day, the way a window that crosses
 * midnight is stored (21:00-06:00 is 1260-1800).
 */
export function instantInZone(isoDate: string, minute: number, timeZone: string): string {
  const [y, m, d] = isoDate.split("-").map(Number);
  const wall = Date.UTC(y, m - 1, d, 0, minute);
  let t = wall;
  // Two passes settle the zone's offset, including on a day it changes.
  for (let i = 0; i < 2; i++) {
    const seen = partsInZone(new Date(t).toISOString(), timeZone);
    const [sy, sm, sd] = seen.date.split("-").map(Number);
    t += wall - Date.UTC(sy, sm - 1, sd, 0, seen.minutes);
  }
  return new Date(t).toISOString();
}

function isWeekdayIso(iso: string): boolean {
  const dow = new Date(`${iso}T00:00:00`).getDay();
  return dow !== 0 && dow !== 6;
}

function covers(ab: ScheduleAbsence, iso: string): boolean {
  return ab.startsOn <= iso && (!ab.endsOn || iso <= ab.endsOn);
}

/**
 * Whether a row belongs on the day view's card for its window.
 *
 * A standing window is somebody's ordinary day, not a turn, so its card lists
 * only the teams whose ordinary day it is: Americas cover lists the Americas
 * team (a Brazil rotation is filed under Americas, so it stays), and Regular
 * hours and the India region shift list the ABTs -- not Migration, not a day
 * somebody on another team was put on Americas cover by hand. Turns (morning,
 * evening, weekend, on-call, escalation) are not touched. Until the server
 * says which teams are ABTs, the regular-hours rule is not applied.
 *
 * A standing window on the Americas side that is no team's ordinary day any
 * more -- Americas cover, once the Americas team's day moved to Americas
 * regular hours on Pacific time -- still belongs to the team whose day is on
 * that side, so its stored rows keep listing the Americas team only.
 */
export function onItsStandingCard(
  row: ScheduleAssignment,
  shifts: ReadonlyMap<string, ScheduleShift>,
  teams: readonly ScheduleTeam[],
): boolean {
  const window = shifts.get(row.shiftCode);
  if (!window || window.isRotation) return true;
  const team = teams.find((t) => t.key.toLowerCase() === row.teamKey.toLowerCase());
  if (teams.some((t) => t.defaultShiftCode === row.shiftCode)) return team?.defaultShiftCode === row.shiftCode;
  if (coverSideOf(window) === "americas") {
    const americasDay = (t: ScheduleTeam | undefined): boolean => {
      const own = t?.defaultShiftCode ? shifts.get(t.defaultShiftCode) : undefined;
      return own !== undefined && coverSideOf(own) === "americas";
    };
    if (teams.some(americasDay)) return americasDay(team);
  }
  const abtKnown = teams.some((t) => t.isAbt !== undefined);
  return !abtKnown || team?.isAbt === true;
}

/**
 * Who is working their team's ordinary day without a row on the rota to say so.
 *
 * The month roster paints every blank weekday of a team member as that team's
 * ordinary hours -- Regular hours, or Americas cover on the Americas team --
 * because that is what a blank weekday means. No row is stored for it, and the
 * day view drew only stored rows, so somebody the roster showed at work was
 * missing from "Who is on today". This makes the same rows up for the day
 * view, from the same rule:
 *
 *  - a weekday, for a member of a team in scope (not a manager above the rota),
 *    with no row of their own that rota day and no leave or allocation over it;
 *  - and the crews either side of the day: somebody whose turn that day is a
 *    window that touches their team's ordinary hours -- the evening that
 *    starts as they end (18:00-21:00 after 09:00-18:00), or the morning that
 *    ends as they start (06:00-09:00, on-call included) -- works the ordinary
 *    day too, so they are listed in it as well. An escalation tier is not a
 *    turn of that kind and adds nothing.
 *
 * Someone on two teams is listed once, under the first in catalogue order.
 * Display only: nothing here is saved, and paging reads the stored rota, never
 * these rows.
 */
export function standingDayRows({
  rotaDates,
  teams,
  shifts,
  assignments,
  absences,
}: {
  rotaDates: readonly string[];
  /** The teams on screen, members included, in display order. */
  teams: readonly ScheduleTeam[];
  shifts: ReadonlyMap<string, ScheduleShift>;
  assignments: readonly ScheduleAssignment[];
  absences: readonly ScheduleAbsence[];
}): ScheduleAssignment[] {
  const out: ScheduleAssignment[] = [];
  const windowOf = (team: ScheduleTeam, isLead: boolean): ScheduleShift | undefined =>
    shifts.get(ordinaryDayCode(team.defaultShiftCode, isLead));
  const teamByKey = new Map(teams.map((t) => [t.key.toLowerCase(), t]));

  for (const iso of rotaDates) {
    if (!isWeekdayIso(iso)) continue;
    const rowsToday = assignments.filter((a) => a.rotaDate === iso);
    const withRow = new Set(rowsToday.map((a) => a.engineer.userId));
    // Someone on half-day leave still works part of the day and stays listed;
    // the Leave card says which half they are away.
    const away = new Set(
      absences
        .filter((ab) => covers(ab, iso) && !HALF_DAY_LEAVE_CODES.has(ab.kindCode))
        .map((ab) => ab.engineer.userId),
    );
    const listed = new Set<string>();

    const add = (team: ScheduleTeam, window: ScheduleShift, engineer: ScheduleAssignment["engineer"]): void => {
      listed.add(engineer.userId);
      const end = window.endMinute > window.startMinute ? window.endMinute : window.endMinute + 1440;
      out.push({
        id: `standing:${engineer.userId}:${iso}`,
        engineer,
        teamKey: team.key,
        shiftCode: window.code,
        rotaDate: iso,
        startsAt: instantInZone(iso, window.startMinute, window.authoringTimeZone),
        endsAt: instantInZone(iso, end, window.authoringTimeZone),
        isOnCall: false,
        source: STANDING_DAY_SOURCE,
      });
    };

    // A blank weekday: the team's ordinary day.
    for (const team of teams) {
      for (const m of team.members ?? []) {
        if (MANAGEMENT_ROLES.has(m.role) || listed.has(m.userId)) continue;
        if (withRow.has(m.userId) || away.has(m.userId)) continue;
        const window = windowOf(team, m.isLead);
        if (!window) continue;
        add(team, window, { userId: m.userId, name: m.name, email: m.email, isLead: m.isLead });
      }
    }

    // The morning and evening crews work the ordinary day around their turn.
    for (const a of rowsToday) {
      const id = a.engineer.userId;
      if (listed.has(id) || away.has(id)) continue;
      const team = teamByKey.get(a.teamKey.toLowerCase());
      const window = team ? windowOf(team, a.engineer.isLead) : undefined;
      const turn = shifts.get(a.shiftCode);
      if (!team || !window || !turn || !turn.isRotation || turn.isEscalation) continue;
      if (turn.startMinute !== window.endMinute && turn.endMinute !== window.startMinute) continue;
      // Already holding a standing window that day: nothing to add.
      const standing = rowsToday.some((r) => r.engineer.userId === id && shifts.get(r.shiftCode)?.isRotation === false);
      if (standing) continue;
      add(team, window, a.engineer);
    }
  }
  return out;
}

/**
 * Stored Americas-side standing rows, drawn at the team's own ordinary hours.
 *
 * The Americas team's weekdays are mostly stored rows on Americas cover
 * (21:00-06:00 on the Colombo clock), written by the roster import and by
 * hand, while the team actually works its ordinary day, Americas regular hours
 * on the Pacific clock. Drawn as stored, the day view never shows where that
 * day meets the Sri Lankan one. So, for the day view only, a row on a standing
 * Americas-side window that is not its team's own ordinary day -- on a weekday,
 * for a team whose ordinary day is itself on the Americas side -- is moved onto
 * that ordinary day's hours for the same rota date.
 *
 * The Americas weekend turn (and its on-call) is stored 21:00-06:00 on the
 * Colombo clock, back to back with the Sri Lankan weekend rotation, so from
 * Saturday evening to Monday morning the two days never overlapped on screen.
 * Its rows are drawn at the same Americas ordinary hours for their rota date,
 * keeping their own window (title, on-call) and team, so the weekend reads
 * like every other day.
 *
 * Display only: nothing is saved, and paging reads the stored rota, never
 * these rows. Other turns and every Sri Lankan row are left as they are.
 */
export function atTeamOrdinaryHours(
  rows: readonly ScheduleAssignment[],
  shifts: ReadonlyMap<string, ScheduleShift>,
  teams: readonly ScheduleTeam[],
): ScheduleAssignment[] {
  const teamByKey = new Map(teams.map((t) => [t.key.toLowerCase(), t]));
  const at = (row: ScheduleAssignment, own: ScheduleShift, shiftCode: string): ScheduleAssignment => {
    const end = own.endMinute > own.startMinute ? own.endMinute : own.endMinute + 1440;
    return {
      ...row,
      shiftCode,
      startsAt: instantInZone(row.rotaDate, own.startMinute, own.authoringTimeZone),
      endsAt: instantInZone(row.rotaDate, end, own.authoringTimeZone),
    };
  };
  // The Americas ordinary day, when exactly one is set as a team's own.
  const americasDays = new Set(
    teams
      .map((t) => (t.defaultShiftCode ? shifts.get(t.defaultShiftCode) : undefined))
      .filter((s): s is ScheduleShift => !!s && !s.isRotation && coverSideOf(s) === "americas")
      .map((s) => s.code),
  );
  const americasDay = americasDays.size === 1 ? shifts.get([...americasDays][0]) : undefined;
  return rows.map((row) => {
    const stored = shifts.get(row.shiftCode);
    if (!stored) return row;
    if (stored.isRotation) {
      const weekendTurn = stored.dayScope === "WEEKEND" && !stored.isEscalation && coverSideOf(stored) === "americas";
      return weekendTurn && americasDay ? at(row, americasDay, stored.code) : row;
    }
    if (coverSideOf(stored) !== "americas") return row;
    if (!isWeekdayIso(row.rotaDate)) return row;
    const team = teamByKey.get(row.teamKey.toLowerCase());
    const own = team?.defaultShiftCode ? shifts.get(team.defaultShiftCode) : undefined;
    if (!own || own.code === stored.code || own.isRotation || coverSideOf(own) !== "americas") return row;
    return at(row, own, own.code);
  });
}

/**
 * Stored rows of people who are not at work that day, left out of the day
 * view's cards.
 *
 * A row on the rota survives the leave or allocation granted against it
 * afterwards -- the two are stored apart -- so a card read straight from the
 * rota counted someone on annual leave in Regular hours and listed them under
 * Leave in the off-rota column as well. A row is dropped when its person has a
 * full-day leave or allocation covering its rota date. Kept:
 *  - half-day leave: they still work part of the day, and the Leave card says
 *    which half;
 *  - a stint worked on another team's rota (the Brazil rotation on the
 *    Americas rota) -- its rows on that team are the stint itself;
 *  - a retired kind, which no longer marks anybody away (the off-rota column
 *    leaves it out too), and Excluded from rota, which is not a day's absence;
 *  - a rotation turn on a day with an allocation (customer or internal work):
 *    the allocation takes their ordinary hours, not the turn they hold around
 *    them, so the Evening card still lists whoever the roster shows on it.
 *    A move to a team that works no rota (Migration) is different: they take
 *    no turns there, so it drops every row.
 *
 * Display only, as Case Alerts already leaves out whoever is away when it
 * reads who is on duty.
 */
export function atWorkOnTheDay(
  rows: readonly ScheduleAssignment[],
  absences: readonly ScheduleAbsence[],
  kinds: readonly ScheduleAbsenceKind[],
  shifts?: ReadonlyMap<string, ScheduleShift>,
): ScheduleAssignment[] {
  const kindByCode = new Map(kinds.map((k) => [k.code, k]));
  const awayAll = absences.filter((ab) => {
    const kind = kindByCode.get(ab.kindCode);
    if (!kind || kind.retired || HALF_DAY_LEAVE_CODES.has(ab.kindCode)) return false;
    return kind.bucket === "LEAVE" || kind.bucket === "ALLOCATION";
  });
  if (awayAll.length === 0) return [...rows];
  const byPerson = new Map<string, ScheduleAbsence[]>();
  for (const ab of awayAll) byPerson.set(ab.engineer.userId, [...(byPerson.get(ab.engineer.userId) ?? []), ab]);
  return rows.filter((row) => {
    const spans = byPerson.get(row.engineer.userId)?.filter((ab) => covers(ab, row.rotaDate)) ?? [];
    const turn = shifts?.get(row.shiftCode)?.isRotation === true;
    return spans.every((ab) => {
      const kind = kindByCode.get(ab.kindCode);
      if (kind?.worksRotaThere && ab.teamKey.toLowerCase() === row.teamKey.toLowerCase()) return true;
      return turn && kind?.bucket === "ALLOCATION" && !kind.movesToTeamKey;
    });
  });
}

/**
 * Who is at work on CRE's rota days, as the day view and the week view both
 * show it.
 *
 * Everyone the month roster shows at work, not only those with a row: a blank
 * weekday is the team's ordinary day, and the morning and evening crews work
 * the ordinary day around their turn (standingDayRows). Stored Americas-side
 * standing days are drawn at the team's own hours (atTeamOrdinaryHours), a
 * stored row of somebody on full-day leave or an allocation that day is left
 * out (atWorkOnTheDay), and each standing window lists only the teams whose
 * ordinary day it is (onItsStandingCard).
 *
 * One function for both views: This week used to count the stored rows alone,
 * so its Regular hours numbers disagreed with Today's for the same day --
 * blank weekdays missing, people on an allocation still counted.
 *
 * Display only: nothing here is saved, and paging reads the stored rota.
 */
export function creRowsAtWork({
  rotaDates,
  teams,
  teamKeysInView,
  shifts,
  assignments,
  absences,
  absenceKinds,
}: {
  rotaDates: readonly string[];
  /** The whole catalogue's teams: which team a window is the ordinary day
   *  of does not depend on the team picked. */
  teams: readonly ScheduleTeam[];
  /** The teams in view; empty for the whole family. */
  teamKeysInView: readonly string[];
  shifts: ReadonlyMap<string, ScheduleShift>;
  assignments: readonly ScheduleAssignment[];
  absences: readonly ScheduleAbsence[];
  absenceKinds: readonly ScheduleAbsenceKind[];
}): ScheduleAssignment[] {
  const inScope = new Set(teamKeysInView.map((k) => k.toLowerCase()));
  const allCreTeams = teams.filter((t) => t.family === "CRE");
  const creTeams = allCreTeams
    .filter((t) => inScope.size === 0 || inScope.has(t.key.toLowerCase()))
    .sort((a, b) => a.sortOrder - b.sortOrder);
  const standing = standingDayRows({ rotaDates, teams: creTeams, shifts, assignments, absences });
  return [
    ...atTeamOrdinaryHours(atWorkOnTheDay(assignments, absences, absenceKinds, shifts), shifts, allCreTeams),
    ...standing,
  ].filter((a) => onItsStandingCard(a, shifts, allCreTeams));
}
