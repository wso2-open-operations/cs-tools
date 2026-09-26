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
import { qsFromPastedFilter, shareUrl } from "@features/saved-filter-views/shareLink";

describe("shareUrl", () => {
  it("builds the list page URL for each list", () => {
    expect(shareUrl("cases", "state=open&severity=S1", "http://localhost:3001")).toBe(
      "http://localhost:3001/cases?state=open&severity=S1",
    );
    expect(shareUrl("incidents", "q=1", "http://localhost:3001")).toBe(
      "http://localhost:3001/operations/incidents?q=1",
    );
    expect(shareUrl("change_requests", "q=1", "http://localhost:3001")).toBe(
      "http://localhost:3001/operations/change-requests?q=1",
    );
    expect(shareUrl("problems", "q=1", "http://localhost:3001")).toBe(
      "http://localhost:3001/operations/problems?q=1",
    );
  });

  it("omits the query when the view has no filters", () => {
    expect(shareUrl("cases", "", "http://localhost:3001")).toBe("http://localhost:3001/cases");
  });
});

describe("qsFromPastedFilter", () => {
  it("reads the query string from a page URL and from a bare query string", () => {
    expect(qsFromPastedFilter("http://localhost:3001/cases?state=open&severity=S1", "cases")).toEqual({
      ok: true,
      qs: "state=open&severity=S1",
    });
    expect(qsFromPastedFilter("state=open&severity=S1", "cases")).toEqual({
      ok: true,
      qs: "state=open&severity=S1",
    });
    expect(qsFromPastedFilter("?state=open", "cases")).toEqual({ ok: true, qs: "state=open" });
  });

  it("accepts a list URL with no query as an empty filter", () => {
    expect(qsFromPastedFilter("http://localhost:3001/cases", "cases")).toEqual({ ok: true, qs: "" });
  });

  it("rejects a link for a different list", () => {
    expect(
      qsFromPastedFilter("http://localhost:3001/operations/incidents?q=1", "cases"),
    ).toEqual({ ok: false, error: "That link is for a different list." });
  });

  it("rejects empty text and text that is not a filter", () => {
    expect(qsFromPastedFilter("   ", "cases")).toEqual({ ok: false, error: "Paste a filter link." });
    expect(qsFromPastedFilter("hello", "cases")).toEqual({
      ok: false,
      error: "That link doesn't contain a filter.",
    });
  });
});
