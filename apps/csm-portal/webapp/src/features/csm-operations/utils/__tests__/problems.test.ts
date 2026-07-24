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
  countActiveProblemFilters,
  DEFAULT_PROBLEM_FILTERS,
  PROBLEM_STATES,
  problemStateColor,
  problemStateLabel,
} from "@features/csm-operations/utils/problems";

describe("problemStateLabel", () => {
  it.each([
    ["NEW", "New"],
    ["ASSESS", "Assess"],
    ["ROOT_CAUSE_ANALYSIS", "Root Cause Analysis"],
    ["FIX_IN_PROGRESS", "Fix In Progress"],
    ["RESOLVED", "Resolved"],
    ["CLOSED", "Closed"],
  ] as const)("labels %s as %s", (state, label) => {
    expect(problemStateLabel(state)).toBe(label);
  });

  it("returns an em dash for a missing state", () => {
    expect(problemStateLabel(null)).toBe("—");
    expect(problemStateLabel(undefined)).toBe("—");
  });

  it("humanizes an unrecognized state rather than throwing", () => {
    expect(problemStateLabel("SOME_NEW_STATE")).toBe("SOME NEW STATE");
  });
});

describe("problemStateColor", () => {
  it("returns success for terminal-good states", () => {
    expect(problemStateColor("RESOLVED")).toBe("success");
    expect(problemStateColor("CLOSED")).toBe("success");
  });

  it("returns warning for the active fix state", () => {
    expect(problemStateColor("FIX_IN_PROGRESS")).toBe("warning");
  });

  it("falls back to default for an unrecognized state", () => {
    expect(problemStateColor("SOME_NEW_STATE")).toBe("default");
  });
});

describe("PROBLEM_STATES", () => {
  it("lists all 6 documented states", () => {
    expect(PROBLEM_STATES).toHaveLength(6);
    expect(PROBLEM_STATES).toEqual(
      expect.arrayContaining([
        "NEW",
        "ASSESS",
        "ROOT_CAUSE_ANALYSIS",
        "FIX_IN_PROGRESS",
        "RESOLVED",
        "CLOSED",
      ]),
    );
  });
});

describe("countActiveProblemFilters", () => {
  it("is 0 for the default filters", () => {
    expect(countActiveProblemFilters(DEFAULT_PROBLEM_FILTERS)).toBe(0);
  });

  it("is 1 when a state filter is set", () => {
    expect(countActiveProblemFilters({ ...DEFAULT_PROBLEM_FILTERS, states: ["NEW"] })).toBe(1);
  });
});
