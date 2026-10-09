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

import { useMemo, useRef, useState, type JSX } from "react";
import { Users } from "@wso2/oxygen-ui-icons-react";
import type {
  ScheduleAbsence,
  ScheduleAbsenceKind,
  ScheduleAssignment,
  ScheduleShift,
} from "../types";
import {
  addDays,
  escalationGrid,
  escalationTurnOf,
  groupBy,
  initialsOf,
  isPeerRotation,
  isTierlessEscalation,
  shortDayName,
  timeOf,
  toIsoDate,
} from "../utils/rota";
import { accentOf } from "../utils/rotaHues";
import { useTeamColour } from "../utils/teamColourContext";

interface MyWeekStripProps {
  weekStart: Date;
  /** The signed-in engineer's own rota for the week. */
  mine: ScheduleAssignment[];
  /** Everyone's rota for the week, for the day a reader opens. */
  everyone: ScheduleAssignment[];
  shifts: Map<string, ScheduleShift>;
  tz: string;
  /** The engineer's own leave and allocations over the week. Without them a
   *  day on leave, or lent to a customer, read "—" -- the same as a day with
   *  nothing on it, which is the one thing it is not. */
  myAbsences?: ScheduleAbsence[];
  absenceKinds?: ScheduleAbsenceKind[];
  /** Open a day in "Who is on today". Clicking a card does this when it
   *  is given -- the reader wants that day's full view -- and the card's own
   *  "Who's on" cue still lists the day in place. */
  onShowDay?: (iso: string) => void;
}

/** How long the cursor must rest on a day before it opens, so sweeping across
 *  the strip does not fire seven times. */
const HOVER_DELAY_MS = 110;

/**
 * My week: seven day cards, and the day the cursor rests on opens underneath
 * with everyone who is on the rota that day.
 *
 * It opens on hover and does not close on the way out -- a reader who just
 * opened a day is almost always heading down to read it, and closing it under
 * them would snatch it away mid-move. Clicking the open day closes it.
 *
 * That a card opens at all has to be visible before anyone hovers: a cue that
 * only appears on hover teaches nothing, and on a touch screen there is no
 * hover. So every card carries a "Who's on" cue at rest, and until a day is
 * opened the space the list will fill says what to do.
 */
export default function MyWeekStrip({
  weekStart,
  mine,
  everyone,
  shifts,
  tz,
  myAbsences = [],
  absenceKinds = [],
  onShowDay,
}: MyWeekStripProps): JSX.Element {
  const [openDay, setOpenDay] = useState<string | null>(null);
  const timer = useRef<number | null>(null);

  const days = useMemo(
    () => Array.from({ length: 7 }, (_, i) => addDays(weekStart, i)),
    [weekStart],
  );
  const mineByDay = useMemo(() => groupBy(mine, (a) => a.rotaDate), [mine]);
  const kindByCode = useMemo(() => new Map(absenceKinds.map((k) => [k.code, k])), [absenceKinds]);
  /** The absence covering a day, if any. An open-ended one (no end date) is a
   *  standing allocation and covers every day from its start. */
  const absenceOn = (iso: string): ScheduleAbsence | undefined =>
    myAbsences.find((ab) => ab.startsOn <= iso && (!ab.endsOn || iso <= ab.endsOn));
  const todayIso = toIsoDate(new Date());

  const hoverOpen = (iso: string): void => {
    if (openDay === iso) return;
    if (timer.current) window.clearTimeout(timer.current);
    timer.current = window.setTimeout(() => setOpenDay(iso), HOVER_DELAY_MS);
  };
  const cancelHover = (): void => {
    if (timer.current) window.clearTimeout(timer.current);
  };

  // Only the rotations this reader could be sharing: not regular hours, which
  // on a weekday is most of the team, and not the Americas night cover, which
  // is that team's own standing shift rather than a turn in the rota.
  const openRows = openDay
    ? everyone.filter(
        (a) =>
          a.rotaDate === openDay &&
          isPeerRotation(shifts.get(a.shiftCode)) &&
          // A tier-less turn on a zone's escalation window is that zone's
          // regular hours, not a rotation.
          !isTierlessEscalation(a, shifts),
      )
    : [];
  // Two different counts. "Rostered" is any day with something on it, regular
  // hours included; "on rotation" is only a turn on the rota. Counting the first
  // under the second's name told an engineer on plain regular hours all week
  // that they were on rotation five days out of seven.
  const rosteredCount = days.filter((d) => (mineByDay.get(toIsoDate(d)) ?? []).length > 0).length;
  const onRotaCount = days.filter((d) =>
    (mineByDay.get(toIsoDate(d)) ?? []).some((a) => isPeerRotation(shifts.get(a.shiftCode))),
  ).length;

  return (
    <>
      <div className="strip">
        {days.map((d) => {
          const iso = toIsoDate(d);
          const rows = mineByDay.get(iso) ?? [];
          const first = rows[0];
          const shift = first ? shifts.get(first.shiftCode) : undefined;
          const past = iso < todayIso;
          const weekend = d.getDay() === 0 || d.getDay() === 6;
          // A weekend inside a span of leave or an allocation reads as off:
          // leave is not taken on a weekend, and nobody works an engagement
          // on a Saturday.
          const absence = weekend ? undefined : absenceOn(iso);

          return (
            <div
              key={iso}
              className={[
                "dayc",
                weekend ? "wknd" : "",
                iso === todayIso ? "today" : "",
                past ? "past" : "",
                rows.length ? "rot" : "",
                openDay === iso ? "picked" : "",
              ]
                .filter(Boolean)
                .join(" ")}
              style={{ ["--rc" as string]: accentOf(shift?.colourToken ?? "lk") }}
              role="button"
              tabIndex={0}
              aria-label={
                onShowDay ? `${d.toDateString()}: open in Who is on today` : d.toDateString()
              }
              onMouseEnter={() => hoverOpen(iso)}
              onMouseLeave={cancelHover}
              onFocus={() => setOpenDay(iso)}
              // The card opens the day in "Who is on today", which is
              // where a reader clicking a day wants to go. Without that view
              // to go to, it lists the day in place as it always did.
              onClick={() => (onShowDay ? onShowDay(iso) : setOpenDay(openDay === iso ? null : iso))}
              onKeyDown={(e) => {
                if (e.target !== e.currentTarget) return;
                if (e.key === "Enter" || e.key === " ") {
                  e.preventDefault();
                  if (onShowDay) onShowDay(iso);
                  else setOpenDay(openDay === iso ? null : iso);
                }
              }}
            >
              {iso === todayIso ? <span className="daytag">Today</span> : null}
              {/* The in-place list, on its own control now the card itself
                  goes to the day view. It stops the click reaching the card. */}
              <button
                type="button"
                className="daysee"
                aria-expanded={openDay === iso}
                aria-controls="mywk-peek"
                aria-label={`${d.toDateString()}: show everyone on rotation`}
                onClick={(e) => {
                  e.stopPropagation();
                  cancelHover();
                  setOpenDay(openDay === iso ? null : iso);
                }}
              >
                <Users size={11} />
                <span className="dslabel">{openDay === iso ? "Hide" : "Who's on"}</span>
              </button>
              <span className="dw">{shortDayName(d)}</span>
              <span className="dn tn">{d.getDate()}</span>
              {absence ? (
                // Leave or an allocation wins the day, as it does on the
                // roster: someone away is not on the rota, whatever a
                // generated row says.
                <>
                  <span
                    className={`chip ${kindByCode.get(absence.kindCode)?.colourToken ?? ""}`}
                    title={[kindByCode.get(absence.kindCode)?.label, absence.allocatedTo].filter(Boolean).join(" · ")}
                  >
                    {kindByCode.get(absence.kindCode)?.shortCode ?? absence.kindCode}
                  </span>
                  <span className="tm">
                    {absence.allocatedTo ?? kindByCode.get(absence.kindCode)?.label ?? ""}
                  </span>
                </>
              ) : first && shift ? (
                <>
                  <span className={`chip ${shift.colourToken}`}>{shift.shortCode}</span>
                  <span className="tm">
                    {timeOf(first.startsAt, tz)} – {timeOf(first.endsAt, tz)}
                  </span>
                </>
              ) : (
                <span className="tm">{weekend ? "Off" : "—"}</span>
              )}
            </div>
          );
        })}
      </div>

      {openDay ? (
        <div className="peek" id="mywk-peek">
          <div className="ph">
            <b>
              {new Date(`${openDay}T00:00:00`).toLocaleDateString(undefined, {
                weekday: "long",
                day: "numeric",
                month: "long",
                year: "numeric",
              })}
            </b>
            {openDay === todayIso ? <span className="tag today">Today</span> : null}
            <span className="count">{openRows.length}</span>
            <span className="sub">on rotation</span>
            <button className="pk-x" onClick={() => setOpenDay(null)} aria-label="Close">
              ×
            </button>
          </div>
          <PeekRows rows={openRows} shifts={shifts} iso={openDay} />
        </div>
      ) : (
        <div className="peek peekhint" id="mywk-peek">
          <Users size={16} />
          <span>
            <b>See who's on rotation with you.</b>{" "}
            {onShowDay
              ? "Hover over a day above, or tap Who's on, to list everyone rostered that day. Click a day to open it in Who is on today."
              : "Hover over a day above, or tap it, to list everyone rostered that day."}
          </span>
        </div>
      )}

      <div className="wkstat">
        <span className="kv">
          <b>{rosteredCount}</b> of 7 days rostered this week · <b>{onRotaCount}</b> on rotation
        </span>
        {openDay ? (
          <span className="grp hint">
            {onShowDay
              ? "Who's on again, or ×, to close it · click a day to open it in Who is on today"
              : "Click the open day again, or ×, to close it"}
          </span>
        ) : null}
      </div>
    </>
  );
}

/** Everyone on the opened day, grouped by the rotation they are on. */
function PeekRows({
  rows,
  shifts,
  iso,
}: {
  rows: ScheduleAssignment[];
  shifts: Map<string, ScheduleShift>;
  iso: string;
}): JSX.Element {
  const teamColourOf = useTeamColour();

  /** SRE escalation as a column per zone -- TZ1, TZ2, TZ3 across, and in
   *  each, L1, L2 and L3 support down -- so "who is L2 in TZ2" is read off
   *  one column rather than hunted for across a wrapped row of cards. Every
   *  tier of every zone worked that day is listed, empty ones included, so a
   *  tier nobody holds reads as a gap. Anything else on the rota that day
   *  keeps a card per window below. */
  const { zones, rest } = useMemo(() => {
    type Group = { key: string; label: string; token: string; list: ScheduleAssignment[]; sort: number };
    const zoneCols: { zoneCode: string; label: string; tiers: Group[] }[] = [];
    const byTurn = new Map<string, Group>();
    // The zone grid of whichever zoned rota the day's turns are on -- SRE's
    // time zones, or an SME rotation's Day and Night. The page passes only the
    // reader's own rota's windows, so another rota's zones never appear here.
    const zoned = rows.map((r) => shifts.get(r.shiftCode)).find((sh) => sh?.zoneCode && sh.family !== "CRE");
    if (zoned) {
      const zonedShifts = [...shifts.values()].filter((sh) => sh.family === zoned.family);
      for (const row of escalationGrid(zonedShifts, iso)) {
        const tiers = row.tiers.map(({ tier }, ti) => {
          const g: Group = { key: `esc:${row.zoneCode}|${tier}`, label: `${tier} support`, token: tier, list: [], sort: ti };
          byTurn.set(g.key, g);
          return g;
        });
        zoneCols.push({ zoneCode: row.zoneCode, label: row.label, tiers });
      }
    }
    const others: ScheduleAssignment[] = [];
    for (const r of rows) {
      const turn = escalationTurnOf(r, shifts);
      const g = turn ? byTurn.get(`esc:${turn.zoneCode}|${turn.tier}`) : undefined;
      if (g) g.list.push(r);
      else others.push(r);
    }
    const restGroups: Group[] = [...groupBy(others, (r) => r.shiftCode).entries()]
      .map(([code, list]) => {
        const shift = shifts.get(code);
        return { key: code, label: shift?.label ?? code, token: shift?.colourToken ?? "", list, sort: shift?.sortOrder ?? 999 };
      })
      .sort((x, y) => x.sort - y.sort);
    return { zones: zoneCols, rest: restGroups };
  }, [rows, shifts, iso]);

  if (rows.length === 0) {
    return <div className="offnone">Nobody is on the rota that day.</div>;
  }

  const names = (list: ScheduleAssignment[]) =>
    list.map((a) => (
      <div className="nm" key={a.id}>
        <span className="av" style={{ background: teamColourOf(a.teamKey) }}>{initialsOf(a.engineer.name)}</span>
        <span className="who">{a.engineer.name}</span>
        {a.engineer.isLead ? <span className="tag lead-t">Lead</span> : null}
      </div>
    ));

  return (
    <>
      {zones.length > 0 ? (
        <div className="peekzones">
          {zones.map((z) => (
            <div className="pz" key={z.zoneCode}>
              <h5 className="pzh">
                <span className={`chip sm ${z.zoneCode}`}>{z.label}</span>
                <span className="count">{z.tiers.reduce((n, t) => n + t.list.length, 0)}</span>
              </h5>
              {z.tiers.map((t) => (
                <div className={`pzt${t.list.length === 0 ? " pgempty" : ""}`} key={t.key}>
                  <h6>
                    <span className={`chip sm ${t.token}`}>{t.label}</span>
                    <span className="count">{t.list.length}</span>
                  </h6>
                  {t.list.length === 0 ? <div className="pgnone">Nobody rostered</div> : names(t.list)}
                </div>
              ))}
            </div>
          ))}
        </div>
      ) : null}
      {rest.length > 0 ? (
        <div className="peekgrid">
          {rest.map((g) => (
            <div className="pg" key={g.key}>
              <h5>
                <span className={`chip sm ${g.token}`}>{g.label}</span>
                <span className="count">{g.list.length}</span>
              </h5>
              {names(g.list)}
            </div>
          ))}
        </div>
      ) : null}
    </>
  );
}
