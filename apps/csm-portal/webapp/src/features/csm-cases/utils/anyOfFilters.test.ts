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
  ANY_OF_FILTER_FIELDS,
  anyOfBranchToPayload,
  isCompleteAnyOfBranch,
  isCompleteAnyOfFilterRow,
  type AnyOfBranch,
} from "./anyOfFilters";

describe("ANY_OF_FILTER_FIELDS — branch field allowlist", () => {
  it("offers exactly the backend-accepted CaseFilterGroup fields", () => {
    const fields = ANY_OF_FILTER_FIELDS.map((m) => m.field).sort();
    expect(fields).toEqual(
      [
        "assignedUserId",
        "deploymentId",
        "engagementType",
        "escalationLevel",
        "issueType",
        "projectId",
        "severity",
        "state",
        "type",
        "workState",
      ].sort(),
    );
  });

  it("every field is in-only (no notIn/isEmpty inside a branch)", () => {
    for (const meta of ANY_OF_FILTER_FIELDS) {
      expect(meta.op).toBe("in");
    }
  });
});

describe("isCompleteAnyOfFilterRow / isCompleteAnyOfBranch", () => {
  it("a row with no values is incomplete", () => {
    expect(isCompleteAnyOfFilterRow({ field: "type", values: [] })).toBe(false);
  });

  it("a row with a value is complete", () => {
    expect(isCompleteAnyOfFilterRow({ field: "type", values: ["case"] })).toBe(true);
  });

  it("a branch with only incomplete rows is incomplete", () => {
    const branch: AnyOfBranch = { filters: [{ field: "type", values: [] }] };
    expect(isCompleteAnyOfBranch(branch)).toBe(false);
  });

  it("a branch with at least one complete row is complete", () => {
    const branch: AnyOfBranch = {
      filters: [{ field: "type", values: [] }, { field: "severity", values: ["critical"] }],
    };
    expect(isCompleteAnyOfBranch(branch)).toBe(true);
  });
});

describe("anyOfBranchToPayload", () => {
  it("emits {filters: [...]} for a branch with complete conditions, dropping incomplete ones", () => {
    const branch: AnyOfBranch = {
      filters: [
        { field: "type", values: ["case"] },
        { field: "severity", values: [] },
        { field: "workState", values: ["ongoing", "paused"] },
      ],
    };
    expect(anyOfBranchToPayload(branch)).toEqual({
      filters: [
        { field: "type", op: "in", values: ["case"] },
        { field: "workState", op: "in", values: ["ongoing", "paused"] },
      ],
    });
  });

  it("returns undefined for a branch with zero complete conditions (never emits {filters: []})", () => {
    const branch: AnyOfBranch = { filters: [{ field: "type", values: [] }] };
    expect(anyOfBranchToPayload(branch)).toBeUndefined();
  });
});
