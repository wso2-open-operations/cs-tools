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

import { describe, expect, it } from "vitest";
import { holidayFileToLines } from "./holidayFile";
import { parseHolidayLines } from "./holidayLines";

const ics = (events: string[]): string =>
  ["BEGIN:VCALENDAR", "VERSION:2.0", ...events.flatMap((e) => ["BEGIN:VEVENT", ...e.split("\n"), "END:VEVENT"]), "END:VCALENDAR", ""].join("\r\n");

describe("holidayFileToLines: a calendar (.ics)", () => {
  const calendar = ics([
    "DTSTART;VALUE=DATE:20270115\nDTEND;VALUE=DATE:20270116\nSUMMARY:Tamil Thai Pongal Day",
    // Two days, end day not included; a folded, escaped name.
    "DTSTART;VALUE=DATE:20270413\nDTEND;VALUE=DATE:20270415\nSUMMARY:Sinhala and Tamil New Year\\, \n holidays",
    // A second holiday on the same day.
    "DTSTART;VALUE=DATE:20270413\nSUMMARY:Bak Poya",
    // Google's observances are not days off.
    "DTSTART;VALUE=DATE:20270214\nSUMMARY:Valentine's Day\nDESCRIPTION:Observance\\nTo hide observances",
    // A meeting has a time of day.
    "DTSTART:20270120T090000Z\nDTEND:20270120T100000Z\nSUMMARY:Team meeting",
    // Another year.
    "DTSTART;VALUE=DATE:20260115\nSUMMARY:Tamil Thai Pongal Day",
  ]);

  it("reads the year's whole-day holidays, one line per day, oldest first", () => {
    const r = holidayFileToLines("holidays.ics", calendar, 2027);
    expect(r.text.split("\n")).toEqual([
      "2027-01-15, Tamil Thai Pongal Day",
      "2027-04-13, Sinhala and Tamil New Year, holidays / Bak Poya",
      "2027-04-14, Sinhala and Tamil New Year, holidays",
    ]);
    expect(r.count).toBe(3);
    expect(r.note).toBe("3 holidays in 2027 read from the calendar; left out 1 observance and 1 event with a time of day.");
  });

  it("gives lines the paste box accepts as they are", () => {
    const parsed = parseHolidayLines(holidayFileToLines("x.ics", calendar, 2027).text);
    expect(parsed.errors).toEqual([]);
    expect(parsed.holidays).toHaveLength(3);
  });

  it("knows a calendar by its content whatever the file is called", () => {
    expect(holidayFileToLines("export.txt", calendar, 2026).text).toBe("2026-01-15, Tamil Thai Pongal Day");
  });
});

describe("holidayFileToLines: a table or plain lines", () => {
  it("reads a CSV with a header row and quoted names", () => {
    const r = holidayFileToLines(
      "lk.csv",
      '\uFEFFDate,Holiday,Type\r\n2027-01-15,"Tamil Thai Pongal Day",Public\r\n2027-02-04,"National Day, Sri Lanka",Public\r\n',
      2027,
    );
    expect(r.text).toBe("2027-01-15, Tamil Thai Pongal Day\n2027-02-04, National Day, Sri Lanka");
    expect(r.note).toBe("2 holiday lines read from the file; skipped the header row.");
    expect(parseHolidayLines(r.text).errors).toEqual([]);
  });

  it("reads a spreadsheet column copied as tab-separated text", () => {
    expect(holidayFileToLines("x.tsv", "2027-01-15\tTamil Thai Pongal Day\n2027-02-04\tNational Day", 2027).text).toBe(
      "2027-01-15, Tamil Thai Pongal Day\n2027-02-04, National Day",
    );
  });

  it("keeps plain lines and comments as written, so the paste box names a bad one", () => {
    const r = holidayFileToLines("lk.txt", "# Sri Lanka 2027\n2027-01-15 Tamil Thai Pongal Day\n15/04/2027, Wrong\n", 2027);
    expect(r.text).toBe("# Sri Lanka 2027\n2027-01-15 Tamil Thai Pongal Day\n15/04/2027, Wrong");
    expect(r.count).toBe(1);
    expect(parseHolidayLines(r.text).errors.map((e) => e.line)).toEqual([3]);
  });
});

describe("holidayFileToLines: what CodeRabbit found", () => {
  it("walks only the chosen year of an event that spans centuries", () => {
    const calendar = ics(["DTSTART;VALUE=DATE:20270101\nDTEND;VALUE=DATE:99991231\nSUMMARY:Forever"]);
    const started = Date.now();
    const r = holidayFileToLines("x.ics", calendar, 2027);
    expect(Date.now() - started).toBeLessThan(1000);
    expect(r.count).toBe(365);
    expect(r.text.split("\n").at(-1)).toBe("2027-12-31, Forever");
  });

  it("keeps an event that crosses into the chosen year, end day still left out", () => {
    const calendar = ics(["DTSTART;VALUE=DATE:20261231\nDTEND;VALUE=DATE:20270102\nSUMMARY:New Year"]);
    expect(holidayFileToLines("x.ics", calendar, 2027).text).toBe("2027-01-01, New Year");
    expect(holidayFileToLines("x.ics", calendar, 2026).text).toBe("2026-12-31, New Year");
  });

  it("leaves out a day that does not exist instead of moving it", () => {
    const calendar = ics([
      "DTSTART;VALUE=DATE:20270230\nSUMMARY:Not a day",
      "DTSTART;VALUE=DATE:20270301\nDTEND;VALUE=DATE:20270232\nSUMMARY:Bad end",
      "DTSTART;VALUE=DATE:20270302\nSUMMARY:Real",
    ]);
    const r = holidayFileToLines("x.ics", calendar, 2027);
    expect(r.text).toBe("2027-03-02, Real");
    expect(r.note).toBe("1 holiday in 2027 read from the calendar; left out 2 events with a date that does not exist.");
  });

  it("keeps the whole name of a plain line with commas", () => {
    const r = holidayFileToLines("x.txt", "2027-02-04, National Day, Example\n", 2027);
    expect(r.text).toBe("2027-02-04, National Day, Example");
    expect(parseHolidayLines(r.text).holidays).toEqual([{ date: "2027-02-04", title: "National Day, Example" }]);
  });

  it("takes the separator from outside the quotes", () => {
    const r = holidayFileToLines("x.csv", '2027-02-04;"National Day, Example";Public\n2027-02-05,Plain,Public\n', 2027);
    expect(r.text).toBe("2027-02-04, National Day, Example\n2027-02-05, Plain");
  });
});
