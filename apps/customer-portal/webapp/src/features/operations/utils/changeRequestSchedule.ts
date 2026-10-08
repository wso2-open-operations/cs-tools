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

/**
 * Pure helpers for the customer's "Propose New Time" dialog: reading the change
 * request's current window, working out the end a proposed start implies,
 * checking the proposed start, and building the PATCH body.
 *
 * A proposal is a new START. The planned length stays (the record of a customer's
 * proposed time holds one instant, and WSO2 answers it by accepting
 * it or suggesting another time), so the end is derived, never typed.
 *
 * Every `datetime-local` value here is a civil time in the viewer's IANA time
 * zone; every API value is UTC (`YYYY-MM-DD HH:MM:SS`).
 */

import { parseBackendTimestamp } from "@utils/dateTime";
import {
  datetimeLocalWallTimeToUtcMs as scanDatetimeLocalWallTimeToUtcMs,
  instantToDatetimeLocalStringInZone,
  resolveCallSchedulingTimeZone,
} from "@features/support/utils/support";
import type { ChangeRequestDetails } from "@features/operations/types/changeRequests";

const DATETIME_LOCAL_RE = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})(?::\d{2})?$/;
const MINUTE_MS = 60 * 1000;
const DAY_MS = 24 * 60 * MINUTE_MS;

const zonedFormatters = new Map<string, Intl.DateTimeFormat>();

/** The wall clock `timeZone` shows at `instantMs`, as the UTC ms of that wall time (minute precision). */
function zonedWallMinuteMs(instantMs: number, timeZone: string): number {
  let formatter = zonedFormatters.get(timeZone);
  if (!formatter) {
    formatter = new Intl.DateTimeFormat("en-CA", {
      timeZone,
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
      hour: "2-digit",
      minute: "2-digit",
      hourCycle: "h23",
    });
    zonedFormatters.set(timeZone, formatter);
  }
  const parts = formatter.formatToParts(new Date(instantMs));
  const part = (type: Intl.DateTimeFormatPartTypes) =>
    Number(parts.find((p) => p.type === type)?.value ?? NaN);
  return Date.UTC(
    part("year"),
    part("month") - 1,
    part("day"),
    part("hour"),
    part("minute"),
  );
}

/**
 * Converts a `datetime-local` value (civil time in `timeZone`) to UTC epoch ms.
 *
 * Same answer as the shared `datetimeLocalWallTimeToUtcMs` (the earlier instant
 * for a time that happens twice; null for one that never happens), but found
 * from the zone's UTC offsets a day either side instead of scanning minute by
 * minute. The shared scan costs ~150 ms a call, which is felt on every edit of
 * a date field; it is kept as the fallback for times that fall in a gap.
 *
 * @param localValue - `YYYY-MM-DDTHH:mm` from an `input type="datetime-local"`.
 * @param timeZone - IANA zone (or the API's alias for one).
 * @returns UTC ms, or null when the value is empty, malformed or does not exist.
 */
export function datetimeLocalToUtcMs(
  localValue: string | null | undefined,
  timeZone: string | null | undefined,
): number | null {
  const match = DATETIME_LOCAL_RE.exec(localValue?.trim() ?? "");
  if (!match) return null;
  const [year, month, day, hour, minute] = match.slice(1, 6).map(Number);
  const asUtc = Date.UTC(year, month - 1, day, hour, minute);
  const roundTrip = new Date(asUtc);
  if (
    roundTrip.getUTCFullYear() !== year ||
    roundTrip.getUTCMonth() !== month - 1 ||
    roundTrip.getUTCDate() !== day ||
    roundTrip.getUTCHours() !== hour ||
    roundTrip.getUTCMinutes() !== minute
  ) {
    return null;
  }

  const zone = resolveCallSchedulingTimeZone(timeZone);
  const offsets = new Set<number>();
  for (const probe of [asUtc - DAY_MS, asUtc + DAY_MS]) {
    offsets.add(
      zonedWallMinuteMs(probe, zone) - Math.floor(probe / MINUTE_MS) * MINUTE_MS,
    );
  }
  const instants = [...offsets]
    .map((offset) => asUtc - offset)
    .filter((instant) => zonedWallMinuteMs(instant, zone) === asUtc)
    .sort((a, b) => a - b);
  if (instants.length > 0) return instants[0];

  return scanDatetimeLocalWallTimeToUtcMs(localValue, timeZone);
}

/**
 * Formats an instant the way the PATCH body wants it: `YYYY-MM-DD HH:MM:SS` in UTC.
 *
 * @param utcMs - UTC epoch ms.
 * @returns The API date-time.
 */
export function formatUtcMsAsApiDatetime(utcMs: number): string {
  const d = new Date(utcMs);
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${d.getUTCFullYear()}-${pad(d.getUTCMonth() + 1)}-${pad(d.getUTCDate())} ${pad(d.getUTCHours())}:${pad(d.getUTCMinutes())}:${pad(d.getUTCSeconds())}`;
}

/** The change request's planned window as instants; null where the API has none. */
export type ChangeRequestWindow = {
  startMs: number | null;
  endMs: number | null;
  /** End minus start, only when both exist and the end is after the start. */
  durationMs: number | null;
};

/**
 * Reads a change request's current planned window.
 *
 * @param changeRequest - Anything carrying `startDate` / `endDate`.
 * @returns The window as UTC instants.
 */
export function getChangeRequestWindow(
  changeRequest: Pick<ChangeRequestDetails, "startDate" | "endDate">,
): ChangeRequestWindow {
  const startMs = parseBackendTimestamp(changeRequest.startDate)?.getTime() ?? null;
  const endMs = parseBackendTimestamp(changeRequest.endDate)?.getTime() ?? null;
  const durationMs =
    startMs != null && endMs != null && endMs > startMs ? endMs - startMs : null;
  return { startMs, endMs, durationMs };
}

/**
 * The end that keeps `durationMs` after a new start, as the instant the server
 * derives (start + the planned length on the timeline, not on the wall clock).
 *
 * @param startLocal - The new start (`datetime-local`).
 * @param durationMs - Duration to keep; null when the current window has none.
 * @param timeZone - The viewer's zone.
 * @returns The end as `datetime-local`, or "" when it cannot be worked out
 *   (no duration, empty or non-existent start).
 */
export function shiftEndKeepingDuration(
  startLocal: string,
  durationMs: number | null,
  timeZone: string,
): string {
  if (durationMs == null) return "";
  const startMs = datetimeLocalToUtcMs(startLocal, timeZone);
  if (startMs == null) return "";
  return instantToDatetimeLocalStringInZone(
    startMs + durationMs,
    resolveCallSchedulingTimeZone(timeZone),
  );
}

export const PROPOSED_WINDOW_MESSAGES = {
  startRequired: "Enter the proposed start date and time.",
  startInvalid: "Enter a valid start date and time.",
  startPast: "The proposed start must be in the future.",
  unchanged:
    "This is the same as the current schedule. Choose a different start.",
  alreadyProposed:
    "That time is already proposed and is waiting for WSO2's response. Choose a different start.",
  noWindow:
    "This change request has no planned time yet, so a new time cannot be proposed for it.",
} as const;

export type ProposedStartInput = {
  /** Proposed start, `datetime-local` in `timeZone`. */
  start: string;
  /** Current planned start in the same form; "" when the change request has none. */
  currentStart: string;
  /** The standing proposal's start in the same form, when one waits for WSO2. */
  standingStart?: string;
  /** The planned length to keep; null when the change request has no window. */
  durationMs: number | null;
  timeZone: string;
  /** Clock override for tests. */
  nowMs?: number;
};

export type ProposedStartErrors = {
  start?: string;
  /** About the proposal as a whole (the same as today's, no window to move), not the field. */
  window?: string;
};

/**
 * Checks a proposed start before it is sent: the change request has a window to
 * move, the start is real and in the future, and it is not the planned start
 * already or the time that already waits for WSO2 (the backend refuses both).
 *
 * @param input - The field, the current window, the standing proposal and the viewer's zone.
 * @returns Errors; empty when the proposal can be sent.
 */
export function validateProposedStart(
  input: ProposedStartInput,
): ProposedStartErrors {
  const { start, currentStart, standingStart, durationMs, timeZone } = input;
  const nowMs = input.nowMs ?? Date.now();
  const errors: ProposedStartErrors = {};

  if (durationMs == null) {
    errors.window = PROPOSED_WINDOW_MESSAGES.noWindow;
    return errors;
  }

  const startMs = datetimeLocalToUtcMs(start, timeZone);
  if (!start.trim()) errors.start = PROPOSED_WINDOW_MESSAGES.startRequired;
  else if (startMs == null) errors.start = PROPOSED_WINDOW_MESSAGES.startInvalid;
  else if (startMs <= nowMs) errors.start = PROPOSED_WINDOW_MESSAGES.startPast;
  else if (start === currentStart) {
    errors.window = PROPOSED_WINDOW_MESSAGES.unchanged;
  } else if (standingStart && start === standingStart) {
    errors.window = PROPOSED_WINDOW_MESSAGES.alreadyProposed;
  }

  return errors;
}

/** True when {@link validateProposedStart} found nothing wrong. */
export function hasProposedStartErrors(errors: ProposedStartErrors): boolean {
  return Boolean(errors.start || errors.window);
}

/**
 * The PATCH body for a proposal: the proposed start and the end that keeps the
 * planned length, both in UTC. The service accepts the start alone or the start
 * with exactly that end; the end is sent so the previous system's data source, which
 * takes a whole window, keeps working.
 *
 * @param start - Proposed start (`datetime-local`).
 * @param durationMs - The planned length to keep.
 * @param timeZone - The viewer's zone, which `start` is in.
 * @returns The body, or null when the start is not a real time in that zone or
 *   there is no length to keep.
 */
export function buildProposedWindowPayload(
  start: string,
  durationMs: number | null,
  timeZone: string,
): { plannedStartOn: string; plannedEndOn: string } | null {
  if (durationMs == null || durationMs <= 0) return null;
  const startMs = datetimeLocalToUtcMs(start, timeZone);
  if (startMs == null) return null;
  return {
    plannedStartOn: formatUtcMsAsApiDatetime(startMs),
    plannedEndOn: formatUtcMsAsApiDatetime(startMs + durationMs),
  };
}

/**
 * A planned length in words: "2 hours", "1 hour 30 minutes", "45 minutes".
 *
 * @param durationMs - A positive length.
 * @returns The text, rounded to the minute ("less than a minute" below that).
 */
export function formatPlannedLength(durationMs: number): string {
  const totalMinutes = Math.round(durationMs / MINUTE_MS);
  if (totalMinutes < 1) return "less than a minute";
  const hours = Math.floor(totalMinutes / 60);
  const minutes = totalMinutes % 60;
  const parts: string[] = [];
  if (hours > 0) parts.push(`${hours} hour${hours === 1 ? "" : "s"}`);
  if (minutes > 0) parts.push(`${minutes} minute${minutes === 1 ? "" : "s"}`);
  return parts.join(" ");
}

/**
 * What a proposal does, in the customer's words. The same for every change type:
 * the change request stays in Customer Approval, and WSO2 answers the proposal
 * by accepting the time or suggesting another one (no further internal approval
 * is involved, which is why none is promised).
 *
 * @param durationMs - The planned length that stays, when the change has one.
 * @returns The dialog notice and the success message.
 */
export function getProposalCopy(
  durationMs: number | null,
): { notice: string; success: string } {
  const lengthText =
    durationMs != null
      ? `The planned length of ${formatPlannedLength(durationMs)} stays the same. `
      : "";
  return {
    notice: `You are proposing a new start time, not approving one. WSO2 will either accept it or suggest a different time. ${lengthText}To ask for a different length, contact WSO2.`,
    success:
      "New time proposed. WSO2 will accept it or suggest a different time, and the answer will appear on this page.",
  };
}
