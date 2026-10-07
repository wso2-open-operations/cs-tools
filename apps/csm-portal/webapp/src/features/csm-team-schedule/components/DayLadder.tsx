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

import { useEffect, useLayoutEffect, useMemo, useRef, useState, type JSX } from "react";
import RotaPicker, { type RotaOption } from "./RotaPicker";
import type { ScheduleAbsence, ScheduleAbsenceKind, ScheduleAssignment, ScheduleShift, ScheduleZone, RotaFamily } from "../types";
import {
  dayLabel,
  groupBy,
  initialsOf,
  partsInZone,
  placeOnDay,
  timeOf,
  toIsoDate,
} from "../utils/rota";
import { accentOf } from "../utils/rotaHues";
import { useTeamColour, useTeamName } from "../utils/teamColourContext";

/** Pixels per hour, as the prototype draws it. */
const HOUR_PX = 38;

/**
 * The narrowest a column of names can get before a name stops being a name.
 *
 * Three zones each split into escalation and ordinary hours is six columns; in
 * the width of a browser that is about ninety pixels each, which turns every
 * engineer into "Apollo Engin...". The lanes widen past the viewport instead
 * and the day scrolls sideways, which is what a calendar does when a day is
 * busy.
 *
 * 170 is what lets three zones, each split in two, plus the off-rota column
 * fit a laptop without scrolling sideways -- measured at 1350px of usable
 * width. It only works because the "others in the zone" cards drop their team
 * tag: inside a zone lane the avatar colour already says which team someone is
 * on, and the full name and team stay on the row's tooltip.
 */
const MIN_COLUMN_PX = 148;

/** A lane holding a single column of cards -- TZ3, whose one card is its
 *  whole night -- still has to show an engineer's full name, so it gets a
 *  floor of its own above one column's, and a little more than one column's
 *  share of the width. */
const SINGLE_LANE_MIN_PX = 220;
const SINGLE_LANE_SHARE = 1.25;

/** How wide a lane must be, and what share of the spare width it takes. */
function laneSizing(columns: number): { minWidth: number; flexGrow: number } {
  const n = Math.max(1, columns);
  return n === 1
    ? { minWidth: SINGLE_LANE_MIN_PX, flexGrow: SINGLE_LANE_SHARE }
    : { minWidth: n * MIN_COLUMN_PX, flexGrow: n };
}

const px = (minutes: number): number => Math.round((minutes / 60) * HOUR_PX);

export interface LadderLane {
  /** Zone code for SRE, or the single rotations lane for CRE. */
  name: string;
  sub?: string;
  colour?: string;
  assignments: ScheduleAssignment[];
  /**
   * How the lane is laid out.
   *
   * `flat` is one column of cards, one per window -- what CRE wants, since
   * everything it runs happens on one clock.
   *
   * `zone` splits the lane in two, as the reviewed design does: the escalation
   * card with a row per tier, and beside it everyone in the zone on ordinary
   * hours. Reading "who is L2 in TZ2" off a list sorted by window means
   * scanning; off a labelled row it is immediate.
   */
  layout?: "flat" | "zone";
}

interface DayLadderProps {
  day: Date;
  /** The clock the reader has picked; blocks are placed on it. */
  tz: string;
  /** Short name for that clock, shown at the head of the hour axis. */
  zoneLabel: string;
  lanes: LadderLane[];
  shifts: Map<string, ScheduleShift>;
  zones: ScheduleZone[];
  absences: ScheduleAbsence[];
  absenceKinds: ScheduleAbsenceKind[];
  /** The page's own group and team state. Rendered here as well as in the
   *  toolbar -- one control in two places, as the prototype has it. */
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
}

interface BlockRow {
  /** A heading inside the card; shown when a card carries more than one row. */
  label: string;
  list: ScheduleAssignment[];
}

interface Block {
  key: string;
  shift?: ScheduleShift;
  /** Overrides the shift label when a card spans several windows. */
  title?: string;
  chipToken?: string;
  startMin: number;
  endMin: number;
  note: string;
  rows: ScheduleAssignment[];
  /** Labelled rows, for a card that holds more than one kind of person. */
  sections?: BlockRow[];
  /** Drop the trailing tag on each name, where the surrounding card already
   *  says what it would have said. */
  hideTags?: boolean;
  /** Lay the sections out as columns rather than stacked -- for a card that
   *  holds several windows with the same hours, in a lane wide enough for it. */
  sideBySide?: boolean;
  /** The sections are the escalation ladder, L1 to L3: every tier is shown,
   *  empty ones included, each ruled off from the next. */
  tiered?: boolean;
}

/** The escalation tiers, in the order the rota talks about them. */
const TIERS = ["L1", "L2", "L3"] as const;

/**
 * What a card says about itself when it is cut by midnight: a block running
 * off the bottom of today, or one that arrived from yesterday evening.
 */
function noteFor(
  placement: { continuesNextDay: boolean; startedPreviousDay: boolean },
  row: ScheduleAssignment,
  tz: string,
): string {
  if (placement.continuesNextDay) {
    return `runs to ${timeOf(row.endsAt, tz)} · ${dayLabel(row.endsAt, tz)}`;
  }
  if (placement.startedPreviousDay) {
    return `started ${timeOf(row.startsAt, tz)} · ${dayLabel(row.startsAt, tz)}`;
  }
  return "";
}

/** How many names a card shows before it counts the rest. */
/**
 * The off-rota column's width on a day with a single lane (CRE). The narrow
 * width it has elsewhere is SRE's budget -- three zone lanes have to fit a
 * laptop -- but CRE's one lane leaves room, and off rota is where CRE carries
 * the most: leave by team, allocations, and who each is for.
 */
const OFF_ROTA_WIDE_PX = 340;

/**
 * Off rota's width on a day with several lanes (SRE's zones). The stylesheet
 * sizes the column; this is the allowance the ladder's minimum width makes for
 * it. The two move together -- 80px over the original 240 on both sides -- so
 * off rota widens by adding to the day rather than by taking from the zone
 * lanes beside it.
 */
const OFF_ROTA_PX = 320;

const NAME_LIMIT = 12;

/**
 * The day as the prototype draws it: an hour axis down the left, a lane per
 * time zone (or one lane for CRE), cards placed on the grid, a line at the
 * current time, and the off-rota column pinned beside it.
 *
 * A window that crosses midnight is drawn on both days it touches, because the
 * crew that started last night is still working this morning and a reader
 * looking at today needs to see them.
 */
export default function DayLadder({
  day,
  lanes,
  shifts,
  absences,
  absenceKinds,
  tz,
  zoneLabel,
  family,
  onFamilyChange,
  teamKey,
  onTeamKeyChange,
  teams,
  families,
  rotas,
  rotaCode,
  onRotaChange,
}: DayLadderProps): JSX.Element {
  const teamNameOf = useTeamName();
  // Who is off the rota. Two kinds of span are left out:
  //  - a stint that is rota work on another team -- the Brazil rotation,
  //    worked on the Americas rota -- which is not time off it;
  //  - a retired tag, such as Onboarding: no longer a reason anybody is away
  //    today. Its old days keep their label on the month roster, which is
  //    the record; this column answers who to plan around now.
  const offRota = useMemo(() => {
    const leaveOut = new Set(absenceKinds.filter((k) => k.worksRotaThere || k.retired).map((k) => k.code));
    return absences.filter((a) => !leaveOut.has(a.kindCode));
  }, [absences, absenceKinds]);
  const wrapRef = useRef<HTMLDivElement | null>(null);
  const touched = useRef(false);

  const built = useMemo(() => {
    /** One card per window, per contiguous piece of it on this day. */
    const cardsPerWindow = (laneName: string, rows: ScheduleAssignment[]): Block[] => {
      const out: Block[] = [];
      for (const [shiftCode, shiftRows] of groupBy(rows, (a) => a.shiftCode)) {
        for (const [placementKey, placed] of groupBy(shiftRows, (a) => {
          const p = placeOnDay(a, day, tz);
          return p ? `${p.startMin}-${p.endMin}` : "off";
        })) {
          if (placementKey === "off") continue;
          const p = placeOnDay(placed[0], day, tz);
          if (!p) continue;
          out.push({
            key: `${laneName}:${shiftCode}:${placementKey}`,
            shift: shifts.get(shiftCode),
            startMin: p.startMin,
            endMin: p.endMin,
            note: noteFor(p, placed[0], tz),
            rows: placed,
          });
        }
      }
      return mergeSameHours(out).sort((a, b) => a.startMin - b.startMin);
    };

    /**
     * One card for windows that cover exactly the same hours.
     *
     * Cards are placed by time alone, so two windows with the same hours --
     * the weekend on-call and the Americas cover, both 21:00-06:00; the
     * morning and its on-call, both 06:00-09:00 -- were drawn on top of each
     * other, titles and names printed through one another. Held in one card
     * with a row per window instead, the way the escalation card holds a row
     * per tier: the hours are said once and each window names its own people.
     *
     * Not when a window is crowded: a card that big shows its teams rather
     * than its names, and folding a second window into it would lose that.
     * Those, and hours that overlap without matching, are set side by side
     * by packIntoColumns below.
     */
    const mergeSameHours = (cards: Block[]): Block[] => {
      const out: Block[] = [];
      for (const group of groupBy(cards, (c) => `${c.startMin}-${c.endMin}`).values()) {
        if (group.length < 2) {
          out.push(...group);
          continue;
        }
        const crowded = group.some((c) => c.rows.length > NAME_LIMIT);
        if (crowded) {
          // A card this size lists its teams rather than its names, and for
          // windows that differ only in which team works them that team list
          // is already the whole distinction -- regular hours and the India
          // region shift are the same nine-to-five in the same zone, and the
          // second card said nothing the first one's Phoenix row would not.
          // So they fold together and the teams do the telling.
          //
          // On-call and escalation windows never fold in, whatever the hours:
          // being on call is not a fact about which team you are on, and a
          // team list cannot say it. Those stay their own card.
          const plain = group.filter((c) => !c.shift?.isOnCall && !c.shift?.isEscalation);
          if (plain.length < 2) {
            out.push(...group);
            continue;
          }
          // The biggest window leads, so the card keeps the name and colour a
          // reader already knows it by.
          const ordered = [...plain].sort((a, b) => b.rows.length - a.rows.length);
          out.push({
            ...ordered[0],
            key: ordered.map((c) => c.key).join("+"),
            rows: ordered.flatMap((c) => c.rows),
          });
          out.push(...group.filter((c) => !plain.includes(c)));
          continue;
        }
        // The first window leads -- its colour and chip -- and an on-call
        // variant follows the window it is on call for.
        const ordered = [...group].sort(
          (a, b) => Number(a.shift?.isOnCall ?? false) - Number(b.shift?.isOnCall ?? false),
        );
        const lead = ordered[0];
        out.push({
          ...lead,
          key: ordered.map((c) => c.key).join("+"),
          title: ordered.map((c) => c.shift?.label ?? c.rows[0].shiftCode).join(" · "),
          rows: ordered.flatMap((c) => c.rows),
          // Side by side: the roles are peers, and a short evening card
          // cannot stack two of them without clipping the second.
          sideBySide: true,
          sections: ordered.map((c) => ({
            label: c.shift?.label ?? c.rows[0].shiftCode,
            list: c.rows,
          })),
        });
      }
      return out;
    };

    /**
     * Cards whose hours overlap without matching go side by side rather than
     * on top of each other: each takes the first column it fits in, the way a
     * calendar lays out a double-booked afternoon. One column when nothing
     * overlaps, which is the common case.
     */
    const packIntoColumns = (cards: Block[]): Block[][] => {
      const columns: Block[][] = [];
      for (const card of [...cards].sort((a, b) => a.startMin - b.startMin)) {
        const free = columns.find((col) => col[col.length - 1].endMin <= card.startMin);
        if (free) free.push(card);
        else columns.push([card]);
      }
      return columns.length > 0 ? columns : [[]];
    };

    /**
     * One escalation card per stretch of the day, carrying a row per tier.
     *
     * The tiers run on slightly different windows -- TZ1's L1 hands over at
     * 13:30 while the zone runs to 15:00 -- so the card spans the union of
     * them and the rows say who holds what. Drawing one card per window
     * instead puts L1 and L2 side by side in a column too narrow to read,
     * which is what the reviewed design avoids.
     */
    const escalationCards = (laneName: string, rows: ScheduleAssignment[]): Block[] => {
      const placed = rows
        .map((a) => ({ a, p: placeOnDay(a, day, tz) }))
        .filter((x): x is { a: ScheduleAssignment; p: NonNullable<ReturnType<typeof placeOnDay>> } => x.p !== null);
      if (placed.length === 0) return [];

      // A zone that crosses midnight shows up twice: the piece it worked
      // yesterday and the piece it works today. They are different cards.
      const bySegment = groupBy(placed, (x) => (x.p.startedPreviousDay ? "carried-in" : "own"));

      return [...bySegment.entries()]
        .map(([segment, items]) => {
          const startMin = Math.min(...items.map((x) => x.p.startMin));
          const endMin = Math.max(...items.map((x) => x.p.endMin));
          const earliest = items.reduce((a, b) => (a.p.startMin <= b.p.startMin ? a : b));
          // Every tier, empty ones included: an escalation that has nobody at
          // L3 says so, rather than reading as though L3 did not exist.
          const sections: BlockRow[] = TIERS.map((tier) => ({
            label: `${tier} escalation`,
            list: items.filter((x) => x.a.tier === tier).map((x) => x.a),
          }));

          return {
            key: `${laneName}:escalation:${segment}`,
            title: "Escalation",
            chipToken: "L1",
            startMin,
            endMin,
            note: noteFor(earliest.p, earliest.a, tz),
            rows: items.map((x) => x.a),
            sections,
            tiered: true,
          };
        })
        .sort((a, b) => a.startMin - b.startMin);
    };

    return lanes.map((lane) => {
      if (lane.layout !== "zone") {
        return { ...lane, columns: packIntoColumns(cardsPerWindow(lane.name, lane.assignments)) };
      }
      const tiered = lane.assignments.filter((a) => a.tier);
      const ordinary = lane.assignments.filter((a) => !a.tier);
      const others = cardsPerWindow(lane.name, ordinary).map((c) => ({
        ...c,
        title: `Others in ${lane.name}`,
        hideTags: true,
      }));
      // A zone whose regular hours are the same stretch as its escalation
      // window -- TZ3, 21:00-06:00, with no middle of the day to cover -- is
      // one card, not two side by side saying the same hours twice: the
      // regular-hours people become a last section of the escalation card.
      // TZ1 and TZ2 work regular hours wider than their escalation block, so
      // theirs stay cards of their own.
      const escalation = escalationCards(lane.name, tiered);
      const apart = others.filter((o) => {
        const same = escalation.find((e) => e.startMin === o.startMin && e.endMin === o.endMin);
        if (!same) return true;
        same.sections = [...(same.sections ?? []), { label: "Regular hours", list: o.rows }];
        same.rows = [...same.rows, ...o.rows];
        return false;
      });
      // An empty column is left out rather than drawn: a zone with nobody
      // on ordinary hours that day should not hold half its width open for
      // them, while the zones beside it cut their card titles short.
      const columns = [escalation, apart].filter((c) => c.length > 0);
      return { ...lane, columns: columns.length > 0 ? columns : [[]] };
    });
  }, [day, lanes, shifts, tz]);

  const nowMin = useMemo(() => {
    const here = partsInZone(new Date().toISOString(), tz);
    return here.date === toIsoDate(day) ? here.minutes : null;
  }, [day, tz]);

  // Open at the hour the reader is in, the way a calendar does. Once they have
  // scrolled it is theirs; re-centring under them would be maddening.
  useEffect(() => {
    const el = wrapRef.current;
    if (!el || touched.current) return;
    const at = px(nowMin ?? 8 * 60);
    const go = (): void => {
      if (!touched.current) el.scrollTop = Math.max(0, at - Math.round(el.clientHeight * 0.28));
    };
    go();
    const t = window.setTimeout(go, 120);
    return () => window.clearTimeout(t);
  }, [nowMin]);

  // Wide enough that every column can hold a name, plus the hour axis and the
  // off-rota column.
  const ladderWidth = useMemo(
    () =>
      56 +
      built.reduce((sum, lane) => sum + laneSizing(lane.columns.length).minWidth + 12, 0) +
      (built.length === 1 ? OFF_ROTA_WIDE_PX : OFF_ROTA_PX),
    [built],
  );

  const hours: number[] = [];
  for (let h = 0; h <= 24; h += 2) hours.push(h);

  /** Distinct people on the rota today, not cards: one engineer holding an
   *  escalation tier and an ordinary window is one engineer. */
  const headcount = useMemo(
    () => new Set(lanes.flatMap((l) => l.assignments.map((a) => a.engineer.userId))).size,
    [lanes],
  );

  return (
    <>
      {/* The card's own head, as the prototype has it: which group you are
          looking at, and which team within it, stated where the day is read
          rather than only in the page toolbar above. */}
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
          On the rota <span className="count">{headcount}</span>
        </h2>

        <div className="tools">
          <span className="rng">
            {/* `day` is a local calendar date, the one the rota was fetched for.
                Converting it through the profile zone would name the day
                before it for a reader whose profile is west of the browser. */}
            {day.toLocaleDateString("en-GB", { weekday: "short", day: "numeric", month: "short", year: "numeric" })}
            {day.getDay() === 0 || day.getDay() === 6 ? " · weekend" : ""}
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

    <div className="ladwrap" ref={wrapRef} onScroll={() => (touched.current = true)}>
      {/* Inside the scroller, not above it. Sitting outside, the headings kept
          their own horizontal position while the lanes moved under them, so
          the moment the day was wider than the window TZ2's heading sat over
          TZ1's column. Sticky to the top keeps them visible while the day
          scrolls down; being in the same scroller keeps them over the right
          column while it scrolls across. */}
      <div className={`ladhd${built.length === 1 ? " onelane" : ""}`} style={{ minWidth: ladderWidth }}>
        {/* The clock sits at the head of the column of times it describes,
            aligned with the hours below it -- the same place a table puts a
            unit. A sentence above the card said the same thing in a whole row
            of space, nowhere near the numbers it was about. */}
        <span className="axistz" title={`${tz} — from your CSM profile`}>
          {zoneLabel}
        </span>
        {built.map((lane) => (
          <div
            key={lane.name}
            className="lnh"
            style={{
              ["--zc" as string]: lane.colour ?? "var(--faint)",
              // The same floor and share its lane has. Without them the heading
              // row and the lane row distribute their flex space differently
              // and each heading sits off the column it names.
              ...laneSizing(lane.columns.length),
            }}
          >
            <span className="zchip">{lane.name}</span>
            <span className="lnt">{lane.sub ?? ""}</span>
          </div>
        ))}
        <div className="lnh offhd" style={{ ["--zc" as string]: "var(--muted)" }}>
          <span className="zchip off">Off rota</span>
          <span className="lnt">{offRota.length} not available</span>
        </div>
      </div>

      <div
        className={`ladder${built.length === 1 ? " onelane" : ""}`}
        style={{ height: px(1440) + 8, minWidth: ladderWidth }}
      >
          <div className="lax">
            {hours.map((h) => (
              <span key={h} className="hr" style={{ top: px(h * 60) }}>
                {String(h % 24).padStart(2, "0")}:00
              </span>
            ))}
          </div>
          <div className="lgrid">
            {hours.map((h) => (
              <span key={h} className="gl" style={{ top: px(h * 60) }} />
            ))}
          </div>

          <div className="lanes">
            {built.map((lane, laneIndex) => (
              <div
                key={lane.name}
                className="lane"
                style={{
                  ["--zc" as string]: lane.colour ?? "var(--faint)",
                  // A share of the day's width per column of cards it holds,
                  // not an equal share per zone: a zone with two cards side
                  // by side needs twice the room of one with a single card.
                  ...laneSizing(lane.columns.length),
                }}
              >
                <div className={`lncols${lane.columns.length < 2 ? " one" : ""}`}>
                  {lane.columns.map((column, columnIndex) => (
                    <div className="lncol" key={columnIndex}>
                      {column.map((block) => (
                        <LadderBlock key={block.key} block={block} tz={tz} />
                      ))}
                    </div>
                  ))}
                </div>
                {/* The time indicator belongs to the rota, not to the off-rota
                    column, so only the first lane carries its label. */}
                {nowMin !== null ? (
                  <div className={`lnow${laneIndex === 0 ? " first" : ""}`} style={{ top: px(nowMin) }}>
                    {laneIndex === 0 ? (
                      <i>
                        {String(Math.floor(nowMin / 60)).padStart(2, "0")}:
                        {String(nowMin % 60).padStart(2, "0")}
                      </i>
                    ) : null}
                  </div>
                ) : null}
              </div>
            ))}

            <div className="lane offlane" style={{ ["--zc" as string]: "var(--muted)" }}>
              <OffRotaStack
                // Leave is not taken on a weekend, so a span that runs across
                // one does not list its holder as away on the Saturday.
                absences={
                  day.getDay() === 0 || day.getDay() === 6
                    ? offRota.filter(
                        (a) => absenceKinds.find((k) => k.code === a.kindCode)?.bucket !== "LEAVE",
                      )
                    : offRota
                }
                kinds={absenceKinds}
              />
            </div>
        </div>
      </div>
    </div>
    </>
  );
}

/** One card on the ladder: its hours as a chip, its label, and who is on it. */
function LadderBlock({ block, tz }: { block: Block; tz: string }): JSX.Element {
  const token = block.chipToken ?? block.shift?.colourToken ?? "";
  const first = block.rows[0];
  const shown = block.rows.slice(0, NAME_LIMIT);
  const hidden = block.rows.length - shown.length;
  // A card with more people than it can list is not a list -- seventy names is
  // nothing anyone reads. The teams become the list, and a team hands over its
  // own people when the cursor rests on it.
  const crowded = block.rows.length > NAME_LIMIT;

  // A card is as tall as its hours, and some hours are short: the Americas
  // weekend night shows only 21:00-24:00 on the Sunday it starts, three hours
  // that held one name and now hold two roles. What did not fit was clipped
  // with nothing to say it was there. So a card measures itself, says when it
  // is holding more than it shows, and opens to its full height on hover,
  // focus or a tap.
  const ref = useRef<HTMLDivElement | null>(null);
  const [clipped, setClipped] = useState(false);
  const [open, setOpen] = useState(false);
  useLayoutEffect(() => {
    const el = ref.current;
    if (!el) return;
    const measure = (): void => {
      if (!el.classList.contains("open")) setClipped(el.scrollHeight > el.clientHeight + 2);
    };
    measure();
    const ro = new ResizeObserver(measure);
    ro.observe(el);
    return () => ro.disconnect();
  }, [block]);
  // Late in the day there is no room below -- the ladder ends at midnight -- so
  // a card there opens upward from its own end instead.
  const opensUp = block.startMin >= 18 * 60;
  const isOpen = clipped && open;

  return (
    <div
      ref={ref}
      className={`zblk ${token.toLowerCase()}${clipped ? " canopen" : ""}${isOpen ? " open" : ""}`}
      style={{
        ...(isOpen && opensUp
          ? { bottom: `calc(100% - ${px(block.endMin)}px)`, top: "auto" }
          : { top: px(block.startMin) }),
        ...(isOpen
          ? { height: "auto", minHeight: px(block.endMin - block.startMin) }
          : { height: px(block.endMin - block.startMin) }),
        ["--zc" as string]: accentOf(token),
      }}
      tabIndex={clipped ? 0 : undefined}
      aria-expanded={clipped ? isOpen : undefined}
      onMouseEnter={clipped ? () => setOpen(true) : undefined}
      onMouseLeave={clipped ? () => setOpen(false) : undefined}
      onFocus={clipped ? () => setOpen(true) : undefined}
      onBlur={clipped ? () => setOpen(false) : undefined}
      onClick={clipped ? () => setOpen((o) => !o) : undefined}
    >
      <div className="zbh">
        <span className={`chip sm ${token}`}>
          {timeOf(first.startsAt, tz)} – {timeOf(first.endsAt, tz)}
        </span>
        <b className="zbt" title={block.title ?? block.shift?.label ?? first.shiftCode}>
          {block.title ?? block.shift?.label ?? first.shiftCode}
        </b>
        <span className="zbn">{block.rows.length}</span>
      </div>
      {block.note ? <div className="zbw">{block.note}</div> : null}
      <div
        className={`zbp${block.sections && block.sideBySide ? " sides" : ""}${block.tiered ? " tiers" : ""}`}
      >
        {block.sections ? (
          // A row per tier, each with its own heading: "who is L2 here" is a
          // question the reader should not have to answer by scanning.
          block.sections.map((section) => (
            <div className="zsec" key={section.label}>
              <span className="zsl">
                {section.label}
                <b>{section.list.length}</b>
              </span>
              {section.list.map((a) => (
                // The tier is the row's heading, so repeating it against every
                // name costs width that the name itself needs.
                <NameRow key={a.id} assignment={a} hideTag />
              ))}
              {section.list.length === 0 ? <span className="gap">Nobody rostered</span> : null}
            </div>
          ))
        ) : crowded ? (
          <TeamSplit rows={block.rows} />
        ) : (
          <>
            <div className="zsec">
              {shown.map((a) => (
                <NameRow key={a.id} assignment={a} hideTag={block.hideTags} />
              ))}
            </div>
            {hidden > 0 ? <span className="lmore">+{hidden} more</span> : null}
          </>
        )}
      </div>
      {clipped && !isOpen ? (
        <span className="zbmore" aria-hidden="true">
          Show all {block.rows.length}
        </span>
      ) : null}
    </div>
  );
}

/**
 * The teams down the side, and the team the cursor is on filling the rest of
 * the card. The first team is open at rest, so the card is never a wall of
 * nothing, and hovering only swaps which pane is shown -- nothing re-renders
 * underneath the cursor.
 */
function TeamSplit({ rows }: { rows: ScheduleAssignment[] }): JSX.Element {
  const teamColourOf = useTeamColour();
  const teamNameOf = useTeamName();
  const byTeam = useMemo(() => [...groupBy(rows, (r) => r.teamKey).entries()], [rows]);
  const [active, setActive] = useState<string>(() => byTeam[0]?.[0] ?? "");
  const current = byTeam.find(([team]) => team === active) ?? byTeam[0];

  return (
    <div className="teamsplit">
      <div className="teamlist">
        {byTeam.map(([team, teamRows]) => (
          <span
            key={team}
            className={`tl${team === current?.[0] ? " on" : ""}`}
            tabIndex={0}
            onMouseEnter={() => setActive(team)}
            onFocus={() => setActive(team)}
            onClick={() => setActive(team)}
          >
            <i style={{ background: teamColourOf(team) }} />
            <span className="tn" title={teamNameOf(team)}>
              {teamNameOf(team)}
            </span>
            <b>{teamRows.length}</b>
          </span>
        ))}
      </div>
      <div className="teampane">
        {current ? (
          <div className="tlp on">
            <div className="tlph">
              <i style={{ background: teamColourOf(current[0]) }} />
              {current[0]}
              <b>{current[1].length}</b>
            </div>
            <div className="tlpn">
              {current[1].map((a) => (
                <NameRow key={a.id} assignment={a} hideTag />
              ))}
            </div>
          </div>
        ) : null}
      </div>
    </div>
  );
}

/** One person in a card. */
function NameRow({
  assignment,
  hideTag,
}: {
  assignment: ScheduleAssignment;
  /** Inside a team pane the team is already the heading, so repeating it on
   *  every row would be noise. */
  hideTag?: boolean;
}): JSX.Element {
  const teamColourOf = useTeamColour();
  const teamNameOf = useTeamName();
  /* Only what the row does not already say. A tier ("L2") and on-call status
     are not readable anywhere else on the card; the team is -- it is the
     avatar's colour, and the row's own tooltip. Spelling it out a third time
     cost about a quarter of the width the name needed, so the name was the
     thing that got cut. */
  const tag = assignment.tier ?? (assignment.isOnCall ? "OC" : null);
  return (
    <span className="lnm" title={`${assignment.engineer.name} · ${teamNameOf(assignment.teamKey)}`}>
      <span className="av" style={{ background: teamColourOf(assignment.teamKey) }}>
        {initialsOf(assignment.engineer.name)}
      </span>
      <span className="who">{assignment.engineer.name}</span>
      {assignment.engineer.isLead ? <i className="tag lead-t">Lead</i> : null}
      {hideTag || !tag ? null : (
        <i className={`tier-t${assignment.isOnCall ? " oc-t" : ""}`}>{tag}</i>
      )}
    </span>
  );
}

/** The off-rota column: one card per reason, leave grouped by team. */
function OffRotaStack({
  absences,
  kinds,
}: {
  absences: ScheduleAbsence[];
  kinds: ScheduleAbsenceKind[];
}): JSX.Element {
  const teamColourOf = useTeamColour();
  const teamNameOf = useTeamName();
  const byKind = groupBy(absences, (a) => a.kindCode);

  // Leave first, then allocations.
  //
  // Both take someone off the rota, but they answer different questions. Who
  // is on leave is what a lead scans this column for -- it is the cover they
  // may have to arrange today. An allocation is planned work someone is doing
  // instead; useful to know, but it is not a gap. The catalogue's own order
  // decides the rest, so two kinds in the same bucket keep their usual
  // sequence.
  const BUCKET_ORDER: Record<string, number> = { LEAVE: 0, ALLOCATION: 1, EXCLUDED: 2 };
  const cards = kinds
    .map((kind) => ({ kind, rows: byKind.get(kind.code) ?? [] }))
    .filter((c) => c.rows.length > 0)
    .sort((a, b) => (BUCKET_ORDER[a.kind.bucket] ?? 9) - (BUCKET_ORDER[b.kind.bucket] ?? 9));

  return (
    <div className="offstack">
      {cards.length === 0 ? <div className="offnone">nobody today</div> : null}
      {cards.map(({ kind, rows }) => {
        const byTeam = kind.bucket === "LEAVE" ? groupBy(rows, (r) => r.teamKey) : null;
        return (
          <div key={kind.code} className={`offcard ${kind.colourToken}`}>
            <div className="offh">
              <span className={`chip sm ${kind.colourToken}`}>{kind.shortCode}</span>
              <span className="offl" title={kind.label}>
                {kind.label}
              </span>
              <b>{rows.length}</b>
            </div>
            {byTeam ? (
              [...byTeam.entries()].map(([team, teamRows]) => (
                <div key={team}>
                  <div className="offgh">
                    {teamNameOf(team)}
                    <b>{teamRows.length}</b>
                  </div>
                  <div className="offp">
                    {teamRows.map((r) => (
                      <span className="lnm" key={r.id} title={r.engineer.name}>
                        <span className="av" style={{ background: teamColourOf(r.teamKey) }}>
                          {initialsOf(r.engineer.name)}
                        </span>
                        <span className="who">{r.engineer.name}</span>
                      </span>
                    ))}
                  </div>
                </div>
              ))
            ) : (
              <div className="offp">
                {rows.map((r) => (
                  <span
                    className="lnm"
                    key={r.id}
                    title={[r.engineer.name, teamNameOf(r.teamKey), r.allocatedTo].filter(Boolean).join(" · ")}
                  >
                    <span className="av" style={{ background: teamColourOf(r.teamKey) }}>
                      {initialsOf(r.engineer.name)}
                    </span>
                    <span className="who">{r.engineer.name}</span>
                    {/* Who the time is for, where it is known -- the answer to
                        "can I reach them" more often than their team is. */}
                    <i className="tier-t">{r.allocatedTo ?? teamNameOf(r.teamKey)}</i>
                  </span>
                ))}
              </div>
            )}
          </div>
        );
      })}
    </div>
  );
}
