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

import { useEffect, useMemo, useRef, useState, type CSSProperties, type JSX, type KeyboardEvent as ReactKeyboardEvent, type ReactNode } from "react";
import RosterLegend, { type LegendItem } from "./RosterLegend";
import RotaPicker, { type RotaOption } from "./RotaPicker";
import type {
  CellAbsence,
  EngagementAllocation,
  ScheduleAbsence,
  ScheduleAbsenceKind,
  ScheduleAssignment,
  ScheduleShift,
  ScheduleTeamMember,
  ScheduleTier,
  RotaFamily,
} from "../types";
import { addDays, initialsOf, isRotationShift, kindsOfferedOn, mondayOf, movesOfferedOn, toIsoDate, zoneColumnOf, zoneLabelOn, type RosterSpan } from "../utils/rota";
import {
  ENGAGEMENTS_FAILED_NOTE,
  ENGAGEMENT_MARK_LEGEND_ITEM,
  allocationsByPersonAndDay,
  dayMarkerAriaLabel,
  dayMarkerTitle,
} from "../utils/engagementAllocations";
import { ordinaryDayCode, REGULAR_HOURS_CODE } from "../utils/standingDay";
import { useTeamColour, useTeamName } from "../utils/teamColourContext";

export type { RosterSpan };
const SPANS: readonly RosterSpan[] = [1, 3, 6];

interface MonthRosterProps {
  /** The first month on the grid. */
  month: Date;
  /** How many calendar months run across it, starting at `month`. The rota
   *  sheet this replaces showed a whole year; one month was too little to
   *  check a swap against last month or plan the next. */
  monthCount?: number;
  /** The exact first and last day, where the grid is a window around a day
   *  rather than whole months. Both or neither; with them, `month` and
   *  `monthCount` are ignored. */
  from?: Date;
  to?: Date;
  /** The span the reader has picked, and the way to change it. The page owns
   *  it because the page fetches the months; without a handler there is no
   *  choice to offer. */
  span?: RosterSpan;
  onSpanChange?: (span: RosterSpan) => void;
  /** Confirmed customer engagement allocations over the days shown, drawn
   *  read-only as a strip under each day an engineer is on one. */
  engagementAllocations?: EngagementAllocation[];
  /** The allocations could not be read: the roster is drawn without them,
   *  with a quiet note saying so. */
  engagementsFailed?: boolean;
  assignments: ScheduleAssignment[];
  absences: ScheduleAbsence[];
  shifts: Map<string, ScheduleShift>;
  absenceKinds: ScheduleAbsenceKind[];
  /** Counts presses of Today. The day alone cannot express "take me back
   *  there" once the reader has scrolled away without changing it, so the
   *  press is what the grid listens to. */
  focusRequest?: number;
  /** The day the date picker is sitting on, as YYYY-MM-DD.
   *
   *  Distinct from today: today is a fact, this is a choice. The roster is a
   *  month wide, so a choice the grid does not show is a choice the reader
   *  cannot see they made. */
  selectedIso: string;
  /** The group and team the page is filtered to, and the means to change them.
   *
   *  The prototype puts these in the card's own head rather than only in the
   *  page toolbar, and it is right to: the roster is the view where "which
   *  engineers am I looking at" is the whole question, so the answer belongs
   *  where the answer is read. They are the page's own state passed down, not
   *  a second copy -- one control rendered in two places, which is why the
   *  toolbar above stays in step with them. */
  family: RotaFamily;
  onFamilyChange: (family: RotaFamily) => void;
  teamKey: string;
  onTeamKeyChange: (teamKey: string) => void;
  teams: string[];
  /** Each team's leads, by team key. A lead is seated at the top of their
   *  team even on a month they hold no window, which is most months. */
  teamMembers?: Readonly<Record<string, readonly ScheduleTeamMember[]>>;
  /** The teams each person is a member of, by user id, across every team --
   *  not only the ones on screen. Leave or an allocation filed under a team
   *  the person does not belong to (marked on a row they no longer have, or
   *  before they changed team) is drawn on their own team's row instead of
   *  seating them on a second team the same day. */
  ownTeams?: ReadonlyMap<string, readonly string[]>;
  /** Each team's ordinary-weekday window, by team key, where it is not
   *  Regular hours -- Americas cover for the Americas team. */
  teamDefaultShift?: Readonly<Record<string, string>>;
  /** Teams whose unmarked weekday draws nothing. LK is a CRE day across every
   *  zone; an SRE SaaS engineer works TZ1 or TZ2 and nights, which the
   *  generated SUP says, so drawing LK there would claim all three. */
  teamsWithoutDefault?: ReadonlySet<string>;
  /** CRE and SRE in the order they should read -- the reader's own group
   *  first, because the first of a pair reads as the default. */
  families: readonly RotaFamily[];
  /** The real zone behind a roster column for one team -- the Day column is
   *  ASG_D for Asgardeo and MOE_D for Moesif. Absent: the column is the zone. */
  zoneCodeFor?: (teamKey: string, column: string) => string | undefined;
  /** Controls that belong to the roster alone (Recent changes), at the end of
   *  its own head -- where the page toolbar would otherwise wrap onto a second
   *  line beside the tabs on this one view. */
  actions?: ReactNode;
  /** The rotas of the family on screen, the one shown, and the change; the
   *  picker appears only when there is more than one. */
  rotas?: readonly RotaOption[];
  rotaCode?: string;
  onRotaChange?: (code: string) => void;
  /** The signed-in reader, so their own row can be marked and brought into
   *  view. A month of a hundred-odd engineers is a haystack otherwise. */
  meEmail?: string;
  /** The teams this reader leads, and may therefore edit. Empty for everyone
   *  else, which is most people. */
  leadTeams?: readonly string[];
  /** Which cells somebody has changed by hand, keyed "userId|rotaDate".
   *
   *  Optional and arrives late on purpose: the grid renders from the rota
   *  alone and picks these up when they load, so a slow history lookup never
   *  holds up a three-month page. */
  editedCells?: ReadonlyMap<string, { actor: string; changedAt: string }>;
  /** Whether the page is in edit mode. A lead reads this grid far more often
   *  than they change it, so cells are inert until editing is switched on --
   *  a rota that writes on a single stray click is worse than one that needs
   *  two. The toggle lives in the page toolbar, beside the other controls
   *  that change what a click does. */
  editing?: boolean;
  /** `${userId}|${YYYY-MM-DD}` for every day changed in this editing session,
   *  so those cells can be marked until the lead clicks Done editing. */
  changedCells?: ReadonlySet<string>;
  /** Called when a lead picks a cell to change. The roster does not own the
   *  picker -- it only reports which slot was chosen, what is on it, and
   *  where on screen it is, so the picker can open against the cell rather
   *  than in the middle of the grid it is about. */
  onEditCell?: (edit: {
    userId: string;
    name: string;
    teamKey: string;
    rotaDate: string;
    shiftCode?: string;
    /** The tier held on that window, where it holds one. */
    tier?: ScheduleTier;
    absenceKindCode?: string;
    /** Which zone column was clicked, on an SRE day split across them. The
     *  picker narrows to that zone's own windows: offering TZ1's windows
     *  from the TZ2 column is a mis-click waiting to happen. Absent on a CRE
     *  day, and on leave, which belongs to the whole day rather than a zone. */
    zoneCode?: string;
    /** The whole absence this cell is one day of, so the picker can offer to
     *  remove all of it rather than the day that was clicked. */
    absence?: CellAbsence;
    anchor: { top: number; left: number; bottom: number; right: number };
  }) => void;
}



interface Cell {
  code: string;
  token: string;
  title: string;
  /** A turn on the rota, as opposed to leave, an allocation, or the standing
   *  regular-hours window that most of the team sits in on a normal day. */
  isRotation: boolean;
  /** The window this cell came from, where it came from a rota row at all.
   *  Absent for leave and allocations, which are not a window. */
  shiftCode?: string;
  /** The tier held on that window, where it holds one. */
  tier?: ScheduleTier;
  /** The absence kind covering this day, where one does. The picker marks it
   *  as what is held so leave reads the same as a rotation does. */
  absenceKindCode?: string;
  /** The span that kind comes from, which the picker can remove whole. */
  absence?: CellAbsence;
  /** Drawn, not stored: what an unmarked weekday means for the row's team. */
  isDefault?: boolean;
  /** A turn filed under a team the person is no longer on, drawn on their own
   *  team's row: the team it is really filed under, so changing or clearing
   *  the day acts on that turn rather than on the row's team. */
  filedUnder?: string;
  /** An on-call window. Two windows can share a short code ("6-9am" and
   *  "6-9am on-call"), so the chip is drawn dashed -- the grid's mark for
   *  on-call, as on L3 -- rather than relying on a tooltip to tell them apart. */
  onCall?: boolean;
  /** A move to another team drawn as that team's hours (Americas cover on
   *  the Brazil rotation): it reads, and is explained, as those hours. */
  asHours?: boolean;
  /** A day of a move onto a team that is worked as that team's own rota with
   *  no hours of its own (the SaaS Night cover), on that team's row: being on
   *  the row says it all, so it draws as an empty day -- and still opens the
   *  move, so it can be ended or removed. */
  quiet?: boolean;
}

/** What a weekday nobody has marked is: an ordinary working day.
 *
 *  Drawn, never stored. The rota sheet this replaces wrote LK into every
 *  such cell by hand; here it is simply what an empty weekday means, so it
 *  gives way the moment leave, an allocation or a turn is marked, and there
 *  is nothing to clean up when one is. Not a turn, so "Rotations only" fades
 *  it like the rest of the standing hours. */
const WORKING_DAY: Cell = {
  code: "LK",
  token: "LK",
  title: "Working day (LK)",
  isRotation: false,
  isDefault: true,
};

/**
 * The month as engineers down the side and days across the top -- the shape of
 * the rota sheet this replaces, so a lead reading it recognises it.
 *
 * Each cell is the short code the rest of the page uses, which is the point of
 * storing a short code alongside the label: the roster is only readable if a
 * day fits in a column three characters wide.
 */
export default function MonthRoster({
  month,
  monthCount = 1,
  from: rangeFrom,
  to: rangeTo,
  span,
  onSpanChange,
  engagementAllocations,
  engagementsFailed = false,
  assignments,
  absences,
  shifts,
  absenceKinds,
  selectedIso,
  focusRequest,
  family,
  onFamilyChange,
  teamKey,
  onTeamKeyChange,
  teams,
  teamMembers,
  ownTeams,
  teamDefaultShift,
  teamsWithoutDefault,
  families,
  rotas,
  rotaCode,
  onRotaChange,
  zoneCodeFor,
  actions,
  meEmail,
  leadTeams,
  editedCells,
  editing = false,
  changedCells,
  onEditCell,
}: MonthRosterProps): JSX.Element {
  const teamColourOf = useTeamColour();
  const teamNameOf = useTeamName();
  const [query, setQuery] = useState("");
  /** Fade everything that is not a turn on the rota.
   *
   *  A month of 122 engineers is mostly leave and allocations by volume, and
   *  they are the same size and weight as the rota chips, so "who is actually
   *  on next Tuesday" is a hard question to read off the grid. This does not
   *  filter -- the cells stay where they are, so the shape of the month does
   *  not change under the reader; they simply stop competing. */
  //  Off by default: the roster's first job is to show the month as it
  //  stands -- leave and allocations included -- and hiding most of that
  //  before the reader has asked is a view they did not choose. Fading is
  //  one click away when they want it.
  const [rotationsOnly, setRotationsOnly] = useState(false);

  const wrapRef = useRef<HTMLDivElement | null>(null);
  const touched = useRef(false);

  // Keyed on the days themselves: the page hands a fresh Date each render.
  const fromIso = rangeFrom ? toIsoDate(rangeFrom) : "";
  const toIso = rangeTo ? toIsoDate(rangeTo) : "";
  const days = useMemo(() => {
    const first = fromIso
      ? new Date(`${fromIso}T00:00:00`)
      : new Date(month.getFullYear(), month.getMonth(), 1);
    const last = toIso
      ? new Date(`${toIso}T00:00:00`)
      : new Date(month.getFullYear(), month.getMonth() + monthCount, 0);
    const count = Math.round((last.getTime() - first.getTime()) / 86_400_000) + 1;
    return Array.from({ length: count }, (_, i) => new Date(first.getFullYear(), first.getMonth(), first.getDate() + i));
  }, [month, monthCount, fromIso, toIso]);
  /** Customer engagements, engineer then day, over the days on the grid. */
  const engByPerson = useMemo(
    () => allocationsByPersonAndDay(engagementAllocations ?? [], days.map(toIsoDate)),
    [engagementAllocations, days],
  );
  /** The 1st of each month after the first: where the grid draws a month rule
   *  and names the month, so ninety columns still read as three months. */
  const opensMonth = (d: Date): boolean => d.getDate() === 1 && d.getTime() !== days[0].getTime();

  /** The zones SRE actually staffs, per kind of day.
   *
   *  Not a constant: the catalogue is what knows this. Its weekday windows
   *  cover TZ1, TZ2 and TZ3; its weekend windows are TZ1 and TZ2 only, and
   *  there is no weekend TZ3 shift to assign anyone to. Reading it from the
   *  shifts means a rota change lands here without a code change, and a
   *  column can never appear that nobody can be rostered into. */
  const zoneColumns = useMemo(() => {
    const pick = (scope: "WEEKDAY" | "WEEKEND"): string[] => {
      const codes = new Set<string>();
      for (const sh of shifts.values()) {
        if (sh.family !== family || !sh.zoneCode) continue;
        if (sh.dayScope === scope || sh.dayScope === "ANY") codes.add(zoneColumnOf(sh.zoneCode));
      }
      // SRE's time zones in order, then a rotation's Day and Night
      const rank = (c: string) => (c === "Day" ? 1 : c === "Night" ? 2 : 0);
      return [...codes].sort((a, b) => rank(a) - rank(b) || a.localeCompare(b));
    };
    return { weekday: pick("WEEKDAY"), weekend: pick("WEEKEND") };
  }, [shifts, family]);

  /** Only a zoned rota splits a day by zone (SRE's time zones, an SME
   *  rotation's Day and Night) -- CRE has no zones at all, and a day with
   *  no staffed zone at all is not split either: colSpan={0} means "span every
   *  remaining column" in HTML, not "span nothing", so an empty list would
   *  silently swallow the rest of the month. */
  const split =
    family !== "CRE" && zoneColumns.weekday.length > 0 && zoneColumns.weekend.length > 0;
  const zonesOn = (weekend: boolean): string[] =>
    weekend ? zoneColumns.weekend : zoneColumns.weekday;

  const kindByCode = useMemo(
    () => new Map(absenceKinds.map((k) => [k.code, k])),
    [absenceKinds],
  );

  /** engineer -> rota date -> what they were doing */
  const grid = useMemo(() => {
    const people = new Map<
      string,
      {
        userId: string;
        name: string;
        email: string;
        teamKey: string;
        /** Leads their team: sorted to the top of it, and tagged. */
        isLead: boolean;
        /** On this team's member list. Only a member's blank weekday reads as
         *  a working day: somebody who has left, still listed for the days
         *  they worked, is not "working" the days after. */
        isMember: boolean;
        /** Days a span moved this person to this row's team (the Brazil
         *  rotation, on the Americas row). Every other day on such a row is
         *  their own team's, and is edited as such. */
        movedDays: Set<string>;
        /** On such a row, whether the person is a member of their own team,
         *  whose ordinary day the row's other days then read as. */
        homeMember: boolean;
        /** The person's own teams, where this row is a team a span moved them
         *  to (the Americas team, for the Brazil rotation). Those teams' leads
         *  still own the span, so its cells are theirs to change too. */
        homeTeams: Set<string>;
        /** The one team this row's person belongs to, where the row is under
         *  a team a span moved them to: what its other days are, and where
         *  editing them goes. Set where the row is seated, not picked out of
         *  homeTeams, which can hold more than one key. */
        homeTeam?: string;
        /** Whole-day facts: an absence, or a window that belongs to no zone. */
        days: Map<string, Cell>;
        /** Zoned facts, keyed `${iso}|${zoneCode}` -- one per sub-column. */
        zoned: Map<string, Cell>;
        /** An allocation on a day that also holds a rotation turn. Kept beside
         *  the turn rather than over it: an engineer on RnD who is also L1
         *  for TZ1 that day is both, and the cell says both. */
        allocs: Map<string, Cell>;
        /** A zone's regular hours on a day the engineer also holds a turn in
         *  that zone, keyed like `zoned`. Kept under the turn, the same way an
         *  allocation is, so the cell says both: TZ1 regular hours and L1. */
        zonedBase: Map<string, Cell>;
        /** Turns still rostered on a day the person is on leave. Leave is
         *  the cell -- they are away -- but the turn is kept beside it, so the
         *  gap a lead has to cover shows instead of hiding under the leave. */
        clash: Map<string, Cell[]>;
      }
    >();

    // Somebody a span moves to another team (Migration, or the Americas team
    // on the Brazil rotation) has one row, under the team they are on today --
    // or on the first day shown, when today is not in view. On it now: their
    // row is that team's, their own team's days drawn on it too. Back already
    // (or not gone yet): their row is their own team's, the move's days drawn
    // on it. So moving somebody back shows them back at once.
    const firstIso = toIsoDate(days[0]);
    const nowIso = toIsoDate(new Date());
    const refIso = nowIso >= firstIso && nowIso <= toIsoDate(days[days.length - 1]) ? nowIso : firstIso;
    const lastIso = toIsoDate(days[days.length - 1]);
    const movedTo = new Map<string, { target: string; home: string; active: boolean }>();
    for (const ab of absences) {
      const kind = kindByCode.get(ab.kindCode);
      if (!kind?.movesToTeamKey || !ab.homeTeamKey) continue;
      // A move with no hours of its own (SaaS Night cover: being on the team
      // is the whole of it) seats them under that team whenever it is in
      // view, not only once it has started -- otherwise a move starting
      // tomorrow left them on their own team, the move drawn there as a row
      // of NC chips, which is the one thing it should never read as.
      const teamOnly = kind.worksRotaThere && !kind.showsAsShiftCode;
      const active = teamOnly
        ? ab.startsOn <= lastIso && (!ab.endsOn || ab.endsOn >= firstIso)
        : ab.startsOn <= refIso && (!ab.endsOn || ab.endsOn >= refIso);
      if (active || !movedTo.has(ab.engineer.userId)) {
        movedTo.set(ab.engineer.userId, { target: ab.teamKey, home: ab.homeTeamKey, active });
      }
    }

    // A row per person per team -- one row, since a moved person's own-team
    // entries are drawn on their moved row.
    const seat = (userId: string, name: string, email: string, filedUnder: string, isLead: boolean) => {
      const move = movedTo.get(userId);
      const from = filedUnder.toLowerCase();
      const redirected = Boolean(
        move && (move.active ? from === move.home.toLowerCase() : from === move.target.toLowerCase()),
      );
      const teamKey = redirected && move ? (move.active ? move.target : move.home) : filedUnder;
      const key = `${userId}|${teamKey.toLowerCase()}`;
      let row = people.get(key);
      if (!row) {
        row = {
          userId, name, email, teamKey, isLead, homeTeams: new Set(),
          // With no member list from the server, every row is taken as a
          // member -- the behaviour before the list existed.
          isMember: !teamMembers,
          movedDays: new Set(),
          homeMember: !teamMembers,
          days: new Map(), zoned: new Map(), allocs: new Map(), zonedBase: new Map(), clash: new Map(),
        };
        people.set(key, row);
      } else if (isLead && !redirected) {
        row.isLead = true;
      }
      if (move?.active && teamKey === move.target) {
        row.homeTeams.add(move.home.toLowerCase());
        row.homeTeam = move.home;
      }
      return row;
    };

    // One person, one team at a time. An entry filed under a team the person
    // is not a member of -- nor moved to or from -- goes on their own team's
    // row: leave marked on a row they no longer have, or turns left on a team
    // they have since left. With that row not in view it is not drawn here.
    // Somebody on no team at all (a leaver) keeps the team it names.
    const inView = new Set((teamKey ? [teamKey] : teams).map((k) => k.toLowerCase()));
    const ownRowFor = (userId: string, filed: string): { team: string; foreign?: string } | null => {
      const mine = ownTeams?.get(userId);
      if (!mine || mine.length === 0) return { team: filed };
      const from = filed.toLowerCase();
      const move = movedTo.get(userId);
      if (
        mine.some((k) => k.toLowerCase() === from) ||
        (move !== undefined && (move.target.toLowerCase() === from || move.home.toLowerCase() === from))
      ) {
        return { team: filed };
      }
      const own = mine.find((k) => inView.has(k.toLowerCase()));
      return own ? { team: own, foreign: filed } : null;
    };

    for (const a of assignments) {
      const shift = shifts.get(a.shiftCode);
      const placed = ownRowFor(a.engineer.userId, a.teamKey);
      if (!placed) continue;
      // Lead of the team it is filed under, not of the row it is drawn on.
      const row = seat(a.engineer.userId, a.engineer.name, a.engineer.email, placed.team, !placed.foreign && a.engineer.isLead);
      const existing = row.days.get(a.rotaDate);
      // A tier beats the plain window it sits in: "L1" says more than "TZ1".
      const code = a.tier ?? shift?.shortCode ?? a.shiftCode;
      const token = a.tier ?? shift?.colourToken ?? "";
      // The helper reads the shift's code; when the catalogue has not got the
      // shift we still hold that code on the assignment, so fall back to it
      // rather than calling a real rota turn something else.
      const isRotation = shift ? isRotationShift(shift) : !a.shiftCode.includes("REGULAR");
      const made: Cell = {
        code,
        token,
        title: placed.foreign
          ? `${shift?.label ?? a.shiftCode} · filed under ${teamNameOf(placed.foreign)}`
          : (shift?.label ?? a.shiftCode),
        filedUnder: placed.foreign,
        isRotation,
        shiftCode: a.shiftCode,
        tier: a.tier,
        onCall: Boolean(shift?.isOnCall && isRotation && !a.tier),
      };

      // A zoned window lands in its own sub-column; anything else is a fact
      // about the whole day and spans them.
      const zoneCode = a.zoneCode ?? shift?.zoneCode;
      const zone = zoneCode ? zoneColumnOf(zoneCode) : undefined;
      if (zone) {
        const key = `${a.rotaDate}|${zone}`;
        const held = row.zoned.get(key);
        if (!held) {
          row.zoned.set(key, made);
        } else if (Boolean(made.filedUnder) !== Boolean(held.filedUnder) && made.isRotation === held.isRotation) {
          // One of the two is filed under a team the person has left: neither
          // may hide the other. The stale one is the cell, so it can be found
          // and cleared; the person's own sits under it.
          const stale = made.filedUnder ? made : held;
          row.zoned.set(key, stale);
          row.zonedBase.set(key, stale === made ? held : made);
        } else if (made.isRotation && !held.isRotation) {
          // A turn arriving over the zone's regular hours: the turn is the
          // cell, the regular hours sit under it.
          row.zonedBase.set(key, held);
          row.zoned.set(key, made);
        } else if (!made.isRotation && held.isRotation) {
          row.zonedBase.set(key, made);
        } else if (a.tier) {
          row.zoned.set(key, made);
        }
        continue;
      }
      if (existing && Boolean(made.filedUnder) !== Boolean(existing.filedUnder)) {
        // A whole day holding both the person's own entry and a turn filed
        // under a team they have left: the stale turn is the cell, so it can
        // be found and cleared, and their own entry is kept beside it.
        const stale = made.filedUnder ? made : existing;
        row.days.set(a.rotaDate, stale);
        if (!row.allocs.has(a.rotaDate)) row.allocs.set(a.rotaDate, stale === made ? existing : made);
        continue;
      }
      if (!existing || a.tier) row.days.set(a.rotaDate, made);
    }

    /** Does this engineer hold a rotation turn on this day -- anything but
     *  regular hours -- in any zone or none? */
    const holdsTurn = (row: { days: Map<string, Cell>; zoned: Map<string, Cell> }, iso: string) =>
      turnsOn(row, iso).length > 0;
    /** The turns this engineer holds on this day, in any zone or none. */
    function turnsOn(row: { days: Map<string, Cell>; zoned: Map<string, Cell> }, iso: string): Cell[] {
      const whole = row.days.get(iso);
      return [
        ...(whole?.isRotation ? [whole] : []),
        ...[...row.zoned.entries()].filter(([k, c]) => k.startsWith(`${iso}|`) && c.isRotation).map(([, c]) => c),
      ];
    }

    // Leave wins: someone on leave is not on the rota that day, whatever a
    // generated row says. An allocation is different -- time given elsewhere
    // does not stop someone holding a turn the same day -- so on a day with a
    // turn it is kept beside it rather than over it.
    // Who is on a team in view. Somebody who has left is still listed for the
    // months they worked, but an "excluded from rota" span -- how the rota
    // marks a leaver from their last day on -- does not put them on a month
    // they never worked.
    const memberIds = new Set(
      (teamKey ? [teamKey] : teams).flatMap((k) => (teamMembers?.[k] ?? []).map((m) => m.userId)),
    );
    for (const ab of absences) {
      const kind = kindByCode.get(ab.kindCode);
      if (
        teamMembers &&
        kind?.bucket === "EXCLUDED" &&
        !memberIds.has(ab.engineer.userId) &&
        ![...people.values()].some((r) => r.userId === ab.engineer.userId)
      ) {
        continue;
      }
      // Leave and allocations belong to the person, not to the row they were
      // marked on (see ownRowFor). A move names its own teams and is left as
      // it is. An "excluded from rota" span is about the team it names: once
      // the person is on another team, it no longer puts them on this one.
      let filedUnder = ab.teamKey;
      let foreign = false;
      if (!kind?.movesToTeamKey) {
        const placed = ownRowFor(ab.engineer.userId, ab.teamKey);
        if (!placed || (placed.foreign && kind?.bucket === "EXCLUDED")) continue;
        filedUnder = placed.team;
        foreign = Boolean(placed.foreign);
      }
      const row = seat(ab.engineer.userId, ab.engineer.name, ab.engineer.email, filedUnder, !foreign && ab.engineer.isLead);
      if (ab.homeTeamKey) row.homeTeams.add(ab.homeTeamKey.toLowerCase());
      const end = ab.endsOn ?? toIsoDate(days[days.length - 1]);
      for (const d of days) {
        const iso = toIsoDate(d);
        // Leave is not taken on a weekend, so it is not drawn on one (the week
        // views already leave it off): a Friday-to-Monday span reads as two
        // days of leave, not four. The span is still stored whole and still
        // keeps them off the rota those days -- a weekend turn inside it stays
        // flagged as the clash it is, so nothing the lead must cover is hidden.
        const weekendDay = d.getDay() === 0 || d.getDay() === 6;
        // Days on the team this row is: only a row under the team the span
        // moved them to has moved days; on their own team's row they are the
        // move's days, drawn as the span, edited as their own team's.
        if (iso >= ab.startsOn && iso <= end && kind?.movesToTeamKey?.toLowerCase() === row.teamKey.toLowerCase()) {
          row.movedDays.add(iso);
        }
        if (iso >= ab.startsOn && iso <= end) {
          // Drawn as the team's normal hours on that team's row only; on the
          // person's own team's row (moved back since) the days say what
          // they were -- Mig, not another LK.
          const shownAs =
            kind?.showsAsShiftCode && kind.movesToTeamKey?.toLowerCase() === row.teamKey.toLowerCase()
              ? shifts.get(kind.showsAsShiftCode)
              : undefined;
          if (shownAs) {
            // On the team the span moved them to, they work its normal hours,
            // so the day reads as those -- NLK on the Brazil rotation, LK on
            // Migration -- not as the tag. A standing week: weekdays only,
            // unless the window is a weekend one. A turn that day already says
            // what they are doing. The span is still what the cell opens, so
            // leave can be marked over it and it can be ended.
            const weekendWindow = shownAs.dayScope === "WEEKEND";
            if (weekendDay !== weekendWindow || holdsTurn(row, iso)) continue;
            row.days.set(iso, {
              absenceKindCode: ab.kindCode,
              absence: {
                id: ab.id,
                kindCode: ab.kindCode,
                startsOn: ab.startsOn,
                endsOn: ab.endsOn,
                allocatedTo: ab.allocatedTo,
                homeTeamKey: ab.homeTeamKey,
              },
              code: shownAs.shortCode,
              token: shownAs.colourToken,
              title: `${shownAs.label} · ${kind?.label ?? ab.kindCode}`,
              isRotation: false,
              asHours: true,
            });
            continue;
          }
          // A move onto a team of the same rota (the SaaS Night cover) has no
          // hours of its own: on that team's row a day is just a day of theirs
          // there, and a turn that day is what it shows.
          if (
            kind?.worksRotaThere &&
            !kind.showsAsShiftCode &&
            kind.movesToTeamKey?.toLowerCase() === row.teamKey.toLowerCase()
          ) {
            if (holdsTurn(row, iso)) continue;
            row.days.set(iso, {
              absenceKindCode: ab.kindCode,
              absence: {
                id: ab.id,
                kindCode: ab.kindCode,
                startsOn: ab.startsOn,
                endsOn: ab.endsOn,
                allocatedTo: ab.allocatedTo,
                homeTeamKey: ab.homeTeamKey,
              },
              code: "",
              token: "",
              title: kind.label,
              isRotation: false,
              quiet: true,
            });
            continue;
          }
          if (kind?.bucket === "LEAVE" && weekendDay && !holdsTurn(row, iso)) continue;
          // An allocation (Allo-INT, Allo-EXT, ...) is working-day time given
          // to someone else, so it is not drawn on a weekend either; a weekend
          // turn that day shows on its own. Moves to another team are not
          // allocations here and keep their own rule above.
          if (kind?.bucket === "ALLOCATION" && !kind.movesToTeamKey && weekendDay) continue;
          const target =
            kind?.bucket === "ALLOCATION" && holdsTurn(row, iso) ? row.allocs : row.days;
          if (kind?.bucket === "LEAVE" && iso >= ab.startsOn && !row.clash.has(iso)) {
            const turns = turnsOn(row, iso);
            if (turns.length > 0) row.clash.set(iso, turns);
          }
          target.set(iso, {
            absenceKindCode: ab.kindCode,
            absence: {
              id: ab.id,
              kindCode: ab.kindCode,
              startsOn: ab.startsOn,
              endsOn: ab.endsOn,
              allocatedTo: ab.allocatedTo,
              homeTeamKey: ab.homeTeamKey,
            },
            // An allocation names who it is for, where it knows: a lead
            // scanning the month wants "TFL", not six identical "CUS-OFF"s.
            // Clipped to what a day column holds; the title has it in full.
            code: ab.allocatedTo ? ab.allocatedTo.slice(0, 6) : (kind?.shortCode ?? ab.kindCode),
            token: kind?.colourToken ?? "",
            title: [kind?.label ?? ab.kindCode, ab.allocatedTo].filter(Boolean).join(" · "),
            isRotation: false,
          });
        }
      }
    }

    // Every member of each team in view, whether or not they hold an entry
    // this month: a lead is rarely on the rota, and somebody whose only entry
    // was an allocation would otherwise vanish the moment it was cleared --
    // taking with them the row their leave is marked on. Seated last, and
    // only where the person has no row yet: someone moved to the Americas
    // team this month already has their one row there.
    const seated = new Set([...people.values()].map((r) => r.userId));
    for (const key of teamKey ? [teamKey] : teams) {
      for (const m of teamMembers?.[key] ?? []) {
        const own = people.get(`${m.userId}|${key.toLowerCase()}`);
        if (own) {
          own.isLead ||= m.isLead;
          own.isMember = true;
          continue;
        }
        if (seated.has(m.userId)) {
          for (const r of people.values()) {
            if (r.userId === m.userId && r.homeTeams.has(key.toLowerCase())) r.homeMember = true;
          }
          continue;
        }
        seat(m.userId, m.name, m.email, key, m.isLead).isMember = true;
      }
    }

    // Rota order, not alphabetical: the ABTs first, in the order the rota
    // itself runs them, and the teams that hold no ABT rotation -- Americas,
    // Migration -- after. Sorting by name put Americas above Atlas, which is
    // backwards for a reader scanning for their own ABT.
    const rank = new Map(teams.map((t, i) => [t, i]));
    const orderOf = (k: string) => rank.get(k) ?? Number.MAX_SAFE_INTEGER;

    return [...people.values()]
      .sort(
        (a, b) =>
          orderOf(a.teamKey) - orderOf(b.teamKey) ||
          a.teamKey.localeCompare(b.teamKey) ||
          // Each team opens with its lead, the person a reader looks for first.
          Number(b.isLead) - Number(a.isLead) ||
          a.name.localeCompare(b.name),
      );
  }, [absences, assignments, days, kindByCode, shifts, teams, teamKey, teamMembers, ownTeams, teamNameOf]);

  const q = query.trim().toLowerCase();
  // The SaaS Night cover is a team like any other -- people a lead moved
  // onto it sit under it for the move's months -- so it needs no group of its
  // own here.
  const rows = q ? grid.filter((r) => r.name.toLowerCase().includes(q) || r.teamKey.toLowerCase().includes(q)) : grid;

  const todayIso = toIsoDate(new Date());

  /** The week the reader is actually in, Monday to Sunday.
   *
   *  Today's column answers "where am I now"; the week answers "what am I in
   *  the middle of", which is the question a rota is usually opened with --
   *  and across three months of columns today alone is a single 44px stripe
   *  that is easy to scroll straight past.
   *
   *  Marked as a band with its two edges ruled rather than as a fill, so it
   *  composes with the marks already on the grid instead of competing with
   *  them: today stays the circled date inside it, and the reader's own row
   *  stays the filled row crossing it. */
  const weekFrom = toIsoDate(mondayOf(new Date()));
  // addDays rather than six times 86,400,000ms: a week can contain a DST
  // change that makes one local day 25 hours long, and the arithmetic then
  // lands on Saturday 23:00. The band would drop the Sunday and put its
  // closing edge on the Saturday, once a year, in one timezone.
  const weekTo = toIsoDate(addDays(mondayOf(new Date()), 6));
  const inThisWeek = (iso: string): boolean => iso >= weekFrom && iso <= weekTo;

  /** The same marks for one zone sub-column of a split day: only the first
   *  carries an opening edge, only the last a closing one. */
  const zoneMarks = (marks: string, i: number, total: number): string => {
    let out = marks;
    if (i !== 0) out = out.replace(" mstart", "").replace(" cwa", "");
    if (i !== total - 1) out = out.replace(" cwz", "");
    return out;
  };
  /** The band's own edges, so it reads as one block seven columns wide. */
  const weekMarks = (iso: string): string =>
    inThisWeek(iso)
      ? ` cw${iso === weekFrom ? " cwa" : ""}${iso === weekTo ? " cwz" : ""}`
      : "";
  const me = meEmail?.trim().toLowerCase() ?? "";

  /** Which rows this reader may change. A lead edits their own ABT only, so
   *  most rows in a 122-engineer grid are not theirs to touch -- and an edit
   *  control on every one of them would say otherwise. */
  /** What an unmarked weekday on a row reads as: its team's ordinary day --
   *  Americas cover on the Americas team, Regular hours elsewhere -- and only
   *  for somebody on that team, so a leaver's month stops at their last day.
   *  On a row a span moved someone to, the days outside the span are their
   *  own team's ordinary day. The lead of a team on the India region shift
   *  keeps Regular hours (ordinaryDayCode). */
  const defaultFor = useMemo(() => {
    const cellFor = (team: string, isLead: boolean): Cell | undefined => {
      if (teamsWithoutDefault?.has(team) || teamsWithoutDefault?.has(team.toLowerCase())) return undefined;
      const own = teamDefaultShift?.[team] ?? teamDefaultShift?.[team.toLowerCase()];
      const code = own ? ordinaryDayCode(own, isLead) : undefined;
      const sh = code && code !== REGULAR_HOURS_CODE ? shifts.get(code) : undefined;
      return sh
        ? { code: sh.shortCode, token: sh.colourToken, title: `Working day (${sh.shortCode})`, isRotation: false, isDefault: true }
        : WORKING_DAY;
    };
    return (
      row: {
        teamKey: string;
        isMember: boolean;
        isLead: boolean;
        movedDays: ReadonlySet<string>;
        homeTeam?: string;
        homeMember: boolean;
      },
      iso: string,
    ): Cell | undefined => {
      if (row.movedDays.size > 0) {
        if (row.movedDays.has(iso)) return cellFor(row.teamKey, row.isLead);
        return row.homeTeam && row.homeMember ? cellFor(row.homeTeam, row.isLead) : undefined;
      }
      return row.isMember ? cellFor(row.teamKey, row.isLead) : undefined;
    };
  }, [teamDefaultShift, teamsWithoutDefault, shifts]);

  /** Whether this reader may change this cell -- the same rule the server
   *  applies, so a cell never looks editable and then refuses the save:
   *   - a day on somebody's own team: its lead (or a rota admin), and only for
   *     somebody on that team -- a leaver's days are nobody's to change;
   *   - a day a span moved them to another team (the Brazil rotation): that
   *     team's lead, or their own team's lead, who still owns the span;
   *   - any other day on such a row: their own team's lead, for their own
   *     team's day. */
  const canEdit = useMemo(() => {
    const own = new Set((leadTeams ?? []).map((t) => t.toLowerCase()));
    return (
      row: { teamKey: string; isMember: boolean; homeTeams: ReadonlySet<string>; movedDays: ReadonlySet<string>; homeMember: boolean },
      iso: string,
    ): boolean => {
      if (!editing || !onEditCell) return false;
      const homeLed = [...row.homeTeams].some((h) => own.has(h));
      if (row.movedDays.size > 0) {
        return row.movedDays.has(iso) ? own.has(row.teamKey.toLowerCase()) || homeLed : homeLed && row.homeMember;
      }
      return own.has(row.teamKey.toLowerCase()) && row.isMember;
    };
  }, [leadTeams, onEditCell, editing]);

  /** The legend: one line per code on screen, in the order a reader meets
   *  them -- turns, hours, leave, allocations -- plus every leave, allocation
   *  and move tag a rota marker can add on this rota, on screen or not. The
   *  labels only: the grid's own marks (a struck-through turn, the dot of an
   *  empty day) say what they are in their tooltips. */
  const legendItems = useMemo((): LegendItem[] => {
    const out = new Map<string, LegendItem>();
    const put = (it: LegendItem): void => {
      if (!out.has(it.key)) out.set(it.key, it);
    };
    const fromCell = (c: Cell): void => {
      if (c.quiet) return;
      const kind = c.absenceKindCode ? kindByCode.get(c.absenceKindCode) : undefined;
      // A move drawn as the other team's hours reads as those hours.
      if (kind && !c.asHours) {
        // A retired kind (Sick leave, the customer allocations) is not
        // explained: the legend lists what can be marked, and the old days
        // still carrying one keep its name in their own tooltip.
        if (kind.retired) return;
        put({
          key: `k|${kind.code}`,
          group: kind.bucket === "LEAVE" ? "Leave" : "Allocations",
          code: kind.shortCode,
          token: kind.colourToken,
          label: kind.label,
        });
        return;
      }
      if (c.tier) {
        put({ key: `t|${c.tier}`, group: "Rota", code: c.tier, token: c.token, label: `${c.tier} escalation tier` });
        return;
      }
      put({
        key: `s|${c.code}|${c.token}|${c.onCall ? "oc" : ""}`,
        group: c.isRotation ? "Rota" : "Hours",
        code: c.code,
        token: c.token,
        label: c.isDefault ? `${c.title.replace(/^Working day.*/, "Working day")} (nothing else marked)` : c.title,
        onCall: c.onCall,
      });
    };
    for (const row of rows) {
      for (const m of [row.days, row.zoned, row.allocs, row.zonedBase]) for (const c of m.values()) fromCell(c);
    }
    // What an unmarked weekday reads as on each team in view (LK, or NLK on
    // the Americas team), so the most common chip on the grid is explained.
    for (const row of rows) {
      for (const d of days) {
        if (d.getDay() === 0 || d.getDay() === 6) continue;
        const iso = toIsoDate(d);
        if (row.days.has(iso)) continue;
        const dflt = defaultFor(row, iso);
        if (dflt) {
          fromCell(dflt);
          break;
        }
      }
    }
    // Every tag a rota marker can add here is listed whether or not anybody
    // carries it in view: a lead looking for how to mark something finds it
    // here. The same tags as the picker (leave, allocations, moves to another
    // team), with the ones on screen, in catalogue order.
    const tags = new Map<string, ScheduleAbsenceKind>();
    for (const key of [...out.keys()]) {
      if (!key.startsWith("k|")) continue;
      const kind = kindByCode.get(key.slice(2));
      if (kind) tags.set(kind.code, kind);
      out.delete(key);
    }
    for (const kind of [...kindsOfferedOn(absenceKinds, family), ...movesOfferedOn(absenceKinds, family)]) {
      tags.set(kind.code, kind);
    }
    for (const kind of [...tags.values()].sort((a, b) => a.sortOrder - b.sortOrder)) {
      put({
        key: `k|${kind.code}`,
        group: kind.bucket === "LEAVE" ? "Leave" : "Allocations",
        code: kind.shortCode,
        token: kind.colourToken,
        label: kind.label,
      });
    }
    // A move worked as that team's hours (the Brazil rotation, picked as
    // "Move to Americas Team") is drawn as those hours, so it reads as them.
    for (const kind of absenceKinds) {
      if (!kind.movesToTeamKey || !kind.worksRotaThere || !kind.showsAsShiftCode || kind.retired) continue;
      if (kind.family && kind.family !== family) continue;
      // Only this rota's windows: the picker offers no other family's hours.
      const sh = shifts.get(kind.showsAsShiftCode);
      if (sh && sh.family === family) put({ key: `s|${sh.shortCode}|${sh.colourToken}|`, group: "Hours", code: sh.shortCode, token: sh.colourToken, label: sh.label });
    }
    // Only when a marker is on a row in view: the legend explains what is
    // drawn, and the marker is not a code a lead can mark.
    if (rows.some((r) => (engByPerson.get(r.userId)?.size ?? 0) > 0)) put(ENGAGEMENT_MARK_LEGEND_ITEM);
    return [...out.values()];
  }, [rows, days, kindByCode, defaultFor, absenceKinds, family, shifts, engByPerson]);

  /** Turn a click on any roster cell into the slot the page should open the
   *  picker on. Shared by the plain grid and by SRE's zone-split one, which
   *  has three ways into the same edit -- a zone column, and a day-wide cell
   *  when leave covers the whole day -- and had none of them before. */
  const openCell = (
    e: { currentTarget: HTMLElement },
    row: { userId: string; name: string; teamKey: string; homeTeam?: string; movedDays?: ReadonlySet<string> },
    iso: string,
    cell: Cell | undefined,
    zoneCode?: string,
  ): void => {
    const r = e.currentTarget.getBoundingClientRect();
    onEditCell?.({
      userId: row.userId,
      name: row.name,
      // On a row a span moved somebody to, a day the span does not cover is
      // still their own team's: edit it there, where they are a member.
      teamKey:
        cell?.filedUnder ??
        (row.homeTeam && row.movedDays && !row.movedDays.has(iso)
          ? row.homeTeam
          : row.teamKey),
      rotaDate: iso,
      shiftCode: cell?.shiftCode,
      tier: cell?.tier,
      absenceKindCode: cell?.absenceKindCode,
      absence: cell?.absence,
      zoneCode,
      anchor: { top: r.top, left: r.left, bottom: r.bottom, right: r.right },
    });
  };

  /** What to add to a cell's tooltip when somebody has changed it, and
   *  whether to mark it at all. Empty for the overwhelming majority of cells,
   *  which nobody has touched since the rota was generated. */
  const changedBy = (userId: string, iso: string): { mark: boolean; note: string } => {
    const hit = editedCells?.get(`${userId}|${iso}`);
    if (!hit) return { mark: false, note: "" };
    const when = new Date(hit.changedAt).toLocaleDateString(undefined, {
      day: "numeric",
      month: "short",
    });
    return { mark: true, note: ` — changed by ${hit.actor} on ${when}` };
  };

  /** A chip as the grid draws it: dashed when the window is on-call, and
   *  marked when it is only what an unmarked weekday means. */
  const chipOf = (c: Cell, extra = ""): JSX.Element =>
    c.quiet ? (
      <span className="none">·</span>
    ) : (
      <span className={`chip sm ${c.token}${c.isDefault ? " dflt" : ""}${c.onCall ? " oc" : ""}${extra}`}>{c.code}</span>
    );
  /** The strip under a day on which the engineer is on a customer
   *  engagement, or nothing. Absolutely placed in the cell (see the css), so
   *  it moves nothing. A day split by zone draws it in every sub-column, so
   *  the strip runs the width of the day, but only the first is named: the
   *  rest are hidden from assistive technology, so the day is announced once.
   *  The title is on all of them so the tooltip shows wherever the pointer
   *  is along the strip. */
  const engMark = (userId: string, iso: string, named = true): JSX.Element | null => {
    const list = engByPerson.get(userId)?.get(iso);
    if (!list || list.length === 0) return null;
    return named ? (
      <span className="engmk" role="img" aria-label={dayMarkerAriaLabel(list)} title={dayMarkerTitle(list)} />
    ) : (
      <span className="engmk" aria-hidden="true" title={dayMarkerTitle(list)} />
    );
  };
  /** Said on a day that is leave but still holds a turn. */
  const clashNote = (turns: Cell[] | undefined): string =>
    turns && turns.length > 0
      ? ` — still rostered for ${turns.map((t) => (t.tier ? `${t.tier} · ${t.title}` : t.title)).join(" and ")}: needs cover`
      : "";
  /** Leave, and the turn it leaves uncovered struck through under it. */
  const clashPair = (leave: Cell, turns: Cell[]): JSX.Element => (
    <span className="duo clash">
      {chipOf(leave)}
      {chipOf(turns[0], " gone")}
    </span>
  );
  /** Why a row cannot be changed, for a lead in edit mode: the row is there
   *  to read, and saying whose it is beats a click that does nothing. */
  const notYours = (row: { teamKey: string }): string =>
    editing && onEditCell
      ? ` — only ${teamNameOf(row.teamKey)}'s lead or a ${family} rota admin can change this`
      : "";

  /** Arrow keys between the cells a lead may change, Enter or Space to open
   *  one. One cell is in the Tab order at a time (the last one used), so the
   *  grid is a single stop rather than a thousand. */
  const [activeCell, setActiveCell] = useState<string | null>(null);
  const onGridKey = (e: ReactKeyboardEvent<HTMLTableElement>): void => {
    const td = (e.target as HTMLElement).closest<HTMLTableCellElement>("td.editable");
    if (!td) return;
    if (e.key === "Enter" || e.key === " ") {
      e.preventDefault();
      td.click();
      return;
    }
    const tr = td.parentElement as HTMLTableRowElement;
    const inRow = (r: Element | null): HTMLElement[] =>
      r ? [...r.querySelectorAll<HTMLElement>("td.editable")] : [];
    const centre = (el: HTMLElement): number => {
      const b = el.getBoundingClientRect();
      return b.left + b.width / 2;
    };
    /** The editable cell in another row that sits under this one. */
    const across = (r: Element | null): HTMLElement | undefined => {
      const x = centre(td);
      const cells = inRow(r);
      return (
        cells.find((c) => {
          const b = c.getBoundingClientRect();
          return x >= b.left && x <= b.right;
        }) ?? cells.sort((a, b) => Math.abs(centre(a) - x) - Math.abs(centre(b) - x))[0]
      );
    };
    /** The next row in a direction that has anything this lead may change. */
    const rowFrom = (r: Element | null, up: boolean): Element | null => {
      let at = r ? (up ? r.previousElementSibling : r.nextElementSibling) : null;
      while (at && inRow(at).length === 0) at = up ? at.previousElementSibling : at.nextElementSibling;
      return at;
    };
    const mine = inRow(tr);
    const i = mine.indexOf(td);
    const next: HTMLElement | undefined =
      e.key === "ArrowRight" ? mine[i + 1]
      : e.key === "ArrowLeft" ? mine[i - 1]
      : e.key === "Home" ? mine[0]
      : e.key === "End" ? mine[mine.length - 1]
      : e.key === "ArrowDown" ? across(rowFrom(tr, false))
      : e.key === "ArrowUp" ? across(rowFrom(tr, true))
      : undefined;
    if (!["ArrowRight", "ArrowLeft", "ArrowUp", "ArrowDown", "Home", "End"].includes(e.key)) return;
    e.preventDefault();
    if (!next) return;
    next.focus();
    next.scrollIntoView?.({ block: "nearest", inline: "nearest" });
  };
  /** The props that put an editable cell in the keyboard path. */
  const keyable = (key: string, firstOfGrid: boolean) => ({
    tabIndex: activeCell === key || (activeCell === null && firstOfGrid) ? 0 : -1,
    onFocus: () => setActiveCell(key),
    "aria-haspopup": "dialog" as const,
  });

  const meRow = useRef<HTMLTableRowElement | null>(null);
  const scrolled = useRef(false);
  useEffect(() => {
    // Once, on open. Doing it on every render would yank the grid back every
    // time the reader scrolled away to look at someone else.
    if (scrolled.current || !meRow.current) return;
    scrolled.current = true;
    meRow.current.scrollIntoView({ block: "center", behavior: "auto" });
  });
  // A new search, group or month is a new question, so the next match earns
  // being scrolled to again.
  useEffect(() => {
    scrolled.current = false;
  }, [query, teamKey, family, month]);

  /** The month as a stable key: the Date itself is a fresh object each render. */
  const monthIso = toIsoDate(days[0]);

  // Open on today, the way the day view opens on the current hour. A month is
  // thirty-odd columns and only a third of them fit, so landing on the 1st
  // means every reader starts by scrolling to where they already were.
  //
  // Today is what they asked for, but it is not always on screen to give: the
  // roster follows the date picker, so in any other month the picked day is
  // the thing they just chose and the honest place to open.
  useEffect(() => {
    const el = wrapRef.current;
    if (!el) return;
    // A new month is a new question, so it re-centres even if they scrolled
    // the last one. Within a month their scroll position is theirs.
    touched.current = false;
    const go = (): void => {
      if (touched.current) return;
      const target =
        el.querySelector<HTMLElement>("thead th.day.today") ??
        el.querySelector<HTMLElement>("thead th.day.sel");
      if (!target) return;
      // The engineer column is sticky, so it covers the left of the scroller;
      // anything parked under it is parked out of sight.
      const nameWidth = el.querySelector<HTMLElement>("thead th.lab")?.offsetWidth ?? 0;
      const inset = Math.round((el.clientWidth - nameWidth) * 0.25);
      const delta = target.getBoundingClientRect().left - el.getBoundingClientRect().left;
      el.scrollLeft = Math.max(0, el.scrollLeft + delta - nameWidth - inset);
    };
    go();
    // Once more after the grid has settled: on the first paint the columns
    // have not been laid out yet and every offset reads zero.
    const t = window.setTimeout(go, 120);
    return () => window.clearTimeout(t);
  }, [monthIso, selectedIso, focusRequest]);

  /** A team's rows, in one table body: the night cover sits between the teams
   *  that share it and the rest, so the rows are drawn by part. */
  /** The first editable cell on the grid takes the Tab stop until a lead
   *  has used one -- counted across both groups of team rows. */
  let tabStopGiven = false;
  const firstStop = (): boolean => {
    if (tabStopGiven) return false;
    tabStopGiven = true;
    return true;
  };
  const teamRowsOf = (list: typeof rows): JSX.Element[] =>
    list.map((row, i) => {
      // The first row of each team earns a rule above it: sorted by team
      // with nothing between them, a hundred and twenty rows read as one
      // undifferentiated block.
      const opensTeam = i > 0 && list[i - 1].teamKey !== row.teamKey;
      // Case-insensitive and trimmed: an identity provider is free to
      // hand back Jane.Doe@Example.com for the address seeded as
      // jane.doe@example.com, and an exact compare would silently
      // never match.
      const isMe = Boolean(me) && row.email.trim().toLowerCase() === me;
      // In edit mode, a row this lead cannot change reads as such.
      const readOnly = editing && Boolean(onEditCell) && !days.some((d) => canEdit(row, toIsoDate(d)));
      return (
      <tr
        key={`${row.userId}|${row.teamKey}`}
        ref={isMe ? meRow : undefined}
        // Every other row banded, so one engineer's month reads across
        // the grid without a hover to follow it.
        // Tinted in the team's own colour, faintly, so where one team
        // ends and the next begins reads at a glance. --rc carries it.
        className={`tinted${isMe ? " me" : ""}${opensTeam ? " teamtop" : ""}${i % 2 === 1 ? " alt" : ""}${readOnly ? " ro" : ""}`}
        style={{ "--rc": teamColourOf(row.teamKey) } as CSSProperties}
        aria-current={isMe ? "true" : undefined}
      >
        <th className="lab">
          <span className="nm">
            <span className="av" style={{ background: teamColourOf(row.teamKey) }}>
              {initialsOf(row.name)}
            </span>
            <span className="who">{row.name}</span>
            {isMe ? <i className="youtag">You</i> : null}
            {row.isLead ? <i className="leadtag">Lead</i> : null}
            <span className="team">{teamNameOf(row.teamKey)}</span>
          </span>
        </th>
        {days.map((d) => {
          const iso = toIsoDate(d);
          const cell = row.days.get(iso);
          const weekend = d.getDay() === 0 || d.getDay() === 6;
          const marks = `${weekend ? "wknd" : ""} ${iso === todayIso ? "today" : ""} ${
            iso === selectedIso ? "sel" : ""
          } ${d.getDay() === 1 ? "wkstart" : ""}${opensMonth(d) ? " mstart" : ""}${weekMarks(iso)}${
            changedCells?.has(`${row.userId}|${iso}`) ? " changed" : ""
          }`;
          const faded = (c: Cell | undefined) =>
            rotationsOnly && c && !c.isRotation ? "muted" : "";

          const alloc = row.allocs.get(iso);
          /** A turn and the allocation beside it, as one thing to
           *  open: the picker shows the turn as held and offers to
           *  remove the allocation. */
          const withAlloc = (c: Cell | undefined): Cell | undefined =>
            alloc
              ? {
                  ...(c ?? alloc),
                  absenceKindCode: alloc.absenceKindCode,
                  absence: alloc.absence,
                  title: c ? `${c.title} · also ${alloc.title}` : alloc.title,
                }
              : c;

          if (!split) {
            const editable = canEdit(row, iso);
            const touched = changedBy(row.userId, iso);
            // An unmarked weekday is a working day; a weekend is not.
            const shown = cell ?? alloc ?? (weekend ? undefined : defaultFor(row, iso));
            const clash = cell?.absenceKindCode ? row.clash.get(iso) : undefined;
            const key = `${row.userId}|${row.teamKey}|${iso}`;
            return (
              <td
                key={iso}
                className={`${marks} ${faded(shown)}${editable ? " c editable" : ""}${
                  touched.mark ? " touched" : ""
                }${clash ? " clash" : ""}`}
                title={
                  editable
                    ? `${row.name} · ${shown ? (withAlloc(cell)?.title ?? shown.title) : "nothing rostered"}${clashNote(clash)}${touched.note} — click to change`
                    : shown || touched.mark || clash
                      ? `${row.name} · ${shown ? shown.title : "nothing rostered"}${clashNote(clash)}${touched.note}${notYours(row)}`
                      : notYours(row)
                        ? `${row.name}${notYours(row)}`
                        : undefined
                }
                onClick={editable ? (e) => openCell(e, row, iso, withAlloc(cell)) : undefined}
                {...(editable ? keyable(key, firstStop()) : {})}
              >
                {clash && cell ? (
                  clashPair(cell, clash)
                ) : cell && alloc ? (
                  <span className="duo">
                    {chipOf(cell)}
                    {chipOf(alloc)}
                  </span>
                ) : shown ? (
                  chipOf(shown)
                ) : (
                  <span className="none">·</span>
                )}
                {engMark(row.userId, iso)}
              </td>
            );
          }

          const zones = zonesOn(weekend);

          // Leave belongs to the day, not to a zone: somebody away is
          // away from all of them, so it spans rather than picking one
          // arbitrarily. So does an unmarked weekday -- a working day,
          // in no zone yet -- which reads as one LK, not three blanks.
          const unmarked =
            !cell && !alloc && !weekend && zones.every((z) => !row.zoned.has(`${iso}|${z}`));
          const whole = cell ?? (unmarked ? defaultFor(row, iso) : undefined);
          if (whole) {
            const editable = canEdit(row, iso);
            const clash = cell?.absenceKindCode ? row.clash.get(iso) : undefined;
            return (
              <td
                key={iso}
                colSpan={zones.length}
                className={`c zwhole ${marks} ${faded(whole)}${
                  editable ? " editable" : ""
                }${clash ? " clash" : ""}`}
                title={
                  editable
                    ? `${row.name} · ${whole.title}${clashNote(clash)} — click to change`
                    : `${row.name} · ${whole.title}${clashNote(clash)}${notYours(row)}`
                }
                onClick={editable ? (e) => openCell(e, row, iso, cell) : undefined}
                {...(editable ? keyable(`${row.userId}|${row.teamKey}|${iso}`, firstStop()) : {})}
              >
                {clash ? clashPair(whole, clash) : chipOf(whole)}
                {engMark(row.userId, iso)}
              </td>
            );
          }

          const editable = canEdit(row, iso);
          return zones.map((z, i) => {
            // A zone the engineer holds a turn in shows the turn; the
            // rest of the day shows the allocation that fills it.
            const turn = row.zoned.get(`${iso}|${z}`);
            const zc = turn ?? alloc;
            // Under a turn: the allocation the rest of the day is
            // given to, else the zone's regular hours.
            const under = turn && turn.isRotation ? (alloc ?? row.zonedBase.get(`${iso}|${z}`)) : alloc;
            return (
              <td
                key={`${iso}|${z}`}
                // A day split across zone columns still has one left
                // edge and one right edge. The month rule and the
                // week band's opening edge belong to the first
                // sub-column and its closing edge to the last;
                // repeating them on each zone would rule the inside
                // of the day as heavily as its boundary.
                className={`c z ${i === 0 ? "zfirst" : ""}${i === zones.length - 1 ? " zlast" : ""} ${
                  zoneMarks(marks, i, zones.length)
                } ${faded(zc)}${editable ? " editable" : ""}`}
                title={
                  editable
                    ? `${row.name} · ${z} · ${zc ? (withAlloc(row.zoned.get(`${iso}|${z}`))?.title ?? zc.title) : "nothing rostered"} — click to change`
                    : zc
                      ? `${row.name} · ${z} · ${zc.title}${notYours(row)}`
                      : notYours(row)
                        ? `${row.name}${notYours(row)}`
                        : undefined
                }
                onClick={editable ? (e) => openCell(e, row, iso, withAlloc(row.zoned.get(`${iso}|${z}`)), zoneCodeFor?.(row.teamKey, z) ?? z) : undefined}
                {...(editable ? keyable(`${row.userId}|${row.teamKey}|${iso}|${z}`, firstStop()) : {})}
              >
                {/* A night held by somebody not on the TZ3 crew stays
                    here, on their own row: the Night cover group lists
                    the crew alone, so nobody is shown twice. */}
                {turn && under ? (
                  // The zone's turn and what it sits on, stacked: L1
                  // in TZ1 on an RnD day, or on TZ1's regular hours --
                  // the cell says both.
                  <span className="duo">
                    {chipOf(turn)}
                    {chipOf(under)}
                  </span>
                ) : zc ? (
                  chipOf(zc)
                ) : (
                  <span className="zempty" />
                )}
                {engMark(row.userId, iso, i === 0)}
              </td>
            );
          });
        })}
      </tr>
      );
    });

  return (
    <>
      <div className="card-head">
        {/* One group means nothing to switch to: only Today, or a manager,
            can look at the other group. */}
        {families.length > 1 ? (
          <div className="seg teamseg" role="tablist" aria-label={`Show ${families.join(" or ")}`}>
            {families.map((f) => (
              <button
                key={f}
                role="tab"
                aria-selected={family === f}
                className={family === f ? "on" : ""}
                onClick={() => onFamilyChange(f)}
              >
                {f}
              </button>
            ))}
          </div>
        ) : null}

        <RotaPicker rotas={rotas} rotaCode={rotaCode} onRotaChange={onRotaChange} />

        <h2>
          Roster <span className="count">{rows.length}</span>
        </h2>

        <div className="tools">
          <label className="rq">
            <svg
              width="14"
              height="14"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              strokeLinecap="round"
              aria-hidden="true"
            >
              <circle cx="11" cy="11" r="7" />
              <path d="M20 20l-3.5-3.5" />
            </svg>
            <input
              type="search"
              placeholder="Find an engineer"
              autoComplete="off"
              aria-label="Find an engineer"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
            />
            {query ? (
              <button type="button" className="rqx" aria-label="Clear" onClick={() => setQuery("")}>
                &times;
              </button>
            ) : null}
          </label>

          {onSpanChange ? (
            // One month to work through, three to check a swap against the
            // months around it, six to plan ahead. The same for CRE and SRE.
            <div className="seg spanseg" role="group" aria-label="Months shown">
              {SPANS.map((n) => (
                <button
                  key={n}
                  type="button"
                  className={span === n ? "on" : ""}
                  aria-pressed={span === n}
                  onClick={() => onSpanChange(n)}
                >
                  {n === 1 ? "1 month" : `${n} months`}
                </button>
              ))}
            </div>
          ) : null}

          <select
            className="teampick"
            aria-label="Show one team"
            value={teamKey}
            onChange={(e) => onTeamKeyChange(e.target.value)}
          >
            <option value="">All teams</option>
            {teams.map((t) => (
              <option key={t} value={t}>
                {teamNameOf(t)}
              </option>
            ))}
          </select>

          <label className="rotonly" title="Fade leave and allocations">
            <input
              type="checkbox"
              checked={rotationsOnly}
              onChange={(e) => setRotationsOnly(e.target.checked)}
            />
            Rotations only
          </label>
          {actions}
        </div>
      </div>

      <RosterLegend items={legendItems} />
      {engagementsFailed ? (
        <div className="engnote" role="status">
          {ENGAGEMENTS_FAILED_NOTE}
        </div>
      ) : null}

      {/* The card's one scroller. Both axes live here, which is what lets the
          date row and the engineer column stay pinned to the grid they label
          instead of to the page. SRE's is the exception (rostflow): its list
          is not scrolled inside the card, only across it, so the page alone
          scrolls down to the last engineer. */}
      <div
        className={`twwrap rostwrap${family === "SRE" ? " rostflow" : ""}`}
        ref={wrapRef}
        onScroll={() => (touched.current = true)}
      >
        <table className={`tw roster${split ? " split" : ""}`} onKeyDown={editing ? onGridKey : undefined}>
          <thead>
            <tr>
              {/* Spans the zone row too on a split day, so the heading sits in
                  the middle of the header rather than on top of an empty cell. */}
              <th className="lab eng" rowSpan={split ? 2 : undefined}>
                Engineer
              </th>
              {days.map((d) => {
                const iso = toIsoDate(d);
                const weekend = d.getDay() === 0 || d.getDay() === 6;
                return (
                  <th
                    key={iso}
                    colSpan={split ? zonesOn(weekend).length : undefined}
                    className={`day ${weekend ? "wknd" : ""} ${iso === todayIso ? "today" : ""} ${
                      iso === selectedIso ? "sel" : ""
                    } ${d.getDay() === 1 ? "wkstart" : ""}${opensMonth(d) ? " mstart" : ""}${weekMarks(iso)}`}
                    aria-current={iso === selectedIso ? "date" : undefined}
                    title={d.toLocaleDateString(undefined, { weekday: "short", day: "numeric", month: "short" })}
                  >
                    <span className="d">{d.getDate()}</span>
                    {/* The month is named where it starts, and on the first
                        column too: a window opening on the 14th still has to
                        say which month the 14th is in. */}
                    {d.getDate() === 1 || d.getTime() === days[0].getTime() ? (
                      <span className="mo">{d.toLocaleDateString(undefined, { month: "short" })}</span>
                    ) : null}
                    <span className="w">{d.toLocaleDateString(undefined, { weekday: "short" })}</span>
                  </th>
                );
              })}
            </tr>

            {/* The zone row. A weekend has two columns rather than three
                because TZ1 and TZ2 are one crew at the weekend ("TZ1+2") and
                TZ3 is as it is -- the catalogue says so, and this follows it. */}
            {split ? (
              <tr className="zrow">
                {days.map((d) => {
                  const iso = toIsoDate(d);
                  const weekend = d.getDay() === 0 || d.getDay() === 6;
                  return zonesOn(weekend).map((z, i) => (
                    <th
                      key={`${iso}|${z}`}
                      className={`zc ${i === 0 ? "zfirst" : ""} ${weekend ? "wknd" : ""}${
                        i === 0 && opensMonth(d) ? " mstart" : ""
                      }${toIsoDate(d) === todayIso ? " today" : ""}${toIsoDate(d) === selectedIso ? " sel" : ""}`}
                      scope="col"
                      title={zoneLabelOn(shifts, z, weekend) === zoneLabelOn(shifts, z, false) ? undefined : `${zoneLabelOn(shifts, z, weekend)}: TZ1 and TZ2 are one crew at the weekend`}
                    >
                      {/* "TZ1+2" at the weekend, when TZ1 and TZ2 are one crew. */}
                      {zoneLabelOn(shifts, z, weekend)}
                    </th>
                  ));
                })}
              </tr>
            ) : null}
          </thead>
          <tbody>{teamRowsOf(rows)}</tbody>
        </table>
      </div>

      {rows.length === 0 ? (
        <div className="offnone">
          {q ? (
            <>No engineer matches “{query}”.</>
          ) : monthCount > 1 ? (
            "Nobody is on the rota in these months."
          ) : (
            "Nobody is on the rota this month."
          )}
        </div>
      ) : null}
    </>
  );
}
