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

import { describe, expect, it } from "vitest";
import {
  environmentAliasText,
  mutedAlertsSummaryText,
  pickerInstant,
  SKIPPED_WALL_TIME,
  validateAlertMuteRule,
} from "@features/csm-operations/utils/alertMuteRules";

describe("mutedAlertsSummaryText", () => {
  it("says when nothing was muted", () => {
    expect(mutedAlertsSummaryText({ total: 0, summary: {} })).toBe("No alerts muted");
  });

  it("lists only the outcomes that happened", () => {
    expect(mutedAlertsSummaryText({ total: 12, summary: { muted: 1, incident: 2, closed_quietly: 9 } })).toBe(
      "12 muted alert groups · 1 still muted · 2 became incidents · 9 closed quietly",
    );
    expect(mutedAlertsSummaryText({ total: 1, summary: { incident: 1 } })).toBe("1 muted alert group · 1 became incident");
  });
});

describe("pickerInstant", () => {
  it("refuses a wall time daylight saving skips", () => {
    // 2027-03-14 02:30 doesn't exist in New York: clocks jump from 02:00 to 03:00.
    // The time picker's value is built on an ordinary day: pickerInstant reads only its hours and minutes, and a
    // test host in New York would turn a 02:30 on March 14 into 03:30 before pickerInstant saw it.
    const day = new Date(2027, 2, 14);
    expect(pickerInstant(day, new Date(2027, 0, 1, 2, 30), "America/New_York")).toBe(SKIPPED_WALL_TIME);
    expect(pickerInstant(day, new Date(2027, 0, 1, 3, 30), "America/New_York")).toBe("2027-03-14T07:30:00.000Z");
  });

  it("names the skipped time when validating", () => {
    const errors = validateAlertMuteRule(
      {
        name: "x",
        description: "",
        timeZone: "America/New_York",
        startOn: SKIPPED_WALL_TIME,
        endOn: "2027-03-14T09:00:00.000Z",
        conditions: [
          { field: "service", value: "a" },
          { field: "environment", value: "prod" },
        ],
      },
      Date.parse("2027-03-01T00:00:00Z"),
      true,
    );
    expect(errors.startOn).toMatch(/doesn't exist in this time zone/);
  });
});

describe("environmentAliasText", () => {
  it("names the other spellings an environment matches", () => {
    expect(environmentAliasText("qa")).toBe("Also matches alerts whose environment is test and testing.");
    expect(environmentAliasText("production")).toBe("Also matches alerts whose environment is prod, prd and live.");
    expect(environmentAliasText("uat")).toBe("");
  });
});
