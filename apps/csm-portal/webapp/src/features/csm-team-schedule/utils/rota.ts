/**
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

import type { ScheduleAssignment, ScheduleShift } from "../types";


/**
 * The short label for an IANA zone, as that zone calls itself right now.
 *
 * Derived rather than hardcoded so it follows daylight saving: Chicago reads
 * CDT in September and CST in January, instead of the page asserting one of
 * them all year.
 */
/**
 * What the teams call these zones, where Intl has no short name for them.
 *
 * Asia/Colombo comes back from Intl as "GMT+5:30" -- correct, and not what
 * anyone on this rota says. The rota is written in IST and everyone refers to
 * it that way, so the page should too. Brazil is the same story: no
 * abbreviation in the CLDR data, and BRT is what the Americas team says.
 *
 * Everything else falls through to Intl, which follows daylight saving --
 * Chicago reads CDT in September and CST in January rather than the page
 * asserting one of them all year.
 */
const LOCAL_ZONE_NAMES: Record<string, string> = {
  "Asia/Colombo": "IST",
  "Asia/Kolkata": "IST",
  "America/Sao_Paulo": "BRT",
};

export function zoneAbbreviation(tz: string): string {
  const known = LOCAL_ZONE_NAMES[tz];
  if (known) return known;
  try {
    const name = new Intl.DateTimeFormat("en-US", { timeZone: tz, timeZoneName: "short" })
      .formatToParts(new Date())
      .find((p) => p.type === "timeZoneName")?.value;
    return name ?? tz;
  } catch {
    return tz;
  }
}


/**
 * An instant broken into the calendar date and minutes-past-midnight that a
 * reader in `tz` would see.
 *
 * Everything the ladder does -- placing a block, splitting one at midnight,
 * deciding which day it belongs on -- is arithmetic on these two numbers, so
 * doing the conversion once here is what lets the whole view switch clock
 * without any other code knowing about time zones.
 */
export function partsInZone(iso: string, tz: string): { date: string; minutes: number } {
  const parts = new Intl.DateTimeFormat("en-GB", {
    timeZone: tz,
    hour12: false,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  })
    .formatToParts(new Date(iso))
    .filter((p) => p.type !== "literal");
  const p = Object.fromEntries(parts.map((x) => [x.type, x.value])) as Record<string, string>;
  return {
    date: `${p.year}-${p.month}-${p.day}`,
    // Intl renders midnight as 24 in some locales; fold it back to 0.
    minutes: (Number(p.hour) % 24) * 60 + Number(p.minute),
  };
}

/** A rota day as YYYY-MM-DD, in the reader's own clock. */
export function toIsoDate(d: Date): string {
  const p = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}`;
}

export function addDays(d: Date, n: number): Date {
  const out = new Date(d);
  out.setDate(out.getDate() + n);
  return out;
}

/** The Monday of the week containing `d`. Weeks run Monday to Sunday here,
 *  which is how the rota is planned and how the roster sheet reads. */
export function mondayOf(d: Date): Date {
  const out = new Date(d);
  const dow = (out.getDay() + 6) % 7;
  out.setDate(out.getDate() - dow);
  out.setHours(0, 0, 0, 0);
  return out;
}

export function isWeekend(d: Date): boolean {
  return d.getDay() === 0 || d.getDay() === 6;
}

const DOW_SHORT = ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"];

/** Monday-first weekday name, matching how the week grid is laid out. */
export function shortDayName(d: Date): string {
  return DOW_SHORT[(d.getDay() + 6) % 7];
}

/**
 * The calendar day an instant falls on, as the reader's clock sees it:
 * "Thu 24 Sep 2026".
 *
 * A card that runs past midnight used to say only "started 21:00 yesterday".
 * "Yesterday" is relative to the day being looked at, not to today, so on any
 * view but the current one it quietly means the wrong thing -- and on a page
 * where the reader can also change timezone, "yesterday" can move under them.
 * The date says it outright.
 */
export function dayLabel(iso: string, tz?: string): string {
  const zone = tz ?? Intl.DateTimeFormat().resolvedOptions().timeZone;
  const [y, m, d] = partsInZone(iso, zone).date.split("-").map(Number);
  // Built as a local calendar date on purpose: the zone conversion already
  // happened above, and re-applying one here would shift it back.
  //
  // The locale is pinned rather than taken from the browser. This rota is read
  // in Colombo, São Paulo and the US, where a numeric date is genuinely
  // ambiguous -- 09/10 is two different days depending on who is reading it --
  // and a card that says when a night shift started cannot afford that. Day,
  // abbreviated month, year reads the same everywhere.
  return new Date(y, m - 1, d).toLocaleDateString("en-GB", {
    weekday: "short",
    day: "numeric",
    month: "short",
    year: "numeric",
  });
}

/** An instant as HH:MM on the clock the reader has picked. */
export function timeOf(iso: string, tz?: string): string {
  const { minutes } = partsInZone(iso, tz ?? Intl.DateTimeFormat().resolvedOptions().timeZone);
  const p = (n: number) => String(n).padStart(2, "0");
  return `${p(Math.floor(minutes / 60))}:${p(minutes % 60)}`;
}

/** Minutes from the reader's midnight, for placing a block on the day ladder. */
export function minutesOfDay(iso: string): number {
  const d = new Date(iso);
  return d.getHours() * 60 + d.getMinutes();
}

/**
 * How a block should be drawn on a given day's ladder.
 *
 * A window that crosses midnight belongs to the day its crew was rostered for,
 * but it has to appear on both: as a block running off the bottom of the first
 * day, and as one arriving at the top of the next. `startMin`/`endMin` are
 * clamped to the day being drawn, and the flags say which edge was cut so the
 * card can say "runs to 06:00 tomorrow" or "started 21:00 yesterday".
 */
export interface LadderPlacement {
  startMin: number;
  endMin: number;
  continuesNextDay: boolean;
  startedPreviousDay: boolean;
}

export function placeOnDay(
  a: ScheduleAssignment,
  day: Date,
  tz?: string,
): LadderPlacement | null {
  const zone = tz ?? Intl.DateTimeFormat().resolvedOptions().timeZone;
  const target = toIsoDate(day);
  const start = partsInZone(a.startsAt, zone);
  const end = partsInZone(a.endsAt, zone);

  const startsToday = start.date === target;
  const endsToday = end.date === target;
  // A block that neither starts nor ends today can still cover it end to end
  // -- a long night window viewed from a clock far enough west.
  const spansToday = start.date < target && end.date > target;

  if (!startsToday && !endsToday && !spansToday) return null;
  // A window ending exactly at midnight belongs to the day it worked, not to
  // the empty minute after it.
  if (endsToday && !startsToday && !spansToday && end.minutes === 0) return null;

  const startMin = startsToday ? start.minutes : 0;
  const endMin = endsToday && !spansToday ? end.minutes : 1440;

  return {
    startMin,
    endMin: Math.max(endMin, startMin + 1),
    continuesNextDay: !endsToday || spansToday,
    startedPreviousDay: !startsToday,
  };
}

/** Is this instant inside the block? Drives the now-line and "on duty". */
export function isLive(a: ScheduleAssignment, at: Date = new Date()): boolean {
  return new Date(a.startsAt) <= at && at < new Date(a.endsAt);
}

/** Group assignments by a key, preserving the order they arrived in. */
export function groupBy<T>(rows: T[], key: (row: T) => string): Map<string, T[]> {
  const out = new Map<string, T[]>();
  for (const row of rows) {
    const k = key(row);
    const bucket = out.get(k);
    if (bucket) bucket.push(row);
    else out.set(k, [row]);
  }
  return out;
}

/** Index the shift catalogue by code, so a row can find its own window. */
export function shiftsByCode(shifts: ScheduleShift[]): Map<string, ScheduleShift> {
  return new Map(shifts.map((s) => [s.code, s]));
}

/** Initials for an avatar, from a display name. */
export function initialsOf(name: string): string {
  const parts = name.trim().split(/\s+/).filter(Boolean);
  if (parts.length === 0) return "?";
  if (parts.length === 1) return parts[0].slice(0, 2).toUpperCase();
  return (parts[0][0] + parts[parts.length - 1][0]).toUpperCase();
}

/**
 * Is this window a rotation, or just ordinary working hours?
 *
 * "Who else is on with me" means the people holding a rotation -- the morning
 * slot, the evening slot, the night, an escalation tier. Everyone else is at
 * their desk between nine and six, which is most of the team and tells a
 * reader nothing about cover.
 */
export function isRotationShift(shift: ScheduleShift | undefined): boolean {
  if (!shift) return false;
  return shift.isRotation;
}

/**
 * Should this window appear in "who else is on with me"?
 *
 * Regular hours are out, because on a normal weekday that is most of the team.
 * So is the Americas night cover: that team works its own standing shift every
 * day rather than taking a turn in the ABT rotation, so listing all twelve of
 * them under every CRE engineer's week says nothing about who is sharing the
 * rota with them.
 */
export function isPeerRotation(shift: ScheduleShift | undefined): boolean {
  return isRotationShift(shift);
}
