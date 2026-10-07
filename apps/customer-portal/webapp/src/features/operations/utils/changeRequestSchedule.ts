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
 * request's current window, keeping its duration when the start moves, checking
 * the proposed window, and building the PATCH body.
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
 * The end that keeps `durationMs` after a new start.
 *
 * @param startLocal - The new start (`datetime-local`).
 * @param durationMs - Duration to keep; null when the current window has none.
 * @param timeZone - The viewer's zone.
 * @returns The end as `datetime-local`, or "" when it cannot be worked out
 *   (no duration, empty or non-existent start), so the caller keeps what it has.
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
  endRequired: "Enter the proposed end date and time.",
  endInvalid: "Enter a valid end date and time.",
  endNotAfterStart: "The proposed end must be after the proposed start.",
  unchanged:
    "This is the same as the current schedule. Change the start or the end to propose a different time.",
} as const;

export type ProposedWindowInput = {
  /** Proposed start, `datetime-local` in `timeZone`. */
  start: string;
  /** Proposed end, `datetime-local` in `timeZone`. */
  end: string;
  /** Current start in the same form; "" when the change request has none. */
  currentStart: string;
  /** Current end in the same form; "" when the change request has none. */
  currentEnd: string;
  timeZone: string;
  /** Clock override for tests. */
  nowMs?: number;
};

export type ProposedWindowErrors = {
  start?: string;
  end?: string;
  /** About the window as a whole (the same as today's), not one field. */
  window?: string;
};

/**
 * Checks a proposed window before it is sent: both ends present and real, the
 * start in the future, the end after the start, and something actually
 * different from the current window (the backend refuses an unchanged one).
 *
 * @param input - The two fields, the current window and the viewer's zone.
 * @returns Errors by field; empty when the proposal can be sent.
 */
export function validateProposedWindow(
  input: ProposedWindowInput,
): ProposedWindowErrors {
  const { start, end, currentStart, currentEnd, timeZone } = input;
  const nowMs = input.nowMs ?? Date.now();
  const errors: ProposedWindowErrors = {};

  const startMs = datetimeLocalToUtcMs(start, timeZone);
  const endMs = datetimeLocalToUtcMs(end, timeZone);

  if (!start.trim()) errors.start = PROPOSED_WINDOW_MESSAGES.startRequired;
  else if (startMs == null) errors.start = PROPOSED_WINDOW_MESSAGES.startInvalid;
  else if (startMs <= nowMs) errors.start = PROPOSED_WINDOW_MESSAGES.startPast;

  if (!end.trim()) errors.end = PROPOSED_WINDOW_MESSAGES.endRequired;
  else if (endMs == null) errors.end = PROPOSED_WINDOW_MESSAGES.endInvalid;
  else if (startMs != null && endMs <= startMs) {
    errors.end = PROPOSED_WINDOW_MESSAGES.endNotAfterStart;
  }

  if (
    !errors.start &&
    !errors.end &&
    start === currentStart &&
    end === currentEnd
  ) {
    errors.window = PROPOSED_WINDOW_MESSAGES.unchanged;
  }

  return errors;
}

/** True when {@link validateProposedWindow} found nothing wrong. */
export function hasProposedWindowErrors(errors: ProposedWindowErrors): boolean {
  return Boolean(errors.start || errors.end || errors.window);
}

/**
 * The PATCH body for a proposal: both ends, in UTC.
 *
 * @param start - Proposed start (`datetime-local`).
 * @param end - Proposed end (`datetime-local`).
 * @param timeZone - The viewer's zone, which both values are in.
 * @returns The body, or null when either value is not a real time in that zone.
 */
export function buildProposedWindowPayload(
  start: string,
  end: string,
  timeZone: string,
): { plannedStartOn: string; plannedEndOn: string } | null {
  const startMs = datetimeLocalToUtcMs(start, timeZone);
  const endMs = datetimeLocalToUtcMs(end, timeZone);
  if (startMs == null || endMs == null) return null;
  return {
    plannedStartOn: formatUtcMsAsApiDatetime(startMs),
    plannedEndOn: formatUtcMsAsApiDatetime(endMs),
  };
}

/**
 * What a proposal does depends on the change type: a Standard change stays in
 * Customer Approval and the customer is asked again straight away; a Normal or
 * Emergency change goes back to WSO2's internal approval first. The words
 * promised to the customer follow that.
 *
 * @param changeRequest - The change request being re-scheduled.
 * @returns The dialog notice and the success message.
 */
export function getProposalCopy(
  changeRequest: Pick<ChangeRequestDetails, "type">,
): { notice: string; success: string } {
  const isStandard = changeRequest.type?.label?.trim().toLowerCase() === "standard";
  return isStandard
    ? {
        notice:
          "You are proposing a new time. It replaces the current schedule, and you will then be asked to approve it.",
        success:
          "New time proposed. Review the updated schedule and approve it when you are ready.",
      }
    : {
        notice:
          "You are proposing a new time, not approving one. WSO2 will review it internally first, and you will then be asked to approve the new time.",
        success:
          "New time proposed. We'll ask for your approval again once it's confirmed internally.",
      };
}
