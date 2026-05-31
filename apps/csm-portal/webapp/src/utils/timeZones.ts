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

import type { TimeZoneOption } from "@features/project-hub/types/projects";

/**
 * Returns the current UTC offset of an IANA time zone, formatted as
 * `UTC+5:30`, `UTC-04:00`, or `UTC` (zero offset). Returns empty string if
 * the runtime cannot resolve the zone.
 */
export function formatUtcOffset(timeZone: string): string {
  if (!timeZone) return "";
  try {
    const parts = new Intl.DateTimeFormat("en-US", {
      timeZone,
      timeZoneName: "longOffset",
    }).formatToParts(new Date());
    const raw =
      parts.find((p) => p.type === "timeZoneName")?.value?.trim() ?? "";
    if (!raw || raw === "GMT") return "UTC";
    // Examples: "GMT+05:30", "GMT-04:00", "UTC+5"
    const normalized = raw.replace(/^GMT/, "UTC").replace(/^UTC/, "UTC");
    // Strip the leading zero on the hour ("UTC+05:30" -> "UTC+5:30") since
    // that's the format the user asked for.
    return normalized.replace(/^(UTC[+-])0(\d)/, "$1$2");
  } catch {
    return "";
  }
}

/**
 * Decorates a time-zone label with its current UTC offset, e.g.
 * `Asia/Colombo (UTC+5:30)`. If the label already includes a UTC/GMT marker,
 * returns it unchanged.
 */
export function decorateTimeZoneLabel(
  id: string,
  label: string | undefined,
): string {
  const base = label && label.length > 0 ? label : id;
  if (/UTC|GMT/i.test(base)) return base;
  const offset = formatUtcOffset(id);
  if (!offset) return base;
  return `${base} (${offset})`;
}

/**
 * Curated set of common IANA time zones, used as the source-of-truth when
 * the metadata backend is unavailable (mock mode). Ordered roughly by UTC
 * offset so the dropdown reads west-to-east.
 */
export const MOCK_TIME_ZONES: TimeZoneOption[] = [
  { id: "Pacific/Pago_Pago", label: "Pago Pago" },
  { id: "Pacific/Honolulu", label: "Hawaii" },
  { id: "America/Anchorage", label: "Alaska" },
  { id: "America/Los_Angeles", label: "Pacific Time (US & Canada)" },
  { id: "America/Phoenix", label: "Arizona" },
  { id: "America/Denver", label: "Mountain Time (US & Canada)" },
  { id: "America/Chicago", label: "Central Time (US & Canada)" },
  { id: "America/Mexico_City", label: "Mexico City" },
  { id: "America/New_York", label: "Eastern Time (US & Canada)" },
  { id: "America/Toronto", label: "Toronto" },
  { id: "America/Halifax", label: "Atlantic Time (Canada)" },
  { id: "America/Sao_Paulo", label: "São Paulo" },
  { id: "America/Argentina/Buenos_Aires", label: "Buenos Aires" },
  { id: "Atlantic/Azores", label: "Azores" },
  { id: "Europe/London", label: "London" },
  { id: "Europe/Dublin", label: "Dublin" },
  { id: "Africa/Casablanca", label: "Casablanca" },
  { id: "UTC", label: "UTC" },
  { id: "Europe/Berlin", label: "Berlin" },
  { id: "Europe/Paris", label: "Paris" },
  { id: "Europe/Madrid", label: "Madrid" },
  { id: "Europe/Rome", label: "Rome" },
  { id: "Europe/Amsterdam", label: "Amsterdam" },
  { id: "Europe/Stockholm", label: "Stockholm" },
  { id: "Europe/Zurich", label: "Zurich" },
  { id: "Africa/Lagos", label: "Lagos" },
  { id: "Africa/Cairo", label: "Cairo" },
  { id: "Europe/Athens", label: "Athens" },
  { id: "Europe/Helsinki", label: "Helsinki" },
  { id: "Europe/Istanbul", label: "Istanbul" },
  { id: "Europe/Moscow", label: "Moscow" },
  { id: "Africa/Johannesburg", label: "Johannesburg" },
  { id: "Asia/Jerusalem", label: "Jerusalem" },
  { id: "Asia/Riyadh", label: "Riyadh" },
  { id: "Asia/Dubai", label: "Dubai" },
  { id: "Asia/Tehran", label: "Tehran" },
  { id: "Asia/Karachi", label: "Karachi" },
  { id: "Asia/Kabul", label: "Kabul" },
  { id: "Asia/Kolkata", label: "India Standard Time" },
  { id: "Asia/Colombo", label: "Colombo" },
  { id: "Asia/Kathmandu", label: "Kathmandu" },
  { id: "Asia/Dhaka", label: "Dhaka" },
  { id: "Asia/Yangon", label: "Yangon" },
  { id: "Asia/Bangkok", label: "Bangkok" },
  { id: "Asia/Jakarta", label: "Jakarta" },
  { id: "Asia/Singapore", label: "Singapore" },
  { id: "Asia/Hong_Kong", label: "Hong Kong" },
  { id: "Asia/Shanghai", label: "Shanghai" },
  { id: "Asia/Taipei", label: "Taipei" },
  { id: "Asia/Manila", label: "Manila" },
  { id: "Asia/Seoul", label: "Seoul" },
  { id: "Asia/Tokyo", label: "Tokyo" },
  { id: "Australia/Perth", label: "Perth" },
  { id: "Australia/Adelaide", label: "Adelaide" },
  { id: "Australia/Brisbane", label: "Brisbane" },
  { id: "Australia/Sydney", label: "Sydney" },
  { id: "Pacific/Auckland", label: "Auckland" },
  { id: "Pacific/Fiji", label: "Fiji" },
];
