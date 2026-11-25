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
 *   A day that does not exist (20270230) is left out and counted, never moved
 *   to the day the date rolls over to.
 * - **A table (.csv or .tsv, or a line with tabs)**: the first column is the
 *   date and the second the name; quotes are removed, the separator is the one
 *   used outside quotes, and a header row (a first line naming "date") is
 *   skipped.
 * - **Anything else** is kept line by line, so a file already written as
 *   `YYYY-MM-DD, Name` lines reads as it is, a name with commas included.
 */
export function holidayFileToLines(fileName: string, content: string, year: number): HolidayFileLines {
  // A file saved with a byte order mark (Excel does) starts with U+FEFF.
  const text = content.charCodeAt(0) === 0xfeff ? content.slice(1) : content;
  if (/\.ics$/i.test(fileName) || text.trimStart().startsWith("BEGIN:VCALENDAR")) {
    return calendarLines(text, year);
  }
  return tableLines(text, /\.(csv|tsv)$/i.test(fileName));
}

function calendarLines(text: string, year: number): HolidayFileLines {
  const days = new Map<string, string[]>();
  let timed = 0;
  let observances = 0;
  let impossible = 0;
  const yearStart = new Date(Date.UTC(year, 0, 1));
  const nextYearStart = new Date(Date.UTC(year + 1, 0, 1));
  for (const ev of calendarEvents(text)) {
    const start = wholeDay(ev.get("DTSTART"));
    const end = wholeDay(ev.get("DTEND"));
    if (start === "impossible" || end === "impossible") {
      impossible++;
      continue;
    }
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
    const endExclusive = end ?? addDays(start, 1);
    // Only the days inside the chosen year are visited: an event can span
    // centuries, and walking all of it would hang the page.
    const from = start > yearStart ? start : yearStart;
    const until = endExclusive < nextYearStart ? endExclusive : nextYearStart;
    for (let d = from; d < until; d = addDays(d, 1)) {
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
  if (impossible) left.push(`${impossible} event${impossible === 1 ? "" : "s"} with a date that does not exist`);
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

/**
 * The day of a whole-day DTSTART / DTEND (YYYYMMDD), null for a timed or
 * missing one, or "impossible" for a day that does not exist (20270230, which
 * Date.UTC would quietly turn into 2 March).
 */
function wholeDay(prop: IcsProp | undefined): Date | "impossible" | null {
  if (!prop) return null;
  const v = prop.value.trim();
  if (prop.params.includes("VALUE=DATE-TIME") || !/^\d{8}$/.test(v)) return null;
  const y = Number(v.slice(0, 4));
  const m = Number(v.slice(4, 6)) - 1;
  const day = Number(v.slice(6, 8));
  const d = new Date(Date.UTC(y, m, day));
  if (d.getUTCFullYear() !== y || d.getUTCMonth() !== m || d.getUTCDate() !== day) return "impossible";
  return d;
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

/**
 * A table file (.csv / .tsv) or a line with tabs (a spreadsheet column) is read
 * as date and name columns; any other line is kept as written, so a plain
 * `YYYY-MM-DD, Name, with commas` keeps its whole name.
 */
function tableLines(text: string, isTable: boolean): HolidayFileLines {
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
    const cells = isTable || line.includes("\t") ? splitCells(line) : [line];
    const first = !seenContent;
    seenContent = true;
    if (cells.length >= 2 && DATE.test(cells[0])) {
      out.push(`${cells[0]}, ${cells[1]}`);
      count++;
    } else if (first && /date/i.test(line) && !/^\d/.test(line)) {
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

/** The row's separator: a tab, else a comma, else a semicolon, counting only those outside quotes. */
function separatorOf(line: string): string | null {
  const found = new Set<string>();
  let quoted = false;
  for (const c of line) {
    if (c === '"') quoted = !quoted;
    else if (!quoted && (c === "\t" || c === "," || c === ";")) found.add(c);
  }
  return ["\t", ",", ";"].find((c) => found.has(c)) ?? null;
}

/** A row's cells, split on its separator (outside quotes), quotes removed. */
function splitCells(line: string): string[] {
  const sep = separatorOf(line);
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
