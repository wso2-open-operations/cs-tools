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
import { formatAbsoluteTimestamp, formatRelativeTime } from "./time";

const NOW = Date.parse("2026-01-01T12:00:00Z");

function isoBefore(ms: number): string {
  return new Date(NOW - ms).toISOString();
}

function isoAfter(ms: number): string {
  return new Date(NOW + ms).toISOString();
}

describe("formatRelativeTime", () => {
  it('renders "just now" for 59 seconds ago', () => {
    expect(formatRelativeTime(isoBefore(59_000), NOW)).toBe("just now");
  });

  it('renders "1m ago" at exactly 1 minute', () => {
    expect(formatRelativeTime(isoBefore(60_000), NOW)).toBe("1m ago");
  });

  it('floors minutes: 30 minutes renders "30m ago", never rounding up to an hour', () => {
    expect(formatRelativeTime(isoBefore(30 * 60_000), NOW)).toBe("30m ago");
  });

  it('renders "59m ago" just under an hour', () => {
    expect(formatRelativeTime(isoBefore(59 * 60_000), NOW)).toBe("59m ago");
  });

  it('renders "1h ago" at exactly 1 hour', () => {
    expect(formatRelativeTime(isoBefore(60 * 60_000), NOW)).toBe("1h ago");
  });

  it('renders "23h ago" at 23h59m, never rolling over to a day', () => {
    expect(formatRelativeTime(isoBefore(23 * 60 * 60_000 + 59 * 60_000), NOW)).toBe("23h ago");
  });

  it('renders "1d ago" at exactly 1 day, with no larger unit than days', () => {
    expect(formatRelativeTime(isoBefore(24 * 60 * 60_000), NOW)).toBe("1d ago");
    expect(formatRelativeTime(isoBefore(400 * 24 * 60 * 60_000), NOW)).toBe("400d ago");
  });

  it('renders a future timestamp as "Xm/Xh/Xd from now"', () => {
    expect(formatRelativeTime(isoAfter(5 * 60_000), NOW)).toBe("5m from now");
    expect(formatRelativeTime(isoAfter(3 * 60 * 60_000), NOW)).toBe("3h from now");
    expect(formatRelativeTime(isoAfter(2 * 24 * 60 * 60_000), NOW)).toBe("2d from now");
  });

  it('renders "—" for null, undefined, empty, or unparseable input', () => {
    expect(formatRelativeTime(null, NOW)).toBe("—");
    expect(formatRelativeTime(undefined, NOW)).toBe("—");
    expect(formatRelativeTime("", NOW)).toBe("—");
    expect(formatRelativeTime("not-a-date", NOW)).toBe("—");
  });
});

describe("formatAbsoluteTimestamp", () => {
  it("formats a valid ISO timestamp with a shift date, time, and zone offset", () => {
    const formatted = formatAbsoluteTimestamp("2026-09-23T09:34:05Z");
    expect(formatted).toMatch(/Sep 23, 2026/);
    expect(formatted).toMatch(/GMT/);
  });

  it("returns null for null, undefined, empty, or unparseable input", () => {
    expect(formatAbsoluteTimestamp(null)).toBeNull();
    expect(formatAbsoluteTimestamp(undefined)).toBeNull();
    expect(formatAbsoluteTimestamp("")).toBeNull();
    expect(formatAbsoluteTimestamp("not-a-date")).toBeNull();
  });
});
