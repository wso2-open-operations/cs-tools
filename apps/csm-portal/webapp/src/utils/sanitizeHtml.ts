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

import DOMPurify from "dompurify";

// Harden all sanitized <a target="_blank"> links against reverse tabnabbing.
// Registered once at module load; applies to every DOMPurify.sanitize() call
// in the app. Mirrors the customer portal's equivalent hook in `utils/common.ts`.
if (typeof window !== "undefined") {
  DOMPurify.addHook("afterSanitizeAttributes", (node) => {
    if (node.tagName === "A" && node.getAttribute("target") === "_blank") {
      node.setAttribute("rel", "noopener noreferrer");
    }
  });
}

/**
 * Sanitize backend rich-text HTML (ServiceNow case comments, change-request
 * descriptions/plans, …) for safe rendering via dangerouslySetInnerHTML.
 *
 * Uses DOMPurify's default policy — no custom tag/attribute allow-list — to
 * stay aligned with the customer portal, which renders the same ServiceNow HTML
 * with bare `DOMPurify.sanitize(html)` (its `INLINE_COMMENT_HTML_PURIFY` is `{}`).
 * The defaults already strip scripts, event handlers, and `javascript:` URLs; a
 * stricter allow-list here silently dropped legitimate content (tables,
 * headings) the portal keeps.
 */
export function sanitizeRichTextHtml(html: string): string {
  return DOMPurify.sanitize(html);
}

/** DOMPurify config for backend description/body HTML: strips tables and code blocks. */
export const DESCRIPTION_PURIFY_CONFIG = {
  FORBID_TAGS: ["table", "thead", "tbody", "tfoot", "tr", "th", "td", "colgroup", "col", "code", "pre"],
  FORBID_CONTENTS: ["table", "thead", "tbody", "tfoot", "tr", "th", "td", "colgroup", "col", "code", "pre"],
};

/**
 * Sanitize a case/CR description for display: same base policy as
 * {@link sanitizeRichTextHtml} but additionally strips tables and code blocks,
 * matching the customer portal's dedicated description policy.
 */
export function sanitizeDescriptionHtml(html: string): string {
  return DOMPurify.sanitize(html, DESCRIPTION_PURIFY_CONFIG);
}

/**
 * Strips pure-white inline background declarations from style attributes so
 * dark-mode containers no longer render white boxes on a dark background.
 * Everything else (code-block backgrounds, borders, shadows, text colors) is
 * intentionally left untouched so light-mode and structural styling stay intact.
 *
 * @param html - Raw HTML string.
 * @returns HTML with pure-white background declarations removed.
 */
export function stripLightModeInlineStyles(html: string): string {
  return html.replace(
    /style\s*=\s*"([^"]*)"/gi,
    (_match, styleContent: string) => {
      const declarations = styleContent.split(";");
      const filtered = declarations.filter((decl) => {
        const normalized = decl.toLowerCase().replace(/\s+/g, " ").trim();
        if (!normalized) return false;
        if (
          /^background(-color)?\s*:\s*(#fff(fff)?|white|#f4f4f4|#f5f5f5|#f0f0f0|#f9f9f9|#f8f8f8|#fafafa|#e9e9e9)\s*$/.test(
            normalized,
          )
        )
          return false;
        if (/^background(-color)?\s*:/.test(normalized) && isNearWhiteRgb(normalized))
          return false;
        if (/^color\s*:/.test(normalized) && isDarkColor(normalized))
          return false;
        return true;
      });
      const cleaned = filtered.join(";").replace(/;+$/, "").trim();
      if (!cleaned) return "";
      return `style="${cleaned}"`;
    },
  );
}

function isNearWhiteRgb(bgDecl: string): boolean {
  const rgbMatch = bgDecl.match(
    /^background(?:-color)?\s*:\s*rgb\(\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)\s*\)\s*$/,
  );
  if (!rgbMatch) return false;
  const [, r, g, b] = rgbMatch.map(Number);
  return r > 230 && g > 230 && b > 230;
}

function isDarkColor(colorDecl: string): boolean {
  // Named dark colors
  if (/^color\s*:\s*(black|#000(000)?|#1[0-9a-f]{5}|#2[0-9a-f]{5})\s*$/.test(colorDecl))
    return true;
  // rgb(r, g, b) where all channels are below 100 (dark)
  const rgbMatch = colorDecl.match(/^color\s*:\s*rgb\(\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)\s*\)\s*$/);
  if (rgbMatch) {
    const [, r, g, b] = rgbMatch.map(Number);
    return r < 100 && g < 100 && b < 100;
  }
  // 3-digit or 6-digit hex colors that are dark (luminance heuristic)
  const hex3 = colorDecl.match(/^color\s*:\s*#([0-9a-f]{3})\s*$/);
  if (hex3) {
    const [rv, gv, bv] = hex3[1].split("").map((c) => parseInt(c + c, 16));
    return rv < 100 && gv < 100 && bv < 100;
  }
  const hex6 = colorDecl.match(/^color\s*:\s*#([0-9a-f]{6})\s*$/);
  if (hex6) {
    const rv = parseInt(hex6[1].slice(0, 2), 16);
    const gv = parseInt(hex6[1].slice(2, 4), 16);
    const bv = parseInt(hex6[1].slice(4, 6), 16);
    return rv < 100 && gv < 100 && bv < 100;
  }
  return false;
}

/** True when an HTML string has no visible content (e.g. `<p></p>`, `&nbsp;`). */
export function isBlankHtml(html: string): boolean {
  // stripHtmlTags decodes entities (see below), so a blank `&nbsp;`-only
  // paragraph comes back as an actual U+00A0 character, not the literal
  // string "&nbsp;" — match that character, not the entity text.
  return stripHtmlTags(html).replace(/\u00A0/g, " ").trim().length === 0;
}

/**
 * Strip HTML tags from free text that's meant to be rendered as plain text
 * (e.g. a case subject stored for later display), not treated as HTML.
 * Unlike {@link sanitizeRichTextHtml} (which sanitizes-but-keeps safe HTML
 * for `dangerouslySetInnerHTML`), this removes tag-like markup outright —
 * appropriate before persisting a plain-text label so a stray `<script>`
 * a customer typed into a case subject can't do anything if a future
 * change ever renders it somewhere less safe than JSX text interpolation.
 *
 * Goes through DOMPurify (real HTML parsing) rather than a `<[^>]*>` regex —
 * that regex treats any `<...>` run as a tag, so plain text like
 * `x < y > z` (comparison operators, not markup) would lose everything
 * between the angle brackets; DOMPurify's parser only removes what the
 * browser would actually treat as an element.
 *
 * `DOMPurify.sanitize` alone isn't quite enough on its own, though: its
 * output is meant to be re-inserted as HTML, so a stray `<`/`>` that
 * *isn't* part of a real tag comes back HTML-entity-encoded (`&lt;`/`&gt;`)
 * rather than as the literal character — which would then render as the
 * literal text "&lt;" once put through plain JSX interpolation instead of
 * a decoded `<`. Round-tripping through a detached element's `innerHTML` →
 * `textContent` decodes those entities back to plain characters. This is
 * safe specifically because the input to that second `innerHTML` assign is
 * already fully tag-stripped by DOMPurify — there's nothing left to parse
 * into a live element, and even if there were, `textContent` never
 * executes anything and strips whatever tags it reads back out anyway.
 */
export function stripHtmlTags(text: string): string {
  const withoutTags = DOMPurify.sanitize(text, { ALLOWED_TAGS: [] });
  const container = document.createElement("div");
  container.innerHTML = withoutTags;
  return container.textContent ?? "";
}
