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

import type { ChipProps } from "@wso2/oxygen-ui";
import type { CsmTimeCard, CsmTimeSheet, Project, TimeCardState, TimeSheetState } from "@src/types";

/** A selectable engineer for the (client-side) engineer filter — derived from
 * the cards currently loaded, since there's no engineer-search endpoint. */
export interface EngineerOption {
  id: string;
  name: string;
}

// The backend rejects search `limit` above 50 with a generic 400, despite the
// OpenAPI spec documenting up to 100 (confirmed live).
export const TIME_CARD_MAX_PAGE_LIMIT = 50;

// Role that grants approve/reject on time cards, matched (case-insensitively)
// against `roles` from `GET /users/me`.
export const TIMECARD_APPROVER_ROLE = "sn_customerservice_timecard_approver";

// Time-card admin (approve by exception). Mapped to the real "admin" role for
// now; revisit once dedicated time-card roles are provisioned.
export const TIMECARD_ADMIN_ROLE = "admin";

/** True when `roles` (from `GET /users/me`) grants time-card approval. */
export function isTimecardApprover(roles: string[]): boolean {
  const lower = roles.map((r) => r.toLowerCase());
  return lower.includes(TIMECARD_APPROVER_ROLE.toLowerCase()) || lower.includes(TIMECARD_ADMIN_ROLE.toLowerCase());
}

// Label + MUI chip colour for each card state (drives the status chip).
// `recalled`/`processed` are unreachable via the portal API but kept for
// type completeness.
export const TIME_CARD_STATE_META: Record<TimeCardState, { label: string; color: ChipProps["color"] }> = {
  pending: { label: "Pending", color: "default" },
  submitted: { label: "Submitted", color: "info" },
  approved: { label: "Approved", color: "success" },
  rejected: { label: "Rejected", color: "error" },
  recalled: { label: "Recalled", color: "warning" },
  processed: { label: "Processed", color: "success" },
};

// Label + MUI chip colour for each rolled-up sheet state.
export const TIME_SHEET_STATE_META: Record<TimeSheetState, { label: string; color: ChipProps["color"] }> = {
  submitted: { label: "Submitted", color: "info" },
  approved: { label: "Approved", color: "success" },
  rejected: { label: "Rejected", color: "error" },
};

// The reachable card states offered in the filter sheet — `recalled`/`processed`
// can't be reached via the portal API, so they're left out.
export const TIME_CARD_FILTER_STATES: TimeCardState[] = ["submitted", "approved", "rejected"];

// UI-facing time-card filters.
// - `projects` → server-side `projectIds`.
// - `states` → server-side, but combining with a large project scope 500s the
//   backend, so it's applied client-side (see the service).
// - `workItems` (case numbers) and `engineers` (user ids) are client-side over
//   the loaded cards — the backend has no filter for either.
// - `from`/`to` (YYYY-MM-DD, inclusive) → server-side startDate/endDate.
export interface TimeCardFilters {
  projects: Project[];
  workItems: string[];
  engineers: string[];
  states: TimeCardState[];
  from: string;
  to: string;
}

export const EMPTY_TIMECARD_FILTERS: TimeCardFilters = {
  projects: [],
  workItems: [],
  engineers: [],
  states: [],
  from: "",
  to: "",
};

/** How many filter groups are active — drives the "Filters (n)" button badge.
 * A set date range counts as one regardless of whether one or both ends are set. */
export function countActiveTimecardFilters(filters: TimeCardFilters): number {
  let count = 0;
  if (filters.projects.length > 0) count += 1;
  if (filters.workItems.length > 0) count += 1;
  if (filters.engineers.length > 0) count += 1;
  if (filters.states.length > 0) count += 1;
  if (filters.from || filters.to) count += 1;
  return count;
}

/** Whole minutes → a short human duration, e.g. 150 → "2h 30m", 45 → "45m". */
export function formatMinutes(totalMinutes: number): string {
  const mins = Math.max(0, Math.round(totalMinutes));
  const h = Math.floor(mins / 60);
  const m = mins % 60;
  if (h === 0) return `${m}m`;
  if (m === 0) return `${h}h`;
  return `${h}h ${m}m`;
}

// ISO-week helpers (weeks run Monday–Sunday). Dates are handled as plain
// YYYY-MM-DD strings in UTC to avoid timezone drift in week boundaries.

function toUtcDate(iso: string): Date {
  return new Date(`${iso.slice(0, 10)}T00:00:00Z`);
}

function isoOf(d: Date): string {
  return d.toISOString().slice(0, 10);
}

/** The Monday (YYYY-MM-DD) of the ISO week containing `iso`. Throws
 * (RangeError) if `iso` is not a parseable date. */
export function weekStartOf(iso: string): string {
  const d = toUtcDate(iso);
  const sinceMonday = (d.getUTCDay() + 6) % 7; // getUTCDay: 0=Sun..6=Sat.
  d.setUTCDate(d.getUTCDate() - sinceMonday);
  return isoOf(d);
}

/** The Sunday (YYYY-MM-DD) ending the week that starts on `weekStart`. */
export function weekEndOf(weekStart: string): string {
  const d = toUtcDate(weekStart);
  d.setUTCDate(d.getUTCDate() + 6);
  return isoOf(d);
}

/** Human label, e.g. "Jun 23 – 29, 2026" (or spanning months/years). */
export function weekLabel(weekStart: string): string {
  const start = toUtcDate(weekStart);
  const end = toUtcDate(weekEndOf(weekStart));
  const mon = (d: Date): string => d.toLocaleDateString("en-US", { month: "short", timeZone: "UTC" });
  const day = (d: Date): number => d.getUTCDate();
  const year = end.getUTCFullYear();
  if (mon(start) === mon(end)) {
    return `${mon(start)} ${day(start)} – ${day(end)}, ${year}`;
  }
  return `${mon(start)} ${day(start)} – ${mon(end)} ${day(end)}, ${year}`;
}

/** Short date label for a single card, e.g. "Jun 25". Falls back to "—". */
export function cardDateLabel(iso: string): string {
  const d = toUtcDate(iso);
  if (Number.isNaN(d.getTime())) return "—";
  return d.toLocaleDateString("en-US", { month: "short", day: "numeric", timeZone: "UTC" });
}

/** Roll a week's cards up into a single display status. */
export function sheetStatus(cards: CsmTimeCard[]): TimeSheetState {
  if (cards.some((c) => c.state === "rejected")) return "rejected";
  if (cards.every((c) => c.state === "approved" || c.state === "processed")) return "approved";
  return "submitted";
}

/**
 * Group a flat list of cards into weekly sheets (Mon–Sun), newest first.
 * `weekStartOf` throws if `createdOn` isn't parseable — confirmed live that at
 * least one real card has a malformed date. Skip that one card rather than
 * losing its whole week, mirroring the "one bad record shouldn't sink the rest"
 * guard used on the server side.
 */
export function groupIntoSheets(cards: CsmTimeCard[], userId: string, userName: string): CsmTimeSheet[] {
  const byWeek = new Map<string, CsmTimeCard[]>();
  for (const c of cards) {
    let wk: string;
    try {
      wk = weekStartOf(c.createdOn);
    } catch {
      continue;
    }
    const bucket = byWeek.get(wk);
    if (bucket) bucket.push(c);
    else byWeek.set(wk, [c]);
  }
  const sheets: CsmTimeSheet[] = [];
  for (const [weekStart, weekCards] of byWeek) {
    weekCards.sort((a, b) => b.createdOn.localeCompare(a.createdOn));
    sheets.push({
      id: `${userId}:${weekStart}`,
      userId,
      userName,
      weekStart,
      weekEnd: weekEndOf(weekStart),
      state: sheetStatus(weekCards),
      cards: weekCards,
      totalMinutes: weekCards.reduce((sum, c) => sum + c.totalMinutes, 0),
    });
  }
  return sheets.sort((a, b) => b.weekStart.localeCompare(a.weekStart));
}

/** Group cards by `userId` into per-user weekly sheets — for the approvals
 * view, where cards from several engineers come back together. */
export function groupCardsByUserIntoSheets(cards: CsmTimeCard[]): CsmTimeSheet[] {
  const byUser = new Map<string, CsmTimeCard[]>();
  for (const c of cards) {
    const bucket = byUser.get(c.userId);
    if (bucket) bucket.push(c);
    else byUser.set(c.userId, [c]);
  }
  return [...byUser.entries()].flatMap(([userId, userCards]) =>
    groupIntoSheets(userCards, userId, userCards[0]?.userName ?? "—"),
  );
}
