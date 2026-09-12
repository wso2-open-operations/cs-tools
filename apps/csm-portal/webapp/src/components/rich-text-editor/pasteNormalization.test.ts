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
