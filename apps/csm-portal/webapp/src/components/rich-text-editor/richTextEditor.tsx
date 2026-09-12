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

import { type ReactElement, type RefObject } from "react";
import { type Theme } from "@wso2/oxygen-ui";
import {
  File as FileIcon,
  FileArchive,
  FileCode,
  FileImage,
  FileText,
} from "@wso2/oxygen-ui-icons-react";
import { createCommand, type LexicalCommand } from "lexical";

/** Payload for `INSERT_IMAGE_COMMAND` in the shared rich text editor. */
export type InsertImagePayload = {
  src: string;
  altText?: string;
};

/**
 * Derives alt text from a URL or filename (e.g. "image.png" -> "image", "/path/to/photo.jpg" -> "photo").
 * Handles absolute URLs, relative paths, and plain filenames.
 */
export function deriveAltFromFilename(src: string): string {
  // Percent-encoded names (e.g. "screen%20shot") read poorly as alt text;
  // decode them, but fall back to the raw value if it isn't valid encoding.
  const decode = (s: string): string => {
    try {
      return decodeURIComponent(s);
    } catch {
      return s;
    }
  };
  try {
    const url = new URL(src);
    const path = url.pathname || "";
    const match = path.match(/\/([^/]+)$/);
    const name = (match?.[1] ?? path) || "Image";
    const base = decode(name).replace(/\.[^.]+$/, "");
    return base ? base : "Image";
  } catch {
    const pathOnly = src.split("?")[0].split("#")[0].trim();
    const lastSegment = pathOnly.replace(/\/+$/, "").split("/").pop() || "";
    const base = decode(lastSegment).replace(/\.[^.]+$/, "");
    return base ? base : "Image";
  }
}

/**
 * Converts HTML to plain text by stripping tags and decoding entities.
 */
export function htmlToPlainText(html: string): string {
  if (!html || typeof html !== "string") return "";
  const parsed = new DOMParser().parseFromString(html, "text/html");
  const body = parsed.body ?? parsed.documentElement;
  return (body?.textContent ?? "").trim();
}

/**
 * Escapes HTML entities in a string.
 *
 * Re-exported from `@utils/sanitizeHtml`, which owns the single implementation
 * — importing it from this module would otherwise pull the whole editor (and
 * its editor-framework dependencies) into any bundle that only wanted to
 * escape a string.
 */
export { escapeHtml } from "@utils/sanitizeHtml";

/**
 * Sanitizes a URL by allowing only safe protocols.
 * Rejects protocol-relative URLs (e.g. //evil.com); allows single leading slash for relative paths.
 * Also allows data:image/* base64 URLs for inline images in the rich text editor.
 */
const SAFE_URL_PATTERN =
  /^(https?:\/\/|mailto:|tel:|\/(?!\/)|#|data:image\/[a-zA-Z0-9.+-]+;base64,)/i;
export function sanitizeUrl(url: string): string {
  const decoded = url.replace(/&amp;/g, "&").trim();
  return SAFE_URL_PATTERN.test(decoded) ? decoded : "";
}

/**
 * Returns the appropriate icon for a file based on its extension or type.
 */
export const getFileIcon = (file: File, theme: Theme): ReactElement => {
  const name = file.name.toLowerCase();
  const type = file.type.toLowerCase();

  if (
    /\.(png|jpe?g|gif|webp|svg|bmp|ico)$/i.test(name) ||
    type.startsWith("image/")
  ) {
    return <FileImage size={16} color={theme.palette.primary.main} />;
  }
  if (/\.pdf$/i.test(name) || type.includes("pdf")) {
    return <FileText size={16} color={theme.palette.primary.main} />;
  }
  if (
    /\.(zip|rar|7z|tar|gz)$/i.test(name) ||
    type.includes("zip") ||
    type.includes("archive")
  ) {
    return <FileArchive size={16} color={theme.palette.primary.main} />;
  }
  if (
    /\.(js|ts|tsx|jsx|py|java|cpp|c|h|cs|go|rs|php|rb|html|css|json|md|xml|yaml|yml|sh|sql)$/i.test(
      name,
    ) ||
    type.includes("code")
  ) {
    return <FileCode size={16} color={theme.palette.primary.main} />;
  }
  if (/\.(txt|log|csv)$/i.test(name) || type.startsWith("text/")) {
    return <FileText size={16} color={theme.palette.primary.main} />;
  }
  return <FileIcon size={16} color={theme.palette.primary.main} />;
};

/**
 * Scrolls an element by a given amount. Accepts either element id or ref.
 */
export const scrollElement = (
  elementIdOrRef: string | RefObject<HTMLElement | null>,
  direction: "left" | "right",
  scrollAmount: number = 200,
) => {
  const container =
    typeof elementIdOrRef === "string"
      ? document.getElementById(elementIdOrRef)
      : elementIdOrRef.current;
  if (container) {
    container.scrollBy({
      left: direction === "left" ? -scrollAmount : scrollAmount,
      behavior: "smooth",
    });
  }
};

/**
 * Lexical Command for inserting an image.
 */
export const INSERT_IMAGE_COMMAND: LexicalCommand<
  string | InsertImagePayload
> = createCommand();

/** A single step used to replay a plain-text clipboard paste into the editor. */
export type PlainTextPasteToken =
  | { type: "text"; value: string }
  | { type: "tab" }
  | { type: "paragraph" };

/**
 * Tokenizes plain-text clipboard content (used whenever the clipboard has
 * text/plain but no text/html -- exactly ChatGPT's own clipboard format) the
 * same way Lexical's default `$insertDataTransferForPlainText` does
 * (`@lexical/clipboard`): split on newlines/tabs, one "paragraph" token per
 * newline, one "text" token per run of plain text, one "tab" token per tab.
 *
 * The one deliberate difference: Lexical's default tokenizer emits a
 * "paragraph" token for every newline unconditionally, so a blank line
 * ("Para1\n\nPara2", which is exactly ChatGPT's blank-line-separated
 * paragraph format) produces two back-to-back paragraph breaks with nothing
 * typed in between -- i.e. a standalone empty paragraph -- instead of a
 * single paragraph break. This tokenizer drops a "paragraph" token when it
 * would immediately follow another one with no text/tab token in between, so
 * one or more consecutive blank lines collapse to a single paragraph break.
 * A lone newline inside real content still starts a new paragraph, same as
 * before -- only the redundant, content-free break is removed.
 */
export function tokenizePlainTextPaste(text: string): PlainTextPasteToken[] {
  const parts = text.split(/(\r?\n|\t)/);
  if (parts[parts.length - 1] === "") {
    parts.pop();
  }

  const tokens: PlainTextPasteToken[] = [];
  let sawContentSinceLastBreak = false;

  for (const part of parts) {
    if (part === "\n" || part === "\r\n") {
      if (!sawContentSinceLastBreak && tokens.length > 0) {
        // Back-to-back paragraph break with nothing typed in between (a
        // blank line) -- drop it instead of emitting an empty paragraph.
        continue;
      }
      tokens.push({ type: "paragraph" });
      sawContentSinceLastBreak = false;
    } else if (part === "\t") {
      tokens.push({ type: "tab" });
      sawContentSinceLastBreak = true;
    } else if (part !== "") {
      tokens.push({ type: "text", value: part });
      sawContentSinceLastBreak = true;
    }
    // part === "" is the empty segment between two consecutive delimiters;
    // it carries no content of its own and is otherwise skipped.
  }

  return tokens;
}

/**
 * Unwraps a `<code>` element that is a direct child of a `<pre>`, moving the
 * `<code>`'s children up to be the `<pre>`'s own children and removing the
 * now-empty `<code>` wrapper. Mutates `dom` in place; returns whether any
 * unwrapping happened.
 *
 * `<pre><code class="language-xxx">...</code></pre>` is the standard shape
 * emitted by ChatGPT, GitHub, and Notion for a fenced code block. Lexical's
 * `@lexical/code` CodeNode.importDOM() registers independent DOM converters
 * for both the `pre` tag and any multi-line `code` tag: converting the
 * `<pre>` creates a CodeNode and then recurses into its children as usual,
 * so the nested multi-line `<code>` gets converted a second time,
 * independently creating a second CodeNode nested inside the first --
 * producing a duplicated `<code class="editor-code"><code
 * class="editor-code">` wrapper instead of a single code block. Flattening
 * the DOM before conversion removes the nested `<code>` element entirely, so
 * only the `<pre>` converter fires.
 *
 * Also carries the language hint from the removed `<code>`'s
 * `class="language-xxx"` onto the `<pre>` as `data-language` (what
 * `$convertPreElement` reads), when the `<pre>` doesn't already have one, so
 * the resulting single CodeNode keeps its language.
 *
 * Leaves standalone `<pre>` (no nested `<code>`) and standalone multi-line
 * `<code>` (not inside a `<pre>`) untouched -- both already convert to
 * exactly one CodeNode today.
 */
export function unwrapNestedPreCodeElements(dom: Document): boolean {
  let changed = false;

  for (const pre of Array.from(dom.querySelectorAll("pre"))) {
    const codeChildren = Array.from(pre.children).filter(
      (child): child is HTMLElement => child.tagName === "CODE",
    );

    for (const code of codeChildren) {
      if (!pre.hasAttribute("data-language")) {
        const languageMatch = code
          .getAttribute("class")
          ?.match(/language-([\w-]+)/i);
        if (languageMatch) {
          pre.setAttribute("data-language", languageMatch[1]);
        }
      }

      while (code.firstChild) {
        pre.insertBefore(code.firstChild, code);
      }
      pre.removeChild(code);
      changed = true;
    }
  }

  return changed;
}

/**
 * Removes paragraph-like elements (`<p>` or `<div>`) that carry no real
 * content -- empty, or containing only whitespace, `&nbsp;`, and/or `<br>` --
 * from pasted HTML. Mutates `dom` in place; returns whether anything was
 * removed.
 *
 * Rich clipboard sources (Google Docs, Gmail, Gemini, Claude, Notion, Word,
 * and even a manual text-selection copy from ChatGPT's rendered page --
 * anything that puts `text/html` on the clipboard) commonly represent a
 * blank line between paragraphs as an empty `<p>&nbsp;</p>`, `<p><br></p>`,
 * or an empty `<div><br></div>` in their clipboard HTML. Left in place, each
 * one becomes its own empty ParagraphNode once converted, which reproduces
 * the same "extra space between paragraphs" bug in the read view that
 * `tokenizePlainTextPaste` above fixes for plain-text paste -- the read
 * view's `"& p + p": { mt: 0.75 }` adjacency rule fires around the empty
 * paragraph too, and the empty paragraph contributes its own line height on
 * top.
 *
 * Only `<p>`/`<div>` elements whose only children are whitespace text nodes
 * and/or `<br>` elements are removed, so a block that carries real content
 * (text, an image, a link, a nested list, etc.) is always left untouched.
 */
export function collapseEmptyParagraphElements(dom: Document): boolean {
  const body = dom.body;
  if (!body) return false;

  const isEffectivelyEmptyBlock = (el: Element): boolean => {
    if (el.tagName !== "P" && el.tagName !== "DIV") return false;
    for (const child of Array.from(el.childNodes)) {
      if (child.nodeType === Node.TEXT_NODE) {
        const text = (child.textContent ?? "").replace(/\u00a0/g, " ").trim();
        if (text !== "") return false;
      } else if (child.nodeType === Node.ELEMENT_NODE) {
        if ((child as Element).tagName !== "BR") return false;
      }
      // Other node types (comments, etc.) carry no visible content.
    }
    return true;
  };

  // querySelectorAll returns elements in document order (ancestors before
  // descendants). Reversing processes descendants first, so an outer wrapper
  // whose only child is itself an empty block (e.g. `<div><div><br></div></div>`)
  // is re-evaluated as empty once its inner child has already been removed,
  // instead of being skipped because it "contained an element" at the time it
  // was checked.
  let changed = false;
  for (const el of Array.from(body.querySelectorAll("p, div")).reverse()) {
    if (isEffectivelyEmptyBlock(el)) {
      el.remove();
      changed = true;
    }
  }

  return changed;
}
