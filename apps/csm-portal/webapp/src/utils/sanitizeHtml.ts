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
