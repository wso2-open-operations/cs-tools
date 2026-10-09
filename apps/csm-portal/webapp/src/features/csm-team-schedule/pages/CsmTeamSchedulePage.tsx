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

import { useMemo, useState, type JSX } from "react";
import QueryErrorState from "@components/QueryErrorState";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import { useCurrentUser } from "@context/current-user/CurrentUserContext";
import {
  useApplyAbsence,
  useCreateAbsenceKind,
  useDeleteAbsenceKind,
  useDeleteAbsence,
  useScheduleEditMarkers,
  useApplyRange,
  useMyLeadTeams,
  useScheduleAbsences,
  useScheduleAbsencesByMonth,
  useScheduleAssignments,
  useScheduleAssignmentsByMonth,
  useScheduleCatalogue,
  useTeamActivity,
  type MonthWindow,
  type RotaRead,
} from "../api/useTeamSchedule";
import CellPicker, { type CellPickerTarget, type NewAbsenceKind } from "../components/CellPicker";
import { BackendApiError } from "@api/backend/client";
import DayLadder, { type LadderLane } from "../components/DayLadder";
import MonthRoster from "../components/MonthRoster";
import MyWeekStrip from "../components/MyWeekStrip";
import NextRotation from "../components/NextRotation";
import RecentChanges from "../components/RecentChanges";
import WeekTable from "../components/WeekTable";
import type {
  RotaFamily,
  ScheduleAbsencesResponse,
  ScheduleAssignment,
  ScheduleAssignmentsResponse,
  ScheduleShift,
  ScheduleTier,
} from "../types";
import { resolveDisplayTimeZone } from "@utils/dateTime";
import { TeamColourProvider } from "../utils/teamColour";
import {
  addDays,
  isRotationShift,
  kindsOfferedOn,
  moveKindFor,
  movesOfferedOn,
  monthPieces,
  readerFamily,
  rosterRange,
  type RosterSpan,
  mondayOf,
  shiftsByCode,
  toIsoDate,
  zoneAbbreviation,
  zoneLabelOn,
  zoneColumnOf,
} from "../utils/rota";
import { zoneColour } from "../utils/rotaHues";
import { SCHEDULE_THEME_VARS } from "../utils/useScheduleTheme";
import "../teamSchedule.css";
import { teamDisplayName } from "../utils/teamDisplayName";
/** What the banner says when an edit is refused: the server's own reason where
 *  it gave one a lead can act on -- the person is not on that team, or already
 *  holds an overlapping window -- and that nothing moved either way. */
function notSaved(what: string, err: unknown): string {
  const reason =
    err instanceof BackendApiError && (err.status === 403 || err.status === 409) && err.message
      ? ` ${err.message.charAt(0).toUpperCase()}${err.message.slice(1)}.`
      : "";
  return `${what}${reason} Nothing has moved.`;
}

type ViewTab = "mine" | "today" | "week" | "roster";
type Family = RotaFamily;

/** Every family in the order the switch lists them, the reader's own first. */
const ALL_FAMILIES: readonly Family[] = ["CRE", "SRE", "SME"];

// The team list used to live here, mirroring CSM_TEAM_REGISTRY. Team names
// are organisation vocabulary and committing them coupled this page to a
// deploy it cannot see, which is the coupling that registry exists to avoid.
// The catalogue serves them now.

/** How far ahead to look for the reader's next rotation. Eight weeks covers
 *  every rotation in the cycle without asking the API for a year of rows. */
const NEXT_ROTATION_HORIZON_DAYS = 56;

const TITLE: Record<ViewTab, string> = {
  mine: "My week",
  today: "Who is on today",
  week: "Who is on this week",
  roster: "Month roster",
};

/** The roster's window as days, "14 Sept – 12 Oct 2026": it opens and closes
 *  on the days either side of the selected one, not on month boundaries. */
const fmtDayRange = (first: Date, last: Date): string => {
  const sameYear = first.getFullYear() === last.getFullYear();
  const a = first.toLocaleDateString(
    undefined,
    sameYear ? { day: "numeric", month: "short" } : { day: "numeric", month: "short", year: "numeric" },
  );
  const b = last.toLocaleDateString(undefined, { day: "numeric", month: "short", year: "numeric" });
  return `${a} – ${b}`;
};

const fmtLong = (d: Date): string =>
  d.toLocaleDateString(undefined, { weekday: "long", day: "numeric", month: "long", year: "numeric" });

/** Previous/Next moves by whatever the tab is about: a day, a week, a month. */
function stepBy(from: Date, tab: ViewTab, direction: 1 | -1): Date {
  if (tab === "today") return addDays(from, direction);
  if (tab === "roster") {
    const next = new Date(from);
    next.setDate(1);
    next.setMonth(next.getMonth() + direction);
    return next;
  }
  return addDays(from, 7 * direction);
}

/** A change made in the current editing session. */
interface RotaChange {
  userId: string;
  name: string;
  from: string;
  to: string;
  /** What the days became: a window's label, a kind of leave, or back on the rota. */
  what: string;
}

/** "Thu 1 Oct", or "Thu 1 – Sat 3 Oct" for a span. */
function fmtRange(from: string, to: string): string {
  const f = new Date(`${from}T00:00:00`);
  const opts = { weekday: "short", day: "numeric", month: "short" } as const;
  if (from === to) return f.toLocaleDateString(undefined, opts);
  return `${f.toLocaleDateString(undefined, { weekday: "short", day: "numeric" })} – ${new Date(
    `${to}T00:00:00`,
  ).toLocaleDateString(undefined, opts)}`;
}

const fmtShort = (d: Date): string =>
  d.toLocaleDateString(undefined, { day: "numeric", month: "short" });

/**
 * Team Schedule: who from CRE and SRE is working, when, and in which
 * escalation tier.
 */
export default function CsmTeamSchedulePage(): JSX.Element {
  /** null until the reader picks a tab, so the default can follow who they
   *  are -- see `tab` below. */
  const [tabChoice, setTab] = useState<ViewTab | null>(null);
  /** null until the reader picks one, so their own group can be the default
   *  once the profile arrives. Derived rather than corrected in an effect: an
   *  SRE engineer must never render a frame on CRE, because the tab gating
   *  would show them three disabled tabs on arrival. */
  const [familyChoice, setFamilyChoice] = useState<Family | null>(null);
  const [teamKey, setTeamKey] = useState<string>("");
  /** The rota picked within a family that runs more than one (SRE's SaaS and
   *  IaaS, SME's products). "" until the reader picks one, so their own rota
   *  -- or the family's first -- is the default. */
  const [rotaChoice, setRotaChoice] = useState<string>("");
  /** How many months the roster shows, centred on the selected day; see
   *  rosterRange. One by default: the fortnight either side of today is what
   *  a lead opens the roster to check. */
  const [rosterSpan, setRosterSpan] = useState<RosterSpan>(1);
  const [anchor, setAnchor] = useState<Date>(() => new Date());
  /** Bumped by Today; a view listens to it to re-centre on the current day. */
  const [focusRequest, setFocusRequest] = useState(0);

  const { user } = useCurrentUser();
  // The clock this reader is on, and the only source of it: their CSM profile
  // timezone, through the portal's own resolver. Set it there once and every
  // view follows, including after a move from Colombo to San Francisco.
  //
  // There is deliberately no picker on this page. A second control could
  // disagree with the profile, and then two people comparing the same rota
  // over a call have no way of knowing whose clock they are each reading.
  const tz = resolveDisplayTimeZone(user?.timeZone);
  const catalogue = useScheduleCatalogue();

  /** The teams of the group on screen, in the order the catalogue gives them.
   *  Empty until it loads, which reads as "no teams yet" rather than as a
   *  wrong list. */
  const teamsOf = useMemo(() => {
    const all = catalogue.data?.teams ?? [];
    return (f: Family, rota?: string): string[] =>
      all.filter((t) => t.family === f && (!rota || t.rotaCode === rota)).map((t) => t.key);
  }, [catalogue.data?.teams]);

  /** The rotas of a family that have a team on them, in the catalogue's order.
   *  A rota with no team yet (IaaS before its teams sync) is left out: it would
   *  only draw an empty day. */
  const rotasOf = useMemo(() => {
    const rotas = catalogue.data?.rotas ?? [];
    const teams = catalogue.data?.teams ?? [];
    return (f: Family) =>
      rotas.filter((r) => r.family === f && teams.some((t) => t.rotaCode === r.code));
  }, [catalogue.data?.rotas, catalogue.data?.teams]);

  /** Which rota each zone belongs to, for keeping another rota's windows out
   *  of the one on screen. */
  const rotaOfZone = useMemo(
    () => new Map((catalogue.data?.zones ?? []).map((z) => [z.code, z.rotaCode])),
    [catalogue.data?.zones],
  );

  // Which teams this reader may edit. Asked once: it changes when somebody is
  // made a lead, not while they are looking at a rota.
  const leadTeams = useMyLeadTeams();
  const { showError } = useErrorBanner();
  const applyRange = useApplyRange();
  const applyAbsence = useApplyAbsence();
  const deleteAbsence = useDeleteAbsence();
  const createKind = useCreateAbsenceKind();
  const deleteKind = useDeleteAbsenceKind();

  /** Edit mode, and the cell it has open.
   *
   *  A mode rather than always-on, and owned here rather than by the roster,
   *  because the toggle belongs with the other page controls that change what
   *  a click does. Turning editing off closes whatever is open with it. */
  const [editing, setEditing] = useState(false);
  const [picker, setPicker] = useState<CellPickerTarget | null>(null);

  /** The changes made in this editing session, newest last.
   *
   *  A change saves the moment it is made, and the cell then simply shows its
   *  new value -- which looks exactly like a value that was always there. So
   *  a lead who has changed six days could not see which six. Each saved
   *  change is kept here and marked on the roster until they click Done
   *  editing, which is the point they have said they are finished. */
  const [changes, setChanges] = useState<RotaChange[]>([]);
  /** Whether the lead has the recent-changes panel open on the roster. */
  const [showRecent, setShowRecent] = useState(false);
  const changedCells = useMemo(() => {
    const out = new Set<string>();
    for (const c of changes) {
      for (let d = new Date(`${c.from}T00:00:00`); toIsoDate(d) <= c.to; d = addDays(d, 1)) {
        out.add(`${c.userId}|${toIsoDate(d)}`);
      }
    }
    return out;
  }, [changes]);

  /** Whether this reader leads anything, which is what decides whether the
   *  toggle is offered. Whether a given row is theirs is a separate question
   *  the roster answers per team, and the server answers again on the write. */
  const canEditRota = (leadTeams.data ?? []).length > 0;


  /** The reader's own group, from their CSM profile: their team, or for a
   *  rota admin the group their role runs (see readerFamily). Absent for
   *  anyone who belongs to neither -- a manager -- which is why it is
   *  optional rather than defaulting to CRE. */
  const myFamily: Family | undefined = useMemo(() => {
    const familyOfTeam = new Map((catalogue.data?.teams ?? []).map((t) => [t.key, t.family]));
    const editable = (leadTeams.data ?? [])
      .map((k) => familyOfTeam.get(k))
      .filter((f): f is Family => Boolean(f));
    return readerFamily(user?.team?.family, user?.roles, editable);
  }, [user?.team?.family, user?.roles, leadTeams.data, catalogue.data?.teams]);

  /** Nobody's group: a manager, who belongs to no team and runs no rota. */
  const isManager = myFamily === undefined;

  /** The tab the page opens on. An engineer's first question is their own
   *  rota, so they land on My week; a manager holds none, and comes here to see
   *  who is covering, so they land on Today. Derived rather than set in an
   *  effect: /users/me has settled before this page mounts, so the first frame
   *  is already the right one. */
  const tab: ViewTab = tabChoice ?? (isManager ? "today" : "mine");

  /**
   * Which tabs are on the strip at all. A manager holds no rota on either
   * group, so My week can never apply to them, and a permanently dead tab is
   * just something to wonder about. It is not offered.
   */
  const visibleTabs: ViewTab[] = (["mine", "today", "week", "roster"] as ViewTab[]).filter(
    (t) => !(isManager && t === "mine"),
  );

  /** The tab actually being shown. A manager's remembered My week (from
   *  before the profile said they hold no rota) falls back to Today. */
  const view: ViewTab = isManager && tab === "mine" ? "today" : tab;

  /**
   * Whether this view can show the other group.
   *
   *   Today         yes. It answers "who is covering right now", which a CRE
   *                 engineer escalating to SRE needs, and the other way round.
   *   the rest      no, for an engineer. My week, This week and the roster are
   *                 planning views for their own rota; the other group's is
   *                 not theirs to check, so there is no switch to offer.
   *   a manager     yes, everywhere. They belong to neither group, and who is
   *                 covering -- today, across the week, across the month -- is
   *                 their question for CRE and SRE alike.
   */
  const crossesGroups = isManager || view === "today";

  /** The group on screen: the reader's own, unless Today (or a manager) has
   *  picked the other. CRE only as a last resort, for a manager who belongs
   *  to neither. The choice made on Today is kept for Today, so leaving it
   *  for the roster and coming back does not lose it. */
  const family: Family = crossesGroups ? (familyChoice ?? myFamily ?? "CRE") : (myFamily ?? "CRE");

  /** The groups the switch offers, the reader's own first: an SRE engineer
   *  reads "SRE | CRE", because the first thing in a pair reads as the
   *  default, and theirs is. One group means no switch at all. */
  const families: Family[] = !crossesGroups
    ? [family]
    : (() => {
        // Only the families the catalogue has a team in: SME appears once its
        // teams exist, and until then the switch reads CRE | SRE as before.
        const present = ALL_FAMILIES.filter((f) => (catalogue.data?.teams ?? []).some((t) => t.family === f));
        const list = present.length > 0 ? present : (["CRE", "SRE"] as Family[]);
        return myFamily && list.includes(myFamily) ? [myFamily, ...list.filter((f) => f !== myFamily)] : list;
      })();

  /** The reader's own rota, from their team -- what an engineer's own views
   *  stay on, and where a rota'd family opens. */
  const myRota = (catalogue.data?.teams ?? []).find((t) => t.key === user?.team?.teamKey)?.rotaCode;

  /** The team filter, where it belongs to the group on screen. A team picked
   *  on Today's SRE side means nothing on the reader's CRE roster, and would
   *  otherwise filter it to nobody. Any team of the family may be picked --
   *  SME's product teams are peers, the way CRE's ABTs are -- so this is read
   *  before the rota, which follows from it. */
  const shownTeamKey = teamKey && teamsOf(family).includes(teamKey) ? teamKey : "";

  /** The rotas the family on screen runs, and the one shown: the rota of the
   *  team picked, else the reader's pick on a view that may cross rotas, else
   *  their own, else the first. */
  const familyRotas = rotasOf(family);
  const rota: string | undefined = (() => {
    if (familyRotas.length === 0) return undefined;
    const has = (code?: string) => Boolean(code) && familyRotas.some((r) => r.code === code);
    const teamRota = (catalogue.data?.teams ?? []).find((t) => t.key === shownTeamKey)?.rotaCode;
    if (has(teamRota)) return teamRota;
    if (crossesGroups && has(rotaChoice)) return rotaChoice;
    if (has(myRota)) return myRota;
    return familyRotas[0].code;
  })();
  /** Only a family that runs more than one rota is narrowed by it. SRE with
   *  SaaS alone reads exactly as before. */
  const rotaScoped = familyRotas.length > 1;

  const weekStart = useMemo(() => mondayOf(anchor), [anchor]);
  /** The group/team controls the cards render in their own heads. It is the
   *  page's state either way -- the toolbar and the card head are two views of
   *  one control, which is why they can never disagree. */
  const scopeControls = {
    family,
    onFamilyChange: (f: Family) => {
      setFamilyChoice(f);
      setRotaChoice("");
      setTeamKey("");
    },
    teamKey: shownTeamKey,
    onTeamKeyChange: setTeamKey,
    // Every team of the family, on every view: picking one in another rota
    // moves the card onto that rota (see `rota` above), so SME's product
    // teams can read each other's Day and Night cover, as CRE's ABTs can.
    teams: teamsOf(family),
    families,
    // The rota picker: offered only where the reader may cross rotas, the
    // same rule as the family switch.
    rotas: crossesGroups ? familyRotas.map((r) => ({ code: r.code, label: r.label })) : [],
    rotaCode: rota,
    onRotaChange: (code: string) => {
      setRotaChoice(code);
      setTeamKey("");
    },
  };

  const dayView = view === "today";
  const rosterView = view === "roster";
  /** The roster's window: its span either side of the selected day, and the
   *  calendar-month pieces it is fetched in. Keyed on the day and the span
   *  rather than on the Date, which is a fresh object each render. */
  const anchorIso = toIsoDate(anchor);
  const { rosterStart, rosterEnd, rosterMonths } = useMemo(() => {
    const { start, end } = rosterRange(new Date(`${anchorIso}T00:00:00`), rosterSpan);
    return { rosterStart: start, rosterEnd: end, rosterMonths: monthPieces(start, end) as MonthWindow[] };
  }, [anchorIso, rosterSpan]);
  const from = dayView ? toIsoDate(anchor) : toIsoDate(weekStart);
  const to = dayView ? toIsoDate(anchor) : toIsoDate(addDays(weekStart, 6));
  // A family running more than one rota reads only the rota on screen's
  // teams; otherwise the family alone, as before.
  /** The rota the data on screen is narrowed to: one at a time, on every
   *  view. Rotas do not share their zones -- SRE SaaS runs TZ1, TZ2 and TZ3,
   *  IaaS a Day and a Night -- and a grid drawn for both gave SaaS's people
   *  Day and Night columns they never work. The rota picker switches; My week
   *  is the reader's own rota, or the family's first for a reader with none
   *  (a rota admin), who otherwise saw IaaS's lanes on their own week. */
  const scopeRota: string | undefined = view === "mine" ? (myRota ?? rota) : rota;
  // A family running more than one rota reads only the scoped rota's teams;
  // otherwise the family alone, as before.
  const teamKeys = shownTeamKey ? [shownTeamKey] : rotaScoped && scopeRota ? teamsOf(family, scopeRota) : undefined;

  const singleRead = useScheduleAssignments(
    {
      from,
      to,
      family,
      teamKeys,
      // Only the day view needs the crew that started last night and is still
      // working this morning; the week groups by rota date, so it does not.
      includeOvernight: dayView,
    },
    !rosterView,
  );
  // The roster spans three months, which is more than one read may ask for,
  // so it is read a month at a time and merged.
  const rosterRead = useScheduleAssignmentsByMonth({ family, teamKeys }, rosterMonths, rosterView);
  const assignments: RotaRead<ScheduleAssignmentsResponse> = rosterView ? rosterRead : singleRead;

  // My week is the signed-in engineer's own rota, found by the email the
  // portal knows them by.
  const mine = useScheduleAssignments(
    { from: toIsoDate(weekStart), to: toIsoDate(addDays(weekStart, 6)), userEmail: user?.email ?? "" },
    view === "mine" && Boolean(user?.email),
  );

  // Their own leave and allocations over the same week, so My week can say
  // "on leave" or "at a customer" rather than showing an empty day.
  const mineAbsences = useScheduleAbsences(
    { from: toIsoDate(weekStart), to: toIsoDate(addDays(weekStart, 6)), userEmail: user?.email ?? "" },
    view === "mine" && Boolean(user?.email),
  );

  // When this engineer is next on a rotation. Its own query, deliberately:
  // it looks forward from today rather than at whatever week the reader has
  // navigated to, so the answer does not change as they page around.
  const nextFrom = useMemo(() => new Date(), []);
  const upcoming = useScheduleAssignments(
    {
      from: toIsoDate(nextFrom),
      to: toIsoDate(addDays(nextFrom, NEXT_ROTATION_HORIZON_DAYS)),
      userEmail: user?.email ?? "",
    },
    Boolean(user?.email),
  );

  // Off rota follows the CRE/SRE choice like everything else on the page.
  // Without this it showed every absence in the company, so SRE's column
  // carried CRE's migration allocations -- people SRE has no relationship to.
  // The search filters by team, so an unfiltered view passes the family's own
  // teams rather than nothing.
  const absenceTeamKeys = teamKeys ?? teamsOf(family, rotaScoped ? scopeRota : undefined);
  // The week view reads leave too, for its leave row; `from`/`to` are already
  // the week there.
  const dayAbsences = useScheduleAbsences({ from, to, teamKeys: absenceTeamKeys }, dayView || view === "week");
  const rosterAbsences = useScheduleAbsencesByMonth(
    { teamKeys: absenceTeamKeys },
    rosterMonths,
    rosterView,
  );
  const absences: RotaRead<ScheduleAbsencesResponse> = rosterView ? rosterAbsences : dayAbsences;

  const allShifts = useMemo(() => shiftsByCode(catalogue.data?.shifts ?? []), [catalogue.data?.shifts]);

  /** The windows of the rota on screen. Every view reads its zones and its
   *  escalation rows off the windows it is given, so another rota's zoned
   *  windows have to be left out here -- or IaaS, which is SRE too, would add
   *  its Day and Night to SaaS's ladder, week and roster. A window with no
   *  zone, or a zone no rota claims, belongs to every rota as before. My week
   *  stays on the reader's own rota whatever Today is showing. */
  const shifts = useMemo(() => {
    const keep = scopeRota;
    if (!keep) return allShifts;
    const out = new Map<string, ScheduleShift>();
    for (const [code, sh] of allShifts) {
      const zoneRota = sh.zoneCode ? rotaOfZone.get(sh.zoneCode) : undefined;
      if (!zoneRota || zoneRota === keep) out.set(code, sh);
    }
    return out;
  }, [allShifts, rotaOfZone, scopeRota]);

  // The history of the lead's own teams over the months on screen. Read only
  // while the panel is open -- it is a question a lead asks now and then, not
  // one every page load should pay for.
  const activity = useTeamActivity(leadTeams.data ?? [], rosterMonths, showRecent && view === "roster");
  // Memoised rather than written inline: `?? []` builds a fresh array on every
  // render, so every memo downstream that depends on it recomputes every time
  // -- and the React Compiler refuses to optimise a component whose manual
  // memoization it cannot preserve. Flagged as a warning since this page was
  // written; adding another consumer turned it into an error.
  const zones = useMemo(() => catalogue.data?.zones ?? [], [catalogue.data?.zones]);
  const rows = useMemo(
    () => assignments.data?.assignments ?? [],
    [assignments.data?.assignments],
  );

  /** Engineers' names by id, for the history, which keeps only ids. Taken from
   *  what the roster already has loaded -- the same people the changes are to. */
  const namesById = useMemo(() => {
    const m = new Map<string, string>();
    for (const a of rows) m.set(a.engineer.userId, a.engineer.name);
    for (const ab of absences.data?.absences ?? []) m.set(ab.engineer.userId, ab.engineer.name);
    return m;
  }, [rows, absences.data?.absences]);
  const nameOf = (userId: string): string | undefined => namesById.get(userId);

  /** A lead picked a cell on the roster. The roster says which slot and
   *  where on screen; the picker does the rest. */
  const editCell = (edit: Omit<CellPickerTarget, "baseShiftCode" | "otherTurns">): void => {
    setPicker({
      ...edit,
      baseShiftCode: baseShiftFor(edit.userId),
      otherTurns: holdsTurnElsewhere(edit.userId, edit.rotaDate, edit.zoneCode, edit.shiftCode),
    });
  };

  /** The standing window this engineer sits in on an ordinary weekday, read
   *  off what they already hold rather than assumed.
   *
   *  CRE runs two of them -- Lanka and India hours -- so which one somebody
   *  goes back to when a rotation is cleared is a fact about that engineer,
   *  not about their group, and guessing it would quietly move people onto
   *  the wrong clock. */
  /** Is the clicked cell an escalation turn, on a day the engineer holds
   *  something else too -- a turn in another zone, or regular hours? Then
   *  clearing takes off that zone's turn only, rather than putting the whole
   *  day back on regular hours. */
  const holdsTurnElsewhere = (userId: string, rotaDate: string, zoneCode?: string, shiftCode?: string): boolean => {
    if (!zoneCode || !shiftCode || !shifts.get(shiftCode)?.isEscalation) return false;
    return rows.some(
      (a) =>
        a.engineer.userId === userId &&
        a.rotaDate === rotaDate &&
        a.shiftCode !== shiftCode &&
        (!shifts.get(a.shiftCode)?.isEscalation ||
          (a.zoneCode ?? shifts.get(a.shiftCode)?.zoneCode) !== zoneCode),
    );
  };

  const baseShiftFor = (userId: string): string | undefined => {
    const seen = new Map<string, number>();
    for (const a of rows) {
      if (a.engineer.userId !== userId) continue;
      if (isRotationShift(shifts.get(a.shiftCode))) continue;
      seen.set(a.shiftCode, (seen.get(a.shiftCode) ?? 0) + 1);
    }
    let best: string | undefined;
    let most = 0;
    for (const [code, n] of seen) {
      if (n > most) [best, most] = [code, n];
    }
    return best;
  };

  /** The windows this group runs, which is what the picker offers. Filtered
   *  by family and nothing narrower: the catalogue's own codes are already
   *  CRE or SRE, so there is no second rule to keep in step with. */
  /** Who changed which cell, over exactly the months the roster is showing.
   *
   *  Only asked for while the roster is open -- no other view marks a cell --
   *  and kept out of the rota's own query so the grid never waits on it. */
  const editMarkers = useScheduleEditMarkers(rosterMonths, view === "roster");

  /** Keyed for the roster to look up a cell in one step rather than scanning. */
  const editedCells = useMemo(() => {
    const out = new Map<string, { actor: string; changedAt: string }>();
    for (const m of editMarkers.markers) {
      out.set(`${m.userId}|${m.rotaDate}`, { actor: m.actor, changedAt: m.changedAt });
    }
    return out;
  }, [editMarkers.markers]);

  /** What a lead can mark somebody away for from the roster: leave, and time
   *  allocated elsewhere -- on this rota. CRE and SRE allocate time to
   *  different things (RnD is SRE's, Migration is CRE's), so each is offered
   *  only its own. EXCLUDED -- off the rota entirely -- is not a lead's to set
   *  from a cell, and a retired kind is served only so old days keep their
   *  label. */
  const awayKinds = useMemo(
    () =>
      kindsOfferedOn(catalogue.data?.absenceKinds ?? [], family),
    [catalogue.data?.absenceKinds, family],
  );

  /** Moves to another team a lead can make from a cell (Move to Migration). */
  const moveKinds = useMemo(
    () => movesOfferedOn(catalogue.data?.absenceKinds ?? [], family),
    [catalogue.data?.absenceKinds, family],
  );

  /** The windows the picker offers for the cell that is open: every window
   *  the group works, in every zone. The zone column a lead clicked is marked
   *  in the picker rather than used to hide the others -- rostering L3 for
   *  TZ2 from a TZ1 cell is an ordinary thing to want, and having to find the
   *  TZ2 column first made it a hunt. */
  const pickerShifts = useMemo(() => {
    // The clicked engineer's own rota's windows: a Moesif engineer is never
    // offered Asgardeo's Day, even on a roster showing every SME rota.
    const own = (catalogue.data?.teams ?? []).find((t) => t.key === picker?.teamKey)?.rotaCode;
    return [...allShifts.values()].filter((sh) => {
      if (sh.family !== family) return false;
      const zoneRota = sh.zoneCode ? rotaOfZone.get(sh.zoneCode) : undefined;
      return !own || !zoneRota || zoneRota === own;
    });
  }, [allShifts, family, picker?.teamKey, catalogue.data?.teams, rotaOfZone]);

  /** The real zone behind a roster column for one team's engineer: the Day
   *  column is ASG_D for Asgardeo, MOE_D for Moesif. */
  const zoneCodeFor = (teamKey: string, column: string): string | undefined => {
    const own = (catalogue.data?.teams ?? []).find((t) => t.key === teamKey)?.rotaCode;
    return zones.find((z) => (!own || z.rotaCode === own) && zoneColumnOf(z.code) === column)?.code;
  };

  const applyToCell = (shiftCode: string, from: string, to: string, tier?: ScheduleTier): void => {
    if (!picker) return;
    const label = shifts.get(shiftCode)?.label ?? shiftCode;
    // Americas cover for somebody not on the Americas team is the Brazil
    // rotation: it moves them there for these days, and the server writes the
    // shifts. Marked as that span, so their own team's lead still owns it.
    const move = moveKindFor(catalogue.data?.absenceKinds ?? [], shiftCode, picker.teamKey);
    if (move) {
      markAway(move.code, from, to);
      return;
    }
    applyRange.mutate(
      {
        userId: picker.userId,
        teamKey: picker.teamKey,
        shiftCode,
        from,
        to,
        note: "set from the month roster",
        ...(tier ? { tier } : {}),
      },
      {
        onSuccess: () => recordChange(picker, from, to, tier ? `${tier} · ${label}` : label),
        onError: (err) =>
          showError(notSaved("That change to the rota was not saved.", err), err),
        onSettled: () => setPicker(null),
      },
    );
  };

  const recordChange = (target: CellPickerTarget, from: string, to: string, what: string): void =>
    setChanges((cs) => [...cs, { userId: target.userId, name: target.name, from, to, what }]);

  /** A tag as the change log names it: a move worked as another team's
   *  normal hours by those hours (Americas cover), anything else by its label. */
  const kindLabel = (kindCode: string): string => {
    const kind = (catalogue.data?.absenceKinds ?? []).find((k) => k.code === kindCode);
    const shownAs = kind?.worksRotaThere && kind.showsAsShiftCode ? shifts.get(kind.showsAsShiftCode) : undefined;
    return shownAs?.label ?? kind?.label ?? kindCode;
  };

  const markAway = (kindCode: string, from: string, to: string, allocatedTo?: string): void => {
    if (!picker) return;
    applyAbsence.mutate(
      {
        userId: picker.userId,
        teamKey: picker.teamKey,
        kindCode,
        from,
        to,
        note: "marked from the month roster",
        ...(allocatedTo ? { allocatedTo } : {}),
      },
      {
        onSuccess: () =>
          recordChange(
            picker,
            from,
            to,
            kindCode
              ? `${kindLabel(kindCode)}${
                  allocatedTo ? ` (${allocatedTo})` : ""
                }`
              : "back on the rota",
          ),
        onError: (err) =>
          showError(notSaved("That change to who is away was not saved.", err), err),
        onSettled: () => setPicker(null),
      },
    );
  };

  /** Remove the whole leave or allocation the clicked cell is part of. */
  const removeAbsence = (absenceId: string): void => {
    if (!picker?.absence) return;
    const ab = picker.absence;
    const label =
      (catalogue.data?.absenceKinds ?? []).find((k) => k.code === ab.kindCode)?.label ?? ab.kindCode;
    deleteAbsence.mutate(
      { id: absenceId, note: "removed from the month roster" },
      {
        onSuccess: () =>
          recordChange(picker, ab.startsOn, ab.endsOn ?? ab.startsOn, `${label} removed`),
        onError: (err) =>
          showError("That leave or allocation was not removed. Nothing has changed.", err),
        onSettled: () => setPicker(null),
      },
    );
  };

  /** Add a tag to the shared catalogue. The picker stays open so the lead can
   *  use it straight away; the form shows why when it is refused. */
  const addKind = (kind: NewAbsenceKind): Promise<void> =>
    createKind.mutateAsync(kind).then(
      () => undefined,
      (err: unknown) => {
        throw new Error(
          err instanceof BackendApiError && err.status === 409
            ? "A tag with that name or short code already exists."
            : err instanceof BackendApiError && err.status === 403
              ? "Only a team lead can add a tag."
              : "The tag was not added. Try again.",
        );
      },
    );

  /** Delete a tag a lead added. The picker stays open and shows why when the
   *  server refuses -- most often because the tag is still in use. */
  const removeKind = (code: string): Promise<void> =>
    deleteKind.mutateAsync(code).then(
      () => undefined,
      (err: unknown) => {
        throw new Error(
          err instanceof BackendApiError && err.status === 409
            ? "That tag is still used on the rota. Remove those entries first."
            : err instanceof BackendApiError && err.status === 403
              ? "Only a team lead can delete a tag, and never a built-in one."
              : "The tag was not deleted. Try again.",
        );
      },
    );

  /** Clearing a cell means two different writes depending on what is on it.
   *
   *  Somebody marked away comes back by removing the absence, and the rota
   *  underneath -- which was never deleted, only covered -- shows through
   *  again. Rewriting the assignment as well would churn the history with a
   *  change that did not happen. Anything else is a rota slot, so it goes
   *  back to the standing window the way it always did. */
  const clearCell = (shiftCode: string, from: string, to: string): void => {
    if (!picker) return;
    // A cell holding a turn and an allocation beside it clears the turn;
    // the allocation has its own Remove.
    if (picker.absenceKindCode && !picker.shiftCode) {
      markAway("", from, to);
      return;
    }
    // One zone's turn, on a day with turns in other zones too: take off this
    // one and leave the rest of the day as it is.
    if (picker.otherTurns && picker.zoneCode) {
      applyRange.mutate(
        {
          userId: picker.userId,
          teamKey: picker.teamKey,
          shiftCode: "",
          zoneCode: picker.zoneCode,
          from,
          to,
          note: "cleared from the month roster",
        },
        {
          onSuccess: () =>
            recordChange(picker, from, to, `off ${picker.zoneCode}${picker.tier ? ` ${picker.tier}` : ""}`),
          onError: (err) => showError(notSaved("That change to the rota was not saved.", err), err),
          onSettled: () => setPicker(null),
        },
      );
      return;
    }
    applyToCell(shiftCode, from, to);
  };

  // SRE works in time zones, so its day is a lane per zone. CRE runs on one
  // clock, so it gets one lane.
  /** The zones SRE actually staffs on the day being viewed.
   *
   *  At the weekend TZ1 and TZ2 are one crew, carried under TZ1, and TZ3 is
   *  as it is on a weekday -- so a weekend earns two lanes and a weekday
   *  three. Read from the shifts rather than written down
   *  here, which is the same rule the month roster follows and from the same
   *  place: a rota change lands in both without a code change, and a lane can
   *  never appear that nobody could be working in.
   *
   *  Falling back to every zone if the catalogue yields none is deliberate --
   *  an empty ladder would read as "nobody is on" rather than as a catalogue
   *  that failed to load. */
  const zonesOnDay = useMemo(() => {
    const weekend = anchor.getDay() === 0 || anchor.getDay() === 6;
    const scope = weekend ? "WEEKEND" : "WEEKDAY";
    const staffed = new Set<string>();
    for (const sh of shifts.values()) {
      if (sh.family !== family || !sh.zoneCode) continue;
      if (sh.dayScope === scope || sh.dayScope === "ANY") staffed.add(sh.zoneCode);
    }
    // The zones of the rota on screen (a zone no rota claims counts as every
    // rota's), so the fallback below can never pull in another rota's lanes.
    const own = zones.filter((z) => !rota || !z.rotaCode || z.rotaCode === rota);
    const kept = own.filter((z) => staffed.has(z.code));
    return kept.length > 0 ? kept : own;
  }, [anchor, shifts, zones, family, rota]);

  const lanes: LadderLane[] = useMemo(() => {
    if (family === "CRE") {
      return [
        { name: "Rotations", sub: "on-call, the night and regular hours", colour: "var(--muted)", assignments: rows },
      ];
    }
    const weekendDay = anchor.getDay() === 0 || anchor.getDay() === 6;
    return zonesOnDay.map((z) => {
      // At the weekend the TZ1 lane is TZ1 and TZ2 together.
      const name = zoneLabelOn(shifts, z.code, weekendDay);
      return {
        name,
        sub: name === zoneLabelOn(shifts, z.code, false) ? z.label : "Weekend crew",
        colour: zoneColour(z.code),
        // The same resolution the month roster uses: an assignment's own zone,
        // else its window's, so a zoned window stored without one still lands.
        assignments: rows.filter((a) => (a.zoneCode ?? shifts.get(a.shiftCode)?.zoneCode) === z.code),
        // Escalation on one side, everyone else in the zone on the other.
        layout: "zone" as const,
      };
    });
  }, [anchor, family, rows, shifts, zonesOnDay]);

  /** The teams the edit bar says this reader can change, by name and in the
   *  catalogue's order. A rota admin can change every team of the group, and
   *  a list of all of them says less than "every team" does. */
  const editableTeamsLabel = useMemo(() => {
    const all = catalogue.data?.teams ?? [];
    const mine = new Set((leadTeams.data ?? []).map((k) => k.toLowerCase()));
    const ofGroup = all.filter((t) => t.family === family);
    if (ofGroup.length > 0 && ofGroup.every((t) => mine.has(t.key.toLowerCase()))) {
      return `Every ${family} team`;
    }
    const known = new Map(all.map((t) => [t.key.toLowerCase(), t]));
    return [...mine]
      .map((k) => known.get(k))
      .filter((t): t is NonNullable<typeof t> => t !== undefined)
      .sort((a, b) => a.sortOrder - b.sortOrder)
      .map((t) => teamDisplayName(t.name))
      .join(", ");
  }, [catalogue.data?.teams, leadTeams.data, family]);

  /** Each team's ordinary-weekday window, where it is not Regular hours. */
  const teamDefaultShift = useMemo(
    () =>
      Object.fromEntries(
        (catalogue.data?.teams ?? []).flatMap((t) => (t.defaultShiftCode ? [[t.key, t.defaultShiftCode]] : [])),
      ),
    [catalogue.data?.teams],
  );

  /** Each team's members, by key, so the roster shows every one of them. */
  const teamMembers = useMemo(
    () => Object.fromEntries((catalogue.data?.teams ?? []).map((t) => [t.key, t.members ?? []])),
    [catalogue.data?.teams],
  );

  if (catalogue.isError) {
    return <QueryErrorState message="Could not load the schedule catalogue." error={catalogue.error} />;
  }

  const busy = catalogue.isLoading || (view === "mine" ? mine.isLoading : assignments.isLoading);

  return (
    <TeamColourProvider teams={catalogue.data?.teams ?? []}>
    <div className="csm-ts" style={SCHEDULE_THEME_VARS}>
      <div className="wrap">
        <h1>Team Schedule</h1>

        <div className="tabrow">
          <div className="tabs" role="tablist">
            {visibleTabs.map((t) => {
              return (
              <button
                key={t}
                id={`ts-tab-${t}`}
                className={`tab ${view === t ? "on" : ""}`}
                role="tab"
                // The CSS class said which tab was active; nothing did for a
                // screen reader, which read four equal buttons and a panel
                // belonging to none of them.
                aria-selected={view === t}
                aria-controls="ts-panel"
                onClick={() => {
                  setTab(t);
                  // My week is the reader's own rota and who they work it
                  // with, whichever team that is -- a team picked on another
                  // view would hide those people, so it opens on All teams.
                  if (t === "mine") setTeamKey("");
                }}
              >
                <span className="tl">{TITLE[t]}</span>
                <span className="tsub">
                  {t === "today"
                    ? fmtShort(anchor)
                    : t === "roster"
                      ? fmtDayRange(rosterStart, rosterEnd)
                      : `${fmtShort(weekStart)} – ${fmtShort(addDays(weekStart, 6))}`}
                </span>
              </button>
              );
            })}
          </div>

          <div className="tabright">
            <div className="controls">
              {/* The group and team controls used to live here. Every card now
                  carries them in its own head, where the reader is actually
                  looking, so a second copy up here is just something else to
                  keep in step. The toolbar keeps what is genuinely about the
                  page rather than the card: which date you are on. */}
              <div className="monthnav">
                {/* Two granularities, because a month roster needs both: the
                    double chevron moves by whatever the view is about -- a
                    week, a month -- and the single one always moves a day.
                    Without the day step, picking out the 14th on the roster
                    meant opening the calendar; without the period step,
                    reaching next month meant thirty clicks.

                    On the day view the two would do the same thing, so only
                    one pair is shown. */}
                {dayView ? null : (
                  <button
                    onClick={() => setAnchor(stepBy(anchor, view, -1))}
                    title={rosterView ? "Previous month" : "Previous week"}
                    aria-label={rosterView ? "Previous month" : "Previous week"}
                  >
                    &laquo;
                  </button>
                )}
                <button
                  onClick={() => setAnchor(addDays(anchor, -1))}
                  title="Previous day"
                  aria-label="Previous day"
                >
                  &lsaquo;
                </button>
                <span className="lbl">
                  <b>
                    {dayView
                      ? fmtLong(anchor)
                      : rosterView
                        ? fmtDayRange(rosterStart, rosterEnd)
                        : `${fmtShort(weekStart)} – ${fmtShort(addDays(weekStart, 6))}`}
                  </b>
                  {/* The day the single chevrons are moving. Without it, a day
                      step inside the same week changes nothing on screen and
                      the button reads as broken. */}
                  {dayView ? null : <i className="on">{fmtShort(anchor)}</i>}
                </span>
                <button
                  onClick={() => setAnchor(addDays(anchor, 1))}
                  title="Next day"
                  aria-label="Next day"
                >
                  &rsaquo;
                </button>
                {dayView ? null : (
                  <button
                    onClick={() => setAnchor(stepBy(anchor, view, 1))}
                    title={rosterView ? "Next month" : "Next week"}
                    aria-label={rosterView ? "Next month" : "Next week"}
                  >
                    &raquo;
                  </button>
                )}
                {/* Stepping a day at a time is fine for next week and hopeless
                    for next quarter, so the date is also directly selectable. */}
                <label className="jump" title="Jump to a date">
                  <svg
                    width="15"
                    height="15"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="2"
                    aria-hidden="true"
                  >
                    <rect x="3" y="5" width="18" height="16" rx="2" />
                    <path d="M8 3v4M16 3v4M3 10h18" />
                  </svg>
                  <input
                    id="ts-date-jump"
                    type="date"
                    aria-label="Jump to a date"
                    value={toIsoDate(anchor)}
                    onChange={(e) => {
                      const picked = e.target.value;
                      if (!picked) return;
                      // Parsed as local midnight, not UTC: `new Date("2026-09-21")`
                      // is UTC midnight, which lands on the 20th for anyone west
                      // of Greenwich and silently shows the wrong day.
                      const [y, m, d] = picked.split("-").map(Number);
                      setAnchor(new Date(y, m - 1, d));
                    }}
                  />
                </label>
              </div>
              <button
                className="btn"
                onClick={() => {
                  setAnchor(new Date());
                  // Pressing Today when today is already the anchor changes no state,
                  // so a view that scrolled away would sit where it is. The press is
                  // the request, not the date, so it is counted rather than compared.
                  setFocusRequest((n) => n + 1);
                }}
              >
                Today
              </button>
            </div>

            {canEditRota ? (
              <button
                className={`btn sm${editing ? " primary" : ""}`}
                aria-pressed={editing}
                onClick={() => {
                  setEditing((v) => !v);
                  setPicker(null);
                  // Done editing is "I am finished": the marks have done their
                  // job, and the next session starts clean.
                  setChanges([]);
                }}
              >
                <svg
                  width="14"
                  height="14"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="2"
                  aria-hidden="true"
                >
                  <path d="M4 20h4l10-10a2.8 2.8 0 0 0-4-4L4 16v4z" />
                </svg>
                <span>{editing ? "Done editing" : "Edit rota"}</span>
              </button>
            ) : null}
          </div>
        </div>

        {/* Only on the roster: it is the only view a cell can be clicked in,
            and a bar saying "click a cell" above a view with none to click
            reads as something broken. Names the teams because the grid shows
            every team and only the reader's own rows are live. */}
        {canEditRota && editing && view === "roster" ? (
          <div className="editbar">
            <span className="pill">Editing</span>
            <span>{editableTeamsLabel}</span>
            {changes.length === 0 ? (
              <span className="hintx">
                click a cell in your own team&rsquo;s rows to change that day
                &middot; changes save as you make them
              </span>
            ) : (
              // What has changed, so a lead can see their own edits: the count,
              // and the last one spelled out. Every changed cell is ringed on
              // the grid below until Done editing.
              <span className="hintx" role="status">
                <b>
                  {changes.length} change{changes.length === 1 ? "" : "s"}
                </b>{" "}
                &middot; last: {changes[changes.length - 1].name},{" "}
                {fmtRange(changes[changes.length - 1].from, changes[changes.length - 1].to)} &rarr;{" "}
                {changes[changes.length - 1].what}
                <i className="chg-key" aria-hidden="true" /> changed this session
              </span>
            )}
          </div>
        ) : null}

        {canEditRota && showRecent && view === "roster" ? (
          <RecentChanges
            activity={activity.data ?? []}
            isLoading={activity.isLoading}
            isError={activity.isError}
            nameOf={nameOf}
            shiftLabel={(code) => shifts.get(code)?.label ?? code}
            kindLabel={(code) =>
              catalogue.data?.absenceKinds.find((k) => k.code === code)?.label ??
              code.replace(/_/g, " ").toLowerCase()
            }
            onClose={() => setShowRecent(false)}
          />
        ) : null}

        {/* Above the card, not in it: this answers a question about the reader,
            so the answer must not change when they click to another view. Not
            for a manager: they hold no rota, so it could only ever say
            "nothing rostered". */}
        {isManager ? null : (
          <NextRotation
            mine={upcoming.data?.assignments ?? []}
            shifts={shifts}
            tz={tz}
            horizonDays={NEXT_ROTATION_HORIZON_DAYS}
            isLoading={upcoming.isLoading}
          />
        )}

        <div className="card" id="ts-panel" role="tabpanel" aria-labelledby={`ts-tab-${view}`}>
          {assignments.isError ? (
            <QueryErrorState message="Could not load the rota." error={assignments.error} />
          ) : busy ? (
            <div className="offnone">Loading the rota…</div>
          ) : view === "today" ? (
            <DayLadder
              day={anchor}
              tz={tz}
              zoneLabel={zoneAbbreviation(tz)}
              lanes={lanes}
              shifts={shifts}
              zones={zones}
              absences={absences.data?.absences ?? []}
              absenceKinds={catalogue.data?.absenceKinds ?? []}
              teamDefaultShift={teamDefaultShift}
              {...scopeControls}
            />
          ) : view === "week" ? (
            <WeekTable
              weekStart={weekStart}
              assignments={rows}
              shifts={shifts}
              absences={absences.data?.absences ?? []}
              absenceKinds={catalogue.data?.absenceKinds ?? []}
              {...scopeControls}
            />
          ) : view === "roster" ? (
            <MonthRoster
              zoneCodeFor={zoneCodeFor}
              // In the roster's own head rather than the page toolbar: it is a
              // roster-only control, and beside the four tabs it pushed the
              // date controls onto a second line on this one view.
              actions={
                canEditRota ? (
                  <button
                    className={`btn sm${showRecent ? " primary" : ""}`}
                    aria-pressed={showRecent}
                    onClick={() => setShowRecent((v) => !v)}
                    title="What has changed on your teams' rota and leave"
                  >
                    Recent changes
                  </button>
                ) : undefined
              }
              selectedIso={toIsoDate(anchor)}
              focusRequest={focusRequest}
              meEmail={user?.email}
              leadTeams={leadTeams.data ?? []}
              teamMembers={teamMembers}
              teamDefaultShift={teamDefaultShift}
              editedCells={editedCells}
              editing={editing}
              onEditCell={editCell}
              changedCells={editing ? changedCells : undefined}
              month={rosterStart}
              from={rosterStart}
              to={rosterEnd}
              span={rosterSpan}
              onSpanChange={setRosterSpan}
              assignments={rows}
              absences={absences.data?.absences ?? []}
              shifts={shifts}
              absenceKinds={catalogue.data?.absenceKinds ?? []}
              {...scopeControls}
            />
          ) : (
            <MyWeekStrip
              weekStart={weekStart}
              mine={mine.data?.assignments ?? []}
              everyone={rows}
              shifts={shifts}
              tz={tz}
              myAbsences={mineAbsences.data?.absences ?? []}
              absenceKinds={catalogue.data?.absenceKinds ?? []}
              // A day card opens that day in "Who is on today".
              onShowDay={(iso) => {
                const [y, m, d] = iso.split("-").map(Number);
                setAnchor(new Date(y, m - 1, d));
                setTab("today");
              }}
            />
          )}
        </div>
      </div>

      {/* Outside the card on purpose: the card is the page's one scroller, and
          anything positioned inside it is clipped at its edge. */}
      {picker ? (
        <CellPicker
          // Keyed on the cell, so picking another one starts a fresh range
          // rather than inheriting the last cell's end date.
          key={`${picker.userId}|${picker.rotaDate}|${picker.zoneCode ?? ""}`}
          target={picker}
          shifts={pickerShifts}
          // A tag being added or deleted counts too: the shared catalogue is
          // mid-change, so a second click would send a second request.
          busy={
            applyRange.isPending ||
            applyAbsence.isPending ||
            deleteAbsence.isPending ||
            createKind.isPending ||
            deleteKind.isPending
          }
          awayKinds={awayKinds}
          allKinds={catalogue.data?.absenceKinds}
          moveKinds={moveKinds}
          onApply={applyToCell}
          onMarkAway={markAway}
          onRemoveAbsence={removeAbsence}
          onCreateKind={addKind}
          onDeleteKind={removeKind}
          onClear={clearCell}
          onClose={() => setPicker(null)}
        />
      ) : null}
    </div>
    </TeamColourProvider>
  );
}

export type { ScheduleAssignment };
