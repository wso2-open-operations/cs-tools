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
  tokenizePlainTextPaste,
  unwrapNestedPreCodeElements,
  collapseEmptyParagraphElements,
  convertWordListsToHtmlLists,
  hasMeaningfulHtmlContent,
  htmlHasTextOrTable,
  htmlToPastePlainText,
  preserveEditorWhitespace,
} from "./richTextEditor";

describe("tokenizePlainTextPaste", () => {
  it("does not emit a standalone empty paragraph between blank-line-separated paragraphs", () => {
    const tokens = tokenizePlainTextPaste("Para1\n\nPara2\n\nPara3");

    expect(tokens).toEqual([
      { type: "text", value: "Para1" },
      { type: "paragraph" },
      { type: "text", value: "Para2" },
      { type: "paragraph" },
      { type: "text", value: "Para3" },
    ]);
  });

  it("collapses multiple consecutive blank lines into a single paragraph break", () => {
    const tokens = tokenizePlainTextPaste("Para1\n\n\n\nPara2");

    expect(tokens).toEqual([
      { type: "text", value: "Para1" },
      { type: "paragraph" },
      { type: "text", value: "Para2" },
    ]);
  });

  it("still starts a new paragraph for a single, non-blank-separated newline", () => {
    const tokens = tokenizePlainTextPaste("Line1\nLine2");

    expect(tokens).toEqual([
      { type: "text", value: "Line1" },
      { type: "paragraph" },
      { type: "text", value: "Line2" },
    ]);
  });

  it("inserts single-line text inline, without a leading/trailing paragraph break", () => {
    const tokens = tokenizePlainTextPaste("Hello world");

    expect(tokens).toEqual([{ type: "text", value: "Hello world" }]);
  });

  it("handles tabs within a line", () => {
    const tokens = tokenizePlainTextPaste("a\tb");

    expect(tokens).toEqual([
      { type: "text", value: "a" },
      { type: "tab" },
      { type: "text", value: "b" },
    ]);
  });

  it("collapses a leading blank line to a single paragraph break", () => {
    const tokens = tokenizePlainTextPaste("\n\nPara1");

    expect(tokens).toEqual([
      { type: "paragraph" },
      { type: "text", value: "Para1" },
    ]);
  });

  it("normalizes CRLF the same way as LF", () => {
    const tokens = tokenizePlainTextPaste("Para1\r\n\r\nPara2");

    expect(tokens).toEqual([
      { type: "text", value: "Para1" },
      { type: "paragraph" },
      { type: "text", value: "Para2" },
    ]);
  });
});

describe("tokenizePlainTextPaste with lineBreaks (enterInsertsLineBreak editors)", () => {
  const lb = (text: string) => tokenizePlainTextPaste(text, { lineBreaks: true });
  const text = (value: string) => ({ type: "text", value });
  const br = { type: "linebreak" };

  it("turns a single newline into one line break", () => {
    expect(lb("Line A\nLine B")).toEqual([text("Line A"), br, text("Line B")]);
  });

  it("keeps a blank line as one blank line (two line breaks)", () => {
    expect(lb("Hi team,\n\nLine A\nLine B")).toEqual([
      text("Hi team,"),
      br,
      br,
      text("Line A"),
      br,
      text("Line B"),
    ]);
  });

  it("collapses three or more newlines to one blank line", () => {
    expect(lb("a\n\n\n\nb")).toEqual([text("a"), br, br, text("b")]);
  });

  it("normalizes CRLF like LF", () => {
    expect(lb("a\r\n\r\nb")).toEqual([text("a"), br, br, text("b")]);
  });

  it("leaves tabs unchanged", () => {
    expect(lb("a\tb")).toEqual([text("a"), { type: "tab" }, text("b")]);
  });

  it("never emits a paragraph token", () => {
    expect(lb("a\n\nb\nc").some((t) => t.type === "paragraph")).toBe(false);
  });

  it("the default mode is unchanged (paragraphs, blank lines dropped)", () => {
    expect(tokenizePlainTextPaste("a\n\nb")).toEqual([
      text("a"),
      { type: "paragraph" },
      text("b"),
    ]);
  });
});

describe("unwrapNestedPreCodeElements", () => {
  const parse = (html: string) =>
    new DOMParser().parseFromString(html, "text/html");

  it("flattens a <pre><code> pair into a single-level <pre> and reports a change", () => {
    const dom = parse(
      '<pre><code class="language-javascript">line1\nline2</code></pre>',
    );

    const changed = unwrapNestedPreCodeElements(dom);

    expect(changed).toBe(true);
    const pre = dom.querySelector("pre");
    expect(pre).not.toBeNull();
    expect(pre?.querySelector("code")).toBeNull();
    expect(pre?.textContent).toBe("line1\nline2");
  });

  it("carries the language hint from <code class=\"language-xxx\"> onto <pre data-language>", () => {
    const dom = parse(
      '<pre><code class="language-python">print(1)</code></pre>',
    );

    unwrapNestedPreCodeElements(dom);

    expect(dom.querySelector("pre")?.getAttribute("data-language")).toBe(
      "python",
    );
  });

  it("does not overwrite an existing data-language on <pre>", () => {
    const dom = parse(
      '<pre data-language="go"><code class="language-python">print(1)</code></pre>',
    );

    unwrapNestedPreCodeElements(dom);

    expect(dom.querySelector("pre")?.getAttribute("data-language")).toBe(
      "go",
    );
  });

  it("leaves a standalone <pre> without a nested <code> untouched", () => {
    const dom = parse("<pre>line1\nline2</pre>");

    const changed = unwrapNestedPreCodeElements(dom);

    expect(changed).toBe(false);
    expect(dom.querySelector("pre")?.textContent).toBe("line1\nline2");
  });

  it("leaves a standalone multi-line <code> not inside a <pre> untouched", () => {
    const dom = parse('<p><code class="language-js">a\nb</code></p>');

    const changed = unwrapNestedPreCodeElements(dom);

    expect(changed).toBe(false);
    expect(dom.querySelector("code")).not.toBeNull();
  });
});

describe("collapseEmptyParagraphElements", () => {
  const parse = (html: string) =>
    new DOMParser().parseFromString(html, "text/html");

  it("removes an empty <p>&nbsp;</p> sitting between two real paragraphs", () => {
    const dom = parse("<p>Para1</p><p>&nbsp;</p><p>Para2</p>");

    const changed = collapseEmptyParagraphElements(dom);

    expect(changed).toBe(true);
    expect(
      Array.from(dom.querySelectorAll("p")).map((p) => p.textContent),
    ).toEqual(["Para1", "Para2"]);
  });

  it("removes an empty <p><br></p> sitting between two real paragraphs", () => {
    const dom = parse("<p>Para1</p><p><br></p><p>Para2</p>");

    const changed = collapseEmptyParagraphElements(dom);

    expect(changed).toBe(true);
    expect(dom.querySelectorAll("p").length).toBe(2);
    expect(dom.body.textContent).toBe("Para1Para2");
  });

  it("removes an empty <div><br></div> sitting between two real paragraphs", () => {
    const dom = parse("<p>Para1</p><div><br></div><p>Para2</p>");

    const changed = collapseEmptyParagraphElements(dom);

    expect(changed).toBe(true);
    expect(dom.querySelector("div")).toBeNull();
    expect(dom.body.textContent).toBe("Para1Para2");
  });

  it("collapses multiple consecutive empty blocks between two real paragraphs", () => {
    const dom = parse(
      "<p>Para1</p><p><br></p><p>&nbsp;</p><div><br></div><p>Para2</p>",
    );

    const changed = collapseEmptyParagraphElements(dom);

    expect(changed).toBe(true);
    expect(
      Array.from(dom.querySelectorAll("p")).map((p) => p.textContent),
    ).toEqual(["Para1", "Para2"]);
  });

  it("does not remove a paragraph or div that carries real content", () => {
    const dom = parse(
      '<p>Para1</p><div><img src="x.png" alt="x" /></div><p>Para2</p>',
    );

    const changed = collapseEmptyParagraphElements(dom);

    expect(changed).toBe(false);
    expect(dom.querySelector("img")).not.toBeNull();
  });

  it("returns false and leaves the DOM untouched when there are no empty blocks", () => {
    const dom = parse("<p>Para1</p><p>Para2</p>");

    const changed = collapseEmptyParagraphElements(dom);

    expect(changed).toBe(false);
    expect(dom.querySelectorAll("p").length).toBe(2);
  });

  it("collapses a nested empty wrapper (<div><div><br></div></div>) entirely", () => {
    const dom = parse("<p>Para1</p><div><div><br></div></div><p>Para2</p>");

    const changed = collapseEmptyParagraphElements(dom);

    expect(changed).toBe(true);
    expect(dom.querySelector("div")).toBeNull();
    expect(dom.body.textContent).toBe("Para1Para2");
  });

  it("collapses a three-level nested empty wrapper", () => {
    const dom = parse(
      "<p>Para1</p><div><div><div><br></div></div></div><p>Para2</p>",
    );

    const changed = collapseEmptyParagraphElements(dom);

    expect(changed).toBe(true);
    expect(dom.querySelector("div")).toBeNull();
    expect(dom.body.textContent).toBe("Para1Para2");
  });
});

describe("collapseEmptyParagraphElements: a bare <br> between two blocks", () => {
  const run = (html: string): string => {
    const dom = new DOMParser().parseFromString(html, "text/html");
    collapseEmptyParagraphElements(dom);
    return dom.body.innerHTML;
  };

  it("drops Google Docs' <br> between a list and a paragraph, inside its <b> wrapper", () => {
    expect(run('<b id="docs-internal-guid-1"><ul><li>One</li></ul><br><p>After</p></b>')).toBe(
      '<b id="docs-internal-guid-1"><ul><li>One</li></ul><p>After</p></b>',
    );
  });

  it("drops it at the top level too, whitespace and comments between do not matter", () => {
    expect(run("<ul><li>One</li></ul>\n<br>\n<!--x--><p>After</p>")).toBe("<ul><li>One</li></ul>\n\n<!--x--><p>After</p>");
  });

  it("keeps a <br> inside a paragraph or a list item, and one with text beside it", () => {
    expect(run("<p>a<br>b</p>")).toBe("<p>a<br>b</p>");
    expect(run("<ul><li>a<br>b</li></ul>")).toBe("<ul><li>a<br>b</li></ul>");
    expect(run("<p>a</p>text<br><p>b</p>")).toBe("<p>a</p>text<br><p>b</p>");
    expect(run("<p>a</p><br>")).toBe("<p>a</p><br>");
  });
});

describe("htmlToPastePlainText", () => {
  it("puts a newline between blocks and for <br>, one list item per line", () => {
    expect(htmlToPastePlainText("<p>one</p><p>two</p>")).toBe("one\ntwo");
    expect(htmlToPastePlainText("<p>a<br>b</p>")).toBe("a\nb");
    expect(htmlToPastePlainText("<ul><li>x</li><li><p>y</p></li></ul><p>z</p>")).toBe("x\ny\nz");
  });

  it("separates table cells with a tab and rows with a newline, and collapses source layout whitespace", () => {
    expect(htmlToPastePlainText("<table>\n <tr>\n <td>A</td>\n <td>B</td></tr>\n<tr><td>C</td><td>D</td></tr></table>")).toBe(
      "A\tB\nC\tD",
    );
    expect(htmlToPastePlainText("<p>hard\n   wrapped   text</p>")).toBe("hard wrapped text");
  });

  it("keeps the tab of an empty cell: leading, middle and trailing, in every row", () => {
    const t = (cells: string) => htmlToPastePlainText(`<table><tr>${cells}</tr></table>`);
    expect(t("<td></td><td>B</td>")).toBe("\tB");
    expect(t("<td>A</td><td></td><td>C</td>")).toBe("A\t\tC");
    expect(t("<td>A</td><td></td>")).toBe("A\t");
    expect(t("<td></td><td></td><td>C</td>")).toBe("\t\tC");
    expect(
      htmlToPastePlainText("<table><tr><td>A</td><td></td></tr><tr><td></td><td>D</td></tr></table>"),
    ).toBe("A\t\n\tD");
  });

  it("keeps a <pre>'s own newlines and ignores <style>", () => {
    expect(htmlToPastePlainText("<style>p{}</style><pre>a\n  b</pre>")).toBe("a\n  b");
  });

  it("is empty for empty input", () => {
    expect(htmlToPastePlainText("")).toBe("");
  });
});

describe("hasMeaningfulHtmlContent / htmlHasTextOrTable", () => {
  it("a lone <meta>, comments and blank blocks are no content", () => {
    expect(hasMeaningfulHtmlContent('<meta charset="utf-8">')).toBe(false);
    expect(hasMeaningfulHtmlContent("<!--StartFragment--><p><br></p>&nbsp;<!--EndFragment-->")).toBe(false);
    expect(hasMeaningfulHtmlContent("")).toBe(false);
  });

  it("text, a picture, a table or a rule is content", () => {
    expect(hasMeaningfulHtmlContent("<p>x</p>")).toBe(true);
    expect(hasMeaningfulHtmlContent('<img src="https://x/y.png">')).toBe(true);
    expect(hasMeaningfulHtmlContent("<table><tr><td></td></tr></table>")).toBe(true);
    expect(hasMeaningfulHtmlContent("<hr>")).toBe(true);
  });

  it("htmlHasTextOrTable ignores a picture's own markup", () => {
    expect(htmlHasTextOrTable('<img src="https://x/y.png">')).toBe(false);
    expect(htmlHasTextOrTable("<table><tr><td></td></tr></table>")).toBe(true);
    expect(htmlHasTextOrTable("<p>caption</p>")).toBe(true);
  });
});

describe("convertWordListsToHtmlLists", () => {
  const run = (html: string): string => {
    const dom = new DOMParser().parseFromString(html, "text/html");
    convertWordListsToHtmlLists(dom);
    return dom.body.innerHTML;
  };
  const bullet = (level: number, glyph: string, text: string, kind = "MsoListParagraph"): string =>
    `<p class=${kind} style="margin-left:.5in;text-indent:-.25in;mso-list:l0 level${level} lfo1"><![if !supportLists]><span style="font-family:Symbol;mso-list:Ignore">${glyph}<span style="font:7.0pt 'Times New Roman'">&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp; </span></span><![endif]>${text}<o:p></o:p></p>`;

  it("turns a two-level bulleted list into nested <ul>s and removes the glyphs", () => {
    const out = run(
      "<p class=MsoNormal>Before<o:p></o:p></p>" +
        bullet(1, "·", "First", "MsoListParagraphCxSpFirst") +
        bullet(2, "o", "Second", "MsoListParagraphCxSpMiddle") +
        bullet(1, "·", "Third", "MsoListParagraphCxSpLast") +
        "<p class=MsoNormal>After<o:p></o:p></p>",
    );
    expect(out).toBe(
      "<p class=\"MsoNormal\">Before<o:p></o:p></p><ul><li>First<ul><li>Second</li></ul></li><li>Third</li></ul><p class=\"MsoNormal\">After<o:p></o:p></p>",
    );
  });

  it("decides ordered from the glyph and starts at the first number; a change of kind starts a new list", () => {
    const out = run(bullet(1, "3.", "Three") + bullet(1, "4.", "Four") + bullet(2, "a)", "sub") + bullet(1, "§", "Square"));
    expect(out).toBe("<ol start=\"3\"><li>Three</li><li>Four<ol><li>sub</li></ol></li></ol><ul><li>Square</li></ul>");
  });

  it("reads the glyph from <!--[if !supportLists]--> comments as well", () => {
    const html =
      '<p class=MsoListParagraph style="mso-list:l1 level1 lfo2"><!--[if !supportLists]--><span>1.<span>&nbsp; </span></span><!--[endif]-->One</p>' +
      '<p class=MsoListParagraph style="mso-list:l1 level1 lfo2"><!--[if !supportLists]--><span>2.<span>&nbsp; </span></span><!--[endif]-->Two</p>';
    expect(run(html)).toBe("<ol><li>One</li><li>Two</li></ol>");
  });

  it("recognises -, • and roman numerals; leaves ordinary paragraphs alone", () => {
    expect(run(bullet(1, "-", "dash") + bullet(1, "•", "dot"))).toBe("<ul><li>dash</li><li>dot</li></ul>");
    expect(run(bullet(1, "iv.", "four"))).toBe("<ol><li>four</li></ol>");
    expect(run("<p class=MsoNormal>plain</p>")).toBe('<p class="MsoNormal">plain</p>');
  });

  it("starts a new list when the Word list id changes at the same level", () => {
    const a = bullet(1, "·", "A");
    const b = bullet(1, "·", "B").replace("l0 level1", "l1 level1");
    expect(run(a + b)).toBe("<ul><li>A</li></ul><ul><li>B</li></ul>");
  });
});

describe("convertWordListsToHtmlLists: outlines and class-only paragraphs", () => {
  const run = (html: string): string => {
    const dom = new DOMParser().parseFromString(html, "text/html");
    convertWordListsToHtmlLists(dom);
    return dom.body.innerHTML;
  };
  const item = (level: number, glyph: string, text: string, list = "l0"): string =>
    `<p class=MsoListParagraph style="mso-list:${list} level${level} lfo1"><![if !supportLists]><span style="mso-list:Ignore">${glyph}<span>&nbsp;&nbsp;</span></span><![endif]>${text}</p>`;

  it("keeps a 1. / 1.1. / 1.2. / 2. legal outline numbered at every level", () => {
    const out = run(item(1, "1.", "A") + item(2, "1.1.", "A1") + item(2, "1.2.", "A2") + item(1, "2.", "B") + item(3, "2.1.1", "B11"));
    expect(out).toBe("<ol><li>A<ol><li>A1</li><li>A2</li></ol></li><li>B<ol><li>B11</li></ol></li></ol>");
  });

  it("recognises A.1. and (a) / dotted glyphs as ordered", () => {
    expect(run(item(1, "A.1.", "x"))).toBe("<ol><li>x</li></ol>");
    expect(run(item(1, "(a)", "x"))).toBe("<ol><li>x</li></ol>");
    expect(run(item(1, "2.3.1", "x"))).toBe("<ol><li>x</li></ol>");
  });

  it("a child with an ambiguous glyph stays ordered under an ordered parent, but a real bullet stays a bullet", () => {
    expect(run(item(1, "1.", "A") + item(2, "", "child"))).toBe("<ol><li>A<ol><li>child</li></ol></li></ol>");
    expect(run(item(1, "1.", "A") + item(2, "o", "child"))).toBe("<ol><li>A<ul><li>child</li></ul></li></ol>");
    expect(run(item(1, "·", "A") + item(2, "", "child"))).toBe("<ul><li>A<ul><li>child</li></ul></li></ul>");
  });

  it("leaves plain text in the List Paragraph style alone", () => {
    const html = "<p class=MsoListParagraph>Just a paragraph</p><p class=MsoListParagraph>Another</p>";
    expect(run(html)).toBe('<p class="MsoListParagraph">Just a paragraph</p><p class="MsoListParagraph">Another</p>');
  });

  it("a class-only paragraph with a leading glyph becomes an item and the glyph is stripped, never doubled", () => {
    expect(run("<p class=MsoListParagraph>·&nbsp;&nbsp;First</p><p class=MsoListParagraph>·&nbsp;&nbsp;Second</p>")).toBe(
      "<ul><li>First</li><li>Second</li></ul>",
    );
    expect(run("<p class=MsoListParagraph>1.  One</p><p class=MsoListParagraph>2.  Two</p>")).toBe("<ol><li>One</li><li>Two</li></ol>");
  });

  it("a class-only paragraph with an Ignore span or the supportLists comment is an item", () => {
    expect(run('<p class=MsoListParagraph><span style="mso-list:Ignore">·</span>x</p>')).toBe("<ul><li>x</li></ul>");
    expect(run("<p class=MsoListParagraph><!--[if !supportLists]-->1.<!--[endif]-->x</p>")).toBe("<ol><li>x</li></ol>");
  });

  it("handles a long run of list paragraphs", () => {
    const html = Array.from({ length: 3000 }, (_, i) => item(1, "·", `n${i}`)).join("");
    const started = Date.now();
    expect(run(html).match(/<li>/g)).toHaveLength(3000);
    expect(Date.now() - started).toBeLessThan(5000);
  });
});

describe("preserveEditorWhitespace", () => {
  const run = (html: string): string => {
    const dom = new DOMParser().parseFromString(html, "text/html");
    preserveEditorWhitespace(dom);
    return dom.body.innerHTML;
  };

  it("wraps text with a tab, a run of spaces or edge spaces in a pre-wrap span", () => {
    expect(run("<p>a\tb</p>")).toBe('<p><span style="white-space: pre-wrap;">a\tb</span></p>');
    expect(run("<p>    x</p>")).toContain("pre-wrap");
    expect(run("<p>a  b</p>")).toContain("pre-wrap");
  });

  it("leaves laid-out HTML source (newlines) and single separating spaces to collapse as before", () => {
    const laidOut = "<ul>\n  <li>\n    a  b\n  </li>\n</ul>\n<p>\n  text\n</p>";
    expect(run(laidOut)).toBe(laidOut);
    expect(run("<p>a <b>b</b> c</p>")).not.toContain("pre-wrap");
    expect(run("<ul><li>a</li> <li>b</li></ul>")).not.toContain("pre-wrap");
  });

  it("does nothing inside <pre>", () => {
    expect(run("<pre>a\tb  c</pre>")).toBe("<pre>a\tb  c</pre>");
  });

  it("does not wrap whitespace-only gaps next to or between block elements", () => {
    for (const html of [
      "<div>  <p>a</p></div>",
      "<ul><li>\t<ul><li>a</li></ul></li></ul>",
      "<blockquote>  <p>q</p></blockquote>",
      "<table><tbody><tr><td>  <p>c</p></td></tr></tbody></table>",
      "<div><p>a</p>   <p>b</p>  </div>",
    ]) {
      expect(run(html), html).not.toContain("pre-wrap");
    }
  });

  it("still wraps typed whitespace between inline content", () => {
    expect(run("<p><b>a</b>  <i>b</i></p>")).toContain("pre-wrap");
  });
});
