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

import { sanitizeRichTextHtmlKeepingLinkTargets } from "@utils/sanitizeHtml";

/**
 * A transform applied to the parsed (inert) document body before the HTML is
 * serialised and sanitised. Transforms edit DOM nodes — they never build or
 * rewrite HTML strings — so text and attribute values can never be
 * re-interpreted as markup.
 */
export type HtmlDomTransform = (body: HTMLElement) => void;

/**
 * Parses `html` into an inert document (no scripts run, no resources load),
 * applies each transform in order to its body, and serialises the result.
 *
 * The returned string is **not** sanitised. Use {@link renderTrustedHtml} for
 * anything that reaches `dangerouslySetInnerHTML`.
 */
export function transformHtml(
  html: string,
  transforms: readonly HtmlDomTransform[],
): string {
  if (!html || typeof html !== "string") return html;
  const doc = new DOMParser().parseFromString(
    `<!doctype html><html><body>${html}</body></html>`,
    "text/html",
  );
  for (const transform of transforms) transform(doc.body);
  return doc.body.innerHTML;
}

/**
 * The single entry point for rendering backend rich-text HTML with in-app
 * decorations (link markers, auto-linked URLs). Order is fixed: DOM
 * transforms first, sanitisation LAST, so nothing a transform emits — and
 * nothing a regex could mis-split inside an attribute value — reaches the
 * DOM unsanitised.
 */
export function renderTrustedHtml(
  html: string,
  transforms: readonly HtmlDomTransform[] = [],
): string {
  if (!html || typeof html !== "string") return html ?? "";
  return sanitizeRichTextHtmlKeepingLinkTargets(transformHtml(html, transforms));
}

/**
 * Walks every text node under `root` (skipping any whose ancestor matches
 * `skipSelector`) and replaces it with the nodes `replacer` returns for its
 * text; `null` leaves the node untouched.
 */
export function replaceTextNodes(
  root: HTMLElement,
  skipSelector: string,
  replacer: (text: string, doc: Document) => Node[] | null,
): void {
  const doc = root.ownerDocument;
  const walker = doc.createTreeWalker(root, NodeFilter.SHOW_TEXT);
  const textNodes: Text[] = [];
  for (let node = walker.nextNode(); node; node = walker.nextNode()) {
    textNodes.push(node as Text);
  }
  for (const textNode of textNodes) {
    if (textNode.parentElement?.closest(skipSelector)) continue;
    const replacement = replacer(textNode.data, doc);
    if (replacement) textNode.replaceWith(...replacement);
  }
}

/**
 * Splits `text` on every match of the (global) `pattern`, producing text nodes
 * for the gaps and `build(match)` nodes for the matches. `null` when nothing
 * matched.
 */
export function splitTextByPattern(
  text: string,
  pattern: RegExp,
  doc: Document,
  build: (match: RegExpExecArray) => Node,
): Node[] | null {
  const re = new RegExp(pattern.source, pattern.flags.includes("g") ? pattern.flags : `${pattern.flags}g`);
  const out: Node[] = [];
  let last = 0;
  for (let match = re.exec(text); match; match = re.exec(text)) {
    if (match[0] === "") {
      re.lastIndex += 1;
      continue;
    }
    if (match.index > last) out.push(doc.createTextNode(text.slice(last, match.index)));
    out.push(build(match));
    last = match.index + match[0].length;
  }
  if (out.length === 0) return null;
  if (last < text.length) out.push(doc.createTextNode(text.slice(last)));
  return out;
}

/**
 * Replaces every `<a href>` whose href matches `hrefPattern` with the node
 * `build` returns for it (the anchor's own children are available to reuse).
 */
export function replaceMatchingAnchors(
  root: HTMLElement,
  hrefPattern: RegExp,
  build: (anchor: HTMLAnchorElement, match: RegExpExecArray) => Node,
): void {
  const re = new RegExp(hrefPattern.source, hrefPattern.flags.replace("g", ""));
  root.querySelectorAll<HTMLAnchorElement>("a[href]").forEach((anchor) => {
    const match = re.exec(anchor.getAttribute("href") ?? "");
    if (match) anchor.replaceWith(build(anchor, match));
  });
}
