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
  compareByCreatedOnThenId,
  convertCodeTagsToHtml,
  deriveFilterLabels,
  extractInlineImageRefId,
  isInlineImageRefSrc,
  hasSingleCodeWrapper,
  hasSubmittableEditorContent,
  linkifyBareUrls,
  normalizeCaseTypeOptions,
  replaceInlineImageSources,
  stripCodeWrapper,
  toUtcEndOfDay,
  toUtcStartOfDay,
} from "@features/support/utils/support";

describe("extractInlineImageRefId", () => {
  it("extracts 32-char id from service-now style .iix paths", () => {
    expect(
      extractInlineImageRefId(
        "https://host/path/1234567890abcdef1234567890abcdef.iix?x=1",
      ),
    ).toBe("1234567890abcdef1234567890abcdef");
  });
});

describe("replaceInlineImageSources", () => {
  it("replaces matching inline image with preview url and keeps unmatched", () => {
    const html =
      "<p>Images</p><img src='/1234567890abcdef1234567890abcdef.iix' /><img src='/no-match.iix' />";
    const output = replaceInlineImageSources(html, [
      {
        id: "1234567890abcdef1234567890abcdef",
        previewUrl: "data:image/png;base64,AAA",
      },
    ]);

    expect(output).toContain('src="data:image/png;base64,AAA"');
    expect(output).toContain('src="/no-match.iix"');
  });
});

describe("code-wrapper helpers", () => {
  it("detects and strips only a single [code] wrapper", () => {
    const content = "[code]Hello[/code]";
    expect(hasSingleCodeWrapper(content)).toBe(true);
    expect(stripCodeWrapper(content)).toBe("Hello");
  });

  it("converts escaped or repeated [code] tags to html code blocks", () => {
    const content = "[code]A[/code][code]B[/code]";
    expect(convertCodeTagsToHtml(content)).toContain("<code>A</code>");
    expect(convertCodeTagsToHtml(content)).toContain("<code>B</code>");
  });
});

describe("hasSubmittableEditorContent", () => {
  it("accepts image-only rich text as submit-worthy", () => {
    expect(hasSubmittableEditorContent("<p><img src='x.png' /></p>")).toBe(true);
  });

  it("rejects empty html wrappers", () => {
    expect(hasSubmittableEditorContent("<p>   </p>")).toBe(false);
  });
});

describe("normalizeCaseTypeOptions", () => {
  it("filters announcement and merges query+incident as Case option", () => {
    const normalized = normalizeCaseTypeOptions([
      { id: "1", label: "Announcement" },
      { id: "2", label: "Incident" },
      { id: "3", label: "Query" },
      { id: "4", label: "Task" },
    ]);

    expect(normalized).toEqual([
      { label: "Task", value: "4" },
      { label: "Case", value: "2,3" },
    ]);
  });
});

describe("compareByCreatedOnThenId", () => {
  it("orders human comment before bot when timestamps tie", () => {
    const rows = [
      { id: "2", createdOn: "2026-02-01 10:00:00", createdBy: "Novera", type: "bot" },
      { id: "1", createdOn: "2026-02-01 10:00:00", createdBy: "Alice", type: "human" },
    ];
    rows.sort(compareByCreatedOnThenId);

    expect(rows.map((r) => r.id)).toEqual(["1", "2"]);
  });
});

describe("linkifyBareUrls", () => {
  it("linkifies plain urls but skips href values", () => {
    const html = '<a href="https://already.linked">ok</a> visit https://wso2.com/docs';
    const output = linkifyBareUrls(html);

    expect(output).toContain('<a href="https://already.linked">ok</a>');
    expect(output).toContain(
      '<a href="https://wso2.com/docs" target="_blank" rel="noopener noreferrer"',
    );
  });
});

// Regression tests: a Date with fewer than 4 digits in its year (reachable
// live from a partially-typed MUI DatePicker year section) used to
// serialize into a malformed, non-zero-padded RFC3339 string (e.g.
// "2-01-10T00:00:00Z") that entity-service's filter parser rejected with a
// 400. Month/day were already zero-padded; only the year was missed.
describe("toUtcStartOfDay", () => {
  it("zero-pads a short year to 4 digits", () => {
    // new Date(2, 0, 10) would NOT give year 2 -- the Date constructor
    // special-cases a 0-99 year argument as 1900+year. setFullYear has no
    // such special-casing, so it's the only way to construct a genuinely
    // short year for this test.
    const date = new Date(2026, 0, 10);
    date.setFullYear(2);
    expect(toUtcStartOfDay(date)).toBe("0002-01-10T00:00:00Z");
  });

  it("formats a normal 4-digit year unchanged", () => {
    const date = new Date(2026, 0, 10);
    expect(toUtcStartOfDay(date)).toBe("2026-01-10T00:00:00Z");
  });
});

describe("toUtcEndOfDay", () => {
  it("zero-pads a short year to 4 digits", () => {
    const date = new Date(2026, 0, 10);
    date.setFullYear(2);
    expect(toUtcEndOfDay(date)).toBe("0002-01-11T00:00:00Z");
  });

  it("formats a normal 4-digit year unchanged", () => {
    const date = new Date(2026, 0, 10);
    expect(toUtcEndOfDay(date)).toBe("2026-01-11T00:00:00Z");
  });
});

// Content migrated from the legacy data source carries inline images as a bare
// attachment id (`<img src="/<uuid>">`) with no `.iix` suffix.
describe("bare attachment-id srcs (migrated content)", () => {
  const UUID = "0f15cbcc-c36b-8310-af2f-404599013196";
  const HEX = UUID.replace(/-/g, "");

  it.each([
    ["hyphenated uuid with leading slash", `/${UUID}`, HEX],
    ["hyphenated uuid without leading slash", UUID, HEX],
    ["uppercase hyphenated uuid", `/${UUID.toUpperCase()}`, HEX],
    ["hyphenated uuid with .iix", `/${UUID}.iix`, HEX],
    ["32-hex id with leading slash", `/${HEX}`, HEX],
    ["32-hex id without leading slash", HEX, HEX],
    ["surrounding whitespace", `  /${UUID} `, HEX],
  ])("treats %s as an attachment reference", (_name, src, id) => {
    expect(isInlineImageRefSrc(src)).toBe(true);
    expect(extractInlineImageRefId(src)).toBe(id);
  });

  it("keeps existing .iix behaviour unchanged", () => {
    expect(isInlineImageRefSrc(`/${HEX}.iix`)).toBe(true);
    expect(extractInlineImageRefId(`/${HEX}.iix`)).toBe(HEX);
    expect(extractInlineImageRefId(`https://host/${HEX}.iix`)).toBe(HEX);
    expect(isInlineImageRefSrc("/no-match.iix")).toBe(true);
  });

  it.each([
    ["query string", `/${UUID}?x=1`],
    ["extra path segment", `/images/${UUID}`],
    ["absolute url", `https://host/${UUID}`],
    ["protocol-relative", `//${UUID}`],
    ["double leading slash", `//${HEX}`],
    ["other extension", `/${UUID}.png`],
    ["data uri", "data:image/png;base64,AAAA"],
    ["short hex", "/abc123"],
    ["malformed uuid", "/0f15cbcc-c36b-8310-af2f-40459901319"],
  ])("does not treat %s as an attachment reference", (_name, src) => {
    expect(isInlineImageRefSrc(src)).toBe(false);
  });

  it("replaces a bare-uuid src when the attachment id is the 32-hex form", () => {
    const out = replaceInlineImageSources(`<p><img src="/${UUID}"><br></p>`, [
      { id: HEX, previewUrl: "data:image/png;base64,AAA" },
    ]);
    expect(out).toContain('src="data:image/png;base64,AAA"');
  });

  it("leaves a bare-uuid src untouched when no attachment matches", () => {
    const out = replaceInlineImageSources(`<img src="/${UUID}">`, [
      { id: "ffffffffffffffffffffffffffffffff", previewUrl: "data:x" },
    ]);
    expect(out).toContain(`src="/${UUID}"`);
  });
});

describe("deriveFilterLabels", () => {
  it("words the state filter as Status, like every other list page", () => {
    expect(deriveFilterLabels("state")).toEqual({
      label: "Status",
      allLabel: "All Statuses",
    });
  });

  it("matches the label and all-option of the status filter", () => {
    expect(deriveFilterLabels("state")).toEqual(deriveFilterLabels("status"));
  });

  it("keeps capitalising and pluralising other ids", () => {
    expect(deriveFilterLabels("severity")).toEqual({
      label: "Severity",
      allLabel: "All Severities",
    });
    expect(deriveFilterLabels("impact")).toEqual({
      label: "Impact",
      allLabel: "All Impacts",
    });
    expect(deriveFilterLabels("caseType")).toEqual({
      label: "Case Type",
      allLabel: "All Case Types",
    });
    expect(deriveFilterLabels("createdBy")).toEqual({
      label: "Created By",
      allLabel: "All Users",
    });
    expect(deriveFilterLabels("status")).toEqual({
      label: "Status",
      allLabel: "All Statuses",
    });
  });
});

describe("collapseHtmlSourceWhitespace", () => {
  it("drops the newlines between laid-out block elements, \\r\\n included", () => {
    expect(collapseHtmlSourceWhitespace("<p>One</p>\r\n<p>Two</p>\r\n<p>Three</p>")).toBe(
      "<p>One</p><p>Two</p><p>Three</p>",
    );
  });

  it("joins a hand-wrapped paragraph and drops the indentation", () => {
    expect(
      collapseHtmlSourceWhitespace("<p>Wrapped by hand\r\n   at a fixed width.</p>\r\n<p>Next.</p>"),
    ).toBe("<p>Wrapped by hand at a fixed width.</p><p>Next.</p>");
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

  it("leaves plain text and editor output alone", () => {
    const text = "Line one\nLine two\n\nLine four";
    expect(collapseHtmlSourceWhitespace(text)).toBe(text);
    const editor = "<p>Two  spaces</p><p>Next<br>line</p>";
    expect(collapseHtmlSourceWhitespace(editor)).toBe(editor);
  });

  it("keeps the line breaks of a note that only mixes in a stray <br>", () => {
    const note = "Line one\nLine two<br>Line three";
    expect(collapseHtmlSourceWhitespace(note)).toBe(note);
  });

  it("trims a newline at the very start and end of the body", () => {
    expect(collapseHtmlSourceWhitespace("\r\n<p>One</p>\r\n")).toBe("<p>One</p>");
  });
});

describe("collapseCodeBlockWhitespace / collapseCommentSourceWhitespace", () => {
  it("collapses only inside the [code] block and leaves the text around it alone", () => {
    const source =
      "Intro line\nnext line\n[code]<ul>\n  <li>One</li>\n</ul>[/code]\nTail text\nmore";
    expect(collapseCodeBlockWhitespace(source)).toBe(
      "Intro line\nnext line\n[code]<ul><li>One</li></ul>[/code]\nTail text\nmore",
    );
  });

  it("keeps the legacy escaped markers as written", () => {
    expect(collapseCodeBlockWhitespace("[\\code]<p>a</p>\n<p>b</p>[\\/code]")).toBe(
      "[\\code]<p>a</p><p>b</p>[\\/code]",
    );
  });

  it("leaves an inline [code] snippet with no markup, and its newlines, alone", () => {
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

  it("picks the per-block or whole-body form by whether the body has [code] markers", () => {
    expect(collapseCommentSourceWhitespace("[code]<p>a</p>\n<p>b</p>[/code]\nplain\ntext")).toBe(
      "[code]<p>a</p><p>b</p>[/code]\nplain\ntext",
    );
    expect(collapseCommentSourceWhitespace("<p>a</p>\n<p>b</p>")).toBe("<p>a</p><p>b</p>");
    expect(collapseCommentSourceWhitespace("")).toBe("");
  });
});
