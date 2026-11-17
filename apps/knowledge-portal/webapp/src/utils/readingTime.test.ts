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
import { formatReadingTime, readingTimeMinutes } from "@utils/readingTime";

describe("readingTimeMinutes", () => {
  it("is 0 for empty text", () => {
    expect(readingTimeMinutes("")).toBe(0);
    expect(readingTimeMinutes("   ")).toBe(0);
  });
  it("never reports less than a minute for real text", () => {
    expect(readingTimeMinutes("a few words")).toBe(1);
  });
  it("rounds to whole minutes at 200 words per minute", () => {
    expect(readingTimeMinutes(Array(500).fill("w").join(" "))).toBe(3);
  });
});

describe("formatReadingTime", () => {
  it("formats minutes and hides zero", () => {
    expect(formatReadingTime(4)).toBe("4 min read");
    expect(formatReadingTime(0)).toBeUndefined();
  });
});
