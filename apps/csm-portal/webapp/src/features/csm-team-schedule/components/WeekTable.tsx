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

import { useEffect, useMemo, useState, type JSX } from "react";
import RotaPicker, { type RotaOption } from "./RotaPicker";
import type {
  ScheduleAbsence,
  ScheduleAbsenceKind,
  ScheduleAssignment,
  ScheduleShift,
  ScheduleTier, RotaFamily } from "../types";
import {
  addDays,
  escalationGrid,
  escalationTurnOf,
  groupBy,
  initialsOf,
  isTierlessEscalation,
  shortDayName,
  standingWindowKey,
  toIsoDate, rotaZoneName } from "../utils/rota";
import { accentOf } from "../utils/rotaHues";
import { useTeamColour, useTeamName } from "../utils/teamColourContext";

interface WeekTableProps {
  weekStart: Date;
  assignments: ScheduleAssignment[];
  shifts: Map<string, ScheduleShift>;
  /** The page's own group and team state, rendered here as well as in the
   *  toolbar -- one control in two places, the way the prototype does it, not
   *  a second copy with its own mind. See MonthRoster for the same pair. */
  family: RotaFamily;
  onFamilyChange: (family: RotaFamily) => void;
  teamKey: string;
  onTeamKeyChange: (teamKey: string) => void;
  teams: string[];
  /** CRE and SRE in the order they should read -- the reader's own group
   *  first, because the first of a pair reads as the default. */
  families: readonly RotaFamily[];
  /** The rotas of the family on screen, the one shown, and the change; the
   *  picker appears only when there is more than one. */
  rotas?: readonly RotaOption[];
  rotaCode?: string;
  onRotaChange?: (code: string) => void;
  /** Absences over the week, for the leave row at the foot of the table. */
  absences?: ScheduleAbsence[];
  absenceKinds?: ScheduleAbsenceKind[];
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
  const teamColourOf = useTeamColour();
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
            <i style={{ background: teamColourOf(team) }} />
            <span className="tn">{team}</span>
            <b>{teamRows.length}</b>
          </span>
        ))}
      </div>

      {shown ? (
        <div className="tlpop" role="group" aria-label={`${shown[0]} engineers`}>
          <div className="tlph">
            <i style={{ background: teamColourOf(shown[0]) }} />
            {shown[0]}
            <b>{shown[1].length}</b>
          </div>
          {shown[1].map((a) => (
            <span className="nm" key={a.id}>
              <span className="av" style={{ background: teamColourOf(a.teamKey) }}>
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
  rotas,
  rotaCode,
  onRotaChange,
  absences = [],
  absenceKinds = [],
}: WeekTableProps): JSX.Element {
  const teamColourOf = useTeamColour();
  const teamNameOf = useTeamName();
  const days = useMemo(() => Array.from({ length: 7 }, (_, i) => addDays(weekStart, i)), [weekStart]);

  const rows = useMemo(() => {
    type Row = {
      code: string;
      shift?: ScheduleShift;
      list: ScheduleAssignment[];
      /** Set for an escalation row, which is named for its zone and tier rather
       *  than for the window underneath it. */
      label?: string;
      token?: string;
      sort: number;
      tiered?: boolean;
    };
    const TIERS: ScheduleTier[] = ["L1", "L2", "L3"];
    const out: Row[] = [];

    // SRE escalation, as a row per zone and tier: "TZ1 L2 support" is the
    // question a reader asks, and it had no row of its own -- L2 sat inside
    // "TZ1 escalation", L3 nowhere. Every weekday zone gets L1, L2 and L3,
    // empty rows included, so a tier nobody holds reads as a gap. Weekend
    // windows keep their own hours and appear only when somebody is on them.
    const escKey = (a: ScheduleAssignment): string | null => {
      const turn = escalationTurnOf(a, shifts);
      return turn ? `${turn.weekend ? "we" : "wd"}|${turn.zoneCode}|${turn.tier}` : null;
    };
    const familyShifts = [...shifts.values()].filter((sh) => sh.family === family);
    const weekdayIso = toIsoDate(days.find((d) => d.getDay() !== 0 && d.getDay() !== 6) ?? days[0]);
    const weekendIso = toIsoDate(days.find((d) => d.getDay() === 0 || d.getDay() === 6) ?? days[0]);
    const windowFor = new Map<string, ScheduleShift | undefined>();
    for (const [scope, iso] of [["wd", weekdayIso], ["we", weekendIso]] as const) {
      escalationGrid(familyShifts, iso).forEach((row, zi) =>
        row.tiers.forEach(({ tier, shift }, ti) => {
          const key = `${scope}|${row.zoneCode}|${tier}`;
          windowFor.set(key, shift);
          if (scope === "wd" && shift) {
            out.push({
              code: `esc:${key}`,
              shift,
              list: [],
              label: `${row.label} ${tier} support`,
              token: tier,
              sort: 110 + zi * 10 + ti,
              tiered: true,
            });
          }
        }),
      );
    }
    const escRows = new Map(out.map((r) => [r.code, r]));

    /** A zone's own regular-hours window, on a weekday. */
    const regularOf = (zone: string) =>
      familyShifts.find((sh) => sh.zoneCode === zone && !sh.isEscalation && !sh.isRotation && sh.dayScope !== "WEEKEND");

    const rest: ScheduleAssignment[] = [];
    for (const a of assignments) {
      // No tier on a zone's escalation window is what "works this zone, not on
      // the escalation rota" looks like -- the zone's regular hours -- so it
      // is read as that, not as a turn with its tier missing.
      const sh = shifts.get(a.shiftCode);
      const zone = a.zoneCode ?? sh?.zoneCode;
      if (zone && isTierlessEscalation(a, shifts) && sh?.dayScope !== "WEEKEND") {
        const reg = regularOf(zone);
        if (reg) {
          rest.push({ ...a, shiftCode: reg.code });
          continue;
        }
      }
      const key = escKey(a);
      if (!key) {
        rest.push(a);
        continue;
      }
      let row = escRows.get(`esc:${key}`);
      if (!row) {
        const [scope, zone, tier] = key.split("|");
        row = {
          code: `esc:${key}`,
          shift: windowFor.get(key) ?? shifts.get(a.shiftCode),
          list: [],
          // A weekend window names its own crew ("Weekend TZ1 + TZ2"), since
          // TZ1 and TZ2 are one crew at the weekend.
          label:
            scope === "we"
              ? `${windowFor.get(key)?.label ?? `Weekend ${zone}`} ${tier} support`
              : `${rotaZoneName(familyShifts, zone)} ${tier} support`,
          token: tier,
          sort: (scope === "we" ? 200 : 110) + TIERS.indexOf(tier as ScheduleTier),
          tiered: true,
        };
        escRows.set(row.code, row);
        out.push(row);
      }
      row.list.push(a);
    }

    // Everything else, by window. Windows that are the same working day under
    // different team names share a row -- see standingWindowKey. This table
    // already lists regular hours by team, so the India region shift arrives
    // as one more team rather than as a row of its own saying the same
    // nine-to-five over again.
    const byWindow = groupBy(rest, (a) => standingWindowKey(shifts.get(a.shiftCode), a.shiftCode));
    for (const [, list] of byWindow.entries()) {
      // The window most of these people are on names the row, so it keeps the
      // label and colour a reader already knows it by.
      const counts = groupBy(list, (a) => a.shiftCode);
      const lead = [...counts.entries()].sort((a, b) => b[1].length - a[1].length)[0][0];
      const shift = shifts.get(lead);
      // An escalation turn with no tier recorded -- marked before the picker
      // asked for one -- has no tier row to go in. It keeps a row of its own,
      // saying what is missing, so a lead can see it and pick the tier.
      const untiered = shift?.isEscalation && shift.zoneCode;
      out.push({
        code: lead,
        shift,
        list,
        label: untiered ? `${shift.label} · tier not set` : undefined,
        sort: untiered ? 199 : (shift?.sortOrder ?? 999),
      });
    }

    // The empty tier rows only belong to a week that has an SRE rota at all.
    if (!out.some((r) => r.list.length > 0)) return [];
    return out.sort((a, b) => a.sort - b.sort);
  }, [assignments, shifts, family, days]);

  const todayIso = toIsoDate(new Date());

  // The clock, for "which rotation is on right now". In state rather than read
  // during render, and ticked each minute so a handover moves the mark
  // without a reload.
  const [nowMs, setNowMs] = useState(() => Date.now());
  useEffect(() => {
    const t = window.setInterval(() => setNowMs(Date.now()), 60_000);
    return () => window.clearInterval(t);
  }, []);
  const isOnNow = (a: ScheduleAssignment): boolean =>
    new Date(a.startsAt).getTime() <= nowMs && nowMs < new Date(a.endsAt).getTime();

  /** Who is on leave each day, for the row at the foot of the week. Leave
   *  only -- allocations are work, and have their own place in the day view --
   *  and weekdays only, because leave is not taken on a weekend. */
  const leaveByDay = useMemo(() => {
    const kindByCode = new Map(absenceKinds.map((k) => [k.code, k]));
    const out = new Map<string, { ab: ScheduleAbsence; kind: ScheduleAbsenceKind }[]>();
    for (const d of days) {
      if (d.getDay() === 0 || d.getDay() === 6) continue;
      const iso = toIsoDate(d);
      const list = absences
        .filter((ab) => ab.startsOn <= iso && (!ab.endsOn || iso <= ab.endsOn))
        .map((ab) => ({ ab, kind: kindByCode.get(ab.kindCode) }))
        .filter((x): x is { ab: ScheduleAbsence; kind: ScheduleAbsenceKind } => x.kind?.bucket === "LEAVE")
        // By name: every kind of leave is one list here (see the row below).
        .sort((a, b) => a.ab.engineer.name.localeCompare(b.ab.engineer.name));
      out.set(iso, list);
    }
    return out;
  }, [absences, absenceKinds, days]);
  const anyLeave = [...leaveByDay.values()].some((l) => l.length > 0);

  /** Distinct people on the rota this week, not rows: one engineer covering
   *  three rotations is one engineer. */
  const headcount = useMemo(
    () => new Set(assignments.map((a) => a.engineer.userId)).size,
    [assignments],
  );

  const head = (
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
              {teamNameOf(t)}
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
          {rows.map(({ code, shift, list, label, token: rowToken, tiered }) => {
            const byDay = groupBy(list, (a) => a.rotaDate);
            const token = rowToken ?? shift?.colourToken ?? "";
            // The rotation running right now, found by its stored instants --
            // so a night crew rostered yesterday still lights up after
            // midnight -- and marked on both its row and its cell.
            const liveIso = list.find(isOnNow)?.rotaDate;
            return (
              <tr
                key={code}
                className={`hued${liveIso ? " nowband" : ""}`}
                style={{ ["--rc" as string]: accentOf(token) }}
              >
                <th className="lab">
                  <span className={`chip sm ${token}`}>
                    {shift ? `${fmtMinute(shift.startMinute)} – ${fmtMinute(shift.endMinute)}` : code}
                  </span>
                  {liveIso ? <span className="nowpill">Now</span> : null}
                  <small>{label ?? shift?.label ?? code}</small>
                </th>
                {days.map((d) => {
                  const iso = toIsoDate(d);
                  const cell = byDay.get(iso) ?? [];
                  const weekend = d.getDay() === 0 || d.getDay() === 6;
                  return (
                    <td
                      key={iso}
                      className={`${weekend ? "wknd" : ""} ${iso === todayIso ? "today" : ""}${
                        iso === liveIso ? " nowcell" : ""
                      }`}
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
                          <div className="nm" key={a.id} title={`${a.engineer.name} · ${teamNameOf(a.teamKey)}`}>
                            <span className="av" style={{ background: teamColourOf(a.teamKey) }}>
                              {initialsOf(a.engineer.name)}
                            </span>
                            <span className="who">{a.engineer.name}</span>
                            {/* A tier row already names the tier. */}
                            {a.tier && !tiered ? <span className="tier-t">{a.tier}</span> : null}
                          </div>
                        ))
                      )}
                    </td>
                  );
                })}
              </tr>
            );
          })}

          {/* Who is away, at the foot of the week, so a lead reading down a day
              sees its cover and its gaps in one column. */}
          {anyLeave ? (
            <tr className="leaverow">
              <th className="lab">
                <span className="chip sm AL">Leave</span>
              </th>
              {days.map((d) => {
                const iso = toIsoDate(d);
                const weekend = d.getDay() === 0 || d.getDay() === 6;
                const list = leaveByDay.get(iso) ?? [];
                return (
                  <td key={iso} className={`${weekend ? "wknd" : ""} ${iso === todayIso ? "today" : ""}`}>
                    {list.length === 0 ? (
                      <span className="none">—</span>
                    ) : (
                      // Who is away, not why: the kind of leave -- maternity,
                      // sick -- is the person's own business on a view the
                      // whole team reads, and it changes nothing about the gap.
                      // The Month roster still shows it to those who need it.
                      list.map(({ ab }) => (
                        <div className="nm" key={ab.id} title={`${ab.engineer.name} · ${teamNameOf(ab.teamKey)}`}>
                          <span className="av" style={{ background: teamColourOf(ab.teamKey) }}>
                            {initialsOf(ab.engineer.name)}
                          </span>
                          <span className="who">{ab.engineer.name}</span>
                        </div>
                      ))
                    )}
                  </td>
                );
              })}
            </tr>
          ) : null}
        </tbody>
      </table>
      </div>
    </>
  );
}
