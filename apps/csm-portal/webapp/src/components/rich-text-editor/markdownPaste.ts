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
  htmlToPastePlainText,
  sanitizeUrl,
  unwrapNestedPreCodeElements,
} from "@components/rich-text-editor/richTextEditor";

/**
 * A `<p>` that may be joined into its neighbour: it has text (or a picture) and carries no
 * `style`, `class` or `align`, so nothing of its own (alignment, indent, Word `Mso*` classes)
 * is lost, and an empty `<p><br></p>` stays a block of its own.
 */
function isMergeableParagraph(el: Element): boolean {
  if (el.tagName !== "P") return false;
  if (el.hasAttribute("style") || el.hasAttribute("class") || el.hasAttribute("align")) return false;
  return (el.textContent ?? "").trim() !== "" || el.querySelector("img") !== null;
}

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
    // Only whitespace-only text and comments may lie between two paragraphs that are joined: any
    // other text (`<p>A</p>middle<p>B</p>`) sits between them, so joining would move it after both.
    const gap: Node[] = [];
    let after: Node | null = child.nextSibling;
    while (
      after &&
      (after.nodeType === Node.COMMENT_NODE ||
        (after.nodeType === Node.TEXT_NODE && (after.textContent ?? "").trim() === ""))
    ) {
      gap.push(after);
      after = after.nextSibling;
    }
    if (after && after.nodeType === Node.ELEMENT_NODE && isMergeableParagraph(child) && isMergeableParagraph(after as Element)) {
      const next = after as Element;
      for (const node of gap) node.parentNode?.removeChild(node);
      child.appendChild(dom.createElement("br"));
      child.appendChild(dom.createElement("br"));
      while (next.firstChild) child.appendChild(next.firstChild);
      next.remove();
      continue; // look at the same paragraph against its new neighbour
    }
    child = child.nextElementSibling;
  }
}

/** A local `blob:` preview or a stored image's `/<uuid>`: what this app's own editors put in an `<img>`. */
const APP_IMAGE_SRC =
  /^(?:blob:https?:\/\/[^\s"'<>]+|\/[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})$/i;

/**
 * Whether an `<img>` source is one this app's editors produce: a `blob:` preview or a stored
 * `/<uuid>` image, and, when the clipboard came from an editor of this app
 * (`application/x-lexical-editor`), any `data:image/...;base64,` source an older draft holds.
 */
export function isAppImageSrc(src: string, fromApp: boolean): boolean {
  const value = src.trim();
  return APP_IMAGE_SRC.test(value) || (fromApp && /^data:image\/[a-z0-9.+-]+;base64,/i.test(value));
}

/**
 * Replaces every `<img>` in `dom`: the app never hot-links or embeds a pasted image
 * (new images are uploaded from the file the clipboard carries). `decide` may keep one
 * as an image (`"keep"`, an image copied inside this app) or turn it into its alt text
 * (`"text"`, e.g. over the inline image limits). One with a safe
 * address becomes a link (`alt` or the address as text); one inside a link becomes its
 * alt text (one link, not a nested one); a `data:` or unsafe address becomes its alt
 * text. With `dropAll` (the clipboard also carries the picture file, which is stored
 * on its own) every image is removed instead. Mutates `dom`.
 */
export function imagesToLinks(
  dom: Document,
  { dropAll = false, decide }: { dropAll?: boolean; decide?: (img: Element) => "keep" | "text" | undefined } = {},
): void {
  for (const img of Array.from(dom.querySelectorAll("img"))) {
    const decision = decide?.(img);
    if (decision === "keep") continue;
    if (decision === "text") {
      img.replaceWith(dom.createTextNode(img.getAttribute("alt") ?? ""));
      continue;
    }
    if (dropAll) {
      img.remove();
      continue;
    }
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
}

/**
 * Replaces every table with one paragraph per row, the cells' text joined by " | ",
 * for a paste into a table cell (a table cannot hold a table in this editor).
 * Empty rows are skipped. Mutates `dom`.
 */
export function flattenTablesToParagraphs(dom: Document): void {
  for (const table of Array.from(dom.querySelectorAll("table"))) {
    if (!table.isConnected) continue; // inside a table already replaced
    const rows = Array.from(table.querySelectorAll("tr")).filter((row) => row.closest("table") === table);
    const paragraphs = rows
      .map((row) =>
        Array.from(row.children)
          .filter((cell) => cell.tagName === "TD" || cell.tagName === "TH")
          // Block/<br> structure becomes whitespace first (`textContent` would glue `a<br>b` into `ab`).
          .map((cell) => htmlToPastePlainText(cell.innerHTML).replace(/\s+/g, " ").trim())
          .join(" | "),
      )
      .filter((text) => text.replace(/[\s|]/g, "") !== "")
      .map((text) => {
        const p = dom.createElement("p");
        p.textContent = text;
        return p;
      });
    table.replaceWith(...paragraphs);
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

  imagesToLinks(dom);

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
