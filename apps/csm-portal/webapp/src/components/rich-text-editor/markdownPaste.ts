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

import { markdownToHtml } from "@utils/renderMarkdown";
import { sanitizeRichTextHtml } from "@utils/sanitizeHtml";
import {
  collapseEmptyParagraphElements,
  sanitizeUrl,
  unwrapNestedPreCodeElements,
} from "@components/rich-text-editor/richTextEditor";

/**
 * Merges runs of adjacent top-level `<p>` siblings into one `<p>` joined by
 * `<br><br>`, so a blank line in pasted Markdown is the same markup as one typed
 * with Enter in an `enterInsertsLineBreak` editor. Lists, headings, code, quotes
 * and tables stay separate blocks. Mutates `dom`.
 */
export function mergeAdjacentParagraphs(dom: Document): void {
  const body = dom.body;
  let child = body.firstElementChild;
  while (child) {
    const next: Element | null = child.nextElementSibling;
    if (child.tagName === "P" && next && next.tagName === "P") {
      child.appendChild(dom.createElement("br"));
      child.appendChild(dom.createElement("br"));
      while (next.firstChild) child.appendChild(next.firstChild);
      next.remove();
      continue; // look at the same paragraph against its new neighbour
    }
    child = next;
  }
}

/**
 * Markdown source to a DOM Lexical can import: `markdownToHtml` (GitHub comment
 * semantics: single newlines are `<br>`, raw HTML is escaped, bare URLs are
 * linked), then the app's sanitizer, then fixes for what Lexical has no node for.
 *
 * - `div.md-table-wrap` (the table's scroll wrapper) is unwrapped.
 * - Images become links (`alt` or the address as the text): the app never
 *   hot-links or embeds a pasted image. One whose address is not a safe URL
 *   becomes its alt text.
 * - An image inside a link becomes its alt text, so there is one link, not a nested one.
 * - A fenced block's trailing newline is removed (markdown-it ends every one with `\n`).
 * - `<hr>` is dropped (the editor has no horizontal rule).
 * - A loose list's `<li><p>` and a quote's `<blockquote><p>` are unwrapped (their
 *   paragraphs joined by `<br><br>` in a quote): both hold inline content in the editor.
 */
export function markdownToPasteDom(markdown: string): Document {
  const html = sanitizeRichTextHtml(markdownToHtml(markdown));
  const dom = new DOMParser().parseFromString(html, "text/html");

  for (const wrap of Array.from(dom.querySelectorAll("div.md-table-wrap"))) {
    wrap.replaceWith(...Array.from(wrap.childNodes));
  }

  for (const img of Array.from(dom.querySelectorAll("img"))) {
    const src = img.getAttribute("src") ?? "";
    const alt = img.getAttribute("alt") || src;
    const href = /^data:/i.test(src) ? "" : sanitizeUrl(src);
    if (img.closest("a")) {
      // An image inside a link (a README badge): keep the outer link, show the alt.
      img.replaceWith(dom.createTextNode(alt));
    } else if (href) {
      const link = dom.createElement("a");
      link.setAttribute("href", href);
      link.textContent = alt;
      img.replaceWith(link);
    } else {
      img.replaceWith(dom.createTextNode(img.getAttribute("alt") ?? ""));
    }
  }

  for (const code of Array.from(dom.querySelectorAll("pre > code"))) {
    const last = code.lastChild;
    if (last && last.nodeType === Node.TEXT_NODE) {
      last.textContent = (last.textContent ?? "").replace(/\n$/, "");
    }
  }

  for (const hr of Array.from(dom.querySelectorAll("hr"))) hr.remove();

  for (const li of Array.from(dom.querySelectorAll("li"))) {
    const paragraphs = Array.from(li.children).filter((c) => c.tagName === "P");
    paragraphs.forEach((p, index) => {
      if (index > 0) p.before(dom.createElement("br"));
      p.replaceWith(...Array.from(p.childNodes));
    });
  }

  for (const quote of Array.from(dom.querySelectorAll("blockquote"))) {
    const paragraphs = Array.from(quote.children).filter((c) => c.tagName === "P");
    paragraphs.forEach((p, index) => {
      if (index > 0) p.before(dom.createElement("br"), dom.createElement("br"));
      p.replaceWith(...Array.from(p.childNodes));
    });
  }

  unwrapNestedPreCodeElements(dom);
  collapseEmptyParagraphElements(dom);
  return dom;
}
