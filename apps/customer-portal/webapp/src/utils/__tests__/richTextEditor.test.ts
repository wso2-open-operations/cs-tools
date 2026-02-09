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
  getBlockDisplay,
  markdownToHtml,
  htmlToMarkdown,
  createCodeBlockHtml,
  toggleList,
} from "@utils/richTextEditor";

describe("richTextEditor utils", () => {
  describe("getBlockDisplay", () => {
    it("should return correct display info for heading tags", () => {
      expect(getBlockDisplay("h1")).toMatchObject({
        label: "Heading 1",
        variant: "h1",
      });
      expect(getBlockDisplay("H1")).toMatchObject({
        label: "Heading 1",
        variant: "h1",
      });
    });

    it("should return default for unknown tags", () => {
      expect(getBlockDisplay("unknown")).toEqual({
        label: "Body 2",
        variant: "body2",
      });
    });
  });

  describe("markdownToHtml", () => {
    it("should convert simple markdown to html", () => {
      const md = "## Title\n\nThis is **bold** and *italic*.\n\n- List item";
      const html = markdownToHtml(md);
      expect(html).toContain("<h2>Title</h2>");
      expect(html).toContain("<strong>bold</strong>");
      expect(html).toContain("<em>italic</em>");
    });

    it("should handle links", () => {
      const md = "[WSO2](https://wso2.com)";
      const html = markdownToHtml(md);
      expect(html).toContain('<a href="https://wso2.com"');
    });

    it("should sanitize dangerous link protocols to prevent XSS", () => {
      const protocols = [
        "javascript:alert(1)",
        "data:text/html,<script>alert(1)</script>",
        "vbscript:msgbox('hi')",
        "JAVASCRIPT:alert(1)",
      ];

      protocols.forEach((uri) => {
        const md = `[click me](${uri})`;
        const html = markdownToHtml(md);
        expect(html).toContain('<a href=""');
        expect(html).not.toContain(uri);
      });
    });

    it("should handle unordered lists", () => {
      const md = "- Item 1\n- Item 2";
      const html = markdownToHtml(md);
      expect(html).toContain("<ul><li>Item 1</li><li>Item 2</li></ul>");
    });

    it("should handle ordered lists", () => {
      const md = "1. First\n2. Second";
      const html = markdownToHtml(md);
      expect(html).toContain("<ol><li>First</li><li>Second</li></ol>");
    });

    it("should handle code blocks with blank lines", () => {
      const md = "```javascript\nfunction test() {\n\n  return true;\n}\n```";
      const html = markdownToHtml(md);
      expect(html).toContain(
        "<pre><code>function test() {\n\n  return true;\n}\n</code></pre>",
      );
      expect(html).not.toContain("<p>function test()");
    });

    it("should not italicize underscores in identifiers or URLs", () => {
      const md =
        "This is a `my_variable_name` and a path like /var/local_storage/data. But _this_ should be italic.";
      const html = markdownToHtml(md);

      expect(html).toContain("my_variable_name");
      expect(html).not.toContain("my<em>variable</em>name");
      expect(html).toContain("/var/local_storage/data");
      expect(html).not.toContain("/var/local<em>storage</em>data");
      expect(html).toContain("<em>this</em>");
    });

    it("should neutralize malicious HTML in markdown to prevent XSS", () => {
      const md =
        "This is a list:\n- <img src=x onerror=alert(1)>\n- Normal item";
      const html = markdownToHtml(md);
      expect(html).toContain("&lt;img src=x onerror=alert(1)&gt;");
      expect(html).not.toMatch(/<img[^>]*onerror/);
    });

    it("should protect markdown markup inside inline code spans", () => {
      const md = "This is some `**not bold**` and `_not italic_` code.";
      const html = markdownToHtml(md);

      expect(html).toContain("<code>**not bold**</code>");
      expect(html).not.toContain("<strong>not bold</strong>");
      expect(html).toContain("<code>_not italic_</code>");
      expect(html).not.toContain("<em>not italic</em>");
    });

    it("should preserve HTML characters in code blocks without double-encoding", () => {
      const md = "```\nif (a < b && c > d) {}\n```";
      const html = markdownToHtml(md);

      expect(html).toContain("<code>if (a &lt; b &amp;&amp; c &gt; d) {}");
      expect(html).not.toContain("&amp;lt;");
    });

    it("should handle inline code with HTML characters correctly", () => {
      const md = "This is `<tag>` and `a & b`.";
      const html = markdownToHtml(md);

      expect(html).toContain("<code>&lt;tag&gt;</code>");
      expect(html).toContain("<code>a &amp; b</code>");
      expect(html).not.toContain("&amp;lt;");
    });

    it("should handle complex nested markdown structures", () => {
      const md =
        "# Header\n\n- Item with **bold** and `code`.\n- Another item with [Link](http://foo.com).\n\n```\nformatted\n  code\n```";
      const html = markdownToHtml(md);

      expect(html).toContain("<h1>Header</h1>");
      expect(html).toContain(
        "<li>Item with <strong>bold</strong> and <code>code</code>.</li>",
      );
      expect(html).toContain('<li>Another item with <a href="http://foo.com"');
      expect(html).toContain("<pre><code>formatted\n  code\n</code></pre>");
    });
  });

  describe("htmlToMarkdown", () => {
    it("should convert simple html to markdown", () => {
      const html = "<h2>Title</h2><p>This is <strong>bold</strong>.</p>";
      const md = htmlToMarkdown(html);
      expect(md).toContain("## Title");
      expect(md).toContain("**bold**");
    });

    it("should handle lists", () => {
      const html = "<ul><li>Item 1</li><li>Item 2</li></ul>";
      const md = htmlToMarkdown(html);
      expect(md).toContain("- Item 1");
      expect(md).toContain("- Item 2");
    });

    it("should handle nested formatting in htmlToMarkdown", () => {
      const html =
        "<ul><li>Item with <strong>bold</strong> and <code>code</code></li></ul>";
      const md = htmlToMarkdown(html);
      expect(md).toContain("- Item with **bold** and `code`");
    });

    it("should sanitize malicious HTML to prevent XSS", () => {
      const maliciousHtml =
        '<p>Normal text</p><img src=x onerror="alert(1)"><script>console.log("XSS")</script>';
      const md = htmlToMarkdown(maliciousHtml);
      // The onerror attribute and script tag should be ignored
      expect(md).not.toContain("onerror");
      expect(md).not.toContain("alert");
      expect(md).not.toContain("console.log");
      expect(md).toContain("Normal text");
    });

    it("should convert backticks in text correctly", () => {
      const html = "<p>This is a `backtick` in text.</p>";
      const md = htmlToMarkdown(html);
      expect(md).toContain("This is a `backtick` in text.");
    });
  });

  describe("createCodeBlockHtml", () => {
    it("should create correct code block structure", () => {
      const code = "const x = 1;";
      const html = createCodeBlockHtml(code);
      expect(html).toContain("<pre><code>const x = 1;</code></pre>");
    });
  });

  describe("toggleList", () => {
    it("should non-destructively unwrap a list item", () => {
      const div = document.createElement("div");
      div.contentEditable = "true";
      div.innerHTML =
        "<ul><li>Item 1</li><li id='target'>Item 2</li><li>Item 3</li></ul>";
      document.body.appendChild(div);

      const target = document.getElementById("target")!;
      const sel = window.getSelection();
      if (!sel) throw new Error("Selection not available");

      const range = document.createRange();
      range.selectNodeContents(target);
      sel.removeAllRanges();
      sel.addRange(range);

      toggleList("ul");

      // Check if it split correctly
      expect(div.innerHTML).toContain(
        "<ul><li>Item 1</li></ul><p>Item 2</p><ul><li>Item 3</li></ul>",
      );

      document.body.removeChild(div);
    });
  });
});
