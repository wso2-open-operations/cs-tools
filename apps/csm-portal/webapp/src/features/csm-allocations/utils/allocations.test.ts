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
  ALLOCATION_STATES,
  allocationStateLabel,
  formatUtilization,
  looksLikeHtml,
} from "./allocations";

describe("allocation helpers", () => {
  it("gives every known state a friendly label", () => {
    for (const s of ALLOCATION_STATES) {
      expect(allocationStateLabel(s)).not.toMatch(/_/);
      expect(allocationStateLabel(s)).not.toBe(s);
    }
    expect(allocationStateLabel("CONFIRMED_VISA_PENDING")).toBe("Confirmed, visa pending");
  });

  it("falls back to readable words for a state it does not know", () => {
    expect(allocationStateLabel("SOME_NEW_STATE")).toBe("Some new state");
  });

  it("formats utilization", () => {
    expect(formatUtilization(50)).toBe("50%");
    expect(formatUtilization(25.55)).toBe("25.6%");
    expect(formatUtilization(undefined)).toBe("—");
  });

  it("tells markup from plain text", () => {
    expect(looksLikeHtml("<p>hi</p>")).toBe(true);
    expect(looksLikeHtml("2 < 3 and 5 > 4")).toBe(false);
    expect(looksLikeHtml("plain\ntext")).toBe(false);
  });
});
