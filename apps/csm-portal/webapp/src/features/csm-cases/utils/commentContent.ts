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

import type { CsmCaseComment } from "@features/csm-cases/types/csmCases";

/**
 * Converts legacy `[code]...[/code]` wrapper tags (some backing data sources
 * carry these instead of real `<code>` elements) into `<code>` elements.
 */
export function convertCodeTagsToHtml(content: string): string {
  if (!content || typeof content !== "string") return "";
  const normalized = content
    .replace(/\[\\\/code\]/gi, "[/code]")
    .replace(/\[\\\/CODE\]/g, "[/code]")
    .replace(/\[\\code\]/gi, "[code]")
    .replace(/\[\\CODE\]/g, "[code]")
    .replace(/\[\/code\]\s*\[code\]/gi, "[/code]\n[code]");
  return normalized
    .replace(/\[code\]([\s\S]*?)\[\/code\]/gi, "<code>$1</code>")
    .replace(/\[\/?code\]/gi, "\n");
}

/**
 * Strips all [code]...[/code] blocks and returns concatenated inner HTML.
 * Used for multi-block content to avoid grey <code> background on structured sections.
 *
 * @param content - Raw content with one or more [code]...[/code] blocks.
 * @returns {string} Inner HTML without code wrappers.
 */
export function stripAllCodeBlocks(content: string): string {
  if (!content || typeof content !== "string") return "";
  const normalized = content
    .replace(/\[\\\/code\]/gi, "[/code]")
    .replace(/\[\\\/CODE\]/g, "[/code]")
    .replace(/\[\\code\]/gi, "[code]")
    .replace(/\[\\CODE\]/g, "[code]")
    .replace(/\[\/code\]\s*\[code\]/gi, "[/code]\n[code]");
  return normalized
    .replace(/\[code\]([\s\S]*?)\[\/code\]/gi, "$1\n")
    .replace(/\[\/?code\]/gi, "\n");
}

/**
 * Removes leading <br>, <br/>, <br /> and whitespace from HTML.
 * Fixes extra blank first line from content like "[code]<br><b>...</b>[/code]".
 *
 * @param html - HTML string.
 * @returns {string} HTML with leading br/whitespace removed.
 */
export function trimLeadingBr(html: string): string {
  if (!html || typeof html !== "string") return "";
  return html.replace(/^(\s*<br\s*\/?>\s*)+/i, "").trimStart();
}

/**
 * Returns true if content has exactly one top-level [code]...[/code] wrapper
 * (no multiple [code] blocks). Used to decide between stripCodeWrapper and convertCodeTagsToHtml.
 *
 * @param content - Raw content string.
 * @returns {boolean} True when single wrapper.
 */
export function hasSingleCodeWrapper(content: string): boolean {
  if (!content || typeof content !== "string") return false;
  const trimmed = content.trim();
  if (!trimmed.startsWith("[code]") || !trimmed.endsWith("[/code]")) {
    return false;
  }
  const codeOpen = trimmed.match(/\[code\]/gi);
  const codeClose = trimmed.match(/\[\/code\]/gi);
  return (codeOpen?.length ?? 0) === 1 && (codeClose?.length ?? 0) === 1;
}

/**
 * Strips the [code]...[/code] wrapper from comment content.
 * Only strips when there is exactly one wrapper (use hasSingleCodeWrapper first).
 *
 * @param content - Raw content string.
 * @returns {string} Content without the code wrapper.
 */
export function stripCodeWrapper(content: string): string {
  if (!content || typeof content !== "string") return "";
  const trimmed = content.trim();
  if (!hasSingleCodeWrapper(content)) return content;
  return trimmed.slice(6, -7).trim();
}

/**
 * Strips "Customer comment added" label from comment content.
 * The backing data source may append this; we hide it from the activity timeline.
 *
 * @param html - HTML content string.
 * @returns {string} Content without the label.
 */
export function stripCustomerCommentAddedLabel(html: string): string {
  if (!html || typeof html !== "string") return "";
  return html
    .replace(/<p>\s*Customer comment added\s*<\/p>/gi, "")
    .replace(/Customer comment added/gi, "")
    .trim();
}

/**
 * Returns true if the comment has content worth displaying (after stripping code wrapper and
 * "Customer comment added" label). Used to hide backend entries that render as empty bubbles.
 *
 * @param comment - Case comment from the API.
 * @returns {boolean} True when comment has non-empty displayable content.
 */
export function hasDisplayableContent(comment: CsmCaseComment): boolean {
  const raw = comment.bodyHtml ?? "";
  const stripped = stripCodeWrapper(raw);
  const withoutLabel = stripCustomerCommentAddedLabel(stripped);
  const textOnly = withoutLabel.replace(/<[^>]+>/g, "").trim();
  if (textOnly.length > 0) return true;
  return /<img\b/i.test(withoutLabel);
}

/**
 * Replaces bare URLs in an HTML string (not already inside an href attribute)
 * with clickable anchor tags that open in a new tab.
 */
export function linkifyBareUrls(html: string): string {
  return html.replace(
    /(?<!href=["'])(https?:\/\/[^\s<>"']+)/g,
    '<a href="$1" target="_blank" rel="noopener noreferrer" style="color:inherit;text-decoration:underline;word-break:break-all;">$1</a>',
  );
}
