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

/**
 * Given to an editor that stores a pasted or picked image itself instead of
 * embedding it as base64. Called with a file that has already passed the type
 * and size checks; starts the upload and returns the `blob:` URL to show in the
 * editor meanwhile. An editor without one embeds the image as before.
 */
export type ImageUploadHandler = (file: File) => string;

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

const PLAIN_TEXT_BLOCK_TAGS = new Set([
  "P", "DIV", "LI", "UL", "OL", "TR", "TABLE", "THEAD", "TBODY", "TFOOT", "H1", "H2", "H3", "H4", "H5", "H6",
  "BLOCKQUOTE", "PRE", "SECTION", "ARTICLE", "HEADER", "FOOTER", "ASIDE", "NAV", "FIGURE", "FIGCAPTION",
  "DL", "DT", "DD", "HR", "ADDRESS", "MAIN", "FORM", "FIELDSET",
]);

/**
 * Plain text of clipboard HTML for "Remove Formatting" when the clipboard has no `text/plain` of
 * its own. Unlike {@link htmlToPlainText} (`textContent`, which glues `<p>one</p><p>two</p>` into
 * `onetwo`) it puts a newline between block elements and for every `<br>`, one list item or table
 * row per line, table cells separated by a tab, and collapses the source's own layout whitespace
 * (outside `<pre>`), so the plain-text tokenizer then applies its normal rules.
 */
export function htmlToPastePlainText(html: string): string {
  if (!html || typeof html !== "string") return "";
  const body = new DOMParser().parseFromString(html, "text/html").body;
  if (!body) return "";
  let out = "";
  const trimLineEnd = () => {
    out = out.replace(/ +$/, "");
  };
  const endLine = () => {
    trimLineEnd();
    if (out !== "" && !out.endsWith("\n")) out += "\n";
  };
  const walk = (node: Node, inPre: boolean): void => {
    if (node.nodeType === Node.TEXT_NODE) {
      const text = node.textContent ?? "";
      if (inPre) {
        out += text;
        return;
      }
      let collapsed = text.replace(/[ \t\r\n\f]+/g, " ");
      if (collapsed.startsWith(" ") && (out === "" || /[\n\t ]$/.test(out))) collapsed = collapsed.slice(1);
      out += collapsed;
      return;
    }
    if (node.nodeType !== Node.ELEMENT_NODE) return;
    const el = node as Element;
    const tag = el.tagName;
    if (tag === "STYLE" || tag === "SCRIPT" || tag === "HEAD" || tag === "TEMPLATE") return;
    if (tag === "BR") {
      trimLineEnd();
      out += "\n";
      return;
    }
    if ((tag === "TD" || tag === "TH") && out !== "" && !out.endsWith("\n")) {
      trimLineEnd();
      out += "\t";
    }
    const block = PLAIN_TEXT_BLOCK_TAGS.has(tag);
    if (block) endLine();
    for (const child of Array.from(el.childNodes)) walk(child, inPre || tag === "PRE");
    if (block) endLine();
  };
  walk(body, false);
  return out.replace(/^\n+/, "").trimEnd();
}

/** Elements that are content on their own, with no text (a picture, a table, a rule). */
const CONTENT_ELEMENTS = "img, table, hr, svg, video, audio, canvas, iframe, object, embed, input";

/**
 * Whether clipboard HTML has anything to paste: some text, or a picture/table/rule. A payload that
 * is only a `<meta>`, comments or empty/`<br>`-only blocks (what some apps put beside the real
 * `text/plain`) has none, and offering "Keep Formatting" for it would insert nothing.
 */
export function hasMeaningfulHtmlContent(html: string): boolean {
  if (!html.trim()) return false;
  const body = new DOMParser().parseFromString(html, "text/html").body;
  if (!body) return false;
  if (body.querySelector(CONTENT_ELEMENTS)) return true;
  for (const el of Array.from(body.querySelectorAll("style, script, template"))) el.remove();
  return (body.textContent ?? "").trim() !== "";
}

/** Whether clipboard HTML has real text or a table (what a picture beside it merely renders). */
export function htmlHasTextOrTable(html: string): boolean {
  if (!html.trim()) return false;
  const body = new DOMParser().parseFromString(html, "text/html").body;
  if (!body) return false;
  if (body.querySelector("table")) return true;
  for (const el of Array.from(body.querySelectorAll("style, script, template"))) el.remove();
  return (body.textContent ?? "").trim() !== "";
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
 * A local preview of an image that is still being (or has just been) uploaded
 * (see {@link ImageUploadHandler}): `blob:` plus the page's own origin. Only an
 * image's `src` may be one of these, never a link, which is why it is not part
 * of {@link sanitizeUrl}. It is only ever a preview: the comment input swaps it
 * for the stored image's id before anything is sent.
 */
const BLOB_IMAGE_SRC_PATTERN = /^blob:https?:\/\/[^\s"'<>]+$/i;

/** {@link sanitizeUrl} for an `<img>` `src`, which also accepts a local `blob:` preview. */
export function sanitizeImageSrc(url: string): string {
  const safe = sanitizeUrl(url);
  if (safe) return safe;
  const decoded = url.replace(/&amp;/g, "&").trim();
  return BLOB_IMAGE_SRC_PATTERN.test(decoded) ? decoded : "";
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
  | { type: "paragraph" }
  | { type: "linebreak" };

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
export function tokenizePlainTextPaste(
  text: string,
  options: { lineBreaks?: boolean } = {},
): PlainTextPasteToken[] {
  if (options.lineBreaks) return tokenizePlainTextPasteAsLineBreaks(text);
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
 * The `lineBreaks` mode of {@link tokenizePlainTextPaste}, for editors whose
 * Enter key inserts a `<br>` instead of a paragraph (`enterInsertsLineBreak`):
 * every newline becomes a "linebreak" token, so a pasted blank line stays one
 * blank line (two consecutive breaks) and a lone newline stays a single break.
 * Three or more consecutive newlines collapse to one blank line.
 */
function tokenizePlainTextPasteAsLineBreaks(text: string): PlainTextPasteToken[] {
  const normalized = text.replace(/\r\n?/g, "\n").replace(/\n{3,}/g, "\n\n");
  const tokens: PlainTextPasteToken[] = [];
  for (const part of normalized.split(/(\n|\t)/)) {
    if (part === "\n") tokens.push({ type: "linebreak" });
    else if (part === "\t") tokens.push({ type: "tab" });
    else if (part !== "") tokens.push({ type: "text", value: part });
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

  // Google Docs puts a bare `<br>` between a list and the next paragraph (and Word/Docs between
  // other blocks): it is the blank line, which would become a `<p><br></p>`. Only a `<br>` whose
  // neighbours are both blocks goes; one inside a paragraph or list item has text beside it.
  const isBlockElement = (node: Node | null): boolean =>
    node !== null && node.nodeType === Node.ELEMENT_NODE && BLOCK_NEIGHBOURS.has((node as Element).tagName);
  const sibling = (node: Node, next: boolean): Node | null => {
    let n: Node | null = next ? node.nextSibling : node.previousSibling;
    while (
      n &&
      (n.nodeType === Node.COMMENT_NODE ||
        (n.nodeType === Node.TEXT_NODE && (n.textContent ?? "").replace(/\u00a0/g, " ").trim() === ""))
    ) {
      n = next ? n.nextSibling : n.previousSibling;
    }
    return n;
  };
  for (const br of Array.from(body.querySelectorAll("br"))) {
    if (isBlockElement(sibling(br, false)) && isBlockElement(sibling(br, true))) {
      br.remove();
      changed = true;
    }
  }

  return changed;
}

const BLOCK_NEIGHBOURS = new Set([
  "P", "DIV", "UL", "OL", "H1", "H2", "H3", "H4", "H5", "H6", "BLOCKQUOTE", "PRE", "TABLE",
]);

/**
 * Removes the `white-space: pre-wrap` inline style that Lexical's default
 * `TextNode.exportDOM()` adds to the exported element of *every* text run
 * (`node_modules/lexical/Lexical.dev.mjs`: `element.style.whiteSpace =
 * 'pre-wrap'`, set unconditionally before any format-specific wrapping).
 * Then unwraps any `<span>` left with no remaining attributes -- a plain
 * text run has no other reason to be wrapped in a span once that style is
 * gone.
 *
 * Per digiops-cs#2933: this HTML is only ever read by renderers we control
 * (CSM Portal, Customer Portal), both via `dangerouslySetInnerHTML` in
 * components we own -- Lexical's own source comment says the per-span style
 * exists "for headless mode where people might use Lexical to generate HTML
 * content and not have the ability to use CSS classes," which doesn't apply
 * here. The same guarantee (don't collapse multiple consecutive spaces the
 * user typed) is declared once on the render container's own CSS instead.
 *
 * IMPORTANT -- this is an unconditional strip, unlike
 * {@link unwrapNestedPreCodeElements} and friends: it is only safe to call
 * this on the generation side once *every* consumer that renders this HTML
 * declares `white-space: pre-wrap` on its own container CSS. A consumer that
 * doesn't will silently collapse multi-space runs and leading/trailing
 * spaces in newly generated comments. See digiops-cs#2933 for the full
 * rollout requirement across both apps.
 */
export function stripWhitespaceStyleAndUnwrapSpans(dom: Document): boolean {
  const body = dom.body;
  if (!body) return false;

  let changed = false;
  for (const el of Array.from(body.querySelectorAll<HTMLElement>("[style]"))) {
    if (!el.style.whiteSpace) continue;
    el.style.removeProperty("white-space");
    if (el.getAttribute("style") === "") {
      el.removeAttribute("style");
    }
    changed = true;

    if (el.tagName === "SPAN" && el.attributes.length === 0) {
      el.replaceWith(...Array.from(el.childNodes));
    }
  }

  return changed;
}

/**
 * Lexical exports a nested list as a wrapper item holding only the sub-list:
 * `<li class="editor-list-item-nested"><ul>...</ul></li>`. Only the editor's own CSS hides that
 * wrapper's marker, so every other renderer drew an empty bullet before each nested list. The
 * exported HTML therefore carries `list-style-type: none` on the wrapper itself.
 */
export function markNestedListItemsInHtml(html: string): string {
  if (!html.includes("editor-list-item-nested")) return html;
  const dom = new DOMParser().parseFromString(html, "text/html");
  let changed = false;
  for (const li of Array.from(dom.body.querySelectorAll<HTMLElement>("li.editor-list-item-nested"))) {
    const style = li.getAttribute("style") ?? "";
    if (/list-style(-type)?\s*:/i.test(style)) continue;
    li.setAttribute("style", style ? `${style.replace(/;?\s*$/, ";")} list-style-type: none` : "list-style-type: none");
    changed = true;
  }
  return changed ? dom.body.innerHTML : html;
}

/**
 * String-level convenience wrapper around
 * {@link stripWhitespaceStyleAndUnwrapSpans} for the composer's
 * HTML-generation path, which only has the serialized
 * `$generateHtmlFromNodes` output, not a `Document`.
 */
export function stripWhitespaceStyleFromHtml(html: string): string {
  if (!html.includes("white-space")) return html;

  const dom = new DOMParser().parseFromString(html, "text/html");
  if (!stripWhitespaceStyleAndUnwrapSpans(dom)) return html;

  return dom.body.innerHTML;
}

/**
 * The exact presentational properties `@lexical/table`'s own `TableCellNode`/
 * `TableNode.exportDOM()` unconditionally bake into every cell on export --
 * see `stripLexicalTableStylingFromHtml`'s own doc comment. Removed by
 * property name, not by clearing the whole `style` attribute, so a
 * declaration the *user* typed by hand in HTML-source mode (e.g. `color:
 * red` on one cell) survives a source-mode round trip instead of being
 * wiped out alongside Lexical's own junk.
 */
const LEXICAL_TABLE_CELL_STYLE_PROPERTIES = new Set([
  "width",
  "border",
  "vertical-align",
  "text-align",
  "background-color",
]);

/**
 * Removes exactly the declarations in
 * {@link LEXICAL_TABLE_CELL_STYLE_PROPERTIES} from a raw `style` attribute
 * string, keeping every other declaration untouched -- including one whose
 * property CSSOM would otherwise expand a shorthand like `border` into
 * (`border-width`/`border-style`/`border-color`/`border-top`/etc). Operating
 * on the attribute's own text instead of `element.style` deliberately
 * sidesteps that expansion: jsdom (and some browsers) normalize `border: 1px
 * solid black` into its longhand components -- or, once those longhands are
 * themselves removed, into yet another equivalent grouping (`border-top`/
 * `border-right`/...) -- on the very first read of `element.style`, so
 * chasing it one property at a time via `removeProperty` never converges.
 * Matching the literal property name Lexical's own `exportDOM` writes,
 * before any such normalization happens, avoids the whole problem.
 */
function stripLexicalStyleDeclarations(styleAttr: string): string {
  return styleAttr
    .split(";")
    .map((decl) => decl.trim())
    .filter((decl) => {
      if (!decl) return false;
      const propName = decl.slice(0, decl.indexOf(":")).trim().toLowerCase();
      return !LEXICAL_TABLE_CELL_STYLE_PROPERTIES.has(propName);
    })
    .join("; ");
}

/**
 * Strips every presentational bit `@lexical/table`'s own `TableCellNode`/
 * `TableNode.exportDOM()` unconditionally bakes into a table on export: a
 * fixed per-cell `style="width: …px"` (its own `COLUMN_WIDTH` constant, 75,
 * whenever a cell carries no explicit width of its own -- true for every
 * table this editor produces, since nothing here exposes a column-resize
 * control), plus `border`/`vertical-align`/`text-align`/header
 * `background-color`, and the `<colgroup>`'s own `<col style="width:...">`
 * entries (a second, independent width constraint on top of the per-cell
 * one). Left in, that inline styling always wins over every consumer's own
 * table CSS (inline beats a stylesheet rule short of `!important`), so a
 * table submitted from this editor renders as a cramped, heavily-wrapped
 * ~75px-per-column grid everywhere else it's displayed -- the CSM portal's
 * own read view, the separate customer portal -- no matter how wide its
 * actual content is. This has to run at generation time, on the HTML that
 * actually gets submitted, because none of those other renderers can be
 * patched from here -- the live composing view's own CSS override
 * (`Editor.tsx`'s `!important` rule) only reaches this one screen.
 *
 * Removes only the specific properties in
 * {@link LEXICAL_TABLE_CELL_STYLE_PROPERTIES} above, not the whole `style`
 * attribute -- a table typed by hand in HTML-source mode can carry its own
 * deliberate styling (e.g. a highlighted cell's `background-color`, though
 * that particular property collides with Lexical's own header shading; a
 * safer example is `color`), which must survive toggling back to rich mode
 * and re-exporting, not just Lexical's own injected defaults.
 */
export function stripLexicalTableStylingFromHtml(html: string): string {
  if (!html.includes("<table")) return html;

  const dom = new DOMParser().parseFromString(html, "text/html");
  const body = dom.body;
  if (!body) return html;

  let changed = false;
  for (const colgroup of Array.from(body.querySelectorAll("table > colgroup"))) {
    colgroup.remove();
    changed = true;
  }
  for (const el of Array.from(
    body.querySelectorAll<HTMLElement>("table, table th, table td"),
  )) {
    const styleAttr = el.getAttribute("style");
    if (!styleAttr) continue;
    const kept = stripLexicalStyleDeclarations(styleAttr);
    if (kept === styleAttr) continue;
    changed = true;
    if (kept) {
      el.setAttribute("style", kept);
    } else {
      el.removeAttribute("style");
    }
  }

  return changed ? body.innerHTML : html;
}

/** The `mso-list` declaration of a Word list paragraph: `l0 level2 lfo1` (list id, level, instance). */
function parseMsoList(el: Element): { id: string; level: number } | null {
  const style = el.getAttribute("style") ?? "";
  const match = /mso-list\s*:\s*([^;]*)/i.exec(style);
  const isListClass = /\bMsoListParagraph/i.test(el.getAttribute("class") ?? "");
  // The List Paragraph style alone is not a list item (Word applies it to plain text too): it
  // counts only when a list glyph is actually there.
  if (!match) return isListClass && hasWordListGlyph(el) ? { id: "", level: 1 } : null;
  const value = match[1].trim();
  if (/^(ignore|none)\b/i.test(value)) return null;
  const id = /^(l\d+)/i.exec(value)?.[1];
  if (!id && !isListClass) return null;
  const level = Number(/\blevel(\d+)/i.exec(value)?.[1] ?? 1);
  return { id: id ?? "", level: level >= 1 ? level : 1 };
}

/** A leading bullet or number glyph in text, followed by spacing: `·  x`, `o\u00a0x`, `1.  x`, `1.1. x`. */
const LEADING_GLYPH = /^[\s\u00a0]*([·•§▪■◦○●-]|o|\(?[0-9a-zA-Z]{1,5}(?:\.[0-9a-zA-Z]{1,5})*[.)])[\s\u00a0]+(?=\S)/;

function firstTextNode(el: Element): Text | null {
  const walker = el.ownerDocument.createTreeWalker(el, NodeFilter.SHOW_TEXT);
  for (let n = walker.nextNode(); n; n = walker.nextNode()) {
    if ((n.textContent ?? "").trim() !== "") return n as Text;
  }
  return null;
}

/** Whether a paragraph carries a list glyph: an `mso-list:Ignore` span, the `supportLists` comment, or a leading glyph. */
function hasWordListGlyph(p: Element): boolean {
  for (const n of Array.from(p.childNodes)) {
    if (n.nodeType === Node.COMMENT_NODE && /if\s+!supportLists/i.test(n.nodeValue ?? "")) return true;
  }
  const ignored = Array.from(p.querySelectorAll("[style]")).some((el) => /mso-list\s*:\s*ignore/i.test(el.getAttribute("style") ?? ""));
  if (ignored) return true;
  const text = firstTextNode(p);
  return text !== null && LEADING_GLYPH.test(text.textContent ?? "");
}

/**
 * Takes the bullet/number glyph out of a Word list paragraph and returns its text. Word writes it
 * as `<span style="mso-list:Ignore">·<span>&nbsp;&nbsp;</span></span>`, wrapped in
 * `<!--[if !supportLists]-->…<!--[endif]-->` (or the `<![if …]>` form, a comment once parsed).
 */
function takeWordListGlyph(p: Element): string {
  let glyph = "";
  const nodes = Array.from(p.childNodes);
  for (let i = 0; i < nodes.length; i += 1) {
    if (nodes[i].nodeType === Node.COMMENT_NODE && /if\s+!supportLists/i.test(nodes[i].nodeValue ?? "")) {
      let j = i + 1;
      while (j < nodes.length && !(nodes[j].nodeType === Node.COMMENT_NODE && /endif/i.test(nodes[j].nodeValue ?? ""))) j += 1;
      if (j < nodes.length) {
        for (let k = i; k <= j; k += 1) {
          if (nodes[k].nodeType !== Node.COMMENT_NODE) glyph += nodes[k].textContent ?? "";
          nodes[k].parentNode?.removeChild(nodes[k]);
        }
        return glyph;
      }
    }
  }
  const ignored = Array.from(p.querySelectorAll<HTMLElement>("[style]")).find((el) =>
    /mso-list\s*:\s*ignore/i.test(el.getAttribute("style") ?? ""),
  );
  if (ignored) {
    glyph = ignored.textContent ?? "";
    ignored.remove();
  }
  if (glyph === "") {
    // The marker span was lost: the glyph is literal leading text.
    const text = firstTextNode(p);
    const match = text ? LEADING_GLYPH.exec(text.textContent ?? "") : null;
    if (text && match) {
      glyph = match[1];
      text.textContent = (text.textContent ?? "").slice(match[0].length);
    }
  }
  return glyph;
}

/**
 * Turns Word's list paragraphs into real lists before conversion. Word (and Outlook) paste a list
 * as `<p class=MsoListParagraph style="mso-list:l0 level2 lfo1">` with the bullet/number as text
 * in a hidden-by-Word span, so without this each item became a paragraph starting with `·` and the
 * nesting was lost. Adjacent list paragraphs become `<ul>`/`<ol>` nested by `levelN`; a
 * list is ordered when its glyph is a number/letter/roman numeral followed by `.` or `)` (`1.`,
 * `a)`, `(iv)`) and unordered for any other (`·`, `o`, `§`, `•`, `-`). The glyph is removed, and a
 * numbered list that starts above 1 keeps its start. Mutates `dom`; returns whether anything changed.
 */
const GLYPH_SEGMENT = "(?:[0-9]+|[a-zA-Z]{1,5})";
const ORDERED_GLYPH = new RegExp(
  `^(?:\\(?${GLYPH_SEGMENT}(?:\\.${GLYPH_SEGMENT})*[.)]|${GLYPH_SEGMENT}(?:\\.${GLYPH_SEGMENT})+)$`,
);

/** A number/letter/roman outline glyph (`1.`, `a)`, `(iv)`, `1.1.`, `2.3.1`, `A.1.`) is ordered; one symbol is a bullet; anything else is unknown. */
function glyphKind(glyph: string): "ordered" | "bullet" | "unknown" {
  if (ORDERED_GLYPH.test(glyph)) return "ordered";
  if (glyph.length === 1 && (glyph === "o" || !/[0-9a-zA-Z]/.test(glyph))) return "bullet";
  return "unknown";
}

export function convertWordListsToHtmlLists(dom: Document): boolean {
  const body = dom.body;
  if (!body) return false;
  const candidates = Array.from(body.querySelectorAll("p")).filter((p) => parseMsoList(p) !== null);
  if (candidates.length === 0) return false;
  const candidateSet = new Set<Element>(candidates);

  const isGap = (n: Node): boolean =>
    n.nodeType === Node.COMMENT_NODE || (n.nodeType === Node.TEXT_NODE && (n.textContent ?? "").trim() === "");
  const placed = new Set<Element>();
  let changed = false;

  for (const first of candidates) {
    if (placed.has(first)) continue;
    // The run: this paragraph and the list paragraphs that follow it as siblings.
    const run: Element[] = [first];
    for (let n: Node | null = first.nextSibling; n; n = n.nextSibling) {
      if (isGap(n)) continue;
      if (n.nodeType === Node.ELEMENT_NODE && candidateSet.has(n as Element)) run.push(n as Element);
      else break;
    }

    type Level = { level: number; list: HTMLElement; ordered: boolean; id: string };
    const stack: Level[] = [];
    const roots: HTMLElement[] = [];
    for (const p of run) {
      placed.add(p);
      const info = parseMsoList(p)!;
      const glyph = takeWordListGlyph(p).replace(/[\s\u00a0]+/g, " ").trim();
      const kind = glyphKind(glyph);
      while (stack.length > 0 && stack[stack.length - 1].level > info.level) stack.pop();
      // An ambiguous glyph follows its list (same level) or its parent (the enclosing list): it stays ordered under an ordered one.
      const context = stack[stack.length - 1];
      const ordered = kind === "ordered" || (kind === "unknown" && context?.ordered === true);
      const newList = (): HTMLElement => {
        const list = dom.createElement(ordered ? "ol" : "ul");
        const number = /^\(?(\d+)[.)]$/.exec(glyph)?.[1];
        if (ordered && number && Number(number) > 1) list.setAttribute("start", number);
        return list;
      };

      let top = stack[stack.length - 1];
      if (top && top.level === info.level && (top.ordered !== ordered || top.id !== info.id)) {
        stack.pop();
        top = stack[stack.length - 1];
        const list = newList();
        if (top) top.list.lastElementChild?.appendChild(list);
        else roots.push(list);
        stack.push({ level: info.level, list, ordered, id: info.id });
      } else if (!top || top.level < info.level) {
        const list = newList();
        const parentItem = top?.list.lastElementChild;
        if (parentItem) parentItem.appendChild(list);
        else roots.push(list);
        stack.push({ level: info.level, list, ordered, id: info.id });
      }
      const li = dom.createElement("li");
      for (const o of Array.from(p.getElementsByTagName("*"))) if (o.tagName.toLowerCase() === "o:p") o.remove();
      while (p.firstChild) li.appendChild(p.firstChild);
      stack[stack.length - 1].list.appendChild(li);
    }

    first.before(...roots);
    for (const p of run) p.remove();
    changed = true;
  }
  return changed;
}

const WHITESPACE_BLOCK_TAGS = new Set([
  "P", "DIV", "UL", "OL", "LI", "H1", "H2", "H3", "H4", "H5", "H6", "BLOCKQUOTE", "PRE", "TABLE", "TR", "TD", "TH",
  "THEAD", "TBODY", "HR",
]);

/** Parents whose text nodes are inline content of a line (not structural whitespace between tags). */
const INLINE_CONTENT_PARENTS = new Set([
  "P", "LI", "DIV", "SPAN", "B", "STRONG", "I", "EM", "U", "S", "STRIKE", "DEL", "CODE", "A", "SUB", "SUP", "FONT",
  "H1", "H2", "H3", "H4", "H5", "H6", "BLOCKQUOTE", "TD", "TH", "MARK", "SMALL",
]);

/**
 * Makes loading the editor's OWN exported HTML lossless for horizontal whitespace. The editor
 * exports runs of spaces, a leading indent and tabs raw (the per-run `white-space: pre-wrap` is
 * stripped on export; the renderers' container declares it), but HTML parsing rules collapse them
 * when the HTML is loaded back (draft restore, the HTML-source toggle). A text node (outside a
 * `<pre>`, which Lexical already keeps) that has a tab, a run of two spaces, or a space at its start
 * or end of a line, and no newline, is wrapped in a `pre-wrap` span, which Lexical's importer reads as
 * "keep as typed" (tabs become tab nodes). The span exists only in this DOM, never in the editor
 * or its exported HTML. Text with a newline keeps collapsing: that is laid-out HTML source (see
 * CLAUDE.md "Comment bodies"), not typed spaces. Used for the initial `value` only; pasted foreign
 * HTML does not go through it. Mutates `dom`; returns whether anything changed.
 */
export function preserveEditorWhitespace(dom: Document): boolean {
  const body = dom.body;
  if (!body) return false;
  const walker = dom.createTreeWalker(body, NodeFilter.SHOW_TEXT);
  const targets: Text[] = [];
  for (let n = walker.nextNode(); n; n = walker.nextNode()) {
    const text = n.textContent ?? "";
    if (text === "" || /[\r\n]/.test(text)) continue;
    // A single edge space matters only at the edge of a line (its neighbour is a <br> or nothing);
    // beside inline content Lexical keeps it anyway.
    const lineStart = !n.previousSibling || n.previousSibling.nodeName === "BR";
    const lineEnd = !n.nextSibling || n.nextSibling.nodeName === "BR";
    if (!(/\t| {2}/.test(text) || (lineStart && text.startsWith(" ")) || (lineEnd && text.endsWith(" ")))) continue;
    const parent = n.parentElement;
    if (!parent || !INLINE_CONTENT_PARENTS.has(parent.tagName) || parent.closest("pre")) continue;
    // Whitespace-only text counts only between inline content: next to a block element (or inside a
    // parent that holds blocks) it is the gap of minified or laid-out HTML, not typed spaces, and
    // wrapping it would load as a blank line.
    if (text.trim() === "") {
      if (!/\t| {2}/.test(text)) continue;
      const isBlock = (node: Node | null): boolean =>
        node !== null && node.nodeType === Node.ELEMENT_NODE && WHITESPACE_BLOCK_TAGS.has((node as Element).tagName);
      if (isBlock(n.previousSibling) || isBlock(n.nextSibling)) continue;
      if (Array.from(parent.children).some((c) => WHITESPACE_BLOCK_TAGS.has(c.tagName))) continue;
    }
    targets.push(n as Text);
  }
  for (const node of targets) {
    const span = dom.createElement("span");
    span.style.whiteSpace = "pre-wrap";
    node.replaceWith(span);
    span.appendChild(node);
  }
  return targets.length > 0;
}
