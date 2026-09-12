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
 * Strips light/pastel inline background declarations from style attributes so
 * dark-mode containers don't end up with washed-out, low-contrast backgrounds
 * (default dark-mode text is light, so any sufficiently light background —
 * not just near-white — reads poorly against it; a ServiceNow call note with
 * e.g. a light pastel teal background is a real example that a pure-white-only
 * check misses). Everything else (code-block backgrounds, borders, shadows,
 * text colors) is intentionally left untouched so light-mode and structural
 * styling stay intact.
 *
 * @param html - Raw HTML string.
 * @returns HTML with light background declarations removed.
 */
export function stripLightModeInlineStyles(html: string): string {
  return html.replace(
    /style\s*=\s*(["'])([\s\S]*?)\1/gi,
    (_match, quote: string, styleContent: string) => {
      const declarations = styleContent.split(";");
      const filtered = declarations.filter((decl) => {
        const normalized = decl.toLowerCase().replace(/\s+/g, " ").trim();
        if (!normalized) return false;
        if (/^background(-color)?\s*:/.test(normalized) && isLightBackground(normalized))
          return false;
        if (/^color\s*:/.test(normalized) && isDarkColor(normalized))
          return false;
        return true;
      });
      const cleaned = filtered.join(";").replace(/;+$/, "").trim();
      if (!cleaned) return "";
      return `style=${quote}${cleaned}${quote}`;
    },
  );
}

// Small set of named CSS colors that show up in ServiceNow-authored HTML
// backgrounds; not a full CSS color table, just enough to mirror the parsing
// coverage (hex3/hex6/rgb/named) already used for the dark-text-color check.
const NAMED_BACKGROUND_COLORS: Record<string, [number, number, number]> = {
  white: [255, 255, 255],
  black: [0, 0, 0],
  whitesmoke: [245, 245, 245],
  silver: [192, 192, 192],
  gainsboro: [220, 220, 220],
};

/**
 * WCAG relative luminance (0 = black, 1 = white) of an sRGB color, used to
 * catch any background light enough to wash out light dark-mode text —
 * not just backgrounds near pure white.
 */
function relativeLuminance(r: number, g: number, b: number): number {
  const [rl, gl, bl] = [r, g, b].map((c) => {
    const cs = c / 255;
    return cs <= 0.03928 ? cs / 12.92 : ((cs + 0.055) / 1.055) ** 2.4;
  });
  return 0.2126 * rl + 0.7152 * gl + 0.0722 * bl;
}

// WCAG AA minimum contrast ratio for normal-size text.
const MIN_CONTRAST_RATIO = 4.5;
// Dark-mode default text renders effectively white; used only to derive the
// background threshold below, not to special-case any particular text color.
const DARK_MODE_TEXT_LUMINANCE = 1;

// A background is stripped once its own contrast against dark-mode text would
// drop below MIN_CONTRAST_RATIO — i.e. WCAG contrast = (L_text + 0.05) /
// (L_bg + 0.05) solved for the L_bg at which that ratio equals the minimum.
// Deriving it this way (rather than an eyeballed constant) means a background
// like #808080 (luminance ~0.22, ~3.95:1 against white — below AA) is caught:
// a fixed 0.55 threshold missed it.
const LIGHT_BACKGROUND_LUMINANCE_THRESHOLD =
  (DARK_MODE_TEXT_LUMINANCE + 0.05) / MIN_CONTRAST_RATIO - 0.05;

function isLightBackground(bgDecl: string): boolean {
  const rgb = parseBackgroundColorRgb(bgDecl);
  if (!rgb) return false;
  return relativeLuminance(...rgb) > LIGHT_BACKGROUND_LUMINANCE_THRESHOLD;
}

function parseBackgroundColorRgb(bgDecl: string): [number, number, number] | null {
  const rgbMatch = bgDecl.match(
    /^background(?:-color)?\s*:\s*rgb\(\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)\s*\)\s*$/,
  );
  if (rgbMatch) {
    const [, r, g, b] = rgbMatch.map(Number);
    return [r, g, b];
  }
  const hex6Match = bgDecl.match(/^background(?:-color)?\s*:\s*#([0-9a-f]{6})\s*$/);
  if (hex6Match) {
    const hex = hex6Match[1];
    return [
      parseInt(hex.slice(0, 2), 16),
      parseInt(hex.slice(2, 4), 16),
      parseInt(hex.slice(4, 6), 16),
    ];
  }
  const hex3Match = bgDecl.match(/^background(?:-color)?\s*:\s*#([0-9a-f]{3})\s*$/);
  if (hex3Match) {
    return hex3Match[1].split("").map((c) => parseInt(c + c, 16)) as [
      number,
      number,
      number,
    ];
  }
  const namedMatch = bgDecl.match(/^background(?:-color)?\s*:\s*([a-z]+)\s*$/);
  if (namedMatch && namedMatch[1] in NAMED_BACKGROUND_COLORS) {
    return NAMED_BACKGROUND_COLORS[namedMatch[1]];
  }
  return null;
}

function isDarkColor(colorDecl: string): boolean {
  // Named dark colors
  if (/^color\s*:\s*(black|#000(000)?)\s*$/.test(colorDecl))
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

/**
 * Plain-text form of an HTML string used for a loose content comparison —
 * tags stripped, whitespace collapsed, case-folded. Not meant for display,
 * only for deciding whether two HTML snippets carry the same text.
 */
function normalizeForComparison(html: string): string {
  return stripHtmlTags(html).replace(/\s+/g, " ").trim().toLowerCase();
}

/**
 * True when `commentHtml` reproduces `descriptionHtml`'s text content —
 * e.g. an origin comment that echoes the record's description, sometimes
 * with extra wrapper text (a signature, a greeting) around it.
 *
 * Deliberately a plain substring check on normalized text, not a similarity
 * score: cheap, legible, and matches the one real shape this needs to
 * handle (an exact echo, possibly wrapped) rather than fuzzy near-matches.
 * A blank description has nothing to echo, so it counts as "reproduced"
 * (the caller should already be gating display on a non-blank description).
 * A missing/absent comment can't reproduce anything.
 */
export function isDescriptionEchoedInComment(
  descriptionHtml: string,
  commentHtml: string | undefined,
): boolean {
  const normalizedDescription = normalizeForComparison(descriptionHtml);
  if (!normalizedDescription) return true;
  if (!commentHtml) return false;
  const normalizedComment = normalizeForComparison(commentHtml);
  return normalizedComment.includes(normalizedDescription);
}

/**
 * Escape the five HTML-significant characters so a plain-text string can be
 * embedded in markup verbatim.
 *
 * Canonical implementation for the app; `components/rich-text-editor` re-exports
 * this rather than keeping its own copy, and nothing should hand-roll a third.
 */
export function escapeHtml(text: string): string {
  return text
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#039;");
}

