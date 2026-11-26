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
import { MAX_HOLIDAYS_PER_ADD, parseHolidayLines } from "./holidayLines";

describe("parseHolidayLines", () => {
  it("reads a date and a title with any of the usual separators", () => {
    const { holidays, errors } = parseHolidayLines(
      [
        "2027-01-14, Tamil Thai Pongal Day",
        "2027-02-04\tIndependence Day",
        "2027-04-13; Day prior to Sinhala and Tamil New Year",
        "2027-05-01 | May Day",
        "2027-05-20 - Vesak Full Moon Poya Day",
        "2027-12-25 Christmas Day",
      ].join("\n"),
    );
    expect(errors).toEqual([]);
    expect(holidays).toEqual([
      { date: "2027-01-14", title: "Tamil Thai Pongal Day" },
      { date: "2027-02-04", title: "Independence Day" },
      { date: "2027-04-13", title: "Day prior to Sinhala and Tamil New Year" },
      { date: "2027-05-01", title: "May Day" },
      { date: "2027-05-20", title: "Vesak Full Moon Poya Day" },
      { date: "2027-12-25", title: "Christmas Day" },
    ]);
  });

  it("skips blank and # lines, and reads Windows line ends", () => {
    const { holidays, errors } = parseHolidayLines("# Sri Lanka 2027\r\n\r\n2027-01-14, Thai Pongal\r\n");
    expect(errors).toEqual([]);
    expect(holidays).toEqual([{ date: "2027-01-14", title: "Thai Pongal" }]);
  });

  it("reports each bad line with its number and why", () => {
    const { holidays, errors } = parseHolidayLines(
      ["14/01/2027, Thai Pongal", "2027-02-30, Not a day", "2027-03-01", "2027-05-01, May Day", "2027-05-01, Labour Day"].join(
        "\n",
      ),
    );
    expect(holidays).toEqual([{ date: "2027-05-01", title: "May Day" }]);
    expect(errors.map((e) => [e.line, e.reason])).toEqual([
      [1, "Write it as YYYY-MM-DD, then the holiday's name."],
      [2, "2027-02-30 is not a real date."],
      [3, "Write it as YYYY-MM-DD, then the holiday's name."],
      [5, "2027-05-01 is already on line 4."],
    ]);
  });

  it("refuses a name over the limit and a paste over the server's limit", () => {
    expect(parseHolidayLines(`2027-01-01, ${"x".repeat(201)}`).errors[0].reason).toMatch(/longer than 200/);
    const many = Array.from({ length: MAX_HOLIDAYS_PER_ADD + 1 }, (_, i) => {
      const d = new Date(Date.UTC(2027, 0, 1 + i)).toISOString().slice(0, 10);
      return `${d}, Day ${i}`;
    }).join("\n");
    expect(parseHolidayLines(many).errors.map((e) => e.reason)).toEqual([`At most ${MAX_HOLIDAYS_PER_ADD} holidays at a time.`]);
  });
});
