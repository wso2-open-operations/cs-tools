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

import type { LegendItem } from "../components/RosterLegend";
import type { EngagementAllocation } from "../types";

/** What the engagement allocations read accepts in one request. */
export const ALLOCATION_MAX_USER_IDS = 500;
export const ALLOCATION_MAX_WINDOW_DAYS = 70;

const DAY_MS = 86_400_000;

/** A YYYY-MM-DD as a day number, on UTC so a DST change cannot shift it. */
function dayNumber(iso: string): number {
  const [y, m, d] = iso.split("-").map(Number);
  return Date.UTC(y, m - 1, d) / DAY_MS;
}

function isoOfDay(n: number): string {
  return new Date(n * DAY_MS).toISOString().slice(0, 10);
}

/**
 * A date window as the pieces one request may ask for.
 *
 * The read refuses more than 70 days, so a longer window is cut into
 * consecutive runs of at most that many days, both ends inclusive and with no
 * day in two pieces. A window that is empty or unreadable gives no pieces.
 */
export function splitWindow(
  from: string,
  to: string,
  maxDays: number = ALLOCATION_MAX_WINDOW_DAYS,
): { from: string; to: string }[] {
  const first = dayNumber(from);
  const last = dayNumber(to);
  if (!(last >= first)) return [];
  const out: { from: string; to: string }[] = [];
  for (let start = first; start <= last; start += maxDays) {
    out.push({ from: isoOfDay(start), to: isoOfDay(Math.min(start + maxDays - 1, last)) });
  }
  return out;
}

/** The ids in runs of at most `size`, in order. */
export function chunk<T>(items: readonly T[], size: number = ALLOCATION_MAX_USER_IDS): T[][] {
  const out: T[][] = [];
  for (let i = 0; i < items.length; i += size) out.push(items.slice(i, i + size));
  return out;
}

/**
 * The distinct user ids of everyone the schedule is drawing, sorted so the
 * same people always make the same request (and the same cache key).
 * Anything without an id is left out rather than guessed at.
 */
export function distinctUserIds(...sources: Iterable<{ userId?: string | null }>[]): string[] {
  const ids = new Set<string>();
  for (const source of sources) {
    for (const p of source) {
      const id = p.userId?.trim();
      if (id) ids.add(id);
    }
  }
  return [...ids].sort();
}

/**
 * A raw enum value as words: underscores to spaces, Title Case.
 * CONSULTANCY becomes "Consultancy", PROFESSIONAL_SERVICES "Professional
 * Services". Used for values this code has never seen, so a new type on the
 * server reads sensibly before anyone has taught the screen about it.
 */
export function humaniseEnum(raw: string | undefined | null): string {
  if (!raw) return "";
  return raw
    .split(/[_\s]+/)
    .filter(Boolean)
    .map((w) => w.charAt(0).toUpperCase() + w.slice(1).toLowerCase())
    .join(" ");
}

/** The account, then " - " and the engagement's own id where it has one. With
 *  no account the id stands alone, and with neither it is just "Engagement". */
export function allocationLabel(a: EngagementAllocation): string {
  const account = a.accountName?.trim();
  const display = a.engagementDisplayId?.trim();
  if (account) return display ? `${account} - ${display}` : account;
  return display || "Engagement";
}

/** The tooltip: type, mode, allocation id, utilisation and dates, with what
 *  the server did not send simply left out. */
export function allocationTitle(a: EngagementAllocation): string {
  return [
    humaniseEnum(a.engagementType),
    humaniseEnum(a.modeOfDelivery),
    a.allocationId?.trim(),
    a.utilization === undefined || a.utilization === null ? "" : `${a.utilization}%`,
    `${a.startDate} - ${a.endDate}`,
  ]
    .filter(Boolean)
    .join(" ● ");
}

/**
 * Allocations laid out as engineer, then day, then the bars on that day.
 *
 * One entry per day an allocation covers inside `days`, so a five-day run is
 * five bars and a day with two allocations holds both. Days outside the window
 * are not drawn. An allocation returned twice (two request pieces both touch
 * it) is counted once. Bars on a day are in label order, then id, so a redraw
 * never shuffles them.
 */
export function allocationsByPersonAndDay(
  allocations: readonly EngagementAllocation[],
  days: readonly string[],
): Map<string, Map<string, EngagementAllocation[]>> {
  const out = new Map<string, Map<string, EngagementAllocation[]>>();
  const seen = new Set<string>();
  for (const a of allocations) {
    if (seen.has(a.id)) continue;
    seen.add(a.id);
    const dismissed = new Set(a.dismissedDates ?? []);
    for (const iso of days) {
      if (iso < a.startDate || iso > a.endDate) continue;
      // A day taken off the roster by a lead is not drawn.
      if (dismissed.has(iso)) continue;
      let person = out.get(a.userId);
      if (!person) out.set(a.userId, (person = new Map()));
      const day = person.get(iso);
      if (day) day.push(a);
      else person.set(iso, [a]);
    }
  }
  for (const person of out.values()) {
    for (const day of person.values()) {
      day.sort((x, y) => allocationLabel(x).localeCompare(allocationLabel(y)) || x.id.localeCompare(y.id));
    }
  }
  return out;
}

/**
 * The days a lead or rota admin took an allocation off a person's roster, laid
 * out like allocationsByPersonAndDay: engineer, then day, then the
 * allocations removed that day. The cell picker offers to restore them.
 */
export function dismissedByPersonAndDay(
  allocations: readonly EngagementAllocation[],
  days: readonly string[],
): Map<string, Map<string, EngagementAllocation[]>> {
  const onGrid = new Set(days);
  const out = new Map<string, Map<string, EngagementAllocation[]>>();
  const seen = new Set<string>();
  for (const a of allocations) {
    if (seen.has(a.id) || !a.dismissedDates?.length) continue;
    seen.add(a.id);
    for (const iso of a.dismissedDates) {
      if (!onGrid.has(iso)) continue;
      let person = out.get(a.userId);
      if (!person) out.set(a.userId, (person = new Map()));
      const day = person.get(iso);
      if (day) day.push(a);
      else person.set(iso, [a]);
    }
  }
  return out;
}

/** The quiet line both views show when the allocations could not be read. */
export const ENGAGEMENTS_FAILED_NOTE = "Customer engagements could not be loaded, so they are not shown.";

/**
 * One day's allocations as the lines of a tooltip, one per allocation: the
 * label, then the same details the week bar's tooltip carries. The month
 * roster's marker has no room for a label, so the label leads each line.
 */
export function dayMarkerLines(list: readonly EngagementAllocation[]): string[] {
  return list.map((a) => `${allocationLabel(a)}: ${allocationTitle(a)}`);
}

/** The marker's tooltip: every allocation covering the day, one per line. */
export function dayMarkerTitle(list: readonly EngagementAllocation[]): string {
  return dayMarkerLines(list).join("\n");
}

/** The marker's accessible name: says how many, then lists them. Separate
 *  from the tooltip because a screen reader has no use for line breaks. */
export function dayMarkerAriaLabel(list: readonly EngagementAllocation[]): string {
  const head = list.length === 1 ? "Customer engagement" : `${list.length} customer engagements`;
  return `${head}. ${dayMarkerLines(list).join(". ")}`;
}

/**
 * The month roster's chip for one allocation: "Allo-" and the engagement's own
 * id (Allo-E07202), or "Allo-ENG" when it has none. The full line is on hover
 * (dayMarkerTitle).
 */
export function allocationChipLabel(a: EngagementAllocation): string {
  return `Allo-${a.engagementDisplayId?.trim() || "ENG"}`;
}

/** How many chips one roster cell stacks at most: a turn and two
 *  allocations, or three allocations. Past that the last place says "+N". */
export const ROSTER_CELL_MAX_CHIPS = 3;

/** What a customer engagement bar is: one line, for any view that draws them. */
export const ENGAGEMENT_LEGEND_ITEM: LegendItem = {
  key: "engagement",
  group: "Engagements",
  code: "",
  token: "",
  label: "Customer engagement: one bar a day, account and engagement id (read-only)",
  swatch: "engagement",
};

/** What the month roster's engagement chip is. The chip stands in for an
 *  unmarked day, sits beside a rotation turn, and becomes a corner dot when a
 *  tag (leave, a move, an allocation tag) holds the day. */
export const ENGAGEMENT_MARK_LEGEND_ITEM: LegendItem = {
  key: "engagement-mark",
  group: "Engagements",
  code: "Allo-…",
  token: "eng",
  label:
    "Customer engagement from the allocation app: Allo- and the engagement id, a dot when a tag holds the day; hover for the details (read-only)",
  swatch: "engagement-mark",
};
