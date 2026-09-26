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
import type { ScheduleAssignment, ScheduleShift } from "../types";
import { dayLabel, isRotationShift, partsInZone, timeOf } from "../utils/rota";

interface NextRotationProps {
  /** The signed-in engineer's own rota, from today forward. */
  mine: ScheduleAssignment[];
  shifts: Map<string, ScheduleShift>;
  tz: string;
  /** How far ahead the query looked, so the empty state can say so honestly
   *  rather than implying the engineer is never on again. */
  horizonDays: number;
  isLoading: boolean;
}

/**
 * Whole days between two instants, by calendar day rather than by 24h --
 * something starting at 21:00 tonight is "today", not "in 0.9 days".
 *
 * Counted in the reader's profile zone, not the browser's. Those are routinely
 * different here -- the whole page is built on the profile being the one clock
 * -- and reading the calendar day off the browser made "in 2 days" disagree
 * with the date printed beside it: a rotation starting 00:30 on the 27th in
 * Colombo is still the 26th to a machine set to Los Angeles.
 */
function daysUntil(iso: string, nowMs: number, tz: string): number {
  const day = (at: string): number => {
    const [y, m, d] = partsInZone(at, tz).date.split("-").map(Number);
    return Date.UTC(y, m - 1, d);
  };
  return Math.round((day(iso) - day(new Date(nowMs).toISOString())) / 86_400_000);
}

function whenWord(days: number): string {
  if (days <= 0) return "today";
  if (days === 1) return "tomorrow";
  if (days < 7) return `in ${days} days`;
  if (days < 14) return "next week";
  return `in ${Math.round(days / 7)} weeks`;
}

/**
 * When the signed-in engineer is next on a rotation, whatever kind it is --
 * morning, morning on-call, evening, weekend, an Americas night or an SRE
 * escalation tier.
 *
 * It sits above the card rather than inside one, because it answers a question
 * about the reader rather than about the view they happen to have open: the
 * answer should not change when they click across to the month roster.
 *
 * Regular hours are deliberately not a rotation. Everybody has them on every
 * weekday, so "your next rotation is regular hours tomorrow" would be true,
 * useless, and would bury the shift the engineer actually needs to plan around.
 */
export default function NextRotation({
  mine,
  shifts,
  tz,
  horizonDays,
  isLoading,
}: NextRotationProps): JSX.Element | null {
  // The clock lives in state, not in render: reading it while rendering is
  // impure, and a minute's tick means the banner turns over to "On now" by
  // itself rather than waiting for a reload.
  const [now, setNow] = useState<number | null>(null);
  useEffect(() => {
    const tick = (): void => setNow(Date.now());
    tick();
    const t = window.setInterval(tick, 60_000);
    return () => window.clearInterval(t);
  }, []);

  const next = useMemo(() => {
    if (now === null) return undefined;
    return (
      mine
        // A rotation still running counts as the one you are on, not one you
        // have finished -- so compare against the end, not the start.
        .filter((a) => isRotationShift(shifts.get(a.shiftCode)) && Date.parse(a.endsAt) > now)
        .sort((a, b) => Date.parse(a.startsAt) - Date.parse(b.startsAt))[0]
    );
  }, [mine, shifts, now]);

  if (isLoading || now === null) {
    return (
      <div className="nextrot loading">
        <span className="nrlab">Your next rotation</span>
        <span className="nrmuted">Looking…</span>
      </div>
    );
  }

  if (!next) {
    return (
      <div className="nextrot none">
        <span className="nrlab">Your next rotation</span>
        <span className="nrmuted">
          Nothing rostered in the next {Math.round(horizonDays / 7)} weeks.
        </span>
      </div>
    );
  }

  const shift = shifts.get(next.shiftCode);
  const days = daysUntil(next.startsAt, now, tz);
  const running = Date.parse(next.startsAt) <= now;

  return (
    <div className={`nextrot${running ? " now" : ""}`}>
      <span className="nrlab">{running ? "On now" : "Your next rotation"}</span>

      <span className="nrwhat">
        <b>{shift?.label ?? next.shiftCode}</b>
        {next.tier ? <i className="nrtier">{next.tier}</i> : null}
        {next.isOnCall ? <i className="nrtier oc">On call</i> : null}
        {next.zoneCode ? <i className="nrtier">{next.zoneCode}</i> : null}
      </span>

      <span className="nrwhen">
        {dayLabel(next.startsAt, tz)} · {timeOf(next.startsAt, tz)}–{timeOf(next.endsAt, tz)}
      </span>

      {running ? null : <span className="nrin">{whenWord(days)}</span>}
    </div>
  );
}
