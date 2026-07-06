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
  return html.replace(/<[^>]*>/g, "").replace(/&nbsp;/g, " ").trim().length === 0;
}
