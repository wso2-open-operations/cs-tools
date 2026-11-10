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
import type { SchedulePublicHoliday } from "../types";
import { holidayNote, holidaysByPersonAndDay, withHolidayNote } from "./publicHolidays";

const poya: SchedulePublicHoliday = {
  userId: "u1",
  date: "2026-11-24",
  title: "Il Full Moon Poya Day",
  country: "Sri Lanka",
  countrySource: "TEAM",
};

describe("public holidays", () => {
  it("indexes by person and day, so a holiday applies to its own person only", () => {
    const m = holidaysByPersonAndDay([poya, { ...poya, userId: "u2", title: "Guru Nanak Jayanti", country: "India" }]);
    expect(m.get("u1|2026-11-24")?.country).toBe("Sri Lanka");
    expect(m.get("u2|2026-11-24")?.country).toBe("India");
    expect(m.has("u3|2026-11-24")).toBe(false);
  });

  it("adds the holiday to a cell's title, or is the title on an empty cell", () => {
    expect(holidayNote(poya)).toBe("Public holiday: Il Full Moon Poya Day (Sri Lanka)");
    expect(withHolidayNote("Ann · LK", poya)).toBe("Ann · LK · Public holiday: Il Full Moon Poya Day (Sri Lanka)");
    expect(withHolidayNote(undefined, poya)).toBe("Public holiday: Il Full Moon Poya Day (Sri Lanka)");
    expect(withHolidayNote("Ann · LK", undefined)).toBe("Ann · LK");
    expect(withHolidayNote(undefined, undefined)).toBeUndefined();
  });
});
