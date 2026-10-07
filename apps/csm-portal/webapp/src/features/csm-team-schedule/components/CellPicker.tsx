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

import {
  Fragment,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
  type JSX,
} from "react";
import type { CellAbsence, ScheduleAbsenceKind, ScheduleShift, ScheduleTier } from "../types";
import { escalationGrid } from "../utils/rota";
import { useTeamName } from "../utils/teamColourContext";


export interface CellPickerTarget {
  userId: string;
  name: string;
  teamKey: string;
  rotaDate: string;
  /** What they currently hold that day, for marking the live code. */
  shiftCode?: string;
  /** The tier they hold on it, where it is an escalation window. */
  tier?: ScheduleTier;
  /** The clicked cell is a turn and they hold something else that day too --
   *  a turn in another zone, or regular hours -- so clearing takes off this
   *  zone's turn only. */
  otherTurns?: boolean;
  /** The leave or allocation covering that day, if any, marked the same way. */
  absenceKindCode?: string;
  /** The zone column this was opened from, on a day split across them. */
  zoneCode?: string;
  /** The leave or allocation this day is part of, if any -- the whole span,
   *  so it can be removed in one click. */
  absence?: CellAbsence;
  /** Where on screen the cell is, so the picker can sit against it. */
  anchor: { top: number; left: number; bottom: number; right: number };
  /** The standing window this engineer sits in on an ordinary weekday, so
   *  clearing a weekday puts them back on it rather than leaving a hole.
   *  Absent when they hold no standing window to go back to. */
  baseShiftCode?: string;
}

interface CellPickerProps {
  target: CellPickerTarget;
  /** The windows this group can be put on, already filtered to CRE or SRE. */
  shifts: ScheduleShift[];
  /** What a lead may mark somebody away for: leave (annual, lieu, sick,
   *  parental) and allocations (customer, RnD, Brazil rotation, ...). Shown
   *  in two groups by bucket; anything else the catalogue holds is not
   *  offered here. */
  awayKinds: ScheduleAbsenceKind[];
  /** Every kind the catalogue has, retired and the other rota's included --
   *  only to name what a cell already holds. A day marked with a kind this
   *  rota no longer offers still says what it is when a lead opens it. */
  allKinds?: ScheduleAbsenceKind[];
  /** Tags that move somebody to another team for a span ("Move to
   *  Migration"), offered in a section of their own. */
  moveKinds?: ScheduleAbsenceKind[];
  /** Put them on a window over the span. `tier` is set for an escalation
   *  window that leaves the tier to the person. */
  onApply: (shiftCode: string, from: string, to: string, tier?: ScheduleTier) => void;
  /** Mark them away over the span, for a leave or allocation kind rather than
   *  a window. `allocatedTo` is who an allocation is for, when given. */
  onMarkAway: (kindCode: string, from: string, to: string, allocatedTo?: string) => void;
  /** Clear the span: bring them back if they are marked away, otherwise put
   *  them back on the standing window -- or, with no code, which is what a
   *  weekend clear sends, take them off entirely. */
  onClear: (shiftCode: string, from: string, to: string) => void;
  /** Remove the whole absence this cell is part of, every day of it. */
  onRemoveAbsence?: (absenceId: string) => void;
  /** Add a leave or allocation tag to the shared catalogue. Resolves once it
   *  exists; rejects with a message the form can show. Absent hides the
   *  "New tag" control. */
  onCreateKind?: (kind: NewAbsenceKind) => Promise<void>;
  /** Delete a tag a lead added. Resolves once gone; rejects with a message
   *  the picker can show (a tag still in use is refused). */
  onDeleteKind?: (code: string) => Promise<void>;
  onClose: () => void;
  busy?: boolean;
}

/** What the "New tag" form collects. */
export interface NewAbsenceKind {
  shortCode: string;
  label: string;
  bucket: "LEAVE" | "ALLOCATION";
  colourToken: string;
}

/** The colours a new tag may take: the ones the stylesheet draws a leave or
 *  allocation chip for. The server holds the same list. */
const TAG_COLOURS = [
  "AL", "LL", "MAT", "PAT", "RND", "EXT", "INT", "BR", "MIG", "ONB", "EXC", "IND",
] as const;

/** An ISO day plus n days -- the end of an open-ended span, for moving
 *  somebody back from it (the server takes a year at most). */
function isoPlusDays(iso: string, n: number): string {
  const d = new Date(`${iso}T00:00:00`);
  d.setDate(d.getDate() + n);
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
}

/** "12 Mar" -- short enough to sit beside a chip in the picker's header. */
function shortDate(iso: string): string {
  return new Date(`${iso}T00:00:00`).toLocaleDateString("en-GB", { day: "numeric", month: "short" });
}

/** Is this window worked on this day at all? `day_scope` already knows. */
function allowedOn(shift: ScheduleShift, iso: string): boolean {
  const d = new Date(`${iso}T00:00:00`);
  const weekend = d.getDay() === 0 || d.getDay() === 6;
  if (shift.dayScope === "WEEKEND") return weekend;
  if (shift.dayScope === "WEEKDAY") return !weekend;
  return true;
}

/** Why a window is greyed out, in the words the grid already uses. */
function notWorked(shift: ScheduleShift): string {
  return shift.dayScope === "WEEKEND" ? "weekends only" : "weekdays only";
}

function isWeekendIso(iso: string): boolean {
  const d = new Date(`${iso}T00:00:00`);
  return d.getDay() === 0 || d.getDay() === 6;
}

/** A date field's value, if it is one yet. A date still being typed is not a
 *  date: a year segment accepts six digits, so "202609-02-09" is a value the
 *  field will genuinely hand over, and a two-digit day is briefly a different
 *  day after its first digit. */
function asDate(v: string): string | null {
  return /^\d{4}-\d{2}-\d{2}$/.test(v) && !Number.isNaN(Date.parse(`${v}T00:00:00`)) ? v : null;
}

function dayCount(from: string, to: string): number {
  const a = new Date(`${from}T00:00:00`).getTime();
  const b = new Date(`${to}T00:00:00`).getTime();
  return Math.max(1, Math.round((b - a) / 86_400_000) + 1);
}

/**
 * The picker a lead gets on a roster cell.
 *
 * Anchored to the cell rather than opened as a modal: the point of this grid is
 * the rows around the one being changed -- who else is on that week, who is
 * already away -- and a dialog in the middle of the screen hides exactly that.
 *
 * The caller keys it on the cell, so a new cell is a new decision: the range
 * starts closed again rather than carrying the last one over onto somebody
 * else, without an effect reaching in to reset it.
 *
 * It edits a span, not a day. "On the evening rotation all next week" is one
 * decision, and making somebody click it five times is how a rota ends up half
 * changed.
 */
export default function CellPicker({
  target,
  shifts,
  awayKinds,
  allKinds,
  moveKinds,
  onApply,
  onMarkAway,
  onClear,
  onRemoveAbsence,
  onCreateKind,
  onDeleteKind,
  onClose,
  busy,
}: CellPickerProps): JSX.Element {
  const [start, setStart] = useState(target.rotaDate);
  const [until, setUntil] = useState(target.rotaDate);
  const [allocatedTo, setAllocatedTo] = useState("");
  const [tagOpen, setTagOpen] = useState(false);
  const [tag, setTag] = useState<NewAbsenceKind>({
    shortCode: "",
    label: "",
    bucket: "ALLOCATION",
    colourToken: "INT",
  });
  const [tagError, setTagError] = useState("");
  const [tagBusy, setTagBusy] = useState(false);
  /** A custom tag asked to be deleted once, awaiting the second click. */
  const [confirmDelete, setConfirmDelete] = useState<string | null>(null);
  const [deleteError, setDeleteError] = useState("");
  const box = useRef<HTMLDivElement | null>(null);
  const endField = useRef<HTMLInputElement | null>(null);
  const [at, setAt] = useState<{ left: number; top: number } | null>(null);


  useEffect(() => {
    const onKey = (e: KeyboardEvent): void => {
      if (e.key === "Escape") onClose();
    };
    const onDown = (e: MouseEvent): void => {
      if (box.current && !box.current.contains(e.target as Node)) onClose();
    };
    window.addEventListener("keydown", onKey);
    // Deferred to the next tick: the click that opened the picker is still
    // propagating, and would otherwise close it immediately.
    const t = window.setTimeout(() => document.addEventListener("mousedown", onDown), 0);
    return () => {
      window.removeEventListener("keydown", onKey);
      document.removeEventListener("mousedown", onDown);
      window.clearTimeout(t);
    };
  }, [onClose]);

  /** The range the picker really has.
   *
   *  The start defaults to the day that was clicked and may be moved either
   *  way -- a lead marking a fortnight of leave from its middle should not
   *  have to find its first day on the grid first. A start that is not a date
   *  yet stays on the clicked day, and an end that is not a date yet, or is
   *  before the start, is taken as "no end chosen yet" rather than acted on:
   *  without that the day count reads "NaN days" and the write would carry
   *  the same nonsense to the server. */
  const firstDay = asDate(start) ?? target.rotaDate;
  const typedEnd = asDate(until);
  const lastDay = typedEnd && typedEnd >= firstDay ? typedEnd : firstDay;
  const span = dayCount(firstDay, lastDay);
  /** Leave is not marked from a weekend -- see the Away buttons below. */
  const onWeekend = isWeekendIso(firstDay);

  const teamNameOf = useTeamName();
  /** Moves offered from this cell: to a team the person is not already on. */
  const moves = (moveKinds ?? []).filter(
    (k) => k.movesToTeamKey && k.movesToTeamKey.toLowerCase() !== target.teamKey.toLowerCase(),
  );

  /** The move this day is part of, where it is one. On a move to a team that
   *  works no rota (Migration) they take no rotation -- the server refuses
   *  one too -- and either way they can be moved back from here. */
  const heldKind = target.absence
    ? (allKinds ?? awayKinds).find((k) => k.code === target.absence?.kindCode)
    : undefined;
  const heldMove = heldKind?.movesToTeamKey ? heldKind : undefined;
  const offRota = Boolean(heldMove && !heldMove.worksRotaThere);
  const offRotaWhy = heldMove ? `on ${heldMove.label}, not rota work` : "";

  /** Rotations first under their own heading, then the standing windows,
   *  each marked with whether it is worked on the day the picker opened on. */
  const groups = useMemo(() => {
    const decorate = (list: ScheduleShift[]) =>
      [...list]
        .sort((a, b) => a.sortOrder - b.sortOrder)
        .map((s) => ({ shift: s, ok: allowedOn(s, firstDay) && !(offRota && s.isRotation) }));
    // Escalation windows are picked from the grid below, by zone and tier,
    // rather than listed here a second time.
    return [
      { heading: "Rotations", items: decorate(shifts.filter((s) => s.isRotation && !s.isEscalation)) },
      { heading: "Standing hours", items: decorate(shifts.filter((s) => !s.isRotation)) },
    ].filter((g) => g.items.length > 0);
  }, [shifts, firstDay, offRota]);

  /** L1, L2 and L3 for every zone worked on the span's first day. */
  const grid = useMemo(() => escalationGrid(shifts, firstDay), [shifts, firstDay]);
  const groupCount = groups.reduce((n, g) => n + g.items.length, 0);

  /** Leave, then allocations, each under its own heading. */
  const awayGroups = useMemo(
    () =>
      [
        { heading: "Leave", bucket: "LEAVE" },
        { heading: "Allocations", bucket: "ALLOCATION" },
      ]
        .map((g) => ({
          ...g,
          kinds: awayKinds
            .filter((k) => k.bucket === g.bucket)
            .sort((a, b) => a.sortOrder - b.sortOrder),
        }))
        .filter((g) => g.kinds.length > 0),
    [awayKinds],
  );

  /** Sit under the cell, or above it when there is no room below, and never
   *  off the side. Measured after the first paint because the height depends
   *  on how many windows this group runs. */
  useLayoutEffect(() => {
    const el = box.current;
    if (!el) return;
    const m = 12;
    const w = el.offsetWidth;
    const h = el.offsetHeight;
    const left = Math.min(Math.max(m, target.anchor.left), window.innerWidth - w - m);
    let top = target.anchor.bottom + 8;
    if (top + h > window.innerHeight - m) top = Math.max(m, target.anchor.top - h - 8);
    if (top + h > window.innerHeight - m) top = Math.max(m, window.innerHeight - h - m);
    setAt({ left: Math.round(left), top: Math.round(top) });
  }, [target.anchor, groupCount, awayGroups.length, tagOpen, grid.length]);

  /** What clearing means here. A weekend has no standing window to fall back
   *  to, so clearing it genuinely empties the day. */
  const backTo = onWeekend ? "" : (target.baseShiftCode ?? "");

  const when = new Date(`${target.rotaDate}T00:00:00`).toLocaleDateString("en-GB", {
    weekday: "short",
    day: "numeric",
    month: "short",
  });

  return (
    <div
      className="picker open"
      ref={box}
      role="dialog"
      aria-label={`Change ${target.name}'s rota`}
      // Off screen until it has been measured, rather than at 0,0 where it
      // would flash in the corner for a frame before landing on the cell.
      style={at ?? { top: -9999, left: -9999 }}
    >
      <div className="pk-head">
        <b>{target.name}</b>
        <span className="pk-d">{when}</span>
        <button type="button" className="pk-x" onClick={onClose} aria-label="Close">
          &times;
        </button>
      </div>

      {target.absence && onRemoveAbsence ? (
        // The whole span, not the day that was clicked: leave is granted as a
        // span, so taking it back is one decision too. Clearing a few days out
        // of it is still what the date range and "Clear" below are for.
        <div className="pk-remove">
          {(() => {
            const ab = target.absence;
            const kind = (allKinds ?? awayKinds).find((k) => k.code === ab.kindCode);
            // A move worked as another team's normal hours reads as those
            // hours -- Americas cover -- not as the tag that records it.
            const shownAs =
              kind?.worksRotaThere && kind.showsAsShiftCode
                ? shifts.find((sh) => sh.code === kind.showsAsShiftCode)
                : undefined;
            const range = ab.endsOn
              ? ab.endsOn === ab.startsOn
                ? shortDate(ab.startsOn)
                : `${shortDate(ab.startsOn)} – ${shortDate(ab.endsOn)}`
              : `from ${shortDate(ab.startsOn)}, until further notice`;
            return (
              <>
                <span className={`chip sm ${shownAs?.colourToken ?? kind?.colourToken ?? ""}`}>
                  {shownAs?.shortCode ?? kind?.shortCode ?? ab.kindCode}
                </span>
                <span className="pkrm-t">
                  <b>{shownAs?.label ?? kind?.label ?? ab.kindCode}</b>
                  {ab.allocatedTo ? ` · ${ab.allocatedTo}` : ""}
                  <small>{range}</small>
                </span>
                <button
                  type="button"
                  className="pkrm-b"
                  disabled={busy}
                  onClick={() => onRemoveAbsence(ab.id)}
                >
                  Remove
                </button>
                {/* Only from a day the move still covers: a later start would
                    send a backwards range, which the server swaps round --
                    clearing the move's last day and whatever else lies in
                    between. */}
                {heldMove && ab.homeTeamKey && (!ab.endsOn || firstDay <= ab.endsOn) ? (
                  // The end of a move: back on their own team from the day
                  // picked, the days before it kept as they were.
                  <button
                    type="button"
                    className="pkrm-b back"
                    disabled={busy}
                    onClick={() => onMarkAway("", firstDay, ab.endsOn ?? isoPlusDays(firstDay, 365))}
                  >
                    Back to {teamNameOf(ab.homeTeamKey)} from {shortDate(firstDay)}
                  </button>
                ) : null}
              </>
            );
          })()}
        </div>
      ) : null}

      <div className="pk-range">
        <label className="pkr-f">
          <span>From</span>
          <input
            type="date"
            // Uncontrolled, like the end date below and for the same reason.
            defaultValue={target.rotaDate}
            onChange={(e) => setStart(e.target.value || target.rotaDate)}
            onBlur={(e) => {
              e.target.value = firstDay;
              setStart(firstDay);
              // A start moved past the end takes the end with it, so the two
              // fields never show a backwards range.
              if (endField.current && endField.current.value < firstDay) {
                endField.current.value = firstDay;
                setUntil(firstDay);
              }
            }}
            aria-label="Mark from"
          />
        </label>
        <label className="pkr-f">
          <span>To</span>
          <input
            ref={endField}
            type="date"
            // Uncontrolled on purpose. A date input keeps its own half-typed
            // segment, and feeding `value` back on every change throws that
            // away: the "2" of "29" lands as the 2nd, React re-renders, and
            // the "9" starts over as the 9th. The date is read out of the
            // field instead, and the component is keyed on the cell so it
            // still starts fresh when another cell is picked.
            defaultValue={target.rotaDate}
            min={firstDay}
            // Taken as typed, and only settled back to the start once focus
            // leaves. Clamping on every change cannot work here: a date input
            // reports a whole date per segment, so the first digit of "25" is
            // the 2nd of the month, which is before the start -- snapping
            // there resets the field under the typist and the second digit
            // can never land. What the range actually means is clamped above
            // instead, so nothing downstream sees a backwards span.
            onChange={(e) => setUntil(e.target.value || firstDay)}
            onBlur={(e) => {
              e.target.value = lastDay;
              setUntil(lastDay);
            }}
            aria-label="Mark until"
          />
        </label>
        <span className="pkr-n">
          {span} day{span === 1 ? "" : "s"}
        </span>
      </div>

      <div className="pk-grid">
        {groups.map((group) => (
          <Fragment key={group.heading}>
            <div className="pk-sec">{group.heading}</div>
            {group.items.map(({ shift, ok }) => (
              <button
                key={shift.code}
                type="button"
                className={`pk-c ${shift.code === target.shiftCode ? "on" : ""} ${ok ? "" : "bad"}`}
                disabled={!ok || busy}
                title={ok ? shift.label : `${shift.label} — ${offRota && shift.isRotation ? offRotaWhy : notWorked(shift)}`}
                onClick={() => onApply(shift.code, firstDay, lastDay)}
              >
                <span className={`chip sm ${shift.colourToken}`}>{shift.shortCode}</span>
                {/* A window that does not apply today still says what it is.
                    "weekends only" on its own left a lead reading chips to
                    work out which weekend window was which. */}
                <span className="pk-l">
                  {shift.label}
                  {ok ? null : (
                    <small className="pk-n">{offRota && shift.isRotation ? offRotaWhy : notWorked(shift)}</small>
                  )}
                </span>
              </button>
            ))}
          </Fragment>
        ))}

        {grid.length > 0 ? (
          <>
            <div className="pk-sec">Escalation</div>
            {/* Every zone, not only the column that was clicked: a lead
                rostering L3 for TZ2 from a TZ1 cell should not have to find
                the TZ2 column first. The clicked zone is marked. */}
            <div className="pk-esc">
              {grid.map((row) => (
                <div
                  key={row.zoneCode}
                  className={`pk-escr${row.zoneCode === target.zoneCode ? " here" : ""}`}
                >
                  <span className="pk-escz">{row.label}</span>
                  {row.tiers.map(({ tier, shift }) => {
                    const held =
                      Boolean(shift) &&
                      shift?.code === target.shiftCode &&
                      (target.tier ?? shift?.tier) === tier;
                    return (
                      <button
                        key={tier}
                        type="button"
                        className={`pk-t${held ? " on" : ""}`}
                        disabled={!shift || busy || offRota}
                        aria-label={`${tier} for ${row.label}`}
                        title={shift ? `${tier} · ${shift.label}` : `${row.label} has no ${tier} window`}
                        onClick={() =>
                          shift && onApply(shift.code, firstDay, lastDay, shift.tier ? undefined : tier)
                        }
                      >
                        <span className={`chip sm ${tier}`}>{tier}</span>
                      </button>
                    );
                  })}
                </div>
              ))}
            </div>
          </>
        ) : null}

        {moves.length > 0 ? (
          <>
            <div className="pk-sec">Move to another team</div>
            {moves.map((kind) => (
              <button
                key={kind.code}
                type="button"
                className={`pk-c ${kind.code === target.absenceKindCode ? "on" : ""}`}
                disabled={busy}
                // A span like any other: from and to above. They show under
                // that team for those days, and come back with "Back to ...".
                title={`${kind.label}: on the ${teamNameOf(kind.movesToTeamKey ?? "")} team for the span`}
                onClick={() => onMarkAway(kind.code, firstDay, lastDay)}
              >
                <span className={`chip sm ${kind.colourToken}`}>{kind.shortCode}</span>
                <span className="pk-l">Move to {teamNameOf(kind.movesToTeamKey ?? "")}</span>
              </button>
            ))}
          </>
        ) : null}

        {awayGroups.map((group) => (
          <Fragment key={group.bucket}>
            <div className="pk-sec">{group.heading}</div>
            {group.bucket === "ALLOCATION" ? (
              // Who the time is for, so a new customer is a value rather than
              // a new kind. Optional: "on RnD" is a complete answer when the
              // product team is not known yet.
              <label className="pk-for">
                <span>For</span>
                <input
                  type="text"
                  value={allocatedTo}
                  maxLength={100}
                  placeholder="Customer or product team (optional)"
                  onChange={(e) => setAllocatedTo(e.target.value)}
                  aria-label="Allocated to"
                />
              </label>
            ) : null}
            {group.kinds.map((kind) => {
              // No leave from a weekend: nobody is rostered to be away from a
              // Saturday. A span that starts on a weekday may still run across
              // one. An allocation has no such rule -- the time is given to
              // someone else whichever day it starts.
              const blocked = onWeekend && kind.bucket === "LEAVE";
              const mark = (
                <button
                  key={kind.code}
                  type="button"
                  className={`pk-c ${kind.code === target.absenceKindCode ? "on" : ""}${blocked ? " bad" : ""}`}
                  disabled={busy || blocked}
                  // No weekday rule inside the span, unlike a rotation. It is
                  // stored as a span rather than a day at a time, so a weekend
                  // inside it is covered too -- somebody away Friday to Monday
                  // is away for the weekend, and skipping it would say they
                  // were back for two days in the middle of it.
                  title={`${kind.label} — the whole span, weekends included`}
                  onClick={() =>
                    onMarkAway(
                      kind.code,
                      firstDay,
                      lastDay,
                      kind.bucket === "ALLOCATION" && allocatedTo.trim() ? allocatedTo.trim() : undefined,
                    )
                  }
                >
                  <span className={`chip sm ${kind.colourToken}`}>{kind.shortCode}</span>
                  <span className="pk-l">{kind.label}</span>
                </button>
              );
              if (!kind.custom || !onDeleteKind) return mark;
              // A tag a lead added can be deleted from where it is used. Two
              // clicks, because it is shared: the tag goes for every team.
              const asking = confirmDelete === kind.code;
              return (
                <div className="pk-cw" key={kind.code}>
                  {mark}
                  <button
                    type="button"
                    className={`pk-del${asking ? " ask" : ""}`}
                    aria-label={asking ? `Confirm deleting the ${kind.label} tag` : `Delete the ${kind.label} tag`}
                    title={asking ? "Click again to delete it for every team" : "Delete this tag"}
                    disabled={busy}
                    onClick={() => {
                      if (!asking) {
                        setConfirmDelete(kind.code);
                        setDeleteError("");
                        return;
                      }
                      onDeleteKind(kind.code)
                        .then(() => setConfirmDelete(null))
                        .catch((err: unknown) =>
                          setDeleteError(err instanceof Error && err.message ? err.message : "The tag was not deleted."),
                        );
                    }}
                  >
                    {asking ? "Delete?" : "×"}
                  </button>
                </div>
              );
            })}
            {deleteError && group.kinds.some((k) => k.code === confirmDelete) ? (
              <div className="tf-err pk-delerr" role="alert">{deleteError}</div>
            ) : null}
          </Fragment>
        ))}

        {onCreateKind ? (
          tagOpen ? (
            <form
              className="pk-newtag tf"
              onSubmit={(e) => {
                e.preventDefault();
                const shortCode = tag.shortCode.trim();
                const label = tag.label.trim();
                if (!shortCode || !label) {
                  setTagError("A short code and a name are both needed.");
                  return;
                }
                if (awayKinds.some((k) => k.shortCode.toLowerCase() === shortCode.toLowerCase())) {
                  setTagError(`${shortCode} is already a tag — pick another short code.`);
                  return;
                }
                setTagBusy(true);
                setTagError("");
                onCreateKind({ ...tag, shortCode, label })
                  .then(() => {
                    setTagOpen(false);
                    setTag({ shortCode: "", label: "", bucket: tag.bucket, colourToken: tag.colourToken });
                  })
                  .catch((err: unknown) =>
                    setTagError(err instanceof Error && err.message ? err.message : "The tag was not added."),
                  )
                  .finally(() => setTagBusy(false));
              }}
            >
              <div className="pk-sec">New tag</div>
              <label>
                <span>Short code</span>
                <input
                  value={tag.shortCode}
                  maxLength={12}
                  placeholder="e.g. Trn"
                  onChange={(e) => setTag({ ...tag, shortCode: e.target.value })}
                  aria-label="Tag short code"
                />
              </label>
              <label>
                <span>Name</span>
                <input
                  value={tag.label}
                  maxLength={100}
                  placeholder="e.g. External training"
                  onChange={(e) => setTag({ ...tag, label: e.target.value })}
                  aria-label="Tag name"
                />
              </label>
              <div className="pk-codes" role="radiogroup" aria-label="Tag type">
                {(["LEAVE", "ALLOCATION"] as const).map((b) => (
                  <button
                    key={b}
                    type="button"
                    role="radio"
                    aria-checked={tag.bucket === b}
                    className={tag.bucket === b ? "on" : ""}
                    onClick={() => setTag({ ...tag, bucket: b })}
                  >
                    {b === "LEAVE" ? "Leave" : "Allocation"}
                  </button>
                ))}
              </div>
              <div className="tf-sw" role="radiogroup" aria-label="Tag colour">
                {TAG_COLOURS.map((c) => (
                  <button
                    key={c}
                    type="button"
                    role="radio"
                    aria-checked={tag.colourToken === c}
                    aria-label={`Colour ${c}`}
                    className={`pk-sw${tag.colourToken === c ? " on" : ""}`}
                    onClick={() => setTag({ ...tag, colourToken: c })}
                  >
                    <span className={`chip sm ${c}`}>{tag.colourToken === c && tag.shortCode.trim() ? tag.shortCode.trim() : "Aa"}</span>
                  </button>
                ))}
              </div>
              <p className="pk-note">Shared with every team once added.</p>
              <div className="tf-err" role="alert">{tagError}</div>
              <div className="pk-tagact">
                <button type="button" className="btn sm" onClick={() => setTagOpen(false)}>
                  Cancel
                </button>
                <button type="submit" className="btn sm primary" disabled={tagBusy}>
                  {tagBusy ? "Adding…" : "Add tag"}
                </button>
              </div>
            </form>
          ) : (
            <button type="button" className="pk-c newtype" onClick={() => setTagOpen(true)}>
              + New tag
            </button>
          )
        ) : null}

        <button
          type="button"
          className="pk-c clear"
          disabled={busy}
          onClick={() => onClear(backTo, firstDay, lastDay)}
        >
          {/* Says what clearing will actually do here, which is not the same
              thing in the three cases. Somebody marked away comes back onto
              whatever the rota already had for them; a weekend has no standing
              window to fall back to at all. */}
          {target.otherTurns && target.zoneCode
            ? `Clear — take off ${target.zoneCode}${target.tier ? ` ${target.tier}` : ""} only`
            : target.absenceKindCode && !target.shiftCode
            ? "Clear — back on the rota"
            : backTo
              ? `Clear — back to ${shifts.find((s) => s.code === backTo)?.label ?? "regular hours"}`
              : "Clear — nothing rostered"}
        </button>
      </div>

      {/* Over a span, a weekday rotation will skip the weekend inside it, and
          the reader should know that before they click rather than after. */}
      {span > 1 ? (
        <p className="pk-note">
          Days a rotation is not worked on are skipped — a weekday rotation over
          a week sets the five weekdays.
        </p>
      ) : null}
    </div>
  );
}
