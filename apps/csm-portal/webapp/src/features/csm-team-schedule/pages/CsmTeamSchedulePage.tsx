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
import { useCurrentUser } from "@context/current-user/CurrentUserContext";
import {
  useScheduleAbsences,
  useScheduleAssignments,
  useScheduleCatalogue,
} from "../api/useTeamSchedule";
import DayLadder, { type LadderLane } from "../components/DayLadder";
import MonthRoster from "../components/MonthRoster";
import MyWeekStrip from "../components/MyWeekStrip";
import NextRotation from "../components/NextRotation";
import WeekTable from "../components/WeekTable";
import type { ScheduleAssignment } from "../types";
import { resolveDisplayTimeZone } from "@utils/dateTime";
import { addDays, mondayOf, shiftsByCode, toIsoDate, zoneAbbreviation } from "../utils/rota";
import { zoneColour } from "../utils/rotaHues";
import { SCHEDULE_THEME_VARS } from "../utils/useScheduleTheme";
import "../teamSchedule.css";

type ViewTab = "mine" | "today" | "week" | "roster";
type Family = "CRE" | "SRE";

/** Teams per family, in rota order. Mirrors CSM_TEAM_REGISTRY. */
const TEAMS: Record<Family, string[]> = {
  CRE: ["castor", "draco", "vega", "sirius", "atlas", "phoenix", "rigel", "americas", "migration"],
  SRE: ["apollo", "artemis"],
};

/** How far ahead to look for the reader's next rotation. Eight weeks covers
 *  every rotation in the cycle without asking the API for a year of rows. */
const NEXT_ROTATION_HORIZON_DAYS = 56;

const TITLE: Record<ViewTab, string> = {
  mine: "My week",
  today: "Who is working today",
  week: "Who is working this week",
  roster: "Month roster",
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

const fmtShort = (d: Date): string =>
  d.toLocaleDateString(undefined, { day: "numeric", month: "short" });

/**
 * Team Schedule: who from CRE and SRE is working, when, and in which
 * escalation tier.
 */
export default function CsmTeamSchedulePage(): JSX.Element {
  const [tab, setTab] = useState<ViewTab>("today");
  /** null until the reader picks one, so their own group can be the default
   *  once the profile arrives. Derived rather than corrected in an effect: an
   *  SRE engineer must never render a frame on CRE, because the tab gating
   *  would show them three disabled tabs on arrival. */
  const [familyChoice, setFamilyChoice] = useState<Family | null>(null);
  const [teamKey, setTeamKey] = useState<string>("");
  const [anchor, setAnchor] = useState<Date>(() => new Date());

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

  /** The reader's own group, from their CSM profile. Absent for anyone who
   *  belongs to no team -- a manager -- which is why it is optional rather
   *  than defaulting to CRE. */
  const myFamily: Family | undefined = useMemo(() => {
    const f = user?.team?.family?.toUpperCase();
    if (!f) return undefined;
    return f.startsWith("SRE") ? "SRE" : "CRE";
  }, [user?.team?.family]);

  /** The group on screen: the reader's own until they choose otherwise.
   *  CRE only as a last resort, for a manager who belongs to neither. */
  const family: Family = familyChoice ?? myFamily ?? "CRE";

  /** Their own group first in the segment. An SRE engineer reads "SRE | CRE",
   *  because the first thing in a pair reads as the default, and theirs is. */
  const families: Family[] =
    myFamily === "SRE" ? ["SRE", "CRE"] : ["CRE", "SRE"];

  /** Nobody's group: a manager, who belongs to no team at all. */
  const isManager = myFamily === undefined;

  /** Whether the group on screen is the reader's own. */
  const ownGroup = !isManager && myFamily === family;

  /**
   * Which views apply to this reader, on this group.
   *
   *   own group     all four -- their rota, their team, their month
   *   other group   Today only. It answers "who is covering right now", which
   *                 a CRE engineer escalating to SRE needs. This week and the
   *                 roster are planning views for a rota they are not part of.
   *   a manager     everything but My week, on both groups. The one thing
   *                 that genuinely has nothing to show them is their own rota,
   *                 because they hold none. Who is covering -- today, across
   *                 the week, across the month -- is their question for CRE
   *                 and SRE alike, so the roster is theirs to read too.
   */
  const appliesToView = (t: ViewTab): boolean => {
    if (isManager) return t !== "mine";
    return ownGroup || t === "today";
  };

  /**
   * Which tabs are on the strip at all.
   *
   * Disabling and removing answer different situations. An engineer looking at
   * the other group sees the tab greyed, because toggling back brings it
   * straight back -- removing it would make the strip change shape under them.
   * A manager has no such toggle: they hold no rota on either group, so My week
   * can never apply to them, and a permanently dead tab is just something to
   * wonder about. It is not offered.
   */
  const visibleTabs: ViewTab[] = (["mine", "today", "week", "roster"] as ViewTab[]).filter(
    (t) => !(isManager && t === "mine"),
  );

  /** The tab actually being shown.
   *
   *  Derived, not corrected after the fact: switching group while on My week
   *  has to land somewhere the same render, or the reader sees one frame of an
   *  empty strip before it rights itself. Their choice of tab is remembered,
   *  so switching back to their own group returns them to My week. */
  const view: ViewTab = appliesToView(tab) ? tab : "today";

  const weekStart = useMemo(() => mondayOf(anchor), [anchor]);
  /** The group/team controls the cards render in their own heads. It is the
   *  page's state either way -- the toolbar and the card head are two views of
   *  one control, which is why they can never disagree. */
  const scopeControls = {
    family,
    onFamilyChange: (f: Family) => {
      setFamilyChoice(f);
      setTeamKey("");
    },
    teamKey,
    onTeamKeyChange: setTeamKey,
    teams: TEAMS[family],
    families,
  };

  const dayView = view === "today";
  const rosterView = view === "roster";
  const monthStart = useMemo(
    () => new Date(anchor.getFullYear(), anchor.getMonth(), 1),
    [anchor],
  );
  const monthEnd = useMemo(
    () => new Date(anchor.getFullYear(), anchor.getMonth() + 1, 0),
    [anchor],
  );
  const from = rosterView
    ? toIsoDate(monthStart)
    : dayView
      ? toIsoDate(anchor)
      : toIsoDate(weekStart);
  const to = rosterView
    ? toIsoDate(monthEnd)
    : dayView
      ? toIsoDate(anchor)
      : toIsoDate(addDays(weekStart, 6));
  const teamKeys = teamKey ? [teamKey] : undefined;

  const assignments = useScheduleAssignments({
    from,
    to,
    family,
    teamKeys,
    // Only the day view needs the crew that started last night and is still
    // working this morning; the week groups by rota date, so it does not.
    includeOvernight: dayView,
  });

  // My week is the signed-in engineer's own rota, found by the email the
  // portal knows them by.
  const mine = useScheduleAssignments(
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
  const absenceTeamKeys = teamKeys ?? TEAMS[family];
  const absences = useScheduleAbsences(
    { from, to, teamKeys: absenceTeamKeys },
    dayView || rosterView,
  );

  const shifts = useMemo(() => shiftsByCode(catalogue.data?.shifts ?? []), [catalogue.data?.shifts]);
  const zones = catalogue.data?.zones ?? [];
  const rows = assignments.data?.assignments ?? [];

  // SRE works in time zones, so its day is a lane per zone. CRE runs on one
  // clock, so it gets one lane.
  /** The zones SRE actually staffs on the day being viewed.
   *
   *  The catalogue's weekend windows are TZ1 and TZ2 only -- there is no
   *  weekend TZ3 for anyone to be rostered into -- so a weekend earns two
   *  lanes and a weekday three. Read from the shifts rather than written down
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
      if (sh.family !== "SRE" || !sh.zoneCode) continue;
      if (sh.dayScope === scope || sh.dayScope === "ANY") staffed.add(sh.zoneCode);
    }
    const kept = zones.filter((z) => staffed.has(z.code));
    return kept.length > 0 ? kept : zones;
  }, [anchor, shifts, zones]);

  const lanes: LadderLane[] = useMemo(() => {
    if (family === "CRE") {
      return [
        { name: "Rotations", sub: "on-call, the night and regular hours", colour: "var(--muted)", assignments: rows },
      ];
    }
    return zonesOnDay.map((z) => ({
      name: z.code,
      sub: z.label,
      colour: zoneColour(z.code),
      // The same resolution the month roster uses: an assignment's own zone,
      // else its window's, so a zoned window stored without one still lands.
      assignments: rows.filter((a) => (a.zoneCode ?? shifts.get(a.shiftCode)?.zoneCode) === z.code),
      // Escalation on one side, everyone else in the zone on the other.
      layout: "zone" as const,
    }));
  }, [family, rows, shifts, zonesOnDay]);

  if (catalogue.isError) {
    return <QueryErrorState message="Could not load the schedule catalogue." error={catalogue.error} />;
  }

  const busy = catalogue.isLoading || (view === "mine" ? mine.isLoading : assignments.isLoading);

  return (
    <div className="csm-ts" style={SCHEDULE_THEME_VARS}>
      <div className="wrap">
        <h1>Team Schedule</h1>

        <div className="tabrow">
          <div className="tabs" role="tablist">
            {visibleTabs.map((t) => {
              const off = !appliesToView(t);
              return (
              <button
                key={t}
                id={`ts-tab-${t}`}
                className={`tab ${view === t ? "on" : ""}${off ? " off" : ""}`}
                role="tab"
                // The CSS class said which tab was active; nothing did for a
                // screen reader, which read four equal buttons and a panel
                // belonging to none of them.
                aria-selected={view === t}
                aria-controls="ts-panel"
                disabled={off}
                aria-disabled={off}
                title={
                  !off
                    ? undefined
                    : isManager
                      ? "You hold no rota, so this view has nothing of yours to show."
                      : `You are on ${myFamily}. Only “Who is working today” applies to ${family}.`
                }
                onClick={() => setTab(t)}
              >
                <span className="tl">{TITLE[t]}</span>
                <span className="tsub">
                  {t === "today"
                    ? fmtShort(anchor)
                    : t === "roster"
                      ? anchor.toLocaleDateString(undefined, { month: "long", year: "numeric" })
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
                        ? anchor.toLocaleDateString(undefined, { month: "long", year: "numeric" })
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
              <button className="btn" onClick={() => setAnchor(new Date())}>
                Today
              </button>
            </div>
          </div>
        </div>

        {/* Above the card, not in it: this answers a question about the reader,
            so the answer must not change when they click to another view. */}
        <NextRotation
          mine={upcoming.data?.assignments ?? []}
          shifts={shifts}
          tz={tz}
          horizonDays={NEXT_ROTATION_HORIZON_DAYS}
          isLoading={upcoming.isLoading}
        />

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
              {...scopeControls}
            />
          ) : view === "week" ? (
            <WeekTable weekStart={weekStart} assignments={rows} shifts={shifts} {...scopeControls} />
          ) : view === "roster" ? (
            <MonthRoster
              selectedIso={toIsoDate(anchor)}
              meEmail={user?.email}
              month={monthStart}
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
            />
          )}
        </div>
      </div>
    </div>
  );
}

export type { ScheduleAssignment };
