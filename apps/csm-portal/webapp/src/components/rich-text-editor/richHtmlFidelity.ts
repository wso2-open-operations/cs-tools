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

/**
 * Whether the rich editor would destroy styling in `html` if it were loaded
 * into it.
 *
 * The rich editor (Lexical) rebuilds its content from the nodes it
 * understands: on load it turns every `<div>` into a paragraph, drops
 * `<span>` wrappers, and throws away every inline `style` it has no node for
 * (a bordered, padded, gradient box; a coloured link). It then emits the
 * simplified HTML through `onChange` straight away, so merely OPENING such
 * content in the rich view silently replaces the stored markup with a plainer
 * version, and the request dialog then reports "unsaved changes" and will not
 * submit or publish until that plainer version is saved. Content authored
 * elsewhere (for example the ServiceNow-era security bulletin layout: a styled
 * box holding one row per advisory) therefore loses its look the moment someone
 * opens the request.
 *
 * This reports when that is about to happen, so the caller can keep the exact
 * HTML in source mode instead. It only flags styling that really would be lost:
 * - `white-space`, `text-align` and `padding-inline-start` (indentation) are
 *   what the editor itself emits, and the `style` it writes on every `<img>`
 *   is its own too, so content it wrote never trips this (a test feeds the
 *   editor's real output through here to keep it that way);
 * - so is `list-style-type` on a nested-list wrapper item (`li.editor-list-item-nested`);
 * - table elements are skipped because the editor deliberately keeps their
 *   styling across a round trip (see `stripLexicalTableStylingFromHtml`);
 * - a `<div>` with no styling only becomes a paragraph, which loses structure
 *   but no appearance, so it is left alone.
 */
const TABLE_ELEMENTS = new Set([
  "table",
  "thead",
  "tbody",
  "tfoot",
  "tr",
  "th",
  "td",
  "col",
  "colgroup",
  "caption",
]);

/** Inline style properties the rich editor writes itself, so they never count as lost. */
const EDITOR_OWN_STYLE_PROPERTIES = new Set(["white-space", "text-align", "padding-inline-start"]);

/**
 * The property names declared in an inline `style` attribute. Split by hand
 * rather than through `CSSStyleDeclaration`, which silently drops declarations
 * it cannot parse (a `linear-gradient` background, say), and respecting
 * parentheses so a `url(data:image/png;base64,...)` value is not cut at its
 * semicolon.
 */
function declaredProperties(styleAttribute: string): string[] {
  const properties: string[] = [];
  let depth = 0;
  let current = "";
  const flush = (): void => {
    const colon = current.indexOf(":");
    if (colon > 0) properties.push(current.slice(0, colon).trim().toLowerCase());
    current = "";
  };
  for (const ch of styleAttribute) {
    if (ch === "(") depth++;
    else if (ch === ")") depth = Math.max(0, depth - 1);
    if (ch === ";" && depth === 0) flush();
    else current += ch;
  }
  flush();
  return properties.filter(Boolean);
}

export function htmlNeedsSourceEditing(html: string): boolean {
  if (!html || !/style\s*=/i.test(html)) return false;
  const doc = new DOMParser().parseFromString(html, "text/html");
  for (const el of doc.body.querySelectorAll("[style]")) {
    const tag = el.tagName.toLowerCase();
    if (TABLE_ELEMENTS.has(tag)) continue;
    // The editor writes its own sizing style on every image, so an image's
    // style is never a sign the content came from somewhere else.
    if (tag === "img") continue;
    const styled = declaredProperties(el.getAttribute("style") ?? "");
    // The editor marks its nested-list wrapper item `list-style-type: none` on export.
    const nestedWrapper = tag === "li" && el.classList.contains("editor-list-item-nested");
    if (
      styled.some(
        (property) =>
          !EDITOR_OWN_STYLE_PROPERTIES.has(property) && !(nestedWrapper && property === "list-style-type"),
      )
    )
      return true;
  }
  return false;
}
