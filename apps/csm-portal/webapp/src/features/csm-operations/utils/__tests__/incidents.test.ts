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
import { getLegalNextIncidentStates } from "@features/csm-operations/utils/incidents";
import type { BeIncidentState } from "@api/backend/types";

describe("getLegalNextIncidentStates", () => {
  it.each([
    ["NEW", ["NEW", "IN_PROGRESS", "CANCELLED"]],
    ["IN_PROGRESS", ["IN_PROGRESS", "ON_HOLD", "RESOLVED", "CANCELLED"]],
    ["ON_HOLD", ["ON_HOLD", "IN_PROGRESS", "CANCELLED"]],
    ["RESOLVED", ["RESOLVED", "CLOSED", "IN_PROGRESS"]],
    ["CLOSED", ["CLOSED"]],
    ["CANCELLED", ["CANCELLED"]],
  ] as const)("from %s returns %j", (current, expected) => {
    expect(getLegalNextIncidentStates(current as BeIncidentState)).toEqual(expected);
  });

  it("always includes the current state even for terminal states", () => {
    expect(getLegalNextIncidentStates("CLOSED")).toContain("CLOSED");
    expect(getLegalNextIncidentStates("CANCELLED")).toContain("CANCELLED");
  });

  it("treats CLOSED and CANCELLED as terminal (no outgoing transitions)", () => {
    expect(getLegalNextIncidentStates("CLOSED")).toHaveLength(1);
    expect(getLegalNextIncidentStates("CANCELLED")).toHaveLength(1);
  });
});
