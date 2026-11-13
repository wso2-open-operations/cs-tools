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
import { markdownToPasteDom, mergeAdjacentParagraphs } from "./markdownPaste";

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
