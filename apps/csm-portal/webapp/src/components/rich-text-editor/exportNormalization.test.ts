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
  stripWhitespaceStyleAndUnwrapSpans,
  stripWhitespaceStyleFromHtml,
} from "./richTextEditor";

describe("stripWhitespaceStyleAndUnwrapSpans", () => {
  const parse = (html: string) => new DOMParser().parseFromString(html, "text/html");

  it("unwraps a bare span with only the whitespace style to a plain text node", () => {
    const dom = parse('<p><span style="white-space: pre-wrap;">plain text</span></p>');

    const changed = stripWhitespaceStyleAndUnwrapSpans(dom);

    expect(changed).toBe(true);
    expect(dom.querySelector("span")).toBeNull();
    expect(dom.querySelector("p")?.textContent).toBe("plain text");
    expect(dom.querySelector("p")?.innerHTML).toBe("plain text");
  });

  it("removes only the white-space style from a formatted element, keeping its class", () => {
    const dom = parse(
      '<p><strong class="editor-text-bold" style="white-space: pre-wrap;">BOLD</strong></p>',
    );

    const changed = stripWhitespaceStyleAndUnwrapSpans(dom);

    expect(changed).toBe(true);
    const strong = dom.querySelector("strong");
    expect(strong).not.toBeNull();
    expect(strong?.getAttribute("style")).toBeNull();
    expect(strong?.className).toBe("editor-text-bold");
    expect(strong?.textContent).toBe("BOLD");
  });

  it("preserves other style properties (e.g. text-transform) while removing only white-space", () => {
    const dom = parse(
      '<p><span style="white-space: pre-wrap; text-transform: uppercase;">shout</span></p>',
    );

    const changed = stripWhitespaceStyleAndUnwrapSpans(dom);

    expect(changed).toBe(true);
    const span = dom.querySelector("span");
    expect(span).not.toBeNull(); // not unwrapped -- still has a style attribute
    expect(span?.style.whiteSpace).toBe("");
    expect(span?.style.textTransform).toBe("uppercase");
  });

  it("does not unwrap a span that has other attributes besides style", () => {
    const dom = parse(
      '<p><span style="white-space: pre-wrap;" data-foo="bar">text</span></p>',
    );

    const changed = stripWhitespaceStyleAndUnwrapSpans(dom);

    expect(changed).toBe(true);
    const span = dom.querySelector("span");
    expect(span).not.toBeNull();
    expect(span?.getAttribute("style")).toBeNull();
    expect(span?.getAttribute("data-foo")).toBe("bar");
  });

  it("does not touch an element with no white-space style", () => {
    const dom = parse('<p><a href="https://example.com" style="color: red;">link</a></p>');

    const changed = stripWhitespaceStyleAndUnwrapSpans(dom);

    expect(changed).toBe(false);
    expect(dom.querySelector("a")?.getAttribute("style")).toBe("color: red;");
  });

  it("handles multiple independent runs in the same document", () => {
    const dom = parse(
      '<p><span style="white-space: pre-wrap;">A</span> plain <span style="white-space: pre-wrap;">B</span></p>',
    );

    const changed = stripWhitespaceStyleAndUnwrapSpans(dom);

    expect(changed).toBe(true);
    expect(dom.querySelectorAll("span").length).toBe(0);
    expect(dom.querySelector("p")?.textContent).toBe("A plain B");
  });

  it("returns false on a document with no white-space styles", () => {
    const dom = parse("<p>plain text, no styling at all</p>");

    const changed = stripWhitespaceStyleAndUnwrapSpans(dom);

    expect(changed).toBe(false);
  });
});

describe("stripWhitespaceStyleFromHtml", () => {
  it("strips the style and unwraps the span, returning a serialized string", () => {
    const html = '<p><span style="white-space: pre-wrap;">hello</span></p>';

    const out = stripWhitespaceStyleFromHtml(html);

    expect(out).toBe("<p>hello</p>");
  });

  it("preserves a double-space run and leading/trailing spaces within the text itself", () => {
    const html = '<p><span style="white-space: pre-wrap;"> hello  world </span></p>';

    const out = stripWhitespaceStyleFromHtml(html);

    // The style is gone, but the actual characters in the text node are untouched --
    // the container-level CSS is what re-establishes the rendering guarantee.
    expect(out).toBe("<p> hello  world </p>");
  });

  it("returns the exact same string reference when there is no white-space style (fast-path bail)", () => {
    const html = "<p>plain text</p>";

    const out = stripWhitespaceStyleFromHtml(html);

    expect(out).toBe(html);
  });

  it("performs well on a large document with many text runs", () => {
    const RUNS = 5000;
    const parts: string[] = [];
    for (let i = 0; i < RUNS; i++) {
      parts.push(
        `<p><span style="white-space: pre-wrap;">Paragraph ${i} plain text run. </span>`,
        `<strong class="editor-text-bold" style="white-space: pre-wrap;">bold ${i}</strong>`,
        `<span style="white-space: pre-wrap;"> more padding text to make this realistically large. </span></p>`,
      );
    }
    const html = parts.join("");
    expect(html.length).toBeGreaterThan(1_000_000);

    const start = performance.now();
    const out = stripWhitespaceStyleFromHtml(html);
    const elapsedMs = performance.now() - start;

    expect(elapsedMs).toBeLessThan(2000);
    expect(out).not.toContain("white-space");
    expect((out.match(/<span/g) ?? []).length).toBe(0);
    expect((out.match(/<strong/g) ?? []).length).toBe(RUNS);
  });
});
