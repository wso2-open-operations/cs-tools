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
  countListSearchAndFilters,
  hasListSearchOrFilters,
  isValidNumericIdFilters,
  normalizeCaseSearchIssueIds,
} from "@features/support/utils/listView";

describe("hasListSearchOrFilters", () => {
  it("is false when search and filters empty", () => {
    expect(hasListSearchOrFilters("", {})).toBe(false);
  });

  it("is true when search non-empty", () => {
    expect(hasListSearchOrFilters("x", {})).toBe(true);
  });

  it("is true when a filter has value", () => {
    expect(hasListSearchOrFilters("", { statusId: "1" })).toBe(true);
  });
});

describe("countListSearchAndFilters", () => {
  it("counts search and each non-empty filter", () => {
    expect(
      countListSearchAndFilters("q", { a: "1", b: "", c: undefined }),
    ).toBe(2);
  });

  it("counts non-empty array filters once", () => {
    expect(countListSearchAndFilters("", { issueTypes: ["1", "2"] })).toBe(1);
  });
});

describe("isValidNumericIdFilters", () => {
  it("accepts an empty object", () => {
    expect(isValidNumericIdFilters({})).toBe(true);
  });

  it("accepts a numeric-id string field", () => {
    expect(isValidNumericIdFilters({ stateId: "2" })).toBe(true);
  });

  it("accepts an undefined field", () => {
    expect(isValidNumericIdFilters({ stateId: undefined })).toBe(true);
  });

  it("rejects a raw enum-label value left over from before a choice-list id format change", () => {
    expect(isValidNumericIdFilters({ stateId: "ACTIVE" })).toBe(false);
  });

  it("rejects a non-object value", () => {
    expect(isValidNumericIdFilters("ACTIVE")).toBe(false);
    expect(isValidNumericIdFilters(null)).toBe(false);
    expect(isValidNumericIdFilters(undefined)).toBe(false);
  });

  it("rejects when any field among several is non-numeric", () => {
    expect(isValidNumericIdFilters({ a: "1", b: "ACTIVE" })).toBe(false);
  });
});

describe("normalizeCaseSearchIssueIds", () => {
  it("returns undefined when no selection", () => {
    expect(normalizeCaseSearchIssueIds(undefined)).toBeUndefined();
    expect(normalizeCaseSearchIssueIds([])).toBeUndefined();
    expect(normalizeCaseSearchIssueIds("")).toBeUndefined();
  });

  it("normalizes single and multiple string ids", () => {
    expect(normalizeCaseSearchIssueIds("3")).toEqual([3]);
    expect(normalizeCaseSearchIssueIds(["3", "5"])).toEqual([3, 5]);
  });
});
