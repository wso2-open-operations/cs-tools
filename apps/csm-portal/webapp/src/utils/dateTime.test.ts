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

import { afterAll, afterEach, beforeAll, describe, expect, it, vi } from "vitest";
import {
  backendUtcToZonedInput,
  clearUserPreferredTimeZone,
  setUserPreferredTimeZone,
  zonedInputToBackendUtc,
  formatDateOnlyForDisplay,
  formatRelativeDateOnly,
  isPastDateOnly,
  isPastDateTime,
  isPastZonedInput,
  parseDateOnly,
  formatUtcDateForDisplay,
  utcDateOnlyValue,
} from "./dateTime";

describe("isPastDateTime", () => {
  it("is true for an instant strictly before now", () => {
    expect(isPastDateTime(new Date(Date.now() - 60_000))).toBe(true);
  });

  it("is false for an instant in the future", () => {
    expect(isPastDateTime(new Date(Date.now() + 60_000))).toBe(false);
  });

  it("is false for null (an empty/unset field isn't 'in the past')", () => {
    expect(isPastDateTime(null)).toBe(false);
  });

  it("is false for an invalid Date", () => {
    expect(isPastDateTime(new Date("not-a-date"))).toBe(false);
  });
});

describe("isPastDateOnly", () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  it("is false for today's local-midnight date, regardless of current time", () => {
    // Freeze the clock so `today` (built here) and the "now" that
    // isPastDateOnly constructs internally can't disagree across an actual
    // local-midnight boundary crossed between this line and the call below.
    vi.useFakeTimers();
    vi.setSystemTime(new Date(2026, 0, 15, 12, 0, 0));
    const today = new Date();
    today.setHours(0, 0, 0, 0);
    expect(isPastDateOnly(today)).toBe(false);
  });

  it("is true for yesterday", () => {
    const yesterday = new Date();
    yesterday.setHours(0, 0, 0, 0);
    yesterday.setDate(yesterday.getDate() - 1);
    expect(isPastDateOnly(yesterday)).toBe(true);
  });

  it("is false for tomorrow", () => {
    const tomorrow = new Date();
    tomorrow.setHours(0, 0, 0, 0);
    tomorrow.setDate(tomorrow.getDate() + 1);
    expect(isPastDateOnly(tomorrow)).toBe(false);
  });

  it("is false for null/invalid", () => {
    expect(isPastDateOnly(null)).toBe(false);
    expect(isPastDateOnly(new Date("not-a-date"))).toBe(false);
  });
});

describe("formatRelativeDateOnly", () => {
  const now = new Date(2026, 0, 15, 12, 0, 0); // Jan 15, 2026, local noon

  it("labels today's date as 'Today'", () => {
    expect(formatRelativeDateOnly("2026-01-15", now)).toBe("Today");
  });

  it("labels yesterday as 'Yesterday'", () => {
    expect(formatRelativeDateOnly("2026-01-14", now)).toBe("Yesterday");
  });

  it("labels tomorrow as 'Tomorrow'", () => {
    expect(formatRelativeDateOnly("2026-01-16", now)).toBe("Tomorrow");
  });

  it("labels 2+ days ago/from now as 'Nd ago' / 'Nd from now'", () => {
    expect(formatRelativeDateOnly("2026-01-13", now)).toBe("2d ago");
    expect(formatRelativeDateOnly("2026-01-08", now)).toBe("7d ago");
    expect(formatRelativeDateOnly("2026-01-17", now)).toBe("2d from now");
  });

  it("returns 'Today' near local midnight regardless of the value/now split", () => {
    // This is the actual bug: the old (hour-diff, UTC-parsed) approach could
    // read a date-only value as being on the wrong calendar day -- or even
    // hours "in the future" -- once the viewer's timezone offset shifted
    // local midnight away from UTC midnight. This function never touches UTC
    // at all (parseDateOnly + local Date field comparisons only), so it can't
    // regress into that: "today", one minute after local midnight, is still
    // unambiguously "Today".
    const justAfterMidnight = new Date(2026, 0, 15, 0, 1, 0);
    expect(formatRelativeDateOnly("2026-01-15", justAfterMidnight)).toBe("Today");
  });

  it("returns '—' for null/undefined/empty", () => {
    expect(formatRelativeDateOnly(null, now)).toBe("—");
    expect(formatRelativeDateOnly(undefined, now)).toBe("—");
    expect(formatRelativeDateOnly("", now)).toBe("—");
  });

  it("returns '—' for an unparseable value", () => {
    expect(formatRelativeDateOnly("not-a-date", now)).toBe("—");
  });
});

describe("parseDateOnly", () => {
  it("parses a valid date to local midnight", () => {
    const date = parseDateOnly("2026-08-01");
    expect(date).not.toBeNull();
    expect(date?.getFullYear()).toBe(2026);
    expect(date?.getMonth()).toBe(7); // 0-indexed: August
    expect(date?.getDate()).toBe(1);
    expect(date?.getHours()).toBe(0);
  });

  it("accepts Feb 29 on a leap year", () => {
    expect(parseDateOnly("2024-02-29")).not.toBeNull();
  });

  // The Date constructor silently normalizes an out-of-range day/month
  // instead of failing (new Date(2026, 1, 31) rolls forward to Mar 3, 2026)
  // -- these must be rejected (null), not silently returned as a different,
  // valid-looking date.
  it("rejects Feb 31 (rolls into March) rather than normalizing it", () => {
    expect(parseDateOnly("2026-02-31")).toBeNull();
  });

  it("rejects Feb 29 on a non-leap year", () => {
    expect(parseDateOnly("2026-02-29")).toBeNull();
  });

  it("rejects a day/month of 00", () => {
    expect(parseDateOnly("2026-01-00")).toBeNull();
    expect(parseDateOnly("2026-00-10")).toBeNull();
  });

  it("rejects a month of 13", () => {
    expect(parseDateOnly("2026-13-01")).toBeNull();
  });

  it("returns null for a malformed string", () => {
    expect(parseDateOnly("not-a-date")).toBeNull();
    expect(parseDateOnly("2026/08/01")).toBeNull();
  });
});

describe("parseDateOnly's invalid-date rejection, through its public callers", () => {
  it("formatDateOnlyForDisplay falls back to null instead of showing the rolled-over date", () => {
    expect(formatDateOnlyForDisplay("2026-02-31")).toBeNull();
  });

  it("formatRelativeDateOnly falls back to '—' instead of a relative label for the wrong day", () => {
    expect(formatRelativeDateOnly("2026-02-31")).toBe("—");
  });
});

describe("zonedInputToBackendUtc / backendUtcToZonedInput", () => {
  afterEach(() => {
    clearUserPreferredTimeZone();
  });

  it("converts a wall-clock value east of UTC to the earlier UTC instant", () => {
    // Asia/Colombo is UTC+05:30 all year.
    expect(zonedInputToBackendUtc("2026-03-01T15:30", "Asia/Colombo")).toBe("2026-03-01 10:00:00");
  });

  it("converts a wall-clock value west of UTC to the later UTC instant, across midnight", () => {
    // America/New_York is UTC-05:00 in early March (before DST starts on the 8th).
    expect(zonedInputToBackendUtc("2026-03-01T22:00", "America/New_York")).toBe("2026-03-02 03:00:00");
  });

  it("applies the offset in force on that date (daylight saving)", () => {
    // Same zone in July is UTC-04:00.
    expect(zonedInputToBackendUtc("2026-07-01T10:00", "America/New_York")).toBe("2026-07-01 14:00:00");
  });

  it("is the identity in UTC", () => {
    expect(zonedInputToBackendUtc("2026-03-01T10:00", "UTC")).toBe("2026-03-01 10:00:00");
  });

  it("uses the user's preferred time zone when none is passed", () => {
    setUserPreferredTimeZone("Asia/Colombo");
    expect(zonedInputToBackendUtc("2026-03-01T15:30")).toBe("2026-03-01 10:00:00");
  });

  it("returns null for an unparseable value", () => {
    expect(zonedInputToBackendUtc("not a date", "UTC")).toBeNull();
    expect(zonedInputToBackendUtc("", "UTC")).toBeNull();
  });

  it("reads a backend UTC timestamp back as the wall-clock value in the user's zone", () => {
    expect(backendUtcToZonedInput("2026-03-01 10:00:00", "Asia/Colombo")).toBe("2026-03-01T15:30");
    expect(backendUtcToZonedInput("2026-03-01T10:00:00Z", "Asia/Colombo")).toBe("2026-03-01T15:30");
  });

  it("returns an empty string for an empty or unparseable backend value", () => {
    expect(backendUtcToZonedInput(null, "UTC")).toBe("");
    expect(backendUtcToZonedInput(undefined, "UTC")).toBe("");
    expect(backendUtcToZonedInput("garbage", "UTC")).toBe("");
  });

  it.each(["Asia/Colombo", "America/New_York", "Pacific/Auckland", "UTC"])(
    "round-trips a backend value through the picker and back in %s",
    (zone) => {
      const stored = "2026-10-25 01:30:00";
      const wallClock = backendUtcToZonedInput(stored, zone);
      expect(zonedInputToBackendUtc(wallClock, zone)).toBe(stored);
    },
  );
});

describe("isPastZonedInput", () => {
  // The whole point of this helper is that it must NOT read the picker digits
  // in the browser zone, so pin the browser zone to UTC (independent of the
  // machine running the tests) and vary the profile zone against it.

  beforeAll(() => {
    vi.stubEnv("TZ", "UTC");
  });

  afterAll(() => {
    vi.unstubAllEnvs();
  });

  afterEach(() => {
    vi.useRealTimers();
    clearUserPreferredTimeZone();
  });

  function freezeNow(): void {
    vi.useFakeTimers({ toFake: ["Date"] });
    vi.setSystemTime(new Date("2030-03-01T12:00:00Z"));
  }

  it("runs with the browser zone pinned to UTC", () => {
    expect(new Date(2030, 2, 1, 12, 0).toISOString()).toBe("2030-03-01T12:00:00.000Z");
  });

  it("is past in the profile zone even though the same digits are in the future in the browser zone", () => {
    freezeNow();
    setUserPreferredTimeZone("Asia/Colombo");
    // 15:30 Colombo (UTC+05:30) is 10:00Z, before now (12:00Z). Read as browser
    // (UTC) digits it would be 15:30Z, i.e. still in the future.
    expect(isPastZonedInput("2030-03-01T15:30")).toBe(true);
    expect(isPastDateTime(new Date(2030, 2, 1, 15, 30))).toBe(false);
  });

  it("is future in the profile zone even though the same digits are in the past in the browser zone", () => {
    freezeNow();
    setUserPreferredTimeZone("America/Los_Angeles");
    // 08:00 Los Angeles (UTC-08:00 on this date) is 16:00Z, after now. Read as
    // browser (UTC) digits it would be 08:00Z, i.e. already past.
    expect(isPastZonedInput("2030-03-01T08:00")).toBe(false);
    expect(isPastDateTime(new Date(2030, 2, 1, 8, 0))).toBe(true);
  });

  it("honours an explicit time zone over the profile zone", () => {
    freezeNow();
    setUserPreferredTimeZone("America/Los_Angeles");
    expect(isPastZonedInput("2030-03-01T15:30", "Asia/Colombo")).toBe(true);
  });

  it("never flags an empty or unparseable value", () => {
    freezeNow();
    expect(isPastZonedInput("")).toBe(false);
    expect(isPastZonedInput("not a date")).toBe(false);
  });
});

describe("utcDateOnlyValue / formatUtcDateForDisplay", () => {
  // A calendar-date field is stored as <day> 00:00 UTC. These must name that
  // day in any viewer timezone: local getters give the previous day west of
  // UTC, which is the off-by-one the auto-closure hold hit.
  it("names the UTC calendar day of a midnight-UTC value", () => {
    expect(utcDateOnlyValue("2026-10-22T00:00:00.000Z")).toBe("2026-10-22");
    expect(utcDateOnlyValue("2026-01-01T00:00:00Z")).toBe("2026-01-01");
    expect(formatUtcDateForDisplay("2026-10-22T00:00:00.000Z")).toBe("Oct 22, 2026");
  });

  it("reads an unzoned backend value as UTC, like every other backend timestamp", () => {
    expect(utcDateOnlyValue("2026-10-22 00:00:00")).toBe("2026-10-22");
  });

  it("keeps the UTC day for an instant late in that day", () => {
    expect(utcDateOnlyValue("2026-10-22T23:59:59.000Z")).toBe("2026-10-22");
    expect(formatUtcDateForDisplay("2026-10-22T23:59:59.000Z")).toBe("Oct 22, 2026");
  });

  it("returns null for empty or unparseable input", () => {
    expect(utcDateOnlyValue(undefined)).toBeNull();
    expect(utcDateOnlyValue("")).toBeNull();
    expect(utcDateOnlyValue("not a date")).toBeNull();
    expect(formatUtcDateForDisplay(null)).toBeNull();
    expect(formatUtcDateForDisplay("not a date")).toBeNull();
  });
});
