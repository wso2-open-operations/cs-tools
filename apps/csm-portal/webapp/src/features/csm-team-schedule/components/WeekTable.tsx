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
import type { ScheduleAssignment, ScheduleShift } from "../types";
import { addDays, groupBy, initialsOf, shortDayName, toIsoDate } from "../utils/rota";
import { teamColour } from "../utils/rotaHues";

interface WeekTableProps {
  weekStart: Date;
  assignments: ScheduleAssignment[];
  shifts: Map<string, ScheduleShift>;
  /** The page's own group and team state, rendered here as well as in the
   *  toolbar -- one control in two places, the way the prototype does it, not
   *  a second copy with its own mind. See MonthRoster for the same pair. */
  family: "CRE" | "SRE";
  onFamilyChange: (family: "CRE" | "SRE") => void;
  teamKey: string;
  onTeamKeyChange: (teamKey: string) => void;
  teams: string[];
  /** CRE and SRE in the order they should read -- the reader's own group
   *  first, because the first of a pair reads as the default. */
  families: readonly ("CRE" | "SRE")[];
}

/**
 * A week cell, as teams rather than as a list of names.
 *
 * Seven columns of names is a wall of text -- the same complaint the day view
 * had, and answered the same way: the teams are what a reader scans for, and
 * the names are what they want once they have found the team. So the cell
 * lists the teams with a count, and the names arrive on hover.
 *
 * A popover rather than the day view's side-by-side pane, because a week cell
 * is one of seven columns and has no room beside it. It is anchored to the
 * cell, which is why `.tw td` is positioned.
 */
function TeamCell({ rows }: { rows: ScheduleAssignment[] }): JSX.Element {
  const byTeam = useMemo(
    () => [...groupBy(rows, (r) => r.teamKey).entries()].sort((a, b) => a[0].localeCompare(b[0])),
    [rows],
  );
  const [open, setOpen] = useState<string | null>(null);
  const shown = byTeam.find(([team]) => team === open);

  return (
    <div className="teamcell" onMouseLeave={() => setOpen(null)}>
      <div className="teamlist">
        {byTeam.map(([team, teamRows]) => (
          <span
            key={team}
            className={`tl${team === open ? " on" : ""}`}
            tabIndex={0}
            role="button"
            aria-expanded={team === open}
            onMouseEnter={() => setOpen(team)}
            onFocus={() => setOpen(team)}
            onBlur={() => setOpen(null)}
            onClick={() => setOpen((t) => (t === team ? null : team))}
          >
            <i style={{ background: teamColour(team) }} />
            <span className="tn">{team}</span>
            <b>{teamRows.length}</b>
          </span>
        ))}
      </div>

      {shown ? (
        <div className="tlpop" role="group" aria-label={`${shown[0]} engineers`}>
          <div className="tlph">
            <i style={{ background: teamColour(shown[0]) }} />
            {shown[0]}
            <b>{shown[1].length}</b>
          </div>
          {shown[1].map((a) => (
            <span className="nm" key={a.id}>
              <span className="av" style={{ background: teamColour(a.teamKey) }}>
                {initialsOf(a.engineer.name)}
              </span>
              <span className="who">{a.engineer.name}</span>
              {a.tier ? <i className="tier-t">{a.tier}</i> : null}
              {a.isOnCall ? <i className="tier-t oc-t">OC</i> : null}
            </span>
          ))}
        </div>
      ) : null}
    </div>
  );
}

/**
 * How many engineers a cell can simply list before the names stop being
 * readable and start being a wall. Six, because that is what the column fitted
 * before -- so anything that already read fine is left alone.
 */
const NAMES_FIT = 6;

/**
 * Should this cell be grouped by team instead of listed by name?
 *
 * Grouping earns its place only when it actually collapses something. Two
 * cases where it does not, and both are real here:
 *
 *   Americas night cover -- twelve engineers, all one team. The list would be
 *   a single row hiding twelve names behind a hover, which is strictly worse
 *   than reading them.
 *
 *   Evening 6-9pm -- seven engineers, one from each ABT. Seven rows each
 *   hiding exactly one name: the same information, one hop further away.
 *
 * So the test is not "how many people" but "how much does grouping save": at
 * least two names per team row, or it is not worth the hover.
 */
function groupsUsefully(rows: ScheduleAssignment[]): boolean {
  if (rows.length <= NAMES_FIT) return false;
  const teams = new Set(rows.map((r) => r.teamKey)).size;
  return teams > 1 && rows.length / teams >= 2;
}

/** Minutes past the authoring midnight as HH:MM, wrapping past 24h. */
function fmtMinute(m: number): string {
  return `${String(Math.floor(m / 60) % 24).padStart(2, "0")}:${String(m % 60).padStart(2, "0")}`;
}

/**
 * The week as rotations down the side and days across the top.
 *
 * The time column carries each rotation's window once. Repeating it against
 * every name -- which the first draft of the prototype did -- tells the reader
 * nothing they have not already read on the left.
 */
export default function WeekTable({
  weekStart,
  assignments,
  shifts,
  family,
  onFamilyChange,
  teamKey,
  onTeamKeyChange,
  teams,
  families,
}: WeekTableProps): JSX.Element {
  const days = useMemo(() => Array.from({ length: 7 }, (_, i) => addDays(weekStart, i)), [weekStart]);

  const rows = useMemo(() => {
    const byShift = groupBy(assignments, (a) => a.shiftCode);
    return [...byShift.entries()]
      .map(([code, list]) => ({ code, shift: shifts.get(code), list }))
      .sort((a, b) => (a.shift?.sortOrder ?? 999) - (b.shift?.sortOrder ?? 999));
  }, [assignments, shifts]);

  const todayIso = toIsoDate(new Date());

  /** Distinct people on the rota this week, not rows: one engineer covering
   *  three rotations is one engineer. */
  const headcount = useMemo(
    () => new Set(assignments.map((a) => a.engineer.userId)).size,
    [assignments],
  );

  const head = (
    <div className="card-head">
      <div className="seg teamseg" role="tablist" aria-label="Show CRE or SRE">
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

      <h2>
        This week <span className="count">{headcount}</span>
      </h2>

      <div className="tools">
        <span className="rng">
          {shortDayName(weekStart)} {weekStart.getDate()} – {shortDayName(addDays(weekStart, 6))}{" "}
          {addDays(weekStart, 6).getDate()}
        </span>
        <select
          className="teampick"
          aria-label="Show one team"
          value={teamKey}
          onChange={(e) => onTeamKeyChange(e.target.value)}
        >
          <option value="">All teams</option>
          {teams.map((t) => (
            <option key={t} value={t}>
              {t.charAt(0).toUpperCase() + t.slice(1)}
            </option>
          ))}
        </select>
      </div>
    </div>
  );

  if (rows.length === 0) {
    return (
      <>
        {head}
        <div className="offnone">Nobody is on the rota this week.</div>
      </>
    );
  }

  return (
    <>
      {head}
      <div className="twwrap weekwrap">
      <table className="tw">
        <thead>
          <tr>
            <th className="lab">Time</th>
            {days.map((d) => {
              const iso = toIsoDate(d);
              const weekend = d.getDay() === 0 || d.getDay() === 6;
              return (
                <th key={iso} className={`${weekend ? "wknd" : ""} ${iso === todayIso ? "today" : ""}`}>
                  {shortDayName(d)} <small>{d.getDate()}</small>
                </th>
              );
            })}
          </tr>
        </thead>
        <tbody>
          {rows.map(({ code, shift, list }) => {
            const byDay = groupBy(list, (a) => a.rotaDate);
            const token = shift?.colourToken ?? "";
            return (
              <tr
                key={code}
                className="hued"
                style={{ ["--rc" as string]: `var(--${token.toLowerCase()}-fg, var(--faint))` }}
              >
                <th className="lab">
                  <span className={`chip sm ${token}`}>
                    {shift ? `${fmtMinute(shift.startMinute)} – ${fmtMinute(shift.endMinute)}` : code}
                  </span>
                  <small>{shift?.label ?? code}</small>
                </th>
                {days.map((d) => {
                  const iso = toIsoDate(d);
                  const cell = byDay.get(iso) ?? [];
                  const weekend = d.getDay() === 0 || d.getDay() === 6;
                  return (
                    <td
                      key={iso}
                      className={`${weekend ? "wknd" : ""} ${iso === todayIso ? "today" : ""}`}
                    >
                      {cell.length === 0 ? (
                        <span className="none">—</span>
                      ) : groupsUsefully(cell) ? (
                        /* Regular hours: most of the team, spread across every
                           ABT. Teams, with the names on hover. */
                        <TeamCell rows={cell} />
                      ) : (
                        /* Few enough to simply read. A team list here would be
                           an extra hop to reach three names that already fit. */
                        cell.map((a) => (
                          /* Name, and only the name.
                             The team is the avatar's colour and the row's own
                             tooltip, so spelling it out a third time was the
                             widest thing in a narrow column. "OC" was worse
                             than redundant -- the row it sits in is already
                             labelled "Morning 6-9am on-call", so the badge
                             restated the row heading against every name in it.
                             The tier stays: L1/L2/L3 is the one thing here
                             that nothing else says. */
                          <div className="nm" key={a.id} title={`${a.engineer.name} · ${a.teamKey}`}>
                            <span className="av" style={{ background: teamColour(a.teamKey) }}>
                              {initialsOf(a.engineer.name)}
                            </span>
                            <span className="who">{a.engineer.name}</span>
                            {a.tier ? <span className="tier-t">{a.tier}</span> : null}
                          </div>
                        ))
                      )}
                    </td>
                  );
                })}
              </tr>
            );
          })}
        </tbody>
      </table>
      </div>
    </>
  );
}
