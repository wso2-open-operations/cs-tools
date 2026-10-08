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
import { datetimeLocalWallTimeToUtcMs } from "@features/support/utils/support";
import {
  PROPOSED_WINDOW_MESSAGES,
  buildProposedWindowPayload,
  datetimeLocalToUtcMs,
  formatPlannedLength,
  formatUtcMsAsApiDatetime,
  getChangeRequestWindow,
  getProposalCopy,
  hasProposedStartErrors,
  shiftEndKeepingDuration,
  validateProposedStart,
} from "@features/operations/utils/changeRequestSchedule";

const HOUR = 60 * 60 * 1000;

describe("datetimeLocalToUtcMs", () => {
  it("converts civil times in zones with different offsets", () => {
    const tenUtc = Date.UTC(2026, 5, 1, 10, 0);
    expect(datetimeLocalToUtcMs("2026-06-01T10:00", "UTC")).toBe(tenUtc);
    expect(datetimeLocalToUtcMs("2026-06-01T15:30", "Asia/Colombo")).toBe(tenUtc);
    expect(datetimeLocalToUtcMs("2026-06-01T03:00", "America/Los_Angeles")).toBe(tenUtc);
    expect(datetimeLocalToUtcMs("2026-01-15T02:00", "America/Los_Angeles")).toBe(
      Date.UTC(2026, 0, 15, 10, 0),
    );
  });

  it("accepts the API's zone aliases", () => {
    expect(datetimeLocalToUtcMs("2026-06-01T15:30", "WSO2/Colombo")).toBe(
      Date.UTC(2026, 5, 1, 10, 0),
    );
  });

  it("picks the earlier instant for a time that happens twice (clocks go back)", () => {
    // 2026-11-01 01:30 happens in EDT (UTC-4) and again in EST (UTC-5).
    expect(datetimeLocalToUtcMs("2026-11-01T01:30", "America/New_York")).toBe(
      Date.UTC(2026, 10, 1, 5, 30),
    );
  });

  it("returns null for a time that never happens (clocks go forward)", () => {
    expect(datetimeLocalToUtcMs("2026-03-08T02:30", "America/New_York")).toBeNull();
  });

  it("returns null for empty, malformed and impossible values", () => {
    expect(datetimeLocalToUtcMs("", "UTC")).toBeNull();
    expect(datetimeLocalToUtcMs(null, "UTC")).toBeNull();
    expect(datetimeLocalToUtcMs("tomorrow", "UTC")).toBeNull();
    expect(datetimeLocalToUtcMs("2026-02-31T10:00", "UTC")).toBeNull();
    expect(datetimeLocalToUtcMs("2026-06-01T25:00", "UTC")).toBeNull();
  });

  // The shared helper scans minute by minute (~150 ms a call); this one must
  // agree with it everywhere, including the edges.
  it("agrees with the shared scan", { timeout: 30_000 }, () => {
    const cases: Array<[string, string]> = [
      ["2026-06-01T15:30", "Asia/Colombo"],
      ["2026-07-04T09:15", "America/Los_Angeles"],
      ["2026-11-01T01:30", "America/New_York"],
      ["2026-03-08T03:30", "America/New_York"],
      ["2026-03-29T01:30", "Europe/London"],
      ["2026-04-05T02:30", "Australia/Sydney"],
    ];
    for (const [local, zone] of cases) {
      expect(datetimeLocalToUtcMs(local, zone), `${local} ${zone}`).toBe(
        datetimeLocalWallTimeToUtcMs(local, zone),
      );
    }
  });
});

describe("formatUtcMsAsApiDatetime", () => {
  it("formats UTC as YYYY-MM-DD HH:MM:SS with zero padding", () => {
    expect(formatUtcMsAsApiDatetime(Date.UTC(2026, 0, 5, 3, 4, 5))).toBe(
      "2026-01-05 03:04:05",
    );
    expect(formatUtcMsAsApiDatetime(Date.UTC(2026, 11, 31, 23, 59, 0))).toBe(
      "2026-12-31 23:59:00",
    );
  });
});

describe("getChangeRequestWindow", () => {
  it("reads both ends and their duration", () => {
    expect(
      getChangeRequestWindow({
        startDate: "2026-06-01 10:00:00",
        endDate: "2026-06-01 12:30:00",
      }),
    ).toEqual({
      startMs: Date.UTC(2026, 5, 1, 10, 0),
      endMs: Date.UTC(2026, 5, 1, 12, 30),
      durationMs: 2.5 * HOUR,
    });
  });

  it("has no duration when an end is missing or the end is not after the start", () => {
    expect(
      getChangeRequestWindow({ startDate: "2026-06-01 10:00:00", endDate: "" }).durationMs,
    ).toBeNull();
    expect(
      getChangeRequestWindow({ startDate: "", endDate: "" }),
    ).toEqual({ startMs: null, endMs: null, durationMs: null });
    expect(
      getChangeRequestWindow({
        startDate: "2026-06-01 12:00:00",
        endDate: "2026-06-01 10:00:00",
      }).durationMs,
    ).toBeNull();
  });
});

describe("shiftEndKeepingDuration", () => {
  it("moves the end so the window keeps its length", () => {
    expect(shiftEndKeepingDuration("2026-06-01T09:00", 2 * HOUR, "Asia/Colombo")).toBe(
      "2026-06-01T11:00",
    );
    expect(shiftEndKeepingDuration("2026-06-01T23:00", 3 * HOUR, "UTC")).toBe(
      "2026-06-02T02:00",
    );
  });

  it("keeps the exact length across a clock change", () => {
    // 23:00 EST + 4 h of real time is 04:00 EDT: the clocks skipped 02:00-03:00.
    expect(shiftEndKeepingDuration("2026-03-07T23:00", 4 * HOUR, "America/New_York")).toBe(
      "2026-03-08T04:00",
    );
  });

  it("returns an empty string when it cannot work the end out", () => {
    expect(shiftEndKeepingDuration("2026-06-01T09:00", null, "UTC")).toBe("");
    expect(shiftEndKeepingDuration("", 2 * HOUR, "UTC")).toBe("");
    expect(shiftEndKeepingDuration("garbage", 2 * HOUR, "UTC")).toBe("");
  });
});

describe("validateProposedStart", () => {
  const nowMs = Date.UTC(2026, 4, 1, 0, 0);
  const base = {
    start: "2026-06-01T10:00",
    currentStart: "2026-05-20T10:00",
    durationMs: 2 * HOUR,
    timeZone: "UTC",
    nowMs,
  };

  it("accepts a future start that differs from the current one", () => {
    const errors = validateProposedStart(base);
    expect(errors).toEqual({});
    expect(hasProposedStartErrors(errors)).toBe(false);
  });

  it("requires the start", () => {
    expect(validateProposedStart({ ...base, start: "" })).toEqual({
      start: PROPOSED_WINDOW_MESSAGES.startRequired,
    });
  });

  it("requires a real date-time", () => {
    expect(validateProposedStart({ ...base, start: "soon" }).start).toBe(
      PROPOSED_WINDOW_MESSAGES.startInvalid,
    );
    // A wall time that never happens (the clocks skip it) is not a time at all.
    expect(
      validateProposedStart({
        ...base,
        start: "2026-03-08T02:30",
        timeZone: "America/New_York",
        nowMs: Date.UTC(2026, 0, 1),
      }).start,
    ).toBe(PROPOSED_WINDOW_MESSAGES.startInvalid);
  });

  it("requires the start to be in the future", () => {
    expect(validateProposedStart({ ...base, start: "2026-04-30T10:00" }).start).toBe(
      PROPOSED_WINDOW_MESSAGES.startPast,
    );
    expect(validateProposedStart({ ...base, start: "2026-05-01T00:00" }).start).toBe(
      PROPOSED_WINDOW_MESSAGES.startPast,
    );
  });

  it("judges the future in the viewer's zone, not as a bare string", () => {
    // 01:00 on 2 June in Colombo is 19:30 UTC on 1 June, so with the clock at
    // 20:00 UTC it is already past -- though read as UTC it would still be ahead.
    const late = { ...base, nowMs: Date.UTC(2026, 5, 1, 20, 0), start: "2026-06-02T01:00" };
    expect(validateProposedStart({ ...late, timeZone: "Asia/Colombo" }).start).toBe(
      PROPOSED_WINDOW_MESSAGES.startPast,
    );
    expect(validateProposedStart({ ...late, timeZone: "UTC" })).toEqual({});
  });

  it("refuses the planned start that is already there", () => {
    const errors = validateProposedStart({
      ...base,
      start: "2026-06-01T10:00",
      currentStart: "2026-06-01T10:00",
    });
    expect(errors).toEqual({ window: PROPOSED_WINDOW_MESSAGES.unchanged });
    expect(hasProposedStartErrors(errors)).toBe(true);
  });

  it("refuses the time that already waits for WSO2, whoever proposed it", () => {
    expect(
      validateProposedStart({ ...base, standingStart: "2026-06-01T10:00" }),
    ).toEqual({ window: PROPOSED_WINDOW_MESSAGES.alreadyProposed });
    // Another start is a new proposal that replaces it.
    expect(
      validateProposedStart({ ...base, standingStart: "2026-06-03T10:00" }),
    ).toEqual({});
  });

  it("refuses a change request that has no window to move, whatever is typed", () => {
    expect(validateProposedStart({ ...base, durationMs: null })).toEqual({
      window: PROPOSED_WINDOW_MESSAGES.noWindow,
    });
    expect(validateProposedStart({ ...base, durationMs: null, start: "" })).toEqual({
      window: PROPOSED_WINDOW_MESSAGES.noWindow,
    });
  });

  it("takes any future start when the change request has a window but no current start to compare", () => {
    expect(validateProposedStart({ ...base, currentStart: "" })).toEqual({});
  });
});

describe("buildProposedWindowPayload", () => {
  it("sends the start and the end that keeps the planned length, in UTC, for a viewer outside UTC", () => {
    expect(
      buildProposedWindowPayload("2026-06-01T15:30", 2 * HOUR, "Asia/Colombo"),
    ).toEqual({
      plannedStartOn: "2026-06-01 10:00:00",
      plannedEndOn: "2026-06-01 12:00:00",
    });
    expect(
      buildProposedWindowPayload("2026-06-01T20:00", 5 * HOUR, "America/Los_Angeles"),
    ).toEqual({
      plannedStartOn: "2026-06-02 03:00:00",
      plannedEndOn: "2026-06-02 08:00:00",
    });
  });

  it("derives the end on the timeline, as the server does, across a clock change", () => {
    // 23:00 EST on 7 March + 4 h of real time is 09:00 UTC: 04:00 EDT the next day.
    expect(
      buildProposedWindowPayload("2026-03-07T23:00", 4 * HOUR, "America/New_York"),
    ).toEqual({
      plannedStartOn: "2026-03-08 04:00:00",
      plannedEndOn: "2026-03-08 08:00:00",
    });
  });

  it("keeps a length that is not a whole number of minutes", () => {
    expect(
      buildProposedWindowPayload("2026-06-01T10:00", 90 * 60 * 1000 + 30 * 1000, "UTC"),
    ).toEqual({
      plannedStartOn: "2026-06-01 10:00:00",
      plannedEndOn: "2026-06-01 11:30:30",
    });
  });

  it("returns null when the start is not a real time in the zone, or there is no length to keep", () => {
    expect(buildProposedWindowPayload("", 2 * HOUR, "UTC")).toBeNull();
    expect(buildProposedWindowPayload("nope", 2 * HOUR, "UTC")).toBeNull();
    expect(buildProposedWindowPayload("2026-06-01T10:00", null, "UTC")).toBeNull();
    expect(buildProposedWindowPayload("2026-06-01T10:00", 0, "UTC")).toBeNull();
  });
});

describe("formatPlannedLength", () => {
  it("says a length in whole hours and minutes", () => {
    expect(formatPlannedLength(2 * HOUR)).toBe("2 hours");
    expect(formatPlannedLength(HOUR)).toBe("1 hour");
    expect(formatPlannedLength(90 * 60 * 1000)).toBe("1 hour 30 minutes");
    expect(formatPlannedLength(45 * 60 * 1000)).toBe("45 minutes");
    expect(formatPlannedLength(60 * 1000)).toBe("1 minute");
    expect(formatPlannedLength(26 * HOUR)).toBe("26 hours");
  });

  it("rounds to the minute and never says nothing", () => {
    expect(formatPlannedLength(90 * 60 * 1000 + 20 * 1000)).toBe("1 hour 30 minutes");
    expect(formatPlannedLength(10 * 1000)).toBe("less than a minute");
  });
});

describe("getProposalCopy", () => {
  it("says a proposal is not an approval, that WSO2 accepts it or suggests another time, and that the length stays", () => {
    const copy = getProposalCopy(2 * HOUR);
    expect(copy.notice).toMatch(/proposing a new start time, not approving one/);
    expect(copy.notice).toMatch(/WSO2 will either accept it or suggest a different time/);
    expect(copy.notice).toMatch(/planned length of 2 hours stays the same/);
    expect(copy.notice).toMatch(/To ask for a different length, contact WSO2/);
  });

  it("promises no internal review, no CAB round trip and no second approval", () => {
    for (const durationMs of [2 * HOUR, 90 * 60 * 1000, null]) {
      const { notice, success } = getProposalCopy(durationMs);
      for (const text of [notice, success]) {
        expect(text).not.toMatch(/internal/i);
        expect(text).not.toMatch(/review it/i);
        expect(text).not.toMatch(/approval again/i);
        expect(text).not.toMatch(/CAB/);
      }
    }
  });

  it("does not depend on the change type, and leaves the length out when there is none", () => {
    expect(getProposalCopy(null).notice).not.toMatch(/planned length of/);
    expect(getProposalCopy(null).notice).toMatch(/new start time/);
  });

  it("promises only what the page does: the answer appears on the page", () => {
    expect(getProposalCopy(HOUR).success).toBe(
      "New time proposed. WSO2 will accept it or suggest a different time, and the answer will appear on this page.",
    );
  });
});
