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
import { formatDate, formatRelativeDate, formatRelativeTime } from "@utils/formatDate";

describe("formatRelativeTime", () => {
  const now = new Date("2026-03-10T12:00:00Z");
  it("steps from seconds to days", () => {
    expect(formatRelativeTime(new Date("2026-03-10T11:59:30Z"), now)).toBe("Just now");
    expect(formatRelativeTime(new Date("2026-03-10T11:55:00Z"), now)).toBe("5 minutes ago");
    expect(formatRelativeTime(new Date("2026-03-10T09:00:00Z"), now)).toBe("3 hours ago");
    expect(formatRelativeTime(new Date("2026-03-09T12:00:00Z"), now)).toBe("yesterday");
    expect(formatRelativeTime(new Date("2026-03-06T12:00:00Z"), now)).toBe("4 days ago");
  });
});

describe("formatRelativeDate", () => {
  const now = new Date("2026-03-10T12:00:00Z");

  it("is relative within a week and absolute after", () => {
    const recent = formatRelativeDate("2026-03-08T12:00:00Z", now);
    expect(recent?.label).toBe("2 days ago");
    expect(recent?.absolute).toBe(formatDate("2026-03-08T12:00:00Z"));

    const old = formatRelativeDate("2026-01-01T00:00:00Z", now);
    expect(old?.label).toBe(formatDate("2026-01-01T00:00:00Z"));
  });

  it("is undefined for missing or invalid input", () => {
    expect(formatRelativeDate(undefined, now)).toBeUndefined();
    expect(formatRelativeDate("nope", now)).toBeUndefined();
  });
});
