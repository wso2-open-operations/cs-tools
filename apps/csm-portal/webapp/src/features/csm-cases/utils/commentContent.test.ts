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
  collapseCodeBlockWhitespace,
  collapseCommentSourceWhitespace,
  collapseHtmlSourceWhitespace,
  convertCodeTagsToHtml,
  hasDisplayableContent,
  hasPublicComment,
  hasSingleCodeWrapper,
  isGithubRaisedCaseNumber,
  isMarkdownComment,
  linkifyBareUrls,
  preprocessCommentBodyHtml,
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

  it("is false for multiple empty/label-only [code] blocks", () => {
    expect(
      hasDisplayableContent(
        makeComment("[code][/code][code]Customer comment added[/code]"),
      ),
    ).toBe(false);
  });
});

describe("hasPublicComment", () => {
  it("is false when comments is undefined (e.g. still loading)", () => {
    expect(hasPublicComment(undefined)).toBe(false);
  });

  it("is false for an empty comment list", () => {
    expect(hasPublicComment([])).toBe(false);
  });

  it("is false when every comment is an internal work note", () => {
    expect(
      hasPublicComment([{ ...makeComment("<p>Note</p>"), internal: true }]),
    ).toBe(false);
  });

  it("is false when the only non-internal comment has no displayable content", () => {
    expect(
      hasPublicComment([{ ...makeComment("<p></p>"), internal: false }]),
    ).toBe(false);
  });

  it("is true when at least one non-internal comment has real content", () => {
    expect(
      hasPublicComment([
        { ...makeComment("<p>Internal only</p>"), internal: true },
        { ...makeComment("<p>Visible to the customer</p>"), internal: false },
      ]),
    ).toBe(true);
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
    expect(out).toContain('href="https://example.com"');
  });

  it("does not produce a nested anchor when the URL is already an anchor's visible text", () => {
    const input = '<a href="https://example.com">https://example.com</a>';
    const out = linkifyBareUrls(input);
    expect(out).not.toContain('<a href="https://example.com"><a href="https://example.com"');
    expect((out.match(/<a /g) ?? []).length).toBe(1);
  });
});

describe("isGithubRaisedCaseNumber", () => {
  it("recognises the numbers entity-service gives records raised from a GitHub issue", () => {
    expect(isGithubRaisedCaseNumber("SR-GH-000012")).toBe(true);
    expect(isGithubRaisedCaseNumber("CHG-GH-000003")).toBe(true);
  });

  it("does not match ordinary or missing numbers", () => {
    expect(isGithubRaisedCaseNumber("CS0412345")).toBe(false);
    expect(isGithubRaisedCaseNumber("WSO2-GH-000012")).toBe(false);
    expect(isGithubRaisedCaseNumber(undefined)).toBe(false);
  });
});

describe("isMarkdownComment / preprocessCommentBodyHtml", () => {
  const issueBody = "### Request Details\n\ntesting\n\n### Priority\n\nCritical";

  it("renders a markdown-marked body's headings instead of leaving them as text", () => {
    const comment = { ...makeComment(issueBody), bodyFormat: "markdown" as const };
    expect(isMarkdownComment(comment)).toBe(true);
    const html = preprocessCommentBodyHtml(comment);
    expect(html).toContain("<h3>Request Details</h3>");
    expect(html).not.toContain("###");
  });

  it("leaves an unmarked body on the HTML path", () => {
    const comment = makeComment(issueBody);
    expect(isMarkdownComment(comment)).toBe(false);
    expect(preprocessCommentBodyHtml(comment)).toContain("### Request Details");
  });
});

describe("collapseHtmlSourceWhitespace", () => {
  it("drops the newlines between laid-out block elements, \\r\\n included", () => {
    expect(collapseHtmlSourceWhitespace("<p>One</p>\r\n<p>Two</p>\r\n<p>Three</p>")).toBe(
      "<p>One</p><p>Two</p><p>Three</p>",
    );
  });

  it("joins hard-wrapped list items and drops the indentation", () => {
    const source =
      "<ul>\n  <li>First point\n    continues here.<br>\n    Second line.\n  </li>\n</ul>";
    expect(collapseHtmlSourceWhitespace(source)).toBe(
      "<ul><li>First point continues here.<br>Second line.</li></ul>",
    );
  });

  it("keeps a single space between inline elements that were on separate lines", () => {
    expect(collapseHtmlSourceWhitespace("<p>\n  Use <b>one</b>\n  <i>two</i> now\n</p>")).toBe(
      "<p>Use <b>one</b> <i>two</i> now</p>",
    );
  });

  it("leaves a <pre> block and a <code> snippet exactly as written", () => {
    const source =
      "<p>Run:</p>\n<pre>line one\n  line two</pre>\n<p>Then <code>a\nb</code> done</p>";
    expect(collapseHtmlSourceWhitespace(source)).toBe(
      "<p>Run:</p><pre>line one\n  line two</pre><p>Then <code>a\nb</code> done</p>",
    );
  });

  it("keeps a space either side of inline code that sat on its own lines", () => {
    expect(collapseHtmlSourceWhitespace("<p>\n  Set <code>x=1</code>\n  then restart\n</p>")).toBe(
      "<p>Set <code>x=1</code> then restart</p>",
    );
  });

  it("does not touch a <pre> block that holds markup-looking text and blank lines", () => {
    const source = "<div>\n<pre><b>\n\n  keep</b>\n</pre>\n</div>";
    expect(collapseHtmlSourceWhitespace(source)).toBe("<div><pre><b>\n\n  keep</b>\n</pre></div>");
  });

  it("trims a newline at the very start and end of the body", () => {
    expect(collapseHtmlSourceWhitespace("\r\n<p>One</p>\r\n")).toBe("<p>One</p>");
  });

  it("leaves plain text alone: its newlines are its line breaks", () => {
    const text = "Line one\nLine two\n\nLine four";
    expect(collapseHtmlSourceWhitespace(text)).toBe(text);
  });

  it("leaves editor output alone: no newlines, and its runs of spaces are content", () => {
    const html = "<p>Two  spaces</p><p>Next<br>line</p>";
    expect(collapseHtmlSourceWhitespace(html)).toBe(html);
  });

  it("keeps the line breaks of a note that only mixes in a stray <br>", () => {
    const note = "Line one\nLine two<br>Line three";
    expect(collapseHtmlSourceWhitespace(note)).toBe(note);
  });

  it("does not treat <pre>, <link> or <track> as p, li or tr", () => {
    const html = '<link rel="x">\n<track src="y">\ntext';
    expect(collapseHtmlSourceWhitespace(html)).toBe(html);
  });

  it("handles upper-case tags", () => {
    expect(collapseHtmlSourceWhitespace("<P>One</P>\n<P>Two</P>")).toBe("<P>One</P><P>Two</P>");
  });
});

describe("collapseCodeBlockWhitespace", () => {
  it("collapses only inside the [code] block and leaves the text around it alone", () => {
    const source =
      "Intro line\nnext line\n[code]<ul>\n  <li>One</li>\n</ul>[/code]\nTail text\nmore";
    expect(collapseCodeBlockWhitespace(source)).toBe(
      "Intro line\nnext line\n[code]<ul><li>One</li></ul>[/code]\nTail text\nmore",
    );
  });

  it("collapses each of several [code] blocks on its own", () => {
    const source = "[code]<p>a</p>\n<p>b</p>[/code]\n[code]<p>c</p>\n<p>d</p>[/code]";
    expect(collapseCodeBlockWhitespace(source)).toBe(
      "[code]<p>a</p><p>b</p>[/code]\n[code]<p>c</p><p>d</p>[/code]",
    );
  });

  it("keeps the legacy escaped markers as written", () => {
    expect(collapseCodeBlockWhitespace("[\\code]<p>a</p>\n<p>b</p>[\\/code]")).toBe(
      "[\\code]<p>a</p><p>b</p>[\\/code]",
    );
  });

  it("leaves an inline [code] snippet that holds no markup alone", () => {
    const source = "Case Task [code]CSTASK1[/code] has been created\nsecond line";
    expect(collapseCodeBlockWhitespace(source)).toBe(source);
  });

  it("leaves the newlines of a [code] block that holds a plain snippet alone", () => {
    const source = "Run [code]line one\nline two[/code] now";
    expect(collapseCodeBlockWhitespace(source)).toBe(source);
  });

  it("also cleans laid-out HTML written outside the [code] blocks", () => {
    const source =
      "<div>\r\n  <p>Intro</p>\r\n</div>\r\n[code]<ul>\r\n  <li>One</li>\r\n</ul>[/code]\r\n<div>\r\n  <p>Outro</p>\r\n</div>";
    expect(collapseCodeBlockWhitespace(source)).toBe(
      "<div><p>Intro</p></div>[code]<ul><li>One</li></ul>[/code]<div><p>Outro</p></div>",
    );
  });

  it("judges a newline beside a block against its neighbours, so inline spacing survives", () => {
    const source =
      "<p>\r\n  See <b>this</b>\r\n[code]<b>that</b>[/code]\r\n  then stop\r\n</p>";
    expect(collapseCodeBlockWhitespace(source)).toBe(
      "<p>See <b>this</b> [code]<b>that</b>[/code] then stop</p>",
    );
  });

  it("does not let markup inside a block turn the plain text around it into laid-out HTML", () => {
    const source = "Line one\nLine two\n[code]<p>a</p>\n<p>b</p>[/code]\nLine three\nLine four";
    expect(collapseCodeBlockWhitespace(source)).toBe(
      "Line one\nLine two\n[code]<p>a</p><p>b</p>[/code]\nLine three\nLine four",
    );
  });

  it("still cleans each block when the text already holds the placeholder characters", () => {
    const source = "\uE000 note\n[code]<p>a</p>\n<p>b</p>[/code]";
    expect(collapseCodeBlockWhitespace(source)).toBe("\uE000 note\n[code]<p>a</p><p>b</p>[/code]");
  });
});

describe("collapseCommentSourceWhitespace", () => {
  it("collapses inside [code] blocks when the body has any, and the whole body otherwise", () => {
    expect(collapseCommentSourceWhitespace("[code]<p>a</p>\n<p>b</p>[/code]\nplain\ntext")).toBe(
      "[code]<p>a</p><p>b</p>[/code]\nplain\ntext",
    );
    expect(collapseCommentSourceWhitespace("<p>a</p>\n<p>b</p>")).toBe("<p>a</p><p>b</p>");
  });

  it("returns an empty string for an empty or missing body", () => {
    expect(collapseCommentSourceWhitespace("")).toBe("");
  });
});

describe("preprocessCommentBodyHtml: laid-out HTML source", () => {
  const laidOut = "<p>Summary:</p>\r\n<ol>\r\n  <li>First\r\n    wrapped</li>\r\n</ol>";
  const flat = "<p>Summary:</p><ol><li>First wrapped</li></ol>";

  it("cleans a body written as plain HTML", () => {
    expect(preprocessCommentBodyHtml(makeComment(laidOut))).toBe(flat);
  });

  it("cleans a body inside a single [code] wrapper", () => {
    expect(preprocessCommentBodyHtml(makeComment(`[code]${laidOut}[/code]`))).toBe(flat);
  });

  it("cleans the HTML inside several [code] blocks but keeps the newline between them", () => {
    const body = `[code]${laidOut}[/code]\n[code]<p>Next:</p>\n<p>Done</p>[/code]`;
    const html = preprocessCommentBodyHtml(makeComment(body));
    expect(html).toContain(flat);
    expect(html).toContain("<p>Next:</p><p>Done</p>");
    expect(html).toMatch(/<\/ol>\n+<p>Next:<\/p>/);
  });

  it("keeps the newlines of the plain text around a [code] block", () => {
    const html = preprocessCommentBodyHtml(
      makeComment("Before\nthe block\n[code]<p>a</p>\n<p>b</p>[/code]\nAfter\nit"),
    );
    expect(html).toContain("Before\nthe block\n");
    expect(html).toContain("\nAfter\nit");
    expect(html).toContain("<p>a</p><p>b</p>");
  });

  it("keeps a code snippet's own line breaks", () => {
    const body = "<p>Run:</p>\n<pre>one\n  two</pre>\n<p>or <code>x\ny</code></p>";
    expect(preprocessCommentBodyHtml(makeComment(body))).toBe(
      "<p>Run:</p><pre>one\n  two</pre><p>or <code>x\ny</code></p>",
    );
  });

  it("leaves a Markdown body to the Markdown renderer", () => {
    const html = preprocessCommentBodyHtml({
      ...makeComment("- one\n- two\n\ntext"),
      bodyFormat: "markdown",
    });
    expect(html).toContain("<li>one</li>");
    expect(html).toContain("\n");
  });
});
