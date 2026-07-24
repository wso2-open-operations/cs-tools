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
import { computeIncidentPriority } from "@features/csm-operations/utils/incidentPriorityMatrix";

describe("computeIncidentPriority", () => {
  it.each([
    ["HIGH", "HIGH", "CRITICAL", "P1"],
    ["HIGH", "MEDIUM", "HIGH", "P2"],
    ["HIGH", "LOW", "MODERATE", "P3"],
    ["MEDIUM", "HIGH", "HIGH", "P2"],
    ["MEDIUM", "MEDIUM", "MODERATE", "P3"],
    ["MEDIUM", "LOW", "LOW", "P4"],
    ["LOW", "HIGH", "MODERATE", "P3"],
    ["LOW", "MEDIUM", "LOW", "P4"],
    ["LOW", "LOW", "PLANNING", "P5"],
  ] as const)("impact=%s urgency=%s -> %s (%s)", (impact, urgency, priority, code) => {
    const result = computeIncidentPriority(impact, urgency);
    expect(result?.priority).toBe(priority);
    expect(result?.code).toBe(code);
  });

  it("returns null when impact is missing", () => {
    expect(computeIncidentPriority("", "HIGH")).toBeNull();
  });

  it("returns null when urgency is missing", () => {
    expect(computeIncidentPriority("HIGH", "")).toBeNull();
  });

  it("returns null when both are missing", () => {
    expect(computeIncidentPriority("", "")).toBeNull();
  });
});
