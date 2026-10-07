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
  formatUtcMsAsApiDatetime,
  getChangeRequestWindow,
  getProposalCopy,
  hasProposedWindowErrors,
  shiftEndKeepingDuration,
  validateProposedWindow,
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

describe("validateProposedWindow", () => {
  const nowMs = Date.UTC(2026, 4, 1, 0, 0);
  const base = {
    start: "2026-06-01T10:00",
    end: "2026-06-01T12:00",
    currentStart: "2026-05-20T10:00",
    currentEnd: "2026-05-20T12:00",
    timeZone: "UTC",
    nowMs,
  };

  it("accepts a future window that differs from the current one", () => {
    const errors = validateProposedWindow(base);
    expect(errors).toEqual({});
    expect(hasProposedWindowErrors(errors)).toBe(false);
  });

  it("accepts a change of the start alone or of the end alone", () => {
    expect(
      validateProposedWindow({
        ...base,
        currentStart: "2026-06-01T10:00",
        currentEnd: "2026-06-01T11:00",
      }),
    ).toEqual({});
    expect(
      validateProposedWindow({
        ...base,
        currentStart: "2026-06-01T09:00",
        currentEnd: "2026-06-01T12:00",
      }),
    ).toEqual({});
  });

  it("requires both fields", () => {
    expect(validateProposedWindow({ ...base, start: "", end: "" })).toEqual({
      start: PROPOSED_WINDOW_MESSAGES.startRequired,
      end: PROPOSED_WINDOW_MESSAGES.endRequired,
    });
    expect(validateProposedWindow({ ...base, end: "" }).end).toBe(
      PROPOSED_WINDOW_MESSAGES.endRequired,
    );
  });

  it("requires real date-times", () => {
    expect(validateProposedWindow({ ...base, start: "soon" }).start).toBe(
      PROPOSED_WINDOW_MESSAGES.startInvalid,
    );
    expect(validateProposedWindow({ ...base, end: "later" }).end).toBe(
      PROPOSED_WINDOW_MESSAGES.endInvalid,
    );
  });

  it("requires the start to be in the future", () => {
    expect(
      validateProposedWindow({ ...base, start: "2026-04-30T10:00", end: "2026-04-30T12:00" })
        .start,
    ).toBe(PROPOSED_WINDOW_MESSAGES.startPast);
    expect(
      validateProposedWindow({ ...base, start: "2026-05-01T00:00", end: "2026-05-01T01:00" })
        .start,
    ).toBe(PROPOSED_WINDOW_MESSAGES.startPast);
  });

  it("requires the end to be after the start", () => {
    expect(validateProposedWindow({ ...base, end: "2026-06-01T10:00" }).end).toBe(
      PROPOSED_WINDOW_MESSAGES.endNotAfterStart,
    );
    expect(validateProposedWindow({ ...base, end: "2026-06-01T09:00" }).end).toBe(
      PROPOSED_WINDOW_MESSAGES.endNotAfterStart,
    );
  });

  it("judges the future in the viewer's zone, not as a bare string", () => {
    // 01:00 on 2 June in Colombo is 19:30 UTC on 1 June, so with the clock at
    // 20:00 UTC it is already past -- though read as UTC it would still be ahead.
    const late = { ...base, nowMs: Date.UTC(2026, 5, 1, 20, 0), start: "2026-06-02T01:00", end: "2026-06-02T03:00" };
    expect(validateProposedWindow({ ...late, timeZone: "Asia/Colombo" }).start).toBe(
      PROPOSED_WINDOW_MESSAGES.startPast,
    );
    expect(validateProposedWindow({ ...late, timeZone: "UTC" })).toEqual({});
  });

  it("refuses a window equal to the current one", () => {
    const errors = validateProposedWindow({
      ...base,
      start: "2026-06-01T10:00",
      end: "2026-06-01T12:00",
      currentStart: "2026-06-01T10:00",
      currentEnd: "2026-06-01T12:00",
    });
    expect(errors).toEqual({ window: PROPOSED_WINDOW_MESSAGES.unchanged });
    expect(hasProposedWindowErrors(errors)).toBe(true);
  });

  it("takes any valid window when the change request has none yet", () => {
    expect(
      validateProposedWindow({ ...base, currentStart: "", currentEnd: "" }),
    ).toEqual({});
  });
});

describe("buildProposedWindowPayload", () => {
  it("sends both ends in UTC for a viewer outside UTC", () => {
    expect(
      buildProposedWindowPayload("2026-06-01T15:30", "2026-06-01T17:30", "Asia/Colombo"),
    ).toEqual({
      plannedStartOn: "2026-06-01 10:00:00",
      plannedEndOn: "2026-06-01 12:00:00",
    });
    expect(
      buildProposedWindowPayload("2026-06-01T20:00", "2026-06-02T01:00", "America/Los_Angeles"),
    ).toEqual({
      plannedStartOn: "2026-06-02 03:00:00",
      plannedEndOn: "2026-06-02 08:00:00",
    });
  });

  it("returns null when an end is not a real time in the zone", () => {
    expect(buildProposedWindowPayload("", "2026-06-01T17:30", "UTC")).toBeNull();
    expect(buildProposedWindowPayload("2026-06-01T10:00", "nope", "UTC")).toBeNull();
  });
});

describe("getProposalCopy", () => {
  it("tells a Normal or Emergency change's customer WSO2 reviews first", () => {
    for (const label of ["Normal", "Emergency", undefined]) {
      const copy = getProposalCopy({ type: label ? { id: "x", label } : null });
      expect(copy.notice).toMatch(/WSO2 will review it internally first/);
      expect(copy.notice).toMatch(/proposing a new time/);
      expect(copy.success).toBe(
        "New time proposed. We'll ask for your approval again once it's confirmed internally.",
      );
    }
  });

  it("tells a Standard change's customer they will be asked straight away", () => {
    const copy = getProposalCopy({ type: { id: "standard", label: "Standard" } });
    expect(copy.notice).not.toMatch(/internally/);
    expect(copy.notice).toMatch(/asked to approve/);
    expect(copy.success).toMatch(/approve it when you are ready/);
  });
});
