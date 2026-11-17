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
  buildFixEtaCommentTemplate,
  fixEtaCommentTextToHtml,
} from "@features/csm-cases/utils/fixEtaComment";

const FULL = {
  product: "API Manager",
  publicTicket: "https://github.com/example/example/issues/1",
  bestCaseFixEta: "2099-06-16",
  mostLikelyFixEta: "2099-06-17",
  worstCaseFixEta: "2099-06-18",
};

describe("buildFixEtaCommentTemplate", () => {
  it("builds the fix wording with every field", () => {
    expect(buildFixEtaCommentTemplate({ kind: "fix", ...FULL })).toBe(
      "ETA information for the fix will be as follows.\n\n" +
        "Product: API Manager\n" +
        "Public git issue: https://github.com/example/example/issues/1\n" +
        "Best Case Estimate: 2099-06-16\n" +
        "Most Likely Estimate: 2099-06-17\n" +
        "Worst Case Estimate: 2099-06-18",
    );
  });

  it("switches only the opening line for a report analysis", () => {
    const fix = buildFixEtaCommentTemplate({ kind: "fix", ...FULL });
    const report = buildFixEtaCommentTemplate({ kind: "report_analysis", ...FULL });
    expect(report).toBe(
      fix.replace(
        "ETA information for the fix will be as follows.",
        "ETA information for the report analysis will be as follows.",
      ),
    );
  });

  it("omits lines whose value is blank", () => {
    expect(
      buildFixEtaCommentTemplate({
        kind: "fix",
        product: "API Manager",
        mostLikelyFixEta: "2099-06-17",
      }),
    ).toBe(
      "ETA information for the fix will be as follows.\n\nProduct: API Manager\nMost Likely Estimate: 2099-06-17",
    );
  });

  it("is just the opening line when no field has a value", () => {
    expect(buildFixEtaCommentTemplate({ kind: "fix" })).toBe(
      "ETA information for the fix will be as follows.",
    );
  });
});

describe("fixEtaCommentTextToHtml", () => {
  it("renders an unedited template as paragraphs with <br> line breaks", () => {
    expect(
      fixEtaCommentTextToHtml(buildFixEtaCommentTemplate({ kind: "fix", ...FULL })),
    ).toBe(
      "<p>ETA information for the fix will be as follows.</p>" +
        "<p>Product: API Manager<br>" +
        "Public git issue: https://github.com/example/example/issues/1<br>" +
        "Best Case Estimate: 2099-06-16<br>" +
        "Most Likely Estimate: 2099-06-17<br>" +
        "Worst Case Estimate: 2099-06-18</p>",
    );
  });

  it("escapes HTML-significant characters", () => {
    expect(fixEtaCommentTextToHtml("Tom & Jerry's <img src=x onerror=alert(1)>")).toBe(
      "<p>Tom &amp; Jerry&#039;s &lt;img src=x onerror=alert(1)&gt;</p>",
    );
  });

  it("normalises CRLF and collapses runs of blank lines", () => {
    expect(fixEtaCommentTextToHtml("a\r\nb\r\n\r\n\r\n\r\nc\n")).toBe(
      "<p>a<br>b</p><p>c</p>",
    );
  });

  it("returns an empty string for blank text", () => {
    expect(fixEtaCommentTextToHtml(" \n\n ")).toBe("");
  });
});
