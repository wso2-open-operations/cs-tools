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
  findTopicMatch,
  HELP_TOPIC_PLAIN_TEXT,
  stripMarkdownToPlainText,
} from "@features/help/utils/helpSearch";

describe("stripMarkdownToPlainText", () => {
  it("removes heading markers, emphasis, links, and code spans", () => {
    const source = "# Title\n\nSome **bold** and _italic_ text with a [link](https://example.com) and `code`.";
    expect(stripMarkdownToPlainText(source)).toBe(
      "Title Some bold and italic text with a link and code.",
    );
  });

  it("removes list markers and collapses whitespace across lines", () => {
    const source = "- one\n- two\n\n1. first\n2. second";
    expect(stripMarkdownToPlainText(source)).toBe("one two first second");
  });
});

describe("findTopicMatch", () => {
  it("matches an empty query against every topic", () => {
    expect(findTopicMatch("Operations", "irrelevant content", "")).toEqual({
      matchedInTitle: true,
    });
  });

  it("matches the label case-insensitively, without needing to search content", () => {
    expect(findTopicMatch("Operations", "content mentioning nothing relevant", "OPERA")).toEqual({
      matchedInTitle: true,
    });
  });

  it("falls back to a content match with a surrounding snippet when the label doesn't match", () => {
    const plain = "Operations covers service requests, change requests, incidents, and problems.";
    const result = findTopicMatch("Operations", plain, "incidents");

    expect(result?.matchedInTitle).toBe(false);
    const snippet = result?.snippet;
    expect(snippet).toBeDefined();
    expect(snippet?.match).toBe("incidents");
    expect(`${snippet?.before}${snippet?.match}${snippet?.after}`).toContain("incidents");
    expect(snippet?.before).toContain("change requests");
  });

  it("returns null when neither the label nor the content match", () => {
    expect(findTopicMatch("Operations", "nothing relevant here", "zzz-not-found")).toBeNull();
  });

  it("finds the real 'Operations' topic's own content when searching a sub-topic term it covers", () => {
    // Regression for the reported gap: "Incidents" is a tab inside Operations,
    // not its own Help topic, so a title-only search for it used to return no
    // results at all even though Operations covers it in depth.
    const result = findTopicMatch("Operations", HELP_TOPIC_PLAIN_TEXT.operations, "incidents");
    expect(result).not.toBeNull();
    expect(result?.matchedInTitle).toBe(false);
  });
});
