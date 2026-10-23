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
import { EVENING, REGULAR, TZ1_L1, assignment, shiftMap } from "../test/fixtures";
import { weekClear } from "./weekEdit";

const SHIFTS = shiftMap(REGULAR, EVENING, TZ1_L1);

describe("weekClear", () => {
  const evening = assignment({ name: "Jane Doe", rotaDate: "2026-09-21", shiftCode: EVENING.code });

  it("puts a weekday back on the window the person holds most", () => {
    expect(weekClear(evening, SHIFTS, { holdsTurnElsewhere: false, baseShiftCode: REGULAR.code })).toEqual({
      shiftCode: REGULAR.code,
      what: "off Evening 6-9pm",
    });
  });

  it("falls back to the team's own window, then to none", () => {
    expect(weekClear(evening, SHIFTS, { holdsTurnElsewhere: false, teamDefaultShiftCode: "CRE_AMERICAS" }).shiftCode).toBe(
      "CRE_AMERICAS",
    );
    expect(weekClear(evening, SHIFTS, { holdsTurnElsewhere: false }).shiftCode).toBe("");
  });

  it("leaves a weekend day empty, whatever the person's standing window", () => {
    const saturday = assignment({ name: "Jane Doe", rotaDate: "2026-09-26", shiftCode: EVENING.code });
    expect(weekClear(saturday, SHIFTS, { holdsTurnElsewhere: false, baseShiftCode: REGULAR.code }).shiftCode).toBe("");
  });

  it("clears only this zone's turn when the person holds another that day", () => {
    const l1 = assignment({ name: "Kasun", rotaDate: "2026-09-21", shiftCode: TZ1_L1.code, tier: "L1" });
    expect(weekClear(l1, SHIFTS, { holdsTurnElsewhere: true, baseShiftCode: REGULAR.code })).toEqual({
      shiftCode: "",
      zoneCode: "TZ1",
      what: "off TZ1 L1",
    });
    // Holding nothing else that day, the whole day goes back as usual.
    expect(weekClear(l1, SHIFTS, { holdsTurnElsewhere: false, baseShiftCode: REGULAR.code })).toEqual({
      shiftCode: REGULAR.code,
      what: "off TZ1 L1 support",
    });
  });
});
