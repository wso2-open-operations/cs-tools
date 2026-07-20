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
import type {
  ActivityBreakdown,
  ActivityKey,
  CsmTimeCard,
  CsmTimeSheet,
  IssueComplexity,
  Project,
  TimeCardState,
  TimeSheetState,
} from "@src/types";

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
export const TIMECARD_APPROVER_ROLE = "timecard_approver";

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
  // Guard the empty case first: `[].every(...)` is `true`, which would otherwise
  // report an empty sheet as "approved". Fall back to the same "submitted" default.
  if (cards.length === 0) return "submitted";
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

// --- Log-time (create) form support --------------------------------------
// Ported from the webapp's timeCardConstants.ts + timeCardTotals.ts, merged
// into this file's single-utils-file convention rather than split across
// separate constants/ and utils/ modules.

/** A labelled activity row in the time-breakdown editor (log-time form only). */
export interface ActivityBucket {
  key: ActivityKey;
  label: string;
}

/** The five fixed activity categories, in display order (ISSU-009). */
export const ACTIVITY_BUCKETS: ActivityBucket[] = [
  { key: "analysisDebugging", label: "Analysis and debugging" },
  { key: "reproduce", label: "Reproduce" },
  { key: "settingUp", label: "Setting up" },
  { key: "providingSolution", label: "Providing solution" },
  { key: "answering", label: "Answering" },
];

const ACTIVITY_KEYS: ActivityKey[] = ACTIVITY_BUCKETS.map((b) => b.key);

/** Issue-complexity options (the ServiceNow "Issue Complexity" field). Write-only. */
export const ISSUE_COMPLEXITY_OPTIONS: readonly IssueComplexity[] = ["N/A", "Low", "Medium", "High"];
export const DEFAULT_ISSUE_COMPLEXITY: IssueComplexity = "N/A";

/** Whether logged time is billable to the customer by default. Most support work is
 * billable; the engineer can flip it per card. */
export const DEFAULT_BILLABLE = true;

/** Character cap mirroring the ServiceNow work-log field. */
export const WORK_LOG_MAX = 4000;

/** A fresh breakdown with every activity at zero minutes. */
export function emptyBreakdown(): ActivityBreakdown {
  return {
    analysisDebugging: 0,
    reproduce: 0,
    settingUp: 0,
    providingSolution: 0,
    answering: 0,
  };
}

/** Total whole minutes across all activity buckets. */
export function totalMinutes(breakdown: ActivityBreakdown): number {
  return ACTIVITY_KEYS.reduce((sum, key) => sum + (breakdown[key] || 0), 0);
}

/** True when at least one activity bucket has logged time. */
export function hasLoggedTime(breakdown: ActivityBreakdown): boolean {
  return ACTIVITY_KEYS.some((key) => (breakdown[key] || 0) > 0);
}

/** Field-level validation errors for the log dialog, keyed by field name. */
export interface TimeCardDraftErrors {
  date?: string;
  minutes?: string;
  workLogComment?: string;
  approver?: string;
}

export interface TimeCardDraft {
  date: string;
  breakdown: ActivityBreakdown;
  workLogComment: string;
  approverId?: string;
}

/**
 * Validate a log-time draft. Returns an errors object; an empty object means the
 * draft is submittable. Mirrors the ServiceNow required fields (Date, Task —
 * preset from the case, Work Log Comment, Approver) plus a "log some time" rule.
 */
export function timeCardDraftErrors(draft: TimeCardDraft): TimeCardDraftErrors {
  const errors: TimeCardDraftErrors = {};
  if (!draft.date) errors.date = "Pick a date.";
  if (!hasLoggedTime(draft.breakdown)) {
    errors.minutes = "Log time against at least one activity.";
  }
  if (!draft.workLogComment.trim()) {
    errors.workLogComment = "Add a work log comment.";
  } else if (draft.workLogComment.length > WORK_LOG_MAX) {
    errors.workLogComment = `Comment must be ${WORK_LOG_MAX} characters or fewer.`;
  }
  if (!draft.approverId) errors.approver = "Choose an approver.";
  return errors;
}
