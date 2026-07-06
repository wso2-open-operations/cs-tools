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
