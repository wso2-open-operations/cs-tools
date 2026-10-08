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

import { useEffect, useMemo, useRef, useState, type CSSProperties, type JSX, type ReactNode } from "react";
import RotaPicker, { type RotaOption } from "./RotaPicker";
import type {
  CellAbsence,
  ScheduleAbsence,
  ScheduleAbsenceKind,
  ScheduleAssignment,
  ScheduleShift,
  ScheduleTeamMember,
  ScheduleTier,
  RotaFamily,
} from "../types";
import { addDays, initialsOf, isRotationShift, mondayOf, toIsoDate, zoneColumnOf, zoneLabelOn, type RosterSpan } from "../utils/rota";
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
  /** Each team's ordinary-weekday window, by team key, where it is not
   *  Regular hours -- Americas cover for the Americas team. */
  teamDefaultShift?: Readonly<Record<string, string>>;
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
  teamDefaultShift,
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
    const movedTo = new Map<string, { target: string; home: string; active: boolean }>();
    for (const ab of absences) {
      const kind = kindByCode.get(ab.kindCode);
      if (!kind?.movesToTeamKey || !ab.homeTeamKey) continue;
      const active = ab.startsOn <= refIso && (!ab.endsOn || ab.endsOn >= refIso);
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
          days: new Map(), zoned: new Map(), allocs: new Map(), zonedBase: new Map(),
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

    for (const a of assignments) {
      const shift = shifts.get(a.shiftCode);
      const row = seat(a.engineer.userId, a.engineer.name, a.engineer.email, a.teamKey, a.engineer.isLead);
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
        title: shift?.label ?? a.shiftCode,
        isRotation,
        shiftCode: a.shiftCode,
        tier: a.tier,
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
      if (!existing || a.tier) row.days.set(a.rotaDate, made);
    }

    /** Does this engineer hold a rotation turn on this day -- anything but
     *  regular hours -- in any zone or none? */
    const holdsTurn = (row: { days: Map<string, Cell>; zoned: Map<string, Cell> }, iso: string) =>
      Boolean(row.days.get(iso)?.isRotation) ||
      [...row.zoned.entries()].some(([k, c]) => k.startsWith(`${iso}|`) && c.isRotation);

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
      const row = seat(ab.engineer.userId, ab.engineer.name, ab.engineer.email, ab.teamKey, ab.engineer.isLead);
      if (ab.homeTeamKey) row.homeTeams.add(ab.homeTeamKey.toLowerCase());
      const end = ab.endsOn ?? toIsoDate(days[days.length - 1]);
      for (const d of days) {
        const iso = toIsoDate(d);
        // Leave is not taken on a weekend -- nobody is rostered to be away
        // from a Saturday -- so a span running across one leaves those days
        // alone rather than painting them as leave.
        const weekendDay = d.getDay() === 0 || d.getDay() === 6;
        if (weekendDay && kind?.bucket === "LEAVE") continue;
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
            });
            continue;
          }
          const target =
            kind?.bucket === "ALLOCATION" && holdsTurn(row, iso) ? row.allocs : row.days;
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
  }, [absences, assignments, days, kindByCode, shifts, teams, teamKey, teamMembers]);

  const q = query.trim().toLowerCase();
  const rows = q
    ? grid.filter((r) => r.name.toLowerCase().includes(q) || r.teamKey.toLowerCase().includes(q))
    : grid;

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
   *  own team's ordinary day. */
  const defaultFor = useMemo(() => {
    const cellFor = (team: string): Cell => {
      const code = teamDefaultShift?.[team] ?? teamDefaultShift?.[team.toLowerCase()];
      const sh = code ? shifts.get(code) : undefined;
      return sh
        ? { code: sh.shortCode, token: sh.colourToken, title: `Working day (${sh.shortCode})`, isRotation: false, isDefault: true }
        : WORKING_DAY;
    };
    return (
      row: { teamKey: string; isMember: boolean; movedDays: ReadonlySet<string>; homeTeam?: string; homeMember: boolean },
      iso: string,
    ): Cell | undefined => {
      if (row.movedDays.size > 0) {
        if (row.movedDays.has(iso)) return cellFor(row.teamKey);
        return row.homeTeam && row.homeMember ? cellFor(row.homeTeam) : undefined;
      }
      return row.isMember ? cellFor(row.teamKey) : undefined;
    };
  }, [teamDefaultShift, shifts]);

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
        row.homeTeam && row.movedDays && !row.movedDays.has(iso)
          ? row.homeTeam
          : row.teamKey,
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

      {/* The card's one scroller. Both axes live here, which is what lets the
          date row and the engineer column stay pinned to the grid they label
          instead of to the page. */}
      <div className="twwrap rostwrap" ref={wrapRef} onScroll={() => (touched.current = true)}>
        <table className={`tw roster${split ? " split" : ""}`}>
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
          <tbody>
            {rows.map((row, i) => {
              // The first row of each team earns a rule above it: sorted by team
              // with nothing between them, a hundred and twenty rows read as one
              // undifferentiated block.
              const opensTeam = i > 0 && rows[i - 1].teamKey !== row.teamKey;
              // Case-insensitive and trimmed: an identity provider is free to
              // hand back Jane.Doe@Example.com for the address seeded as
              // jane.doe@example.com, and an exact compare would silently
              // never match.
              const isMe = Boolean(me) && row.email.trim().toLowerCase() === me;
              return (
              <tr
                key={`${row.userId}|${row.teamKey}`}
                ref={isMe ? meRow : undefined}
                // Every other row banded, so one engineer's month reads across
                // the grid without a hover to follow it.
                // Tinted in the team's own colour, faintly, so where one team
                // ends and the next begins reads at a glance. --rc carries it.
                className={`tinted${isMe ? " me" : ""}${opensTeam ? " teamtop" : ""}${i % 2 === 1 ? " alt" : ""}`}
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
                    return (
                      <td
                        key={iso}
                        className={`${marks} ${faded(shown)}${editable ? " c editable" : ""}${
                          touched.mark ? " touched" : ""
                        }`}
                        title={
                          editable
                            ? `${row.name} · ${shown ? (withAlloc(cell)?.title ?? shown.title) : "nothing rostered"}${touched.note} — click to change`
                            : shown || touched.mark
                              ? `${row.name} · ${shown ? shown.title : "nothing rostered"}${touched.note}`
                              : undefined
                        }
                        onClick={editable ? (e) => openCell(e, row, iso, withAlloc(cell)) : undefined}
                      >
                        {cell && alloc ? (
                          <span className="duo">
                            <span className={`chip sm ${cell.token}`}>{cell.code}</span>
                            <span className={`chip sm ${alloc.token}`}>{alloc.code}</span>
                          </span>
                        ) : shown ? (
                          <span className={`chip sm ${shown.token}${shown.isDefault ? " dflt" : ""}`}>{shown.code}</span>
                        ) : (
                          <span className="none">·</span>
                        )}
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
                    return (
                      <td
                        key={iso}
                        colSpan={zones.length}
                        className={`c zwhole ${marks} ${faded(whole)}${
                          editable ? " editable" : ""
                        }`}
                        title={
                          editable
                            ? `${row.name} · ${whole.title} — click to change`
                            : `${row.name} · ${whole.title}`
                        }
                        onClick={editable ? (e) => openCell(e, row, iso, cell) : undefined}
                      >
                        <span className={`chip sm ${whole.token}${whole.isDefault ? " dflt" : ""}`}>{whole.code}</span>
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
                              ? `${row.name} · ${z} · ${zc.title}`
                              : undefined
                        }
                        onClick={editable ? (e) => openCell(e, row, iso, withAlloc(row.zoned.get(`${iso}|${z}`)), zoneCodeFor?.(row.teamKey, z) ?? z) : undefined}
                      >
                        {turn && under ? (
                          // The zone's turn and what it sits on, stacked: L1
                          // in TZ1 on an RnD day, or on TZ1's regular hours --
                          // the cell says both.
                          <span className="duo">
                            <span className={`chip sm ${turn.token}`}>{turn.code}</span>
                            <span className={`chip sm ${under.token}`}>{under.code}</span>
                          </span>
                        ) : zc ? (
                          <span className={`chip sm ${zc.token}`}>{zc.code}</span>
                        ) : (
                          <span className="zempty" />
                        )}
                      </td>
                    );
                  });
                })}
              </tr>
              );
            })}
          </tbody>
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
