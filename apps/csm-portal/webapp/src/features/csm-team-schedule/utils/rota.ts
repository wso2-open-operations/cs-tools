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

import type { ScheduleAbsenceKind, ScheduleAssignment, ScheduleShift, ScheduleTier } from "../types";


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

/**
 * Today's calendar date (YYYY-MM-DD) on the clock of `tz` -- the same clock the
 * cells render in. Without a zone it falls back to the browser's own calendar.
 * Never derive "today" from `new Date()` directly in a view: near midnight the
 * browser and profile calendars name different days.
 */
export function todayIsoInZone(tz?: string, now: Date = new Date()): string {
  return tz ? partsInZone(now.toISOString(), tz).date : toIsoDate(now);
}

/** A YYYY-MM-DD calendar date as a local-midnight Date (the page's day carrier). */
export function dateFromIso(iso: string): Date {
  const [y, m, d] = iso.split("-").map(Number);
  return new Date(y, m - 1, d);
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
  // Words only: a numbering suffix ("Engineer 11") is not an initial, and
  // made every numbered account read "A1".
  const words = name.trim().split(/\s+/).filter(Boolean);
  const parts = words.filter((w) => !/^\d+$/.test(w));
  if (parts.length === 0) return words.length ? words[0].slice(0, 2).toUpperCase() : "?";
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

/**
 * The key two windows share when they are the same working day and differ
 * only in which team works it.
 *
 * Regular hours and the India region shift are both 09:00-18:00, authored in
 * the same zone, worked on the same days -- the code says which team, not a
 * different set of hours. Anywhere the rota is shown grouped by team, listing
 * them apart says nothing the team names do not already say, and costs the
 * reader a second card to cross-reference.
 *
 * Only standing windows fold. A rotation is a turn somebody takes rather than
 * when their team works, and being on call or on an escalation tier is not a
 * fact about which team you are on -- a team list cannot carry either, so
 * those keep their own identity however their hours line up.
 */
export function standingWindowKey(shift: ScheduleShift | undefined, fallback: string): string {
  if (!shift || shift.isRotation || shift.isOnCall || shift.isEscalation) return fallback;
  return [
    "window",
    shift.family,
    shift.startMinute,
    shift.endMinute,
    shift.authoringTimeZone,
    shift.dayScope,
  ].join(":");
}

/** One zone's row of the escalation grid: which window each tier goes on. */
export interface EscalationRow {
  zoneCode: string;
  /** What the zone is called on this kind of day -- "TZ1+2" at the weekend,
   *  when TZ1 and TZ2 are one crew -- see zoneLabelOn. */
  label: string;
  tiers: { tier: ScheduleTier; shift?: ScheduleShift }[];
}

const ESCALATION_TIERS: ScheduleTier[] = ["L1", "L2", "L3"];

/**
 * The escalation grid a lead picks from: every zone worked on this kind of day
 * (weekday or weekend), and L1, L2 and L3 in each.
 *
 * A tier goes on the zone's window that fixes that tier where there is one
 * (TZ1's own L1 window), else on the zone's escalation window that leaves the
 * tier to the person (SRE_TZ1, SRE_TZ3). A tier with neither is left without
 * a window, and the picker shows it unavailable rather than guessing one.
 */
export function escalationGrid(shifts: ScheduleShift[], iso: string): EscalationRow[] {
  const d = new Date(`${iso}T00:00:00`);
  const weekend = d.getDay() === 0 || d.getDay() === 6;
  const worked = (s: ScheduleShift) =>
    s.dayScope === "ANY" || (s.dayScope === "WEEKEND") === weekend;
  const esc = shifts.filter((s) => s.isEscalation && s.zoneCode && worked(s));
  const zones = [...new Set(esc.map((s) => s.zoneCode as string))].sort();
  return zones.map((zoneCode) => {
    const here = esc.filter((s) => s.zoneCode === zoneCode).sort((a, b) => a.sortOrder - b.sortOrder);
    return {
      zoneCode,
      label: zoneLabelOn(shifts, zoneCode, weekend),
      tiers: ESCALATION_TIERS.map((tier) => ({
        tier,
        shift: here.find((s) => s.tier === tier) ?? here.find((s) => !s.tier),
      })),
    };
  });
}

/** Which zone and tier an escalation turn is, where it is one. */
export interface EscalationTurn {
  zoneCode: string;
  tier: ScheduleTier;
  weekend: boolean;
}

/**
 * The zone and tier an assignment holds, if it is an escalation turn at all.
 *
 * The tier is the assignment's own, else the window's (TZ1's own L1 window
 * fixes it). A turn on a zone's shared escalation window with no tier is not
 * a turn -- see isTierlessEscalation -- and comes back null here.
 */
export function escalationTurnOf(
  a: ScheduleAssignment,
  shifts: Map<string, ScheduleShift>,
): EscalationTurn | null {
  const sh = shifts.get(a.shiftCode);
  const zoneCode = a.zoneCode ?? sh?.zoneCode;
  const tier = a.tier ?? sh?.tier;
  if (!sh?.isEscalation || !zoneCode || !tier) return null;
  return { zoneCode, tier, weekend: sh.dayScope === "WEEKEND" };
}

/**
 * No tier on a zone's shared escalation window. That is what "works this
 * zone, is not on the escalation rota" looks like -- the zone's regular hours
 * -- so the views read it as that rather than as a turn missing its tier.
 * New ones cannot be written any more; this is for rows written before.
 */
export function isTierlessEscalation(a: ScheduleAssignment, shifts: Map<string, ScheduleShift>): boolean {
  const sh = shifts.get(a.shiftCode);
  return Boolean(sh?.isEscalation && (a.zoneCode ?? sh.zoneCode) && !a.tier && !sh.tier);
}

/**
 * What a zone is called on a weekday or at the weekend.
 *
 * At the weekend TZ1 and TZ2 are one crew, carried under TZ1, and the
 * catalogue's window for it says so in its short code ("TZ1+2"). So the name
 * is read off the zone's own escalation window for that kind of day -- the
 * one that leaves the tier open -- rather than written down here, and a
 * weekday, or a zone with no such window, is simply its code.
 */
export function zoneLabelOn(
  shifts: ScheduleShift[] | Map<string, ScheduleShift>,
  zoneCode: string,
  weekend: boolean,
): string {
  if (!weekend) return zoneCode;
  const list = Array.isArray(shifts) ? shifts : [...shifts.values()];
  const own = list.find(
    (s) => s.isEscalation && !s.tier && s.zoneCode === zoneCode && s.dayScope === "WEEKEND",
  );
  // Only a short code that extends the zone's own ("TZ1" -> "TZ1+2") names
  // the crew; anything else ("L1") is the chip, not a zone name.
  return own?.shortCode && own.shortCode !== zoneCode && own.shortCode.startsWith(zoneCode)
    ? own.shortCode
    : zoneCode;
}


/** The kinds a lead may mark somebody away for on one rota: leave, and the
 *  time allocations that rota uses.
 *
 *  CRE and SRE allocate time to different things -- RnD is SRE's, Migration
 *  is CRE's -- so each is offered only its own, plus what both share (every
 *  kind of leave, Allo-INT, Allo-EXT, the Brazil rotation). EXCLUDED, off
 *  the rota entirely, is not a lead's to set from a cell, and a retired kind
 *  is served only so the days already marked with it keep their label. */
export function kindsOfferedOn(
  kinds: readonly ScheduleAbsenceKind[],
  family: "CRE" | "SRE",
): ScheduleAbsenceKind[] {
  return kinds.filter(
    (k) =>
      (k.bucket === "LEAVE" || k.bucket === "ALLOCATION") &&
      !k.retired &&
      (!k.family || k.family === family),
  );
}

/** How many months the roster can show at once. */
export type RosterSpan = 1 | 3 | 6;

/** Days either side of the selected day for each span: a span is centred on
 *  the day the reader is looking at (today, until they move it), not a run of
 *  calendar months. One month is two weeks either way; three and six are a
 *  month and a half and three months either way. */
export const ROSTER_SPAN_HALF_DAYS: Record<RosterSpan, number> = { 1: 14, 3: 45, 6: 91 };

/** The first and last day the roster shows around `anchor`. */
export function rosterRange(anchor: Date, span: RosterSpan): { start: Date; end: Date } {
  const day = new Date(anchor.getFullYear(), anchor.getMonth(), anchor.getDate());
  const half = ROSTER_SPAN_HALF_DAYS[span];
  return { start: addDays(day, -half), end: addDays(day, half) };
}

/** A range cut into the calendar months it touches, each clipped to the
 *  range: the shape the rota is fetched in. A turn belongs to one rota day, so
 *  the pieces never overlap; the whole months in the middle keep the same
 *  query key as the reader steps a day, so only the two ends are re-read. */
export function monthPieces(start: Date, end: Date): { from: string; to: string }[] {
  const out: { from: string; to: string }[] = [];
  let y = start.getFullYear();
  let m = start.getMonth();
  const last = toIsoDate(end);
  const first = toIsoDate(start);
  for (;;) {
    const monthFrom = toIsoDate(new Date(y, m, 1));
    const monthTo = toIsoDate(new Date(y, m + 1, 0));
    if (monthFrom > last) break;
    out.push({ from: monthFrom < first ? first : monthFrom, to: monthTo > last ? last : monthTo });
    m += 1;
    if (m === 12) {
      m = 0;
      y += 1;
    }
  }
  return out;
}

/** The rota a reader belongs to, for deciding what the page shows them.
 *
 *  Their team says it where they have one. A rota admin holds no team, but
 *  runs one group's rota all the same, and reads that group's views as their
 *  own and the other group's Today only. Two things can say which group:
 *
 *   - the cre_rota_admin / sre_rota_admin role, where the sign-in carries it
 *     (matched on the part after any namespace, "x.cre_rota_admin");
 *   - the teams they may edit, which the server grants a rota admin as every
 *     team of their group -- the only signal when the sign-in carries only
 *     its groups, not the portal's own roles.
 *
 *  Both roles, or editable teams spanning both groups, or none of it, is
 *  nobody's group: a manager, who sees both groups everywhere. */
export function readerFamily(
  teamFamily: string | undefined | null,
  roles: readonly string[] | undefined | null,
  editableFamilies: readonly string[] = [],
): "CRE" | "SRE" | undefined {
  const f = teamFamily?.toUpperCase();
  if (f) return f.startsWith("SRE") ? "SRE" : "CRE";
  const held = new Set((roles ?? []).map((r) => r.toLowerCase().replace(/^.*\./, "")));
  const cre = held.has("cre_rota_admin");
  const sre = held.has("sre_rota_admin");
  if (cre !== sre) return cre ? "CRE" : "SRE";
  if (cre && sre) return undefined;
  const edits = new Set(editableFamilies.map((x) => (x.toUpperCase().startsWith("SRE") ? "SRE" : "CRE")));
  return edits.size === 1 ? [...edits][0] as "CRE" | "SRE" : undefined;
}
