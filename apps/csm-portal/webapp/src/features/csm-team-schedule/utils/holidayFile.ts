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

/** The largest file the holiday import reads (a year of holidays is a few KB). */
export const MAX_HOLIDAY_FILE_BYTES = 1024 * 1024;

/** What an imported file gives the paste box: its lines, and a sentence saying what was read. */
export interface HolidayFileLines {
  /** `YYYY-MM-DD, Name` lines (a text file's own lines when they are not a table). */
  text: string;
  /** How many days were found, and what was left out. */
  note: string;
  /** How many holiday lines `text` holds. */
  count: number;
}

const DATE = /^\d{4}-\d{2}-\d{2}$/;

/**
 * Turns an imported file into lines for the paste box, which then checks them
 * like any paste (nothing is sent until Add holidays).
 *
 * - **A calendar (.ics)**, for example a holiday calendar exported from Google:
 *   the whole-day events of `year` only (a calendar spans many years), an
 *   event's end day not included, events with a time of day left out, and
 *   Google's "Observance" entries (not days off) left out. Two holidays on one
 *   day become one line, names joined with " / ".
 * - **A table (.csv, .tsv, or text with commas, semicolons or tabs)**: the first
 *   column is the date and the second the name; quotes are removed and a header
 *   row (a first line naming "date") is skipped.
 * - **Anything else** is kept line by line, so a file already written as
 *   `YYYY-MM-DD, Name` lines reads as it is.
 */
export function holidayFileToLines(fileName: string, content: string, year: number): HolidayFileLines {
  // A file saved with a byte order mark (Excel does) starts with U+FEFF.
  const text = content.charCodeAt(0) === 0xfeff ? content.slice(1) : content;
  if (/\.ics$/i.test(fileName) || text.trimStart().startsWith("BEGIN:VCALENDAR")) {
    return calendarLines(text, year);
  }
  return tableLines(text);
}

function calendarLines(text: string, year: number): HolidayFileLines {
  const days = new Map<string, string[]>();
  let timed = 0;
  let observances = 0;
  for (const ev of calendarEvents(text)) {
    const start = wholeDay(ev.get("DTSTART"));
    if (!start) {
      timed++;
      continue;
    }
    if (unescapeIcs(ev.get("DESCRIPTION")?.value ?? "").toLowerCase().startsWith("observance")) {
      observances++;
      continue;
    }
    const title = unescapeIcs(ev.get("SUMMARY")?.value ?? "");
    if (!title) continue;
    const endExclusive = wholeDay(ev.get("DTEND")) ?? addDays(start, 1);
    for (let d = start; d < endExclusive; d = addDays(d, 1)) {
      if (d.getUTCFullYear() !== year) continue;
      const key = d.toISOString().slice(0, 10);
      const names = days.get(key) ?? [];
      if (!names.includes(title)) names.push(title);
      days.set(key, names);
    }
  }
  const lines = [...days.entries()].sort(([a], [b]) => a.localeCompare(b)).map(([d, names]) => `${d}, ${names.join(" / ")}`);
  const left: string[] = [];
  if (observances) left.push(`${observances} observance${observances === 1 ? "" : "s"}`);
  if (timed) left.push(`${timed} event${timed === 1 ? "" : "s"} with a time of day`);
  const note =
    `${lines.length} holiday${lines.length === 1 ? "" : "s"} in ${year} read from the calendar` +
    (left.length ? `; left out ${left.join(" and ")}.` : ".");
  return { text: lines.join("\n"), note, count: lines.length };
}

type IcsProp = { params: string; value: string };

/** Each VEVENT's properties (the first of each name), after RFC 5545 line unfolding. */
function calendarEvents(text: string): Map<string, IcsProp>[] {
  const lines: string[] = [];
  for (const raw of text.split(/\r\n|\r|\n/)) {
    if ((raw.startsWith(" ") || raw.startsWith("\t")) && lines.length) lines[lines.length - 1] += raw.slice(1);
    else lines.push(raw);
  }
  const events: Map<string, IcsProp>[] = [];
  let current: Map<string, IcsProp> | null = null;
  for (const line of lines) {
    if (line === "BEGIN:VEVENT") current = new Map();
    else if (line === "END:VEVENT") {
      if (current) events.push(current);
      current = null;
    } else if (current) {
      const colon = line.indexOf(":");
      if (colon < 0) continue;
      const [name, ...params] = line.slice(0, colon).split(";");
      const key = name.toUpperCase();
      if (!current.has(key)) current.set(key, { params: params.join(";").toUpperCase(), value: line.slice(colon + 1) });
    }
  }
  return events;
}

/** The day of a whole-day DTSTART / DTEND (YYYYMMDD), or null for a timed or missing one. */
function wholeDay(prop: IcsProp | undefined): Date | null {
  if (!prop) return null;
  const v = prop.value.trim();
  if (prop.params.includes("VALUE=DATE-TIME") || !/^\d{8}$/.test(v)) return null;
  const d = new Date(Date.UTC(Number(v.slice(0, 4)), Number(v.slice(4, 6)) - 1, Number(v.slice(6, 8))));
  return Number.isNaN(d.getTime()) ? null : d;
}

function addDays(d: Date, n: number): Date {
  return new Date(d.getTime() + n * 86_400_000);
}

function unescapeIcs(value: string): string {
  return value
    .replace(/\\([nN,;\\])/g, (_, c: string) => (c === "n" || c === "N" ? " " : c))
    .replace(/\s+/g, " ")
    .trim();
}

function tableLines(text: string): HolidayFileLines {
  const out: string[] = [];
  let count = 0;
  let header = false;
  let seenContent = false;
  for (const raw of text.split(/\r\n|\r|\n/)) {
    const line = raw.trim();
    if (!line || line.startsWith("#")) {
      out.push(line);
      continue;
    }
    const cells = splitCells(line);
    const first = !seenContent;
    seenContent = true;
    if (cells.length >= 2 && DATE.test(cells[0])) {
      out.push(`${cells[0]}, ${cells[1]}`);
      count++;
    } else if (first && /date/i.test(line) && !/^\d/.test(cells[0] ?? "")) {
      header = true;
    } else {
      // Not a table row: kept as written, for the paste box to read or name.
      out.push(line);
      if (/^\d{4}-\d{2}-\d{2}\b/.test(line)) count++;
    }
  }
  const lines = out.join("\n").replace(/^\n+|\n+$/g, "");
  const note = `${count} holiday line${count === 1 ? "" : "s"} read from the file` + (header ? "; skipped the header row." : ".");
  return { text: lines, note, count };
}

/** A row's cells, split on its tab, comma or semicolon (the first one it has), quotes removed. */
function splitCells(line: string): string[] {
  const sep = line.includes("\t") ? "\t" : line.includes(",") ? "," : line.includes(";") ? ";" : null;
  if (!sep) return [line];
  const cells: string[] = [];
  let cur = "";
  let quoted = false;
  for (let i = 0; i < line.length; i++) {
    const c = line[i];
    if (c === '"') {
      if (quoted && line[i + 1] === '"') {
        cur += '"';
        i++;
      } else quoted = !quoted;
    } else if (c === sep && !quoted) {
      cells.push(cur.trim());
      cur = "";
    } else cur += c;
  }
  cells.push(cur.trim());
  return cells;
}
