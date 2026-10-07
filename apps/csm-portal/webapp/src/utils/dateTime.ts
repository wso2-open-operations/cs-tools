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

let cachedUserTimeZone: string | null = null;

const API_TIMEZONE_TO_INTL_ALIASES: Record<string, string> = {
  "WSO2/Colombo": "Asia/Colombo",
};

/**
 * Returns true if timezone is accepted by Intl.DateTimeFormat.
 *
 * @param timeZone - Candidate IANA timezone.
 * @returns {boolean} True when valid.
 */
function isValidIntlTimeZone(timeZone: string): boolean {
  try {
    Intl.DateTimeFormat("en-US", { timeZone }).format(new Date());
    return true;
  } catch {
    return false;
  }
}

/**
 * Normalizes API/profile timezone to a browser-supported IANA timezone.
 *
 * @param timeZone - Raw timezone from API/profile.
 * @returns {string | null} Valid timezone or null.
 */
export function normalizeUserTimeZone(
  timeZone: string | null | undefined,
): string | null {
  const trimmed = timeZone?.trim();
  if (!trimmed) return null;
  const candidate = API_TIMEZONE_TO_INTL_ALIASES[trimmed] ?? trimmed;
  return isValidIntlTimeZone(candidate) ? candidate : null;
}

/**
 * Lists the IANA time zones the runtime supports, for the profile time-zone
 * picker. Uses `Intl.supportedValuesOf` where available; falls back to the
 * browser's own zone on older runtimes so the picker is never empty.
 *
 * @returns {string[]} Sorted IANA time-zone identifiers.
 */
export function listSupportedTimeZones(): string[] {
  try {
    const fn = (Intl as { supportedValuesOf?: (key: string) => string[] })
      .supportedValuesOf;
    if (typeof fn === "function") return fn("timeZone");
  } catch {
    /* older runtime without Intl.supportedValuesOf */
  }
  try {
    const zone = Intl.DateTimeFormat().resolvedOptions().timeZone;
    return zone ? [zone] : [];
  } catch {
    return [];
  }
}

/**
 * Stores user timezone globally for view-only date formatting.
 *
 * @param timeZone - Timezone from users/me response.
 */
export function setUserPreferredTimeZone(
  timeZone: string | null | undefined,
): void {
  cachedUserTimeZone = normalizeUserTimeZone(timeZone);
}

/**
 * Clears cached timezone between authenticated sessions.
 */
export function clearUserPreferredTimeZone(): void {
  cachedUserTimeZone = null;
}

/**
 * Gets user timezone if previously cached.
 *
 * @returns {string | null} Cached timezone.
 */
export function getUserPreferredTimeZone(): string | null {
  return cachedUserTimeZone;
}

/**
 * Resolves timezone in priority order:
 * explicit arg -> cached users/me timezone -> browser timezone -> UTC.
 *
 * @param explicitTimeZone - Optional caller-provided timezone.
 * @returns {string} Effective timezone for display.
 */
export function resolveDisplayTimeZone(explicitTimeZone?: string): string {
  const explicit = normalizeUserTimeZone(explicitTimeZone);
  if (explicit) return explicit;

  if (cachedUserTimeZone) return cachedUserTimeZone;

  try {
    const browserZone = Intl.DateTimeFormat().resolvedOptions().timeZone;
    const normalizedBrowserZone = normalizeUserTimeZone(browserZone);
    if (normalizedBrowserZone) return normalizedBrowserZone;
  } catch {
    /* no-op */
  }

  return "UTC";
}

/**
 * Normalizes backend timestamp to ISO date-time string.
 * Unzoned backend formats are treated as UTC.
 *
 * @param rawTimestamp - Backend timestamp string.
 * @returns {string | null} ISO-like string parseable by Date.
 */
export function normalizeBackendTimestamp(
  rawTimestamp: string | null | undefined,
): string | null {
  const raw = rawTimestamp?.trim();
  if (!raw) return null;

  const spaceSeparated =
    /^(\d{4})-(\d{1,2})-(\d{1,2})\s+(\d{1,2}):(\d{1,2}):(\d{1,2})(\.\d+)?$/.exec(
      raw,
    );
  if (spaceSeparated) {
    const [, yyyy, mm, dd, hh, mi, ss, fractional = ""] = spaceSeparated;
    return `${yyyy}-${mm!.padStart(2, "0")}-${dd!.padStart(2, "0")}T${hh!.padStart(2, "0")}:${mi!.padStart(2, "0")}:${ss!.padStart(2, "0")}${fractional}Z`;
  }

  const mdy =
    /^(\d{1,2})\/(\d{1,2})\/(\d{4})\s+(\d{1,2}):(\d{1,2}):(\d{1,2})(\.\d+)?$/.exec(
      raw,
    );
  if (mdy) {
    const [, mm, dd, yyyy, hh, mi, ss, fractional = ""] = mdy;
    return `${yyyy}-${mm!.padStart(2, "0")}-${dd!.padStart(2, "0")}T${hh!.padStart(2, "0")}:${mi!.padStart(2, "0")}:${ss!.padStart(2, "0")}${fractional}Z`;
  }

  const tSeparated =
    /^(\d{4})-(\d{1,2})-(\d{1,2})T(\d{1,2}):(\d{1,2}):(\d{1,2})(\.\d+)?$/.exec(
      raw,
    );
  if (tSeparated) {
    const [, yyyy, mm, dd, hh, mi, ss, fractional = ""] = tSeparated;
    return `${yyyy}-${mm!.padStart(2, "0")}-${dd!.padStart(2, "0")}T${hh!.padStart(2, "0")}:${mi!.padStart(2, "0")}:${ss!.padStart(2, "0")}${fractional}Z`;
  }

  return raw;
}

/**
 * Parses backend timestamp to Date.
 *
 * @param rawTimestamp - Backend timestamp string.
 * @returns {Date | null} Parsed date, or null when invalid.
 */
export function parseBackendTimestamp(
  rawTimestamp: string | null | undefined,
): Date | null {
  const normalized = normalizeBackendTimestamp(rawTimestamp);
  if (!normalized) return null;
  const date = new Date(normalized);
  return Number.isNaN(date.getTime()) ? null : date;
}

/**
 * Formats backend timestamp in resolved user/browser timezone.
 *
 * @param rawTimestamp - Backend timestamp string.
 * @param options - Intl date-time options.
 * @param explicitTimeZone - Optional timezone override.
 * @param locale - Optional locale, defaults to en-US.
 * @returns {string | null} Formatted date-time or null when invalid.
 */
export function formatBackendTimestampForDisplay(
  rawTimestamp: string | null | undefined,
  options: Intl.DateTimeFormatOptions,
  explicitTimeZone?: string,
  locale = "en-US",
): string | null {
  const date = parseBackendTimestamp(rawTimestamp);
  if (!date) return null;
  const timeZone = resolveDisplayTimeZone(explicitTimeZone);
  return new Intl.DateTimeFormat(locale, { ...options, timeZone }).format(date);
}

/**
 * Offset in milliseconds to ADD to a UTC instant to obtain the wall-clock time
 * shown in `timeZone` at that instant. Positive for zones east of UTC.
 *
 * @param utcMs - UTC instant in epoch milliseconds.
 * @param timeZone - IANA timezone.
 * @returns {number} Offset in milliseconds.
 */
function timeZoneOffsetMs(utcMs: number, timeZone: string): number {
  const parts = new Intl.DateTimeFormat("en-US", {
    timeZone,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hourCycle: "h23", // 00-23; avoids the h24 midnight "24" edge case
  }).formatToParts(new Date(utcMs));
  const get = (t: Intl.DateTimeFormatPartTypes): number =>
    Number(parts.find((p) => p.type === t)?.value ?? "0");
  const asUtc = Date.UTC(
    get("year"),
    get("month") - 1,
    get("day"),
    get("hour"),
    get("minute"),
    get("second"),
  );
  return asUtc - utcMs;
}

/**
 * Interprets a `<input type="datetime-local">` wall-clock value ("YYYY-MM-DDTHH:mm")
 * as being in the resolved user timezone and returns the corresponding UTC ISO
 * instant. This keeps entry and display symmetric: the user types in their own
 * timezone, and we store/submit UTC.
 *
 * @param localValue - datetime-local input value.
 * @param explicitTimeZone - Optional timezone override.
 * @returns {string | null} UTC ISO string, or null when unparseable.
 */
export function zonedInputToUtcIso(
  localValue: string,
  explicitTimeZone?: string,
): string | null {
  const m = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})(?::(\d{2}))?$/.exec(
    localValue.trim(),
  );
  if (!m) {
    const d = new Date(localValue);
    return Number.isNaN(d.getTime()) ? null : d.toISOString();
  }
  const timeZone = resolveDisplayTimeZone(explicitTimeZone);
  const guessUtc = Date.UTC(
    Number(m[1]),
    Number(m[2]) - 1,
    Number(m[3]),
    Number(m[4]),
    Number(m[5]),
    Number(m[6] ?? "0"),
  );
  // Two passes so the offset is correct across a DST boundary.
  let offset = timeZoneOffsetMs(guessUtc, timeZone);
  offset = timeZoneOffsetMs(guessUtc - offset, timeZone);
  return new Date(guessUtc - offset).toISOString();
}

/**
 * Serialises a `<input type="datetime-local">` / picker wall-clock value
 * ("YYYY-MM-DDTHH:mm", entered in the resolved user timezone) to the backend's
 * `YYYY-MM-DD HH:mm:ss` wire format, expressed in UTC. The backend stores and
 * returns these fields as UTC and {@link normalizeBackendTimestamp} reads an
 * unzoned value as UTC, so sending the raw wall-clock digits (as several forms
 * did) shifted every value by the user's UTC offset.
 *
 * @param localValue - datetime-local value in the user's timezone.
 * @param explicitTimeZone - Optional timezone override.
 * @returns {string | null} "YYYY-MM-DD HH:mm:ss" in UTC, or null when unparseable.
 */
export function zonedInputToBackendUtc(
  localValue: string,
  explicitTimeZone?: string,
): string | null {
  const iso = zonedInputToUtcIso(localValue, explicitTimeZone);
  return iso ? `${iso.slice(0, 10)} ${iso.slice(11, 19)}` : null;
}

/**
 * Inverse of {@link zonedInputToBackendUtc}: a backend (UTC) timestamp as the
 * "YYYY-MM-DDTHH:mm" wall-clock value in the resolved user timezone, for
 * seeding an edit form's picker. Returns "" when the input is empty/unparseable.
 */
export function backendUtcToZonedInput(
  rawTimestamp: string | null | undefined,
  explicitTimeZone?: string,
): string {
  const date = parseBackendTimestamp(rawTimestamp);
  return date ? utcMsToZonedInputValue(date.getTime(), explicitTimeZone) : "";
}

/**
 * Formats a UTC instant as a `<input type="datetime-local">` wall-clock value
 * ("YYYY-MM-DDTHH:mm") in the resolved user timezone. Inverse of
 * {@link zonedInputToUtcIso}; used for the input's `min` attribute.
 *
 * @param utcMs - UTC instant in epoch milliseconds.
 * @param explicitTimeZone - Optional timezone override.
 * @returns {string} datetime-local value in the resolved timezone.
 */
export function utcMsToZonedInputValue(
  utcMs: number,
  explicitTimeZone?: string,
): string {
  const timeZone = resolveDisplayTimeZone(explicitTimeZone);
  const parts = new Intl.DateTimeFormat("en-CA", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    hourCycle: "h23", // 00-23; avoids the h24 midnight "24" edge case
    timeZone,
  }).formatToParts(new Date(utcMs));
  const get = (t: Intl.DateTimeFormatPartTypes): string =>
    parts.find((p) => p.type === t)?.value ?? "00";
  return `${get("year")}-${get("month")}-${get("day")}T${get("hour")}:${get("minute")}`;
}

/**
 * Formats a backend (UTC) timestamp as an absolute date-time string in the
 * user's resolved timezone, suitable for tooltip text. Format:
 *   "Jan 23, 2026, 10:14:13 PM GMT+5:30"
 *
 * Returns `null` when the input is empty or unparseable.
 */
export function formatAbsoluteForUser(
  rawTimestamp: string | null | undefined,
  explicitTimeZone?: string,
): string | null {
  const date = parseBackendTimestamp(rawTimestamp);
  if (!date) return null;
  const timeZone = resolveDisplayTimeZone(explicitTimeZone);
  try {
    return new Intl.DateTimeFormat("en-US", {
      year: "numeric",
      month: "short",
      day: "numeric",
      hour: "numeric",
      minute: "2-digit",
      second: "2-digit",
      timeZone,
      timeZoneName: "shortOffset",
    }).format(date);
  } catch {
    return null;
  }
}

/**
 * The UTC calendar day of a backend timestamp as "YYYY-MM-DD", for a field
 * that is a calendar date with no time of day (the auto-closure hold: the
 * backing system stores the picked day as that day at 00:00 UTC). Reading it
 * in the viewer's timezone instead would show the previous day to anyone west
 * of UTC.
 *
 * Returns `null` when the input is empty or unparseable.
 */
export function utcDateOnlyValue(
  rawTimestamp: string | null | undefined,
): string | null {
  const date = parseBackendTimestamp(rawTimestamp);
  return date ? date.toISOString().slice(0, 10) : null;
}

/**
 * {@link utcDateOnlyValue} formatted for display, e.g. "Oct 22, 2026". Always
 * the UTC calendar day, never shifted into the viewer's timezone.
 *
 * Returns `null` when the input is empty or unparseable.
 */
export function formatUtcDateForDisplay(
  rawTimestamp: string | null | undefined,
): string | null {
  const date = parseBackendTimestamp(rawTimestamp);
  if (!date) return null;
  return new Intl.DateTimeFormat("en-US", {
    year: "numeric",
    month: "short",
    day: "numeric",
    timeZone: "UTC",
  }).format(date);
}

/**
 * "YYYY-MM-DD" to a local-midnight Date, for handing a date-only field's wire
 * value to a picker component. Avoids the UTC-parse day-shift a plain
 * `new Date(dateString)` can cause depending on the viewer's timezone.
 */
export function parseDateOnly(value: string): Date | null {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value);
  if (!match) return null;
  const year = Number(match[1]);
  const month = Number(match[2]);
  const day = Number(match[3]);
  const date = new Date(year, month - 1, day);
  if (Number.isNaN(date.getTime())) return null;
  // The Date constructor silently normalizes an out-of-range day/month instead
  // of failing -- new Date(2026, 1, 31) (Feb 31) rolls forward to Mar 3, 2026
  // rather than returning an invalid date. `workDate` (the main caller through
  // formatRelativeDateOnly/formatDateOnlyForDisplay) is documented as
  // "occasionally unparseable on real records", so this isn't hypothetical:
  // silently accepting the rolled-over date would show the wrong calendar day
  // instead of the "—"/null fallback every caller already handles.
  if (
    date.getFullYear() !== year ||
    date.getMonth() !== month - 1 ||
    date.getDate() !== day
  ) {
    return null;
  }
  return date;
}

/**
 * "YYYY-MM-DD" to a human-readable date string (e.g. "Aug 1, 2026"), for
 * read-only display of a date-only field. Uses {@link parseDateOnly}'s
 * local-midnight parse rather than {@link formatAbsoluteForUser}'s UTC parse,
 * so the displayed day never shifts depending on the viewer's timezone.
 * Returns `null` when the input is empty or unparseable.
 */
export function formatDateOnlyForDisplay(
  value: string | null | undefined,
): string | null {
  if (!value) return null;
  const date = parseDateOnly(value);
  if (!date) return null;
  return new Intl.DateTimeFormat("en-US", {
    year: "numeric",
    month: "short",
    day: "numeric",
  }).format(date);
}

/**
 * "YYYY-MM-DD" to a relative-day label ("Today", "Yesterday", "3d ago"), for a
 * date-only field that has no time-of-day component at all (e.g. a time
 * card's `workDate` — "the date the engineer picked in the log form").
 *
 * Deliberately NOT built on {@link formatRelativeTime}'s epoch-millisecond
 * diff: that treats the value as a precise instant, which parses a date-only
 * string as UTC midnight and can drift into the wrong calendar day (or a
 * misleading "Xh ago") once compared against the viewer's local "now",
 * depending on their timezone offset. This compares calendar days instead —
 * both `value` and "today" parsed/rounded to local midnight via
 * {@link parseDateOnly} — so "today" reads as "Today" all day regardless of
 * the current hour or the viewer's zone, exactly like {@link isPastDateOnly}.
 *
 * `Math.round` (not a plain integer divide) absorbs the 23/25-hour day a DST
 * transition can produce between two local-midnight instants.
 *
 * @param value - Date-only string ("YYYY-MM-DD"), or null/empty/unparseable.
 * @param now - Reference "today", for tests; defaults to the current instant.
 * @returns {string} A relative-day label, or "—" when `value` is missing or unparseable.
 */
export function formatRelativeDateOnly(
  value: string | null | undefined,
  now: Date = new Date(),
): string {
  if (!value) return "—";
  const date = parseDateOnly(value);
  if (!date) return "—";

  const dayMs = 24 * 60 * 60 * 1000;
  const today = new Date(now);
  today.setHours(0, 0, 0, 0);
  const target = new Date(date);
  target.setHours(0, 0, 0, 0);

  const diffDays = Math.round((today.getTime() - target.getTime()) / dayMs);
  if (diffDays === 0) return "Today";
  if (diffDays === 1) return "Yesterday";
  if (diffDays === -1) return "Tomorrow";
  return diffDays > 1 ? `${diffDays}d ago` : `${Math.abs(diffDays)}d from now`;
}

/** Local-midnight Date back to "YYYY-MM-DD", the inverse of {@link parseDateOnly}. */
export function formatDateOnly(date: Date): string {
  const y = date.getFullYear();
  const m = String(date.getMonth() + 1).padStart(2, "0");
  const d = String(date.getDate()).padStart(2, "0");
  return `${y}-${m}-${d}`;
}

/**
 * "YYYY-MM-DDTHH:MM" (the `datetime-local` wire format some fields still
 * store their state as) to a local Date, for handing the value to a picker
 * component. Same browser-local semantics a native `datetime-local` input
 * already had — not timezone-aware beyond that.
 */
export function parseDateTimeLocal(value: string): Date | null {
  const match = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})$/.exec(value);
  if (!match) return null;
  const date = new Date(
    Number(match[1]),
    Number(match[2]) - 1,
    Number(match[3]),
    Number(match[4]),
    Number(match[5]),
  );
  return Number.isNaN(date.getTime()) ? null : date;
}

/** Local Date back to "YYYY-MM-DDTHH:MM", the inverse of {@link parseDateTimeLocal}. */
export function formatDateTimeLocal(date: Date): string {
  const y = date.getFullYear();
  const mo = String(date.getMonth() + 1).padStart(2, "0");
  const d = String(date.getDate()).padStart(2, "0");
  const h = String(date.getHours()).padStart(2, "0");
  const mi = String(date.getMinutes()).padStart(2, "0");
  return `${y}-${mo}-${d}T${h}:${mi}`;
}

/**
 * True when `date` represents an instant strictly before now. Shared check
 * behind the past-date/time warning shown on several pickers (some hard-block
 * on it, e.g. {@link SetAutocloseHoldDialog}/`ScheduleCallDialog`'s meeting
 * time; others only warn, e.g. a task's due date). `null`/invalid dates are
 * never flagged — an empty field isn't "in the past".
 *
 * @param date - Candidate Date, or null/invalid.
 * @returns {boolean} True when `date` is a valid instant before now.
 */
export function isPastDateTime(date: Date | null): boolean {
  return !!date && !Number.isNaN(date.getTime()) && date.getTime() < Date.now();
}

/**
 * True when a picker wall-clock value ("YYYY-MM-DDTHH:mm", entered in the
 * resolved user timezone) is an instant strictly before now. Unlike
 * `isPastDateTime(parseDateTimeLocal(value))`, which reads the digits in the
 * BROWSER timezone, this converts through {@link zonedInputToUtcIso} so the
 * check agrees with what is actually submitted whenever the profile timezone
 * differs from the browser's. Empty/unparseable values are never flagged.
 *
 * @param localValue - Picker value in the user's timezone.
 * @param explicitTimeZone - Optional timezone override.
 * @returns {boolean} True when the value denotes an instant before now.
 */
export function isPastZonedInput(
  localValue: string,
  explicitTimeZone?: string,
): boolean {
  if (!localValue) return false;
  const iso = zonedInputToUtcIso(localValue, explicitTimeZone);
  return isPastDateTime(iso ? new Date(iso) : null);
}

/**
 * True when `date` (a local-midnight Date, e.g. from {@link parseDateOnly})
 * falls on a calendar day strictly before today in the viewer's local time.
 * Compares day boundaries rather than instants, so "today" is never flagged
 * regardless of the current time of day. `null`/invalid dates are never
 * flagged.
 *
 * @param date - Candidate local-midnight Date, or null/invalid.
 * @returns {boolean} True when `date`'s calendar day is before today.
 */
export function isPastDateOnly(date: Date | null): boolean {
  if (!date || Number.isNaN(date.getTime())) return false;
  return date.getTime() < new Date().setHours(0, 0, 0, 0);
}

