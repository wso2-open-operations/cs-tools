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

