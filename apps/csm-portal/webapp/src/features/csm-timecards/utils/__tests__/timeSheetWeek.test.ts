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
  weekEndOf,
  weekLabel,
  weekStartOf,
} from "@features/csm-timecards/utils/timeSheetWeek";

const utcDay = (iso: string): number =>
  new Date(`${iso}T00:00:00Z`).getUTCDay();

describe("timeSheetWeek", () => {
  it("weekStartOf returns the Monday of the week", () => {
    // 2026-06-27 is a Saturday.
    expect(utcDay(weekStartOf("2026-06-27"))).toBe(1); // Monday
  });

  it("is idempotent on a week start", () => {
    const ws = weekStartOf("2026-06-27");
    expect(weekStartOf(ws)).toBe(ws);
  });

  it("weekEndOf is the following Sunday, 6 days later", () => {
    const ws = weekStartOf("2026-06-27");
    const we = weekEndOf(ws);
    expect(utcDay(we)).toBe(0); // Sunday
    expect(weekStartOf(we)).toBe(ws); // the end maps back to the same start
  });

  it("weekLabel renders a readable range", () => {
    const ws = weekStartOf("2026-06-27");
    expect(weekLabel(ws)).toMatch(/2026/);
  });
});
