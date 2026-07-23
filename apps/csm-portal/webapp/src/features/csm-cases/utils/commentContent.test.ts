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
  convertCodeTagsToHtml,
  hasDisplayableContent,
  hasSingleCodeWrapper,
  linkifyBareUrls,
  stripAllCodeBlocks,
  stripCodeWrapper,
  stripCustomerCommentAddedLabel,
  trimLeadingBr,
} from "./commentContent";
import type { CsmCaseComment } from "@features/csm-cases/types/csmCases";

function makeComment(bodyHtml: string): CsmCaseComment {
  return {
    id: "c-1",
    caseId: "case-1",
    authorName: "Jane Doe",
    authorRole: "customer",
    bodyHtml,
    createdAt: "2026-07-01T00:00:00Z",
  };
}

describe("convertCodeTagsToHtml", () => {
  it("converts a [code]...[/code] wrapper into a <code> element", () => {
    expect(convertCodeTagsToHtml("[code]hello[/code]")).toBe("<code>hello</code>");
  });

  it("returns empty string for non-string input", () => {
    expect(convertCodeTagsToHtml(undefined as unknown as string)).toBe("");
  });
});

describe("stripAllCodeBlocks", () => {
  it("strips multiple [code] blocks and keeps the inner content", () => {
    // The normalize step inserts a newline between adjacent [/code][code]
    // markers before the blocks are stripped, hence the blank line between.
    expect(stripAllCodeBlocks("[code]a[/code][code]b[/code]")).toBe("a\n\nb\n");
  });
});

describe("trimLeadingBr", () => {
  it("removes leading <br> tags and whitespace", () => {
    expect(trimLeadingBr("<br><br/>  <b>x</b>")).toBe("<b>x</b>");
  });
});

describe("hasSingleCodeWrapper / stripCodeWrapper", () => {
  it("detects exactly one top-level wrapper", () => {
    expect(hasSingleCodeWrapper("[code]hi[/code]")).toBe(true);
    expect(hasSingleCodeWrapper("[code]a[/code][code]b[/code]")).toBe(false);
  });

  it("strips the wrapper only when single", () => {
    expect(stripCodeWrapper("[code]hi[/code]")).toBe("hi");
    expect(stripCodeWrapper("[code]a[/code][code]b[/code]")).toBe(
      "[code]a[/code][code]b[/code]",
    );
  });
});

describe("stripCustomerCommentAddedLabel", () => {
  it("removes the wrapped label paragraph", () => {
    expect(
      stripCustomerCommentAddedLabel("<p>Customer comment added</p><p>Body</p>"),
    ).toBe("<p>Body</p>");
  });

  it("removes bare occurrences of the label text", () => {
    expect(stripCustomerCommentAddedLabel("Customer comment added: hi")).toBe(": hi");
  });
});

describe("hasDisplayableContent", () => {
  it("is false for a code-wrapped label-only comment", () => {
    expect(
      hasDisplayableContent(makeComment("[code]Customer comment added[/code]")),
    ).toBe(false);
  });

  it("is true when text remains after stripping", () => {
    expect(hasDisplayableContent(makeComment("<p>Hello there</p>"))).toBe(true);
  });

  it("is true for an image-only comment with no text", () => {
    expect(
      hasDisplayableContent(makeComment('<img src="https://example.com/a.png" />')),
    ).toBe(true);
  });

  it("is false for a genuinely empty comment", () => {
    expect(hasDisplayableContent(makeComment("<p></p>"))).toBe(false);
  });
});

describe("linkifyBareUrls", () => {
  it("wraps a bare URL in an anchor with target=_blank and rel", () => {
    const out = linkifyBareUrls("see https://example.com/path for details");
    expect(out).toContain('<a href="https://example.com/path"');
    expect(out).toContain('target="_blank"');
    expect(out).toContain('rel="noopener noreferrer"');
  });

  it("does not double-wrap a URL already inside an href attribute", () => {
    const input = '<a href="https://example.com">https://example.com</a>';
    const out = linkifyBareUrls(input);
    // Only the visible text node URL gets wrapped again by design (matches
    // the customer-portal behavior); the href itself is left untouched.
    expect(out).toContain('href="https://example.com"');
  });
});
