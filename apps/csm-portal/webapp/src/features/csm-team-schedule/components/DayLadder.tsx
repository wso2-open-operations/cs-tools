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

import { useEffect, useMemo, useRef, useState, type JSX } from "react";
import type { ScheduleAbsence, ScheduleAbsenceKind, ScheduleAssignment, ScheduleShift, ScheduleZone } from "../types";
import {
  dayLabel,
  groupBy,
  initialsOf,
  partsInZone,
  placeOnDay,
  timeOf,
  toIsoDate,
} from "../utils/rota";
import { teamColour } from "../utils/rotaHues";

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
  family: "CRE" | "SRE";
  onFamilyChange: (family: "CRE" | "SRE") => void;
  teamKey: string;
  onTeamKeyChange: (teamKey: string) => void;
  teams: string[];
  /** CRE and SRE in the order they should read -- the reader's own group
   *  first, because the first of a pair reads as the default. */
  families: readonly ("CRE" | "SRE")[];
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
}: DayLadderProps): JSX.Element {
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
      return out.sort((a, b) => a.startMin - b.startMin);
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
          const sections: BlockRow[] = TIERS.map((tier) => ({
            label: `${tier} escalation`,
            list: items.filter((x) => x.a.tier === tier).map((x) => x.a),
          })).filter((r) => r.list.length > 0);

          return {
            key: `${laneName}:escalation:${segment}`,
            title: "Escalation",
            chipToken: "L1",
            startMin,
            endMin,
            note: noteFor(earliest.p, earliest.a, tz),
            rows: items.map((x) => x.a),
            sections,
          };
        })
        .sort((a, b) => a.startMin - b.startMin);
    };

    return lanes.map((lane) => {
      if (lane.layout !== "zone") {
        return { ...lane, columns: [cardsPerWindow(lane.name, lane.assignments)] };
      }
      const tiered = lane.assignments.filter((a) => a.tier);
      const ordinary = lane.assignments.filter((a) => !a.tier);
      const others = cardsPerWindow(lane.name, ordinary).map((c) => ({
        ...c,
        title: `Others in ${lane.name}`,
        hideTags: true,
      }));
      return { ...lane, columns: [escalationCards(lane.name, tiered), others] };
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
      built.reduce((sum, lane) => sum + Math.max(1, lane.columns.length) * MIN_COLUMN_PX + 12, 0) +
      240,
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
                {t.charAt(0).toUpperCase() + t.slice(1)}
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
      <div className="ladhd" style={{ minWidth: ladderWidth }}>
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
              // The same floor its lane has. Without it the heading row and
              // the lane row distribute their flex space differently and each
              // heading sits a few pixels off the column it names.
              minWidth: Math.max(1, lane.columns.length) * MIN_COLUMN_PX,
            }}
          >
            <span className="zchip">{lane.name}</span>
            <span className="lnt">{lane.sub ?? ""}</span>
          </div>
        ))}
        <div className="lnh offhd" style={{ ["--zc" as string]: "var(--muted)" }}>
          <span className="zchip off">Off rota</span>
          <span className="lnt">{absences.length} not available</span>
        </div>
      </div>

      <div className="ladder" style={{ height: px(1440) + 8, minWidth: ladderWidth }}>
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
                  minWidth: Math.max(1, lane.columns.length) * MIN_COLUMN_PX,
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
              <OffRotaStack absences={absences} kinds={absenceKinds} />
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

  return (
    <div
      className={`zblk ${token.toLowerCase()}`}
      style={{
        top: px(block.startMin),
        height: px(block.endMin - block.startMin),
        ["--zc" as string]: `var(--${token.toLowerCase()}-fg, var(--faint))`,
      }}
    >
      <div className="zbh">
        <span className={`chip sm ${token}`}>
          {timeOf(first.startsAt, tz)} – {timeOf(first.endsAt, tz)}
        </span>
        <b>{block.title ?? block.shift?.label ?? first.shiftCode}</b>
        <span className="zbn">{block.rows.length}</span>
      </div>
      {block.note ? <div className="zbw">{block.note}</div> : null}
      <div className="zbp">
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
            <i style={{ background: teamColour(team) }} />
            <span className="tn">{team}</span>
            <b>{teamRows.length}</b>
          </span>
        ))}
      </div>
      <div className="teampane">
        {current ? (
          <div className="tlp on">
            <div className="tlph">
              <i style={{ background: teamColour(current[0]) }} />
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
  /* Only what the row does not already say. A tier ("L2") and on-call status
     are not readable anywhere else on the card; the team is -- it is the
     avatar's colour, and the row's own tooltip. Spelling it out a third time
     cost about a quarter of the width the name needed, so the name was the
     thing that got cut. */
  const tag = assignment.tier ?? (assignment.isOnCall ? "OC" : null);
  return (
    <span className="lnm" title={`${assignment.engineer.name} · ${assignment.teamKey}`}>
      <span className="av" style={{ background: teamColour(assignment.teamKey) }}>
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
              <span className="offl">{kind.label}</span>
              <b>{rows.length}</b>
            </div>
            {byTeam ? (
              [...byTeam.entries()].map(([team, teamRows]) => (
                <div key={team}>
                  <div className="offgh">
                    {team}
                    <b>{teamRows.length}</b>
                  </div>
                  <div className="offp">
                    {teamRows.map((r) => (
                      <span className="lnm" key={r.id} title={r.engineer.name}>
                        <span className="av" style={{ background: teamColour(r.teamKey) }}>
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
                  <span className="lnm" key={r.id} title={`${r.engineer.name} · ${r.teamKey}`}>
                    <span className="av" style={{ background: teamColour(r.teamKey) }}>
                      {initialsOf(r.engineer.name)}
                    </span>
                    <span className="who">{r.engineer.name}</span>
                    <i className="tier-t">{r.teamKey}</i>
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
