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

import { useState, type JSX } from "react";
import { useHolidayAvailability, useHolidayConfirmation } from "../api/useHolidayAvailability";
import { useDeleteAbsence } from "../api/useTeamSchedule";
import type { HolidayAvailability, HolidayAvailabilityMember, HolidayAvailabilityStatus } from "../types";

/** How far ahead the panel looks: the server places the tags three weeks ahead. */
export const HOLIDAY_LOOKAHEAD_DAYS = 21;

/** Formats a Date as a local YYYY-MM-DD calendar day. */
function isoDay(d: Date): string {
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
}

/** Turns a YYYY-MM-DD day into a short readable label such as "Mon, 16 Nov". */
function longDay(iso: string): string {
  const d = new Date(`${iso}T00:00:00`);
  return d.toLocaleDateString(undefined, { weekday: "short", day: "numeric", month: "short" });
}

const STATUS_LABEL: Record<HolidayAvailabilityStatus, string> = {
  NEEDS_MARKING: "Needs marking",
  AWAITING_CONFIRMATION: "Ready to confirm",
  CONFIRMED: "Confirmed",
  NEEDS_ATTENTION: "Needs attention",
};

/** Says what a member is doing on the holiday: off, working, or away (with the kind of leave). */
function memberLabel(m: HolidayAvailabilityMember): string {
  if (m.status === "TAGGED") return "Off";
  if (m.status === "AVAILABLE") return "Working";
  return m.awayKind ? `Away (${m.awayKind})` : "Away";
}

/** The note under a team's line, saying what is left to do. */
function nextStep(h: HolidayAvailability): string {
  switch (h.status) {
    case "NEEDS_MARKING":
      return "A rota turn has nobody available. Make somebody available, or move the turn, then confirm.";
    case "AWAITING_CONFIRMATION":
      return "Every rota turn has somebody. Confirm the team's availability to stop the reminders.";
    case "NEEDS_ATTENTION":
      return "Confirmed, but a rota turn has since lost its person. The reminders have resumed.";
    default:
      return h.confirmedBy ? `Confirmed by ${h.confirmedBy}.` : "Confirmed.";
  }
}

/** One team's line for a holiday: its status and counts, with the member list and the confirm and make-available actions when opened. */
function HolidayTeamRow({ h }: { h: HolidayAvailability }): JSX.Element {
  const [open, setOpen] = useState(h.canConfirm && !h.fullyMarked);
  const confirm = useHolidayConfirmation();
  const makeAvailable = useDeleteAbsence();
  const off = h.members.filter((m) => m.status === "TAGGED").length;
  const working = h.members.filter((m) => m.status === "AVAILABLE").length;
  const busy = confirm.isPending || makeAvailable.isPending;
  const error = confirm.error ?? makeAvailable.error;
  return (
    <li className={`hol-row ${h.status.toLowerCase()}`}>
      <div className="hol-line">
        <button
          type="button"
          className="hol-toggle"
          aria-expanded={open}
          onClick={() => setOpen((v) => !v)}
        >
          <b>{h.teamName}</b> &middot; {longDay(h.date)} &middot; {h.titles.join(" / ")}
          {h.countries.length > 0 ? <span className="hol-c"> ({h.countries.join(", ")})</span> : null}
        </button>
        <span className={`hol-st ${h.status.toLowerCase()}`}>{STATUS_LABEL[h.status]}</span>
        <span className="hol-n">
          {off} off &middot; {working} working
          {h.uncovered.length > 0 ? <> &middot; {h.uncovered.length} turn(s) uncovered</> : null}
        </span>
      </div>
      {open ? (
        <div className="hol-body">
          <p className="hol-next">{nextStep(h)}</p>
          {h.uncovered.length > 0 ? (
            <p className="hol-unc">
              Nobody available for:{" "}
              {h.uncovered.map((t) => `${t.shiftLabel} (${t.name})`).join(", ")}
            </p>
          ) : null}
          <ul className="hol-members">
            {h.members.map((m) => (
              <li key={m.userId}>
                <span className="hol-name">{m.name}</span>
                <span className={`hol-ms ${m.status.toLowerCase()}`}>{memberLabel(m)}</span>
                {h.canConfirm && m.status === "TAGGED" && m.absenceId ? (
                  <button
                    type="button"
                    className="btn sm"
                    disabled={busy}
                    onClick={() =>
                      makeAvailable.mutate({ id: m.absenceId as string, note: "Works on the public holiday" })
                    }
                  >
                    Make available
                  </button>
                ) : null}
              </li>
            ))}
          </ul>
          {h.canConfirm ? (
            <div className="hol-actions">
              {h.confirmedOn ? (
                <button
                  type="button"
                  className="btn sm"
                  disabled={busy}
                  onClick={() => confirm.mutate({ teamKey: h.teamKey, date: h.date, confirm: false })}
                >
                  Withdraw confirmation
                </button>
              ) : (
                <button
                  type="button"
                  className="btn sm primary"
                  disabled={busy || h.uncovered.length > 0}
                  title={h.uncovered.length > 0 ? "Every rota turn needs somebody available first" : undefined}
                  onClick={() => confirm.mutate({ teamKey: h.teamKey, date: h.date, confirm: true })}
                >
                  Confirm team availability
                </button>
              )}
            </div>
          ) : null}
          {error ? (
            <p className="hol-err" role="alert">
              {error.message}
            </p>
          ) : null}
        </div>
      ) : null}
    </li>
  );
}

/**
 * Working on a public holiday (CRE). A member is off on a public holiday of
 * their country: the server tags them "Public holiday" three weeks ahead,
 * which takes the day off their timesheet and them off the rota. The team's
 * lead or rota manager makes somebody available by removing the tag (here,
 * or on the roster), then confirms the team; the reminders stop once every
 * rota turn has somebody and the team is confirmed.
 *
 * Shows the teams the reader may confirm; everybody else's are behind
 * "Show all teams". Nothing at all when no holiday is coming, and nothing
 * when the read fails: the roster still shows each tag.
 */
export default function HolidayAvailabilityPanel({ now = new Date() }: { now?: Date }): JSX.Element | null {
  const from = isoDay(now);
  const until = new Date(now);
  until.setDate(until.getDate() + HOLIDAY_LOOKAHEAD_DAYS);
  const read = useHolidayAvailability(from, isoDay(until));
  const [all, setAll] = useState(false);
  const holidays = read.data?.holidays ?? [];
  if (holidays.length === 0) return null;
  const mine = holidays.filter((h) => h.canConfirm);
  const shown = all || mine.length === 0 ? holidays : mine;
  const others = holidays.length - mine.length;
  return (
    <section className="holbar" aria-label="Public holidays coming up">
      <div className="hol-head">
        <b>Public holidays coming up</b>
        <span className="hol-sub">
          Members tagged &quot;Public holiday&quot; are off: the day is left out of their timesheet and Case Alerts does
          not call them. Remove the tag from anyone who works that day.
        </span>
        {mine.length > 0 && others > 0 ? (
          <button type="button" className="btn sm ghost" onClick={() => setAll((v) => !v)}>
            {all ? "Show my teams" : `Show all teams (${holidays.length})`}
          </button>
        ) : null}
      </div>
      <ul className="hol-list">
        {shown.map((h) => (
          <HolidayTeamRow key={`${h.teamKey}|${h.date}`} h={h} />
        ))}
      </ul>
    </section>
  );
}
