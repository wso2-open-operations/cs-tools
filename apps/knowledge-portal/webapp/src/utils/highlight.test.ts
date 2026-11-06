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
import { searchTerms, splitByTerms } from "@utils/highlight";

describe("searchTerms", () => {
  it("splits on whitespace, deduplicates and sorts longest first", () => {
    expect(searchTerms("  api  manager api ")).toEqual(["manager", "api"]);
  });
  it("is empty for nothing", () => {
    expect(searchTerms("")).toEqual([]);
    expect(searchTerms(undefined)).toEqual([]);
  });
});

describe("splitByTerms", () => {
  it("marks case-insensitive matches and keeps the original casing", () => {
    expect(splitByTerms("Gateway returns 900901", "gateway 900901")).toEqual([
      { text: "Gateway", match: true },
      { text: " returns ", match: false },
      { text: "900901", match: true },
    ]);
  });
  it("treats regex characters literally", () => {
    expect(splitByTerms("value (x) and x", "(x)")).toEqual([
      { text: "value ", match: false },
      { text: "(x)", match: true },
      { text: " and x", match: false },
    ]);
  });
  it("returns the text unmarked without a query", () => {
    expect(splitByTerms("plain", "")).toEqual([{ text: "plain", match: false }]);
    expect(splitByTerms("plain", null)).toEqual([{ text: "plain", match: false }]);
  });
  it("drops empty runs", () => {
    expect(splitByTerms("abc", "abc")).toEqual([{ text: "abc", match: true }]);
  });
});
