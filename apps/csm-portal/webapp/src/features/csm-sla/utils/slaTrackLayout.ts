// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

import { parseBackendTimestamp } from "@utils/dateTime";
import {
  sideOfState,
  type SlaTimelineEvent,
  type SlaTimelineSide,
} from "@features/csm-sla/utils/slaImpactTimeline";

/** Who holds the case over a stretch of the track; `closed` is nobody. */
export type SlaTrackSide = SlaTimelineSide | "closed";

export interface SlaTrackSegment {
  side: SlaTrackSide;
  /** Real start and end of the stretch; the closed tail drawn after a closure is not counted. */
  fromMs: number;
  toMs: number;
  /** Calendar time actually spent on this side, in milliseconds. */
  durationMs: number;
  /** True for the stretch that runs to now (an open case's last segment). */
  ongoing: boolean;
  /** Left edge and width, as a percentage of the plot's width. */
  leftPct: number;
  widthPct: number;
}

export interface SlaTrackMarker {
  event: SlaTimelineEvent;
  /** Centre, as a percentage of the plot's width. */
  leftPct: number;
  /** 0 sits on the track; 1, 3, ... stack above it and 2, 4, ... below it. */
  lane: number;
}

export interface SlaTrackTick {
  leftPct: number;
  label: string;
}

export interface SlaTrackLayout {
  /**
   * False when neither an audited state change nor the current state says
   * which side held the case, so any per-side time would be invented.
   */
  sidesKnown: boolean;
  /** Plot width, in pixels, at which every gap below is guaranteed. */
  baseWidthPx: number;
  segments: SlaTrackSegment[];
  markers: SlaTrackMarker[];
  ticks: SlaTrackTick[];
  /** Highest lane in use, so the caller can size the plot. */
  maxLane: number;
}

/** Smallest centre-to-centre distance between two markers in one lane, in pixels. */
export const MARKER_MIN_GAP_PX = 40;
const MIN_WIDTH_PX = 640;
const WIDTH_PER_EVENT_PX = 64;
const TICK_SPACING_PX = 120;
/** Highest lane tried before a marker is nudged sideways instead. */
const MAX_LANE = 4;
/** Share of the span reserved after a closing event, so the closed stretch shows. */
const CLOSED_TAIL = 0.05;
const MIN_SPAN_MS = 24 * 60 * 60 * 1000;

function timeOf(at: string): number | null {
  return parseBackendTimestamp(at)?.getTime() ?? null;
}

export interface SlaTrackInput {
  /** Oldest first, as `buildSlaImpactTimeline` returns them. */
  events: SlaTimelineEvent[];
  /** When the case was opened; falls back to the first event. */
  openedAt?: string;
  /** The case's current state, used for the track colour when no state event exists. */
  currentState?: string;
  nowMs: number;
  /** Label for a tick at the given time, given the whole track's span. */
  formatTick: (ms: number, spanMs: number) => string;
}

/**
 * Place a case's SLA events on one horizontal track from open to now (or to
 * its closure). Positions are proportional to time, then spread so no two
 * markers in a lane are closer than {@link MARKER_MIN_GAP_PX} at
 * `baseWidthPx`: a crowded stretch spills into the lanes above and below the
 * track, and only as a last resort is a marker nudged sideways. Percentages are
 * returned, so a plot wider than `baseWidthPx` only ever adds room.
 */
export function layoutSlaTrack(input: SlaTrackInput): SlaTrackLayout {
  const timed = input.events
    .map((event) => ({ event, ms: timeOf(event.at) }))
    .filter((e): e is { event: SlaTimelineEvent; ms: number } => e.ms !== null);

  const opened = input.openedAt ? timeOf(input.openedAt) : null;
  const startMs = Math.min(opened ?? Infinity, timed[0]?.ms ?? Infinity, input.nowMs);
  const last = timed[timed.length - 1];
  const closedAtEnd = last?.event.kind === "closed";
  const lastMs = Math.max(last?.ms ?? startMs, startMs);
  const naturalEnd = closedAtEnd ? lastMs : Math.max(input.nowMs, lastMs);
  const spanMs = Math.max(naturalEnd - startMs, MIN_SPAN_MS);
  const endMs = closedAtEnd ? startMs + spanMs * (1 + CLOSED_TAIL) : startMs + spanMs;
  const totalMs = endMs - startMs;

  const baseWidthPx = Math.max(MIN_WIDTH_PX, timed.length * WIDTH_PER_EVENT_PX);
  const pct = (ms: number): number => ((ms - startMs) / totalMs) * 100;

  // Colour: the side before the first state event is the one that event left
  // (a case opens on the WSO2 side), or the current state when none exists.
  const isSideEvent = (e: SlaTimelineEvent): boolean =>
    e.kind === "state" || e.kind === "closed" || e.kind === "reopened";
  const firstState = timed.find((e) => isSideEvent(e.event));
  let side: SlaTrackSide =
    firstState === undefined
      ? (sideOfState(input.currentState) ?? (input.currentState ? "closed" : "wso2"))
      : firstState.event.kind === "reopened"
        ? "closed"
        : (firstState.event.previousSide ?? "wso2");
  const segments: SlaTrackSegment[] = [];
  let cursor = startMs;
  const push = (toMs: number): void => {
    if (toMs > cursor) {
      segments.push({
        side,
        fromMs: cursor,
        toMs: Math.min(toMs, naturalEnd),
        durationMs: Math.max(0, Math.min(toMs, naturalEnd) - cursor),
        ongoing: false,
        leftPct: pct(cursor),
        widthPct: pct(toMs) - pct(cursor),
      });
      cursor = toMs;
    }
  };
  for (const { event, ms } of timed) {
    if (isSideEvent(event)) {
      push(ms);
      side = event.kind === "closed" ? "closed" : (event.side ?? "wso2");
    }
  }
  push(endMs);
  const finalSegment = segments[segments.length - 1];
  if (finalSegment && !closedAtEnd) finalSegment.ongoing = true;

  // Markers: lane 0 unless it is crowded.
  const lastX: number[] = Array.from({ length: MAX_LANE + 1 }, () => -Infinity);
  const markers: SlaTrackMarker[] = [];
  let maxLane = 0;
  for (const { event, ms } of timed) {
    let x = (pct(ms) / 100) * baseWidthPx;
    let lane = lastX.findIndex((edge) => edge + MARKER_MIN_GAP_PX <= x);
    if (lane === -1) {
      lane = lastX.indexOf(Math.min(...lastX));
      x = lastX[lane] + MARKER_MIN_GAP_PX;
    }
    lastX[lane] = x;
    maxLane = Math.max(maxLane, lane);
    markers.push({ event, leftPct: Math.min((x / baseWidthPx) * 100, 100), lane });
  }

  const tickCount = Math.min(8, Math.max(2, Math.floor(baseWidthPx / TICK_SPACING_PX)));
  const ticks: SlaTrackTick[] = [];
  for (let i = 0; i < tickCount; i += 1) {
    const ratio = i / (tickCount - 1);
    const label = input.formatTick(startMs + ratio * totalMs, totalMs);
    if (ticks[ticks.length - 1]?.label !== label) ticks.push({ leftPct: ratio * 100, label });
  }

  return {
    sidesKnown: firstState !== undefined || Boolean(input.currentState?.trim()),
    baseWidthPx,
    segments,
    markers,
    ticks,
    maxLane,
  };
}

export interface SlaSideSummary {
  wso2Ms: number;
  customerMs: number;
  /** Time the case spent closed between a closure and a reopening. Not part of the open period. */
  closedMs: number;
  /** WSO2 side plus customer side. */
  openMs: number;
  /** Shares of the open period, 0 to 100; both 0 when the open period is empty. */
  wso2Pct: number;
  customerPct: number;
}

/**
 * Total calendar time on each side, from the very segments the track draws,
 * so the numbers and the bar cannot disagree. Closed time is reported but
 * left out of the open period and of the percentages.
 */
export function summarizeSlaSides(segments: SlaTrackSegment[]): SlaSideSummary {
  const total = (side: SlaTrackSide): number =>
    segments.filter((s) => s.side === side).reduce((sum, s) => sum + s.durationMs, 0);
  const wso2Ms = total("wso2");
  const customerMs = total("customer");
  const openMs = wso2Ms + customerMs;
  return {
    wso2Ms,
    customerMs,
    closedMs: total("closed"),
    openMs,
    wso2Pct: openMs > 0 ? (wso2Ms / openMs) * 100 : 0,
    customerPct: openMs > 0 ? (customerMs / openMs) * 100 : 0,
  };
}

const MINUTE_MS = 60 * 1000;
const HOUR_MS = 60 * MINUTE_MS;
const DAY_MS = 24 * HOUR_MS;

/** "3d 4h", "5h 12m", "12m" or "under 1m": the two most significant units, calendar time. */
export function formatCalendarDuration(ms: number): string {
  if (ms < MINUTE_MS) return "under 1m";
  const days = Math.floor(ms / DAY_MS);
  const hours = Math.floor((ms % DAY_MS) / HOUR_MS);
  const minutes = Math.floor((ms % HOUR_MS) / MINUTE_MS);
  if (days > 0) return `${days}d ${hours}h`;
  if (hours > 0) return `${hours}h ${minutes}m`;
  return `${minutes}m`;
}
