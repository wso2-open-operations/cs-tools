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

import type { ManualPublicHoliday } from "../types";

/** The most days one paste may add (the server's own limit). */
export const MAX_HOLIDAYS_PER_ADD = 200;

/** The longest title the server keeps. */
export const MAX_HOLIDAY_TITLE = 200;

/** A line of the paste that could not be read, and why. */
export interface HolidayLineError {
  /** 1-based line number in the pasted text. */
  line: number;
  text: string;
  reason: string;
}

export interface ParsedHolidayLines {
  holidays: ManualPublicHoliday[];
  errors: HolidayLineError[];
}

/** A date, then a separator (comma, tab, semicolon, bar, " - " or spaces), then the title. */
const LINE = /^(\d{4}-\d{2}-\d{2})(?:\s*[,\t;|]\s*|\s+-\s+|\s+)(.+)$/;

/** Whether a YYYY-MM-DD string is a real calendar day (no 2027-02-30). */
function isRealDate(s: string): boolean {
  const [y, m, d] = s.split("-").map(Number);
  const dt = new Date(Date.UTC(y, m - 1, d));
  return dt.getUTCFullYear() === y && dt.getUTCMonth() === m - 1 && dt.getUTCDate() === d;
}

/**
 * Reads pasted holidays, one per line: `YYYY-MM-DD, Title` (a tab, semicolon,
 * bar, " - " or plain spaces between them work too, so a column copied from a
 * spreadsheet pastes as it is). Blank lines and lines starting with `#` are
 * skipped. A line that is not a real date followed by a title, a title over
 * the limit, or a date listed twice is reported with its line number, and the
 * whole paste should then be fixed before it is sent: the server refuses a
 * request with any bad day.
 */
export function parseHolidayLines(text: string): ParsedHolidayLines {
  const holidays: ManualPublicHoliday[] = [];
  const errors: HolidayLineError[] = [];
  const seen = new Map<string, number>();
  text.split(/\r?\n/).forEach((raw, i) => {
    const line = raw.trim();
    if (!line || line.startsWith("#")) return;
    const m = LINE.exec(line);
    if (!m) {
      errors.push({ line: i + 1, text: line, reason: "Write it as YYYY-MM-DD, then the holiday's name." });
      return;
    }
    const [, date, rawTitle] = m;
    const title = rawTitle.trim();
    if (!isRealDate(date)) {
      errors.push({ line: i + 1, text: line, reason: `${date} is not a real date.` });
      return;
    }
    if (title.length > MAX_HOLIDAY_TITLE) {
      errors.push({ line: i + 1, text: line, reason: `The name is longer than ${MAX_HOLIDAY_TITLE} characters.` });
      return;
    }
    const first = seen.get(date);
    if (first !== undefined) {
      errors.push({ line: i + 1, text: line, reason: `${date} is already on line ${first}.` });
      return;
    }
    seen.set(date, i + 1);
    holidays.push({ date, title });
  });
  if (holidays.length > MAX_HOLIDAYS_PER_ADD) {
    errors.push({ line: 0, text: "", reason: `At most ${MAX_HOLIDAYS_PER_ADD} holidays at a time.` });
  }
  return { holidays, errors };
}
