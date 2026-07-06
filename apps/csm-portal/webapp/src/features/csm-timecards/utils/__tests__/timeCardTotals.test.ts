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
  emptyBreakdown,
  hasLoggedTime,
  timeCardDraftErrors,
  totalMinutes,
} from "@features/csm-timecards/utils/timeCardTotals";
import { WORK_LOG_MAX } from "@features/csm-timecards/constants/timeCardConstants";

describe("timeCardTotals", () => {
  it("emptyBreakdown is all zeros and sums to 0", () => {
    expect(totalMinutes(emptyBreakdown())).toBe(0);
    expect(hasLoggedTime(emptyBreakdown())).toBe(false);
  });

  it("sums activity buckets", () => {
    expect(
      totalMinutes({
        analysisDebugging: 15,
        reproduce: 90,
        settingUp: 20,
        providingSolution: 30,
        answering: 0,
      }),
    ).toBe(155);
  });

  it("detects logged time", () => {
    expect(hasLoggedTime({ ...emptyBreakdown(), answering: 15 })).toBe(true);
  });

  describe("timeCardDraftErrors", () => {
    const valid = {
      date: "2026-06-27",
      breakdown: { ...emptyBreakdown(), analysisDebugging: 60 },
      workLogComment: "Investigated root cause.",
      approverId: "lead-1",
    };

    it("passes a complete draft", () => {
      expect(timeCardDraftErrors(valid)).toEqual({});
    });

    it("flags every missing required field", () => {
      const errors = timeCardDraftErrors({
        date: "",
        breakdown: emptyBreakdown(),
        workLogComment: "   ",
        approverId: undefined,
      });
      expect(errors.date).toBeDefined();
      expect(errors.minutes).toBeDefined();
      expect(errors.workLogComment).toBeDefined();
      expect(errors.approver).toBeDefined();
    });

    it("flags a work log comment over WORK_LOG_MAX", () => {
      const errors = timeCardDraftErrors({
        ...valid,
        workLogComment: "a".repeat(WORK_LOG_MAX + 1),
      });
      expect(errors.workLogComment).toBeDefined();
    });
  });
});
