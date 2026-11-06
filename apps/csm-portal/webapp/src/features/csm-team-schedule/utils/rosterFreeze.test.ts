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
import { fmtIstMinute, isRosterFreeze, msUntilRosterFreezeChange } from "./rosterFreeze";

/** A moment given in IST (UTC+05:30). */
const ist = (hhmm: string, day = "2026-11-03"): Date => new Date(`${day}T${hhmm}:00+05:30`);

describe("isRosterFreeze", () => {
  it("is on from 09:15 IST up to, but not including, 09:30 IST", () => {
    expect(isRosterFreeze(ist("09:14"))).toBe(false);
    expect(isRosterFreeze(new Date(ist("09:15").getTime() - 1))).toBe(false);
    expect(isRosterFreeze(ist("09:15"))).toBe(true);
    expect(isRosterFreeze(ist("09:29"))).toBe(true);
    expect(isRosterFreeze(new Date(ist("09:30").getTime() - 1))).toBe(true);
    expect(isRosterFreeze(ist("09:30"))).toBe(false);
    expect(isRosterFreeze(ist("21:20"))).toBe(false);
  });

  it("goes by IST whatever the instant's own zone: 03:50 UTC is 09:20 IST", () => {
    expect(isRosterFreeze(new Date("2026-11-03T03:50:00Z"))).toBe(true);
    expect(isRosterFreeze(new Date("2026-11-03T09:20:00Z"))).toBe(false);
  });
});

describe("msUntilRosterFreezeChange", () => {
  it("counts to the next edge: the start before it, the end during it, tomorrow's start after it", () => {
    expect(msUntilRosterFreezeChange(ist("09:00"))).toBe(15 * 60_000);
    expect(msUntilRosterFreezeChange(ist("09:20"))).toBe(10 * 60_000);
    expect(msUntilRosterFreezeChange(ist("09:30"))).toBe((24 * 60 - 15) * 60_000);
  });
});

describe("fmtIstMinute", () => {
  it("writes a minute of the day on a 12-hour clock", () => {
    expect(fmtIstMinute(9 * 60 + 15)).toBe("9:15 AM");
    expect(fmtIstMinute(9 * 60 + 30)).toBe("9:30 AM");
    expect(fmtIstMinute(0)).toBe("12:00 AM");
    expect(fmtIstMinute(13 * 60 + 5)).toBe("1:05 PM");
  });
});
