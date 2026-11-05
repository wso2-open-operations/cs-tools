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

import { format } from "date-fns";

const DATE_TIME_LOCAL_RE = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})/;
const DATE_ONLY_RE = /^(\d{4})-(\d{2})-(\d{2})/;

/** Earliest year treated as a finished entry; "2" typed so far is a legal Date but not one anybody meant. */
const MIN_COMPLETE_YEAR = 1000;

/**
 * Parses a `YYYY-MM-DDTHH:mm` wall-clock string into a Date carrying the same
 * wall-clock fields in the browser's zone. No zone conversion happens here: the
 * string stays the source of truth and callers convert it to an instant with
 * their own (profile) time zone.
 *
 * @param {string | null | undefined} value - A datetime-local style string.
 * @returns {Date | null} The Date, or null for an empty or unparseable value.
 */
export function parseDateTimeLocal(value: string | null | undefined): Date | null {
  const match = DATE_TIME_LOCAL_RE.exec(value?.trim() ?? "");
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

/**
 * Formats a Date's wall-clock fields as `YYYY-MM-DDTHH:mm`.
 *
 * @param {Date} date - A valid Date.
 * @returns {string} The datetime-local style string.
 */
export function formatDateTimeLocal(date: Date): string {
  return format(date, "yyyy-MM-dd'T'HH:mm");
}

/**
 * Parses a `YYYY-MM-DD` string into a local-midnight Date.
 *
 * @param {string | null | undefined} value - A date-only string.
 * @returns {Date | null} The Date, or null for an empty or unparseable value.
 */
export function parseDateOnly(value: string | null | undefined): Date | null {
  const match = DATE_ONLY_RE.exec(value?.trim() ?? "");
  if (!match) return null;
  const date = new Date(Number(match[1]), Number(match[2]) - 1, Number(match[3]));
  return Number.isNaN(date.getTime()) ? null : date;
}

/**
 * Formats a Date as `YYYY-MM-DD` from its local calendar fields.
 *
 * @param {Date} date - A valid Date.
 * @returns {string} The date-only string.
 */
export function formatDateOnly(date: Date): string {
  return format(date, "yyyy-MM-dd");
}

/**
 * Whether a picker value is a finished, valid date. A masked field reports an
 * invalid Date while sections are missing, and a year typed only as far as "2"
 * is a legal but unintended one, so both count as unfinished.
 *
 * @param {unknown} date - The value a picker's onChange reported.
 * @returns {boolean} True for a valid Date with a four-digit year.
 */
export function isCompleteDate(date: unknown): date is Date {
  return (
    date instanceof Date &&
    !Number.isNaN(date.getTime()) &&
    date.getFullYear() >= MIN_COMPLETE_YEAR
  );
}
