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
  rangeAfterFromChange,
  rangeAfterToChange,
} from "@features/csm-timecards/utils/timeCardDateRange";

const today = new Date(2026, 9, 2);

describe("timeCardDateRange", () => {
  it("clearing To leaves From untouched", () => {
    expect(
      rangeAfterToChange("", { from: "2026-09-01", to: "2026-09-10" }, today),
    ).toEqual({ from: "2026-09-01", to: "" });
  });

  it("a partial To value does not reset From", () => {
    expect(
      rangeAfterToChange("2026-09", { from: "2026-09-01", to: "" }, today),
    ).toEqual({ from: "2026-09-01", to: "" });
  });

  it("clearing From leaves To untouched", () => {
    expect(
      rangeAfterFromChange("", { from: "2026-09-01", to: "2026-09-10" }, today),
    ).toEqual({ from: "", to: "2026-09-10" });
  });

  it("moves To when From lands after it", () => {
    expect(
      rangeAfterFromChange("2026-09-20", { from: "", to: "2026-09-10" }, today),
    ).toEqual({ from: "2026-09-20", to: "2026-09-20" });
  });

  it("moves From when To lands before it", () => {
    expect(
      rangeAfterToChange("2026-08-01", { from: "2026-09-01", to: "" }, today),
    ).toEqual({ from: "2026-08-01", to: "2026-08-01" });
  });

  it("rejects future dates", () => {
    expect(rangeAfterToChange("2026-10-03", { from: "", to: "" }, today)).toBeNull();
    expect(rangeAfterFromChange("2026-10-03", { from: "", to: "" }, today)).toBeNull();
  });
});
