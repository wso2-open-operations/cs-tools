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
import { flattenTablesToParagraphs, imagesToLinks, isAppImageSrc, markdownToPasteDom, mergeAdjacentParagraphs } from "./markdownPaste";

const parse = (h: string) => new DOMParser().parseFromString(h, "text/html");

const html = (dom: Document) => dom.body.innerHTML;

describe("markdownToPasteDom", () => {
  it("unwraps the table wrapper", () => {
    const dom = markdownToPasteDom("| a | b |\n|---|---|\n| 1 | 2 |");
    expect(dom.querySelector(".md-table-wrap")).toBeNull();
    expect(dom.body.firstElementChild?.tagName).toBe("TABLE");
  });

  it("turns an image into a link with its alt text, or its address when there is no alt", () => {
    expect(html(markdownToPasteDom("![shot](https://example.com/a.png)"))).toContain(
      '<a href="https://example.com/a.png">shot</a>',
    );
    expect(html(markdownToPasteDom("![](https://example.com/b.png)"))).toContain(
      '<a href="https://example.com/b.png">https://example.com/b.png</a>',
    );
  });

  it("never keeps an image that is not a safe link", () => {
    const out = html(markdownToPasteDom("![alt](javascript:alert(1))"));
    expect(out).not.toContain("<img");
    // markdown-it refuses the address, so it stays literal text, never a link.
    expect(out).not.toContain("<a");
  });

  it("escapes raw HTML in the source", () => {
    const out = html(markdownToPasteDom("**x** <script>alert(1)</script> <img src=x onerror=alert(1)>"));
    expect(out).not.toContain("<script");
    expect(out).not.toContain("<img");
  });

  it("an image inside a link becomes one link reading the alt text", () => {
    const out = html(markdownToPasteDom("[![badge](https://i.com/a.png)](https://x.com)"));
    expect(out).toContain('<a href="https://x.com">badge</a>');
    expect(out).not.toContain("<img");
    expect((out.match(/<a /g) ?? []).length).toBe(1);
  });

  it("removes the trailing newline markdown-it puts at the end of a code block", () => {
    const out = html(markdownToPasteDom("```js\nconst a = 1;\nconst b = 2;\n```"));
    expect(out).not.toMatch(/\n<\/code>|\n<\/pre>/);
    expect(out).toContain("const b = 2;");
  });

  it("drops a horizontal rule", () => {
    expect(html(markdownToPasteDom("a\n\n---\n\nb"))).not.toContain("<hr");
  });
});

describe("mergeAdjacentParagraphs", () => {
  const parse = (h: string) => new DOMParser().parseFromString(h, "text/html");

  it("joins a run of paragraphs with <br><br>", () => {
    const dom = parse("<p>a</p><p>b</p><p>c</p>");
    mergeAdjacentParagraphs(dom);
    expect(html(dom)).toBe("<p>a<br><br>b<br><br>c</p>");
  });

  it("does not merge across other blocks", () => {
    const dom = parse("<p>a</p><ul><li>x</li></ul><p>b</p><p>c</p>");
    mergeAdjacentParagraphs(dom);
    expect(html(dom)).toBe("<p>a</p><ul><li>x</li></ul><p>b<br><br>c</p>");
  });
});

describe("imagesToLinks (Keep Formatting HTML)", () => {
  it("links a remote image by its alt, or its address without one", () => {
    const dom = parse('<p><img src="https://x.com/a.png" alt="a"> <img src="https://x.com/b.png"></p>');
    imagesToLinks(dom);
    expect(html(dom)).toBe('<p><a href="https://x.com/a.png">a</a> <a href="https://x.com/b.png">https://x.com/b.png</a></p>');
  });

  it("shows only the alt for an image inside a link, a data: source and an unsafe source", () => {
    const dom = parse(
      '<p><a href="https://x.com"><img src="https://i.com/b.png" alt="badge"></a><img src="data:image/png;base64,AA" alt="d"><img src="javascript:1" alt="j"></p>',
    );
    imagesToLinks(dom);
    expect(html(dom)).toBe('<p><a href="https://x.com">badge</a>dj</p>');
  });

  it("removes every image when the clipboard also carries the picture file", () => {
    const dom = parse('<p>a<img src="https://x.com/a.png" alt="a">b</p>');
    imagesToLinks(dom, { dropAll: true });
    expect(html(dom)).toBe("<p>ab</p>");
  });
});

describe("flattenTablesToParagraphs (a paste into a table cell)", () => {
  it("makes one paragraph per row with the cells joined by ' | ', skipping empty rows", () => {
    const dom = parse(
      "<p>before</p><table><thead><tr><th>H1</th><th>H2</th></tr></thead><tbody><tr><td> 1 </td><td>2\n3</td></tr><tr><td></td><td></td></tr></tbody></table>",
    );
    flattenTablesToParagraphs(dom);
    expect(html(dom)).toBe("<p>before</p><p>H1 | H2</p><p>1 | 2 3</p>");
  });

  it("keeps the words of a cell apart: <br>, paragraphs and list items inside a cell are spaces, not glued", () => {
    const dom = parse(
      "<table><tr><td>first<br>second</td><td><p>one</p><p>two</p></td><td><ul><li>x</li><li>y</li></ul></td><td>a<div>b</div></td></tr></table>",
    );
    flattenTablesToParagraphs(dom);
    expect(html(dom)).toBe("<p>first second | one two | x y | a b</p>");
  });

  it("flattens a nested table into its outer row's text", () => {
    const dom = parse("<table><tr><td>a</td><td><table><tr><td>b</td></tr></table></td></tr></table>");
    flattenTablesToParagraphs(dom);
    expect(dom.querySelector("table")).toBeNull();
    expect(dom.body.textContent).toContain("a | b");
  });
});

describe("mergeAdjacentParagraphs only merges plain, non-empty paragraphs", () => {
  const merged = (h: string) => {
    const dom = parse(h);
    mergeAdjacentParagraphs(dom);
    return html(dom);
  };

  it("merges plain neighbours", () => {
    expect(merged("<p>a</p><p>b</p><p>c</p>")).toBe("<p>a<br><br>b<br><br>c</p>");
  });

  it("leaves a paragraph with style, class or align (and Word Mso classes) as its own block", () => {
    expect(merged('<p>a</p><p style="text-align:center">b</p><p>c</p>')).toBe('<p>a</p><p style="text-align:center">b</p><p>c</p>');
    expect(merged('<p class="MsoNormal">a</p><p class="MsoNormal">b</p>')).toBe('<p class="MsoNormal">a</p><p class="MsoNormal">b</p>');
    expect(merged('<p align="right">a</p><p>b</p>')).toBe('<p align="right">a</p><p>b</p>');
  });

  it("keeps an empty paragraph as its own block", () => {
    expect(merged("<p>a</p><p><br></p><p>b</p>")).toBe("<p>a</p><p><br></p><p>b</p>");
  });

  it("does not merge across text that sits between the paragraphs (order is kept)", () => {
    expect(merged("<p>A</p>middle<p>B</p>")).toBe("<p>A</p>middle<p>B</p>");
    expect(merged("<p>A</p>middle<p>B</p><p>C</p>")).toBe("<p>A</p>middle<p>B<br><br>C</p>");
    expect(merged("<p>A</p><p>B</p>tail<p>C</p>")).toBe("<p>A<br><br>B</p>tail<p>C</p>");
  });

  it("merges across whitespace-only text and comments", () => {
    expect(merged("<p>A</p>\n  <p>B</p>")).toBe("<p>A<br><br>B</p>");
    expect(merged("<p>A</p><!-- x --> <p>B</p>")).toBe("<p>A<br><br>B</p>");
  });
});

describe("isAppImageSrc", () => {
  it("accepts blob: previews and stored /<uuid> images, and data: images only from an in-app clipboard", () => {
    expect(isAppImageSrc("blob:https://csm.example.com/abc", false)).toBe(true);
    expect(isAppImageSrc("/0b9f3c1e-aaaa-4bbb-8ccc-1234567890ab", false)).toBe(true);
    expect(isAppImageSrc("data:image/png;base64,AAAA", false)).toBe(false);
    expect(isAppImageSrc("data:image/png;base64,AAAA", true)).toBe(true);
  });

  it("rejects remote, relative and scripted sources", () => {
    for (const src of ["https://x.com/a.png", "/img/a.png", "//evil.com/0b9f3c1e-aaaa-4bbb-8ccc-1234567890ab", "javascript:1", "blob:javascript:1", ""]) {
      expect(isAppImageSrc(src, true), src).toBe(false);
    }
  });
});
