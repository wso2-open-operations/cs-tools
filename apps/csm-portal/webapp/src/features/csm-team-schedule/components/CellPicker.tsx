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
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
  type JSX,
} from "react";
import RequiredMark from "@components/RequiredMark";
import type { CellAbsence, ScheduleAbsenceKind, ScheduleShift, ScheduleTier } from "../types";
import { escalationGrid } from "../utils/rota";
import { INDIA_HOURS_CODE, REGULAR_HOURS_CODE } from "../utils/standingDay";
import { useTeamName } from "../utils/teamColourContext";
import { tagHoverText, tagSideText } from "../utils/tagHover";


/** A customer engagement allocation (from the allocation app) covering the
 *  clicked day, offered for removal -- or, once removed, for restoring. */
export interface CellEngagement {
  /** The allocation's id, as the engagement allocation read returns it. */
  id: string;
  /** The chip: Allo- and the engagement id. */
  label: string;
  /** The allocation's full line (account, engagement, type, %, dates). */
  title: string;
  /** Taken off this person's roster on the clicked day. */
  dismissed: boolean;
}

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
  /** Customer engagements from the allocation app covering the day: the
   *  ones on the roster can be taken off it, removed ones put back. */
  engagements?: CellEngagement[];
  /** Where on screen the cell is, so the picker can grow out of it. */
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
  /** Take a synced engagement allocation off the roster over the picked
   *  days, or (restore) put those days back. Absent hides the controls. */
  onDismissEngagement?: (engagement: CellEngagement, from: string, to: string, restore: boolean) => void;
  /** Add a leave or allocation tag to the shared catalogue. Resolves once it
   *  exists; rejects with a message the form can show. Absent hides the
   *  "New tag" control. */
  onCreateKind?: (kind: NewAbsenceKind) => Promise<void>;
  /** The family (CRE, SRE, SME) a new tag is added for: the roster being
   *  edited. Only that family's rosters offer the tag. */
  tagFamily?: string;
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
  /** The tag's days take people off the timesheet (an SRE rotation); not
   *  set, they stay on it (work from home). Allocation tags only: leave
   *  shows on the timesheet as leave. */
  excludesFromTimesheet: boolean;
  /** Optional; shown when hovering the tag. */
  description: string;
}

/** The longest name a new tag may have: it is the chip the roster draws. */
export const TAG_NAME_MAX = 10;

/** The longest description a tag may carry (the server's limit too). */
export const TAG_DESCRIPTION_MAX = 300;

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

/** The last day of n calendar months from an ISO day: 1 Nov plus 3 months
 *  runs to 31 Jan. A day the later month does not have (31 Jan plus one
 *  month) runs to that month's end, 28 Feb -- not the day before it, as the
 *  span ends where the missing day would have been. */
function isoPlusMonths(iso: string, n: number): string {
  const d = new Date(`${iso}T00:00:00`);
  const day = d.getDate();
  const end = new Date(d.getFullYear(), d.getMonth() + n, day);
  if (end.getDate() !== day) end.setDate(0);
  else end.setDate(end.getDate() - 1);
  return `${end.getFullYear()}-${String(end.getMonth() + 1).padStart(2, "0")}-${String(end.getDate()).padStart(2, "0")}`;
}

/** The span lengths offered beside the dates: a week of leave up to a
 *  six-month move. A week is seven days counting the first. */
const SPAN_PRESETS: ReadonlyArray<{ label: string; days: number; months: number }> = [
  { label: "1 week", days: 7, months: 0 },
  { label: "2 weeks", days: 14, months: 0 },
  { label: "1 month", days: 0, months: 1 },
  { label: "2 months", days: 0, months: 2 },
  { label: "3 months", days: 0, months: 3 },
  { label: "6 months", days: 0, months: 6 },
];

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

/** How long the picker takes to fade out, matched by the stylesheet. */
const CLOSE_MS = 140;

/** Everything Tab may land on inside the picker, in order. */
const FOCUSABLE = 'button:not(:disabled), input:not(:disabled), [tabindex]:not([tabindex="-1"])';

function prefersReducedMotion(): boolean {
  return typeof window.matchMedia === "function" && window.matchMedia("(prefers-reduced-motion: reduce)").matches;
}

/**
 * The picker a lead gets on a roster cell.
 *
 * A dialog in the middle of the view, the same place every time. Anchored to
 * the cell, it landed wherever the cell happened to be -- half off a row near
 * the bottom, over the column being edited -- and leads went looking for it.
 * It grows out of the cell that was clicked, so the eye still follows the
 * click, and the header says who and what the day holds now, which is the
 * part of the grid it covers that the decision needs.
 *
 * Two rows: the rota on top (rotations beside standing hours), then leave and
 * allocations with the date range at their head (leave beside moves to another
 * team and allocations). Leave and moves are almost always marked over days, a
 * rotation mostly for the one that was clicked; the range still applies to
 * both, and the rota row says which days it will set.
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
  onDismissEngagement,
  onCreateKind,
  tagFamily,
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
    excludesFromTimesheet: false,
    description: "",
  });
  const [tagError, setTagError] = useState("");
  const [tagBusy, setTagBusy] = useState(false);
  /** A custom tag asked to be deleted once, awaiting the second click. */
  const [confirmDelete, setConfirmDelete] = useState<string | null>(null);
  const [deleteError, setDeleteError] = useState("");
  const box = useRef<HTMLDivElement | null>(null);
  const endField = useRef<HTMLInputElement | null>(null);
  /** Where the clicked cell is, as seen from the dialog: the point it grows
   *  out of. Unset until measured, when it simply grows from its middle. */
  const [origin, setOrigin] = useState<string | undefined>(undefined);
  const [closing, setClosing] = useState(false);
  const closeTimer = useRef<number | undefined>(undefined);

  /** Fade out, then close. Straight away for anyone who asked for less
   *  motion, and only once however many times it is asked for. */
  const close = useCallback((): void => {
    if (closeTimer.current !== undefined) return;
    if (prefersReducedMotion()) {
      onClose();
      return;
    }
    setClosing(true);
    closeTimer.current = window.setTimeout(onClose, CLOSE_MS);
  }, [onClose]);

  useEffect(() => () => window.clearTimeout(closeTimer.current), []);

  /** Focus goes into the dialog when it opens and back to wherever it was when
   *  it closes, so a keyboard user is never left on the page behind it. */
  useEffect(() => {
    const before = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    box.current?.focus({ preventScroll: true });
    return () => {
      if (before && document.contains(before)) before.focus({ preventScroll: true });
    };
  }, []);

  useEffect(() => {
    const onKey = (e: KeyboardEvent): void => {
      if (e.key === "Escape") {
        close();
        return;
      }
      // Tab goes round inside the dialog: the page behind it is not
      // reachable while it is open, by mouse or by keyboard.
      if (e.key !== "Tab" || !box.current) return;
      // In document order: a selector list is not guaranteed to come back
      // that way everywhere (jsdom groups it by selector).
      const stops = [...box.current.querySelectorAll<HTMLElement>("*")].filter((el) => el.matches(FOCUSABLE));
      if (stops.length === 0) return;
      const first = stops[0];
      const last = stops[stops.length - 1];
      const here = document.activeElement;
      if (e.shiftKey && (here === first || here === box.current || !box.current.contains(here))) {
        e.preventDefault();
        last.focus();
      } else if (!e.shiftKey && (here === last || !box.current.contains(here))) {
        e.preventDefault();
        first.focus();
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [close]);

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
  /** "Migration Team", "Americas Team": what a move button names. A team whose
   *  name already ends in "Team" keeps it once. */
  const asTeam = (key: string): string => {
    const name = teamNameOf(key).trim();
    return /\bteam$/i.test(name) ? name : `${name} Team`;
  };

  /** Another team's ordinary hours, offered as the move it is. Americas cover
   *  picked for somebody not on Americas puts them on the Americas team for
   *  the span (the Brazil rotation) -- the server writes it as a move -- so it
   *  sits with the other moves as "Move to Americas Team" rather than among
   *  the standing hours, where it read as a window like their own. For
   *  somebody already on that team it stays their standing hours. */
  const windowMoves = useMemo(() => {
    const seen = new Set<string>();
    const out: { kind: ScheduleAbsenceKind; shift: ScheduleShift }[] = [];
    for (const kind of allKinds ?? awayKinds) {
      if (!kind.movesToTeamKey || !kind.worksRotaThere || !kind.showsAsShiftCode || kind.retired) continue;
      if (kind.movesToTeamKey.toLowerCase() === target.teamKey.toLowerCase()) continue;
      const shift = shifts.find((sh) => sh.code === kind.showsAsShiftCode);
      if (!shift || seen.has(shift.code)) continue;
      seen.add(shift.code);
      out.push({ kind, shift });
    }
    return out;
  }, [allKinds, awayKinds, shifts, target.teamKey]);

  /** What a window move's button names. The team whose ordinary day is the
   *  India region shift is the India team, whatever its own name: "India
   *  Team (Phoenix)". Every other move names the team. */
  const windowMoveTeam = (kind: ScheduleAbsenceKind, shift: ScheduleShift): string =>
    shift.code === INDIA_HOURS_CODE
      ? `India Team (${teamNameOf(kind.movesToTeamKey ?? "")})`
      : asTeam(kind.movesToTeamKey ?? "");

  /** Somebody on the India team: the India region shift is moved to, not
   *  picked, so their standing hours are Regular hours only (IST). */
  const onIndiaTeam = (allKinds ?? awayKinds).some(
    (k) =>
      k.showsAsShiftCode === INDIA_HOURS_CODE &&
      k.worksRotaThere &&
      !k.retired &&
      k.movesToTeamKey?.toLowerCase() === target.teamKey.toLowerCase(),
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
      {
        heading: "Standing hours",
        items: decorate(
          shifts.filter(
            (s) =>
              !s.isRotation &&
              !windowMoves.some((m) => m.shift.code === s.code) &&
              (!onIndiaTeam || s.code === REGULAR_HOURS_CODE),
          ),
        ),
      },
    ].filter((g) => g.items.length > 0);
  }, [shifts, firstDay, offRota, windowMoves, onIndiaTeam]);

  /** L1, L2 and L3 for every zone worked on the span's first day. */
  const grid = useMemo(() => escalationGrid(shifts, firstDay), [shifts, firstDay]);

  /** No window of their rota is served, so there is no rotation or L1-L3 to
   *  put them on. Said even when a move to another team is still offered:
   *  the move buttons alone don't explain why the rota part is missing. */
  const nothingToRoster = shifts.length === 0;

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
  const leaveGroups = awayGroups.filter((g) => g.bucket === "LEAVE");
  const allocationGroups = awayGroups.filter((g) => g.bucket === "ALLOCATION");
  const hasMoves = moves.length > 0 || windowMoves.length > 0;

  /** Grow out of the cell that was clicked. Measured before the first paint,
   *  from the dialog's own untransformed place in the backdrop, so the
   *  opening scale does not skew it. */
  useLayoutEffect(() => {
    const el = box.current;
    if (!el) return;
    const x = (target.anchor.left + target.anchor.right) / 2 - el.offsetLeft;
    const y = (target.anchor.top + target.anchor.bottom) / 2 - el.offsetTop;
    setOrigin(`${Math.round(x)}px ${Math.round(y)}px`);
  }, [target.anchor]);

  /** What clearing means here. A weekend has no standing window to fall back
   *  to, so clearing it genuinely empties the day. */
  const backTo = onWeekend ? "" : (target.baseShiftCode ?? "");

  const when = new Date(`${target.rotaDate}T00:00:00`).toLocaleDateString("en-GB", {
    weekday: "short",
    day: "numeric",
    month: "short",
  });

  /** What the day holds now -- the part of the grid the dialog covers that
   *  the decision needs. */
  const heldShift = target.shiftCode ? shifts.find((s) => s.code === target.shiftCode) : undefined;
  const heldAway = target.absenceKindCode
    ? (allKinds ?? awayKinds).find((k) => k.code === target.absenceKindCode)
    : undefined;

  /** The days a rota pick will set, said where the pick is made: the range
   *  itself sits with leave, on the other side. */
  const spanLabel =
    span === 1
      ? new Date(`${firstDay}T00:00:00`).toLocaleDateString("en-GB", {
          weekday: "short",
          day: "numeric",
          month: "short",
        })
      : `${shortDate(firstDay)} – ${shortDate(lastDay)} · ${span} days`;

  /** One leave or allocation group: its heading and its tags. */
  const renderAwayGroup = (group: (typeof awayGroups)[number]): JSX.Element => (
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
        // No leave or allocation from a weekend: both are working-day
        // time. A span that starts on a weekday may still run across one;
        // the roster leaves the weekend inside it blank.
        const blocked = onWeekend && (kind.bucket === "LEAVE" || kind.bucket === "ALLOCATION");
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
            title={blocked ? `${tagHoverText(kind)} — working days only: start on a weekday` : `${tagHoverText(kind)} — working days in the span`}
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
            <span className="pk-l">
              {tagSideText(kind)}
              {kind.excludesFromTimesheet ? <span className="pk-off">Not on the timesheet</span> : null}
            </span>
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
  );

  return (
    <div
      className={`pk-scrim${closing ? " closing" : ""}`}
      // The backdrop itself, not a press that started inside the dialog and
      // ended out here (selecting a date's text, say).
      onMouseDown={(e) => {
        if (e.target === e.currentTarget) close();
      }}
    >
      <div
        className="picker open"
        ref={box}
        role="dialog"
        aria-modal="true"
        aria-label={`Change ${target.name}'s rota`}
        tabIndex={-1}
        style={origin ? { transformOrigin: origin } : undefined}
      >
        <div className="pk-head">
          <span className="pk-who">
            <b>{target.name}</b>
            <small>
              {teamNameOf(target.teamKey)} · {when}
            </small>
          </span>
          {heldShift || heldAway ? (
            <span className="pk-now" title="What this day holds now">
              {heldShift ? (
                <span className={`chip sm ${heldShift.colourToken}`}>
                  {heldShift.shortCode}
                  {target.tier ? ` ${target.tier}` : ""}
                </span>
              ) : null}
              {heldAway ? <span className={`chip sm ${heldAway.colourToken}`}>{heldAway.shortCode}</span> : null}
              <span className="pk-nowl">{heldAway?.label ?? heldShift?.label}</span>
            </span>
          ) : null}
          <button type="button" className="pk-x" onClick={close} aria-label="Close">
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

        {target.engagements?.length && onDismissEngagement ? (
          // Synced from the allocation app, so never edited here: a removal
          // takes the picked days (the range below) off this roster and the
          // timesheet, and Restore puts them back.
          <div className="pk-remove pk-eng" role="group" aria-label="Customer engagements from the allocation app">
            {target.engagements.map((en) => (
              <div className="pkeng-row" key={en.id}>
                <span className={`chip sm eng${en.dismissed ? " gone" : ""}`}>{en.label}</span>
                <span className="pkrm-t" title={en.title}>
                  <b>{en.dismissed ? "Removed from the roster" : "Customer engagement (allocation app)"}</b>
                  <small>{en.title}</small>
                </span>
                <button
                  type="button"
                  className={`pkrm-b${en.dismissed ? " back" : ""}`}
                  disabled={busy}
                  onClick={() => onDismissEngagement(en, firstDay, lastDay, en.dismissed)}
                >
                  {en.dismissed ? "Restore" : "Remove"} {firstDay === lastDay ? shortDate(firstDay) : `${shortDate(firstDay)} – ${shortDate(lastDay)}`}
                </button>
              </div>
            ))}
          </div>
        ) : null}

        <div className="pk-body">
          <section className="pk-col" aria-labelledby="pk-rota-t">
            <div className="pk-coltitle" id="pk-rota-t">
              Rota
              <small>{spanLabel}</small>
            </div>
            {/* Over a span, a weekday rotation will skip the weekend inside it, and
                the reader should know that before they click rather than after. */}
            {span > 1 ? (
              <p className="pk-note pk-skip">
                Days a rotation is not worked on are skipped — a weekday rotation over
                a week sets the five weekdays.
              </p>
            ) : null}
            {/* A rota whose windows the catalogue does not serve -- IaaS's sat
                switched off for months -- left this side blank, and a lead
                read the blank as not being allowed to roster. Say what it is. */}
            {nothingToRoster ? (
              <p className="pk-note pk-empty">
                {teamNameOf(target.teamKey)} has no rota windows set up yet, so no rotation or
                L1–L3 turn can be set here. Leave and allocations can still be marked.
              </p>
            ) : null}
            {/* Rotations and standing hours side by side, so the whole rota
                reads across one row. */}
            <div className="pk-cols">
            {groups.map((group) => (
              <div className="pk-grid" key={group.heading}>
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
              </div>
            ))}
            </div>

            {grid.length > 0 ? (
              <div className="pk-grid">
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
              </div>
            ) : null}
          </section>

          {/* The range heads this column: leave and allocations are almost
              always marked over days, so the dates sit with them. */}
          <section className="pk-col pk-awaycol" aria-labelledby="pk-away-t">
            <div className="pk-coltitle" id="pk-away-t">
              Leave &amp; allocations
            </div>
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
              {/* Common lengths, so a week of leave or a months-long move (the
                  SaaS Night cover is three to six) is one click rather than a lot
                  of clicking in a date field. Each only fills "To"; nothing is
                  marked until a tag is picked. */}
              <span className="pkr-len">
                {SPAN_PRESETS.map((p) => (
                  <button
                    key={p.label}
                    type="button"
                    onClick={() => {
                      const end = p.days ? isoPlusDays(firstDay, p.days - 1) : isoPlusMonths(firstDay, p.months);
                      if (endField.current) endField.current.value = end;
                      setUntil(end);
                    }}
                  >
                    {p.label}
                  </button>
                ))}
              </span>
              {/* The fields draw dates in the browser's own format (10/20/2026
                  on a US-English browser), which reads either way round; the
                  span is spelled out the way the rest of the page writes dates. */}
              <span className="pkr-when" aria-live="polite">
                {span === 1 ? spanLabel : `${shortDate(firstDay)} – ${shortDate(lastDay)}`}
              </span>
            </div>

            {/* Leave on one side; moving to another team and allocations on
                the other. Both take the From / To dates above, so a move or an
                allocation is marked over a period like leave is. */}
            <div className="pk-cols">
              {leaveGroups.length > 0 ? (
                <div className="pk-grid">{leaveGroups.map(renderAwayGroup)}</div>
              ) : null}
              {hasMoves || allocationGroups.length > 0 ? (
                <div className="pk-grid">
                  {hasMoves ? (
                    <>
                      <div className="pk-sec">Move to another team</div>
                      {/* A move is a rotation period, not a day: say so where
                          it is picked, with the dates it will cover. */}
                      <p className="pk-note pk-movenote">
                        For the dates above: {span === 1 ? spanLabel : `${shortDate(firstDay)} – ${shortDate(lastDay)} · ${span} days`}
                      </p>
                      {windowMoves.map(({ kind, shift }) => {
                        const ok = allowedOn(shift, firstDay);
                        const to = windowMoveTeam(kind, shift);
                        return (
                          <button
                            key={`window:${shift.code}`}
                            type="button"
                            className={`pk-c ${shift.code === target.shiftCode || kind.code === target.absenceKindCode ? "on" : ""} ${ok ? "" : "bad"}`}
                            disabled={!ok || busy}
                            // The same pick as the window itself: the span above, worked
                            // as that team's hours, and back with "Back to ...".
                            title={ok ? `${to}: works ${shift.label} for the span` : `${to} — ${notWorked(shift)}`}
                            onClick={() => onApply(shift.code, firstDay, lastDay)}
                          >
                            <span className={`chip sm ${shift.colourToken}`}>{shift.shortCode}</span>
                            <span className="pk-l">
                              Move to {to}
                              {ok ? null : <small className="pk-n">{notWorked(shift)}</small>}
                            </span>
                          </button>
                        );
                      })}
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
                          <span className="pk-l">Move to {asTeam(kind.movesToTeamKey ?? "")}</span>
                        </button>
                      ))}
                    </>
                  ) : null}
                  {allocationGroups.map(renderAwayGroup)}
                </div>
              ) : null}
            </div>

            <div className="pk-grid">

            {onCreateKind ? (
              tagOpen ? (
                <form
                  className="pk-newtag tf"
                  onSubmit={(e) => {
                    e.preventDefault();
                    // A custom tag is its short name: what the roster cell
                    // draws. Anything longer goes in the description.
                    const name = tag.label.trim();
                    if (!name) {
                      setTagError("A tag name is needed.");
                      return;
                    }
                    // Counted in characters (code points), as the server
                    // counts runes, so an emoji counts once on both sides.
                    if ([...name].length > TAG_NAME_MAX) {
                      setTagError(`A tag name is at most ${TAG_NAME_MAX} characters; put the rest in the description.`);
                      return;
                    }
                    const taken = (s: string) => s.trim().toLowerCase() === name.toLowerCase();
                    if (awayKinds.some((k) => taken(k.shortCode) || taken(k.label))) {
                      setTagError(`${name} is already a tag — pick another name.`);
                      return;
                    }
                    const shortCode = name;
                    const label = name;
                    setTagBusy(true);
                    setTagError("");
                    onCreateKind({
                      ...tag,
                      shortCode,
                      label,
                      description: tag.description.trim(),
                      // Leave shows on the timesheet as leave.
                      excludesFromTimesheet: tag.bucket === "ALLOCATION" && tag.excludesFromTimesheet,
                    })
                      .then(() => {
                        setTagOpen(false);
                        setTag({
                          shortCode: "",
                          label: "",
                          bucket: tag.bucket,
                          colourToken: tag.colourToken,
                          excludesFromTimesheet: false,
                          description: "",
                        });
                      })
                      .catch((err: unknown) =>
                        setTagError(err instanceof Error && err.message ? err.message : "The tag was not added."),
                      )
                      .finally(() => setTagBusy(false));
                  }}
                >
                  <div className="pk-sec">New tag</div>
                  <fieldset className="tf-group">
                    <legend>What the roster shows</legend>
                    <label>
                      <span>
                        Name
                        <RequiredMark />
                      </span>
                      <input
                        value={tag.label}
                        placeholder={`e.g. SRE, WFH (up to ${TAG_NAME_MAX} characters)`}
                        onChange={(e) => setTag({ ...tag, label: e.target.value })}
                        aria-label="Tag name"
                        aria-required="true"
                        aria-describedby="pk-name-help"
                      />
                    </label>
                    <p className="pk-note tf-help" id="pk-name-help">
                      What the roster shows on the day, up to {TAG_NAME_MAX} characters. Explain it in the description.
                    </p>
                    <label>
                      <span>Description</span>
                      <input
                        value={tag.description}
                        maxLength={TAG_DESCRIPTION_MAX}
                        placeholder="Optional, e.g. Working the SRE rota"
                        onChange={(e) => setTag({ ...tag, description: e.target.value })}
                        aria-label="Tag description"
                      />
                    </label>
                    <div className="tf-row">
                      <span>Colour</span>
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
                            <span className={`chip sm ${c}`}>Aa</span>
                          </button>
                        ))}
                      </div>
                    </div>
                  </fieldset>
                  <fieldset className="tf-group">
                    <legend>How it counts</legend>
                    <label className="tf-pick">
                      <span>Tag type</span>
                      <select
                        value={tag.bucket}
                        aria-label="Tag type"
                        aria-describedby="pk-type-help"
                        onChange={(e) => {
                          const bucket = e.target.value as NewAbsenceKind["bucket"];
                          // Leave shows on the timesheet as leave.
                          setTag({ ...tag, bucket, excludesFromTimesheet: bucket === "ALLOCATION" && tag.excludesFromTimesheet });
                        }}
                      >
                        <option value="ALLOCATION">Allocation</option>
                        <option value="LEAVE">Leave</option>
                      </select>
                    </label>
                    <p className="pk-note tf-help" id="pk-type-help">
                      {tag.bucket === "LEAVE"
                        ? "Leave: the person is away (e.g. a vacation). Listed under Leave in the picker and the legend."
                        : "Allocation: the person is working on something else (e.g. an SRE rotation, work from home). Listed under Allocations."}
                    </p>
                    <label className="tf-pick">
                      <span>Timesheet</span>
                      <select
                        value={tag.bucket === "LEAVE" ? "LEAVE" : tag.excludesFromTimesheet ? "EXCLUDE" : "INCLUDE"}
                        aria-label="Timesheet"
                        aria-describedby="pk-ts-help"
                        disabled={tag.bucket === "LEAVE"}
                        onChange={(e) => setTag({ ...tag, excludesFromTimesheet: e.target.value === "EXCLUDE" })}
                      >
                        {tag.bucket === "LEAVE" ? (
                          <option value="LEAVE">Shown as leave</option>
                        ) : (
                          <>
                            <option value="INCLUDE">Include in timesheet</option>
                            <option value="EXCLUDE">Exclude from timesheet</option>
                          </>
                        )}
                      </select>
                    </label>
                    <p className="pk-note tf-help" id="pk-ts-help">
                      {tag.bucket === "LEAVE"
                        ? "Leave days show on the timesheet as leave."
                        : tag.excludesFromTimesheet
                          ? "People with this tag are left off the timesheet on those days (e.g. an SRE rotation)."
                          : "People with this tag stay on the timesheet as usual (e.g. work from home)."}
                    </p>
                  </fieldset>
                  <div className="tf-preview" aria-label="Preview of the new tag">
                    <span className="tf-preview-h">Preview</span>
                    <span className={`chip sm ${tag.colourToken}`}>{tag.label.trim() || "Name"}</span>
                    <span className="tf-preview-d">
                      {tag.description.trim() || (tag.label.trim() ? "No description" : "Type a name to see the tag")}
                    </span>
                    <span className="tf-preview-t">
                      {tag.bucket === "LEAVE"
                        ? "Leave · shown as leave"
                        : tag.excludesFromTimesheet
                          ? "Allocation · not on the timesheet"
                          : "Allocation · on the timesheet"}
                    </span>
                  </div>
                  <p className="pk-note">
                    {tagFamily
                      ? `For the ${tagFamily} family only: only ${tagFamily} rosters will offer this tag.`
                      : "Shared with every team once added."}
                  </p>
                  <div className="tf-err" role="alert">{tagError}</div>
                  <div className="pk-tagact">
                    <button type="button" className="btn sm" onClick={() => setTagOpen(false)}>
                      Cancel
                    </button>
                    <button type="submit" className="btn sm primary" disabled={tagBusy || !tag.label.trim()}>
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
            </div>
          </section>
        </div>

        <div className="pk-foot">
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
      </div>
    </div>
  );
}
