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
 *   "2026-05-31 20:28:33 GMT+5:30"
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
    const parts = new Intl.DateTimeFormat("en-CA", {
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
      hour: "2-digit",
      minute: "2-digit",
      second: "2-digit",
      hourCycle: "h23", // 00-23; avoids the h24 midnight "24" edge case
      timeZone,
      timeZoneName: "shortOffset",
    }).formatToParts(date);
    const find = (type: Intl.DateTimeFormatPartTypes): string =>
      parts.find((p) => p.type === type)?.value ?? "";
    const datePart = `${find("year")}-${find("month")}-${find("day")}`;
    const timePart = `${find("hour")}:${find("minute")}:${find("second")}`;
    const tzPart = find("timeZoneName") || "UTC";
    return `${datePart} ${timePart} ${tzPart}`;
  } catch {
    return null;
  }
}

