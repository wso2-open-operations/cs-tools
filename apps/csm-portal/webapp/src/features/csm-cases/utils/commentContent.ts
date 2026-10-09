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
  const codeBlockCount = raw.match(/\[code\]/gi)?.length ?? 0;
  const stripped = hasSingleCodeWrapper(raw)
    ? stripCodeWrapper(raw)
    : codeBlockCount > 1
      ? stripAllCodeBlocks(raw)
      : convertCodeTagsToHtml(raw);
  const withoutLabel = stripCustomerCommentAddedLabel(stripped);
  const textOnly = withoutLabel.replace(/<[^>]+>/g, "").trim();
  if (textOnly.length > 0) return true;
  return /<img\b/i.test(withoutLabel);
}

/**
 * Records raised from a GitHub issue are numbered by entity-service's own
 * sequences (`next_github_service_request_number` → `SR-GH-000001`, the
 * change-request one → `CHG-GH-000001`), so the prefix is how the frontend
 * tells them apart — the case API carries no other GitHub marker.
 */
const GITHUB_RAISED_NUMBER = /^(SR|CHG)-GH-/i;

/**
 * True when a record was raised from a GitHub issue, whose description is the
 * issue body verbatim: GitHub Markdown (`### Heading` sections from the issue
 * template), not rich-text HTML.
 */
export function isGithubRaisedCaseNumber(caseNumber: string | undefined | null): boolean {
  return GITHUB_RAISED_NUMBER.test(caseNumber ?? "");
}

/**
 * True when a comment's body is Markdown rather than rich-text HTML: chatbot
 * (Novera) messages, and entries explicitly marked `bodyFormat: "markdown"`
 * (the description of a GitHub-raised record).
 */
export function isMarkdownComment(comment: CsmCaseComment): boolean {
  return comment.authorRole === "chatbot" || comment.bodyFormat === "markdown";
}

// Block-level and line-break elements. Whitespace next to one of these is never
// content: a block starts its own line whatever surrounds it.
const BLOCK_TAGS =
  "p|div|ul|ol|li|dl|dt|dd|table|thead|tbody|tfoot|tr|th|td|caption|h[1-6]|blockquote|pre|br|hr";
const HTML_STRUCTURE_TAG = new RegExp(`<(?:${BLOCK_TAGS})\\b[^<>]*>`, "i");
// A newline with the indentation and blank space around it.
const NEWLINE_WHITESPACE = "[ \\t]*[\\r\\n][ \\t\\r\\n]*";
// `<pre>`/`<code>` content is whitespace-sensitive by design (a snippet's own
// line breaks and indentation), so it is matched whole and left untouched.
const PRESERVED_ELEMENT = "(?<kept><(?<tag>pre|code)\\b[\\s\\S]*?<\\/\\k<tag>\\s*>)";
const NEWLINE_AROUND_BLOCK = new RegExp(
  `${PRESERVED_ELEMENT}|${NEWLINE_WHITESPACE}(?=<\\/?(?:${BLOCK_TAGS})\\b)|(?<=<\\/?(?:${BLOCK_TAGS})\\b[^<>]*>)${NEWLINE_WHITESPACE}`,
  "gi",
);
const NEWLINE_IN_TEXT = new RegExp(`${PRESERVED_ELEMENT}|${NEWLINE_WHITESPACE}`, "gi");

/**
 * Removes the whitespace that only exists because HTML *source* was laid out
 * for reading — the newlines and indentation between `<ul>`, `<li>`, `<p>` and
 * `<br>`, and the line wraps inside a paragraph — so it is not printed.
 *
 * The comment container keeps `white-space: pre-wrap` (editor-authored comments
 * no longer carry their own per-run `pre-wrap`, and plain-text notes rely on it
 * for their line breaks), which prints every one of those newlines literally:
 * blank gaps between bullets, ragged indented continuation lines. A browser, and
 * ServiceNow, treat a newline in HTML source as an ordinary space.
 *
 * Deliberately narrow, so nothing else changes:
 * - Only content with real block/`<br>` markup is touched; plain text keeps its
 *   newlines (they are its line breaks).
 * - Only whitespace that contains a newline is touched; runs of spaces typed in
 *   the editor are kept.
 * - `<pre>` and `<code>` content is never touched, so code snippets keep their
 *   line breaks and indentation.
 * - A newline inside text becomes a space only when the source also has a
 *   newline next to a block tag (the sign of laid-out source), so a note that
 *   mixes a stray `<br>` with intentional line breaks keeps them.
 */
export function collapseHtmlSourceWhitespace(html: string): string {
  if (!html || !HTML_STRUCTURE_TAG.test(html)) return html;
  let sawLaidOutSource = false;
  const withoutEdgeNewlines = html
    .replace(/^[ \t]*[\r\n][ \t\r\n]*/, "")
    .replace(/[ \t\r\n]*[\r\n][ \t]*$/, "")
    .replace(NEWLINE_AROUND_BLOCK, (match, ...args) => {
      const groups = args[args.length - 1] as { kept?: string };
      if (groups.kept !== undefined) return match;
      sawLaidOutSource = true;
      return "";
    });
  if (!sawLaidOutSource) return withoutEdgeNewlines;
  return withoutEdgeNewlines.replace(NEWLINE_IN_TEXT, (match, ...args) => {
    const groups = args[args.length - 1] as { kept?: string };
    return groups.kept !== undefined ? match : " ";
  });
}

/**
 * {@link collapseHtmlSourceWhitespace} applied to the inside of each
 * `[code]...[/code]` block and, separately, to the text around the blocks. ServiceNow's
 * `[code]` marks a stretch as raw HTML; text outside it is normally plain, and
 * plain text is left alone by the helper (its newlines are line breaks), but HTML
 * source laid out outside a block is cleaned the same way as inside one. The
 * markers themselves are kept exactly as written, legacy escaped ones
 * (`[\code]`, `[\/code]`) included, so the unwrapping functions see the same
 * input they always did.
 */
export function collapseCodeBlockWhitespace(content: string): string {
  const codeBlock = /(\[\\?code\])([\s\S]*?)(\[\\?\/code\])/gi;
  const collapseBlock = (_match: string, open: string, inner: string, close: string): string =>
    `${open}${collapseHtmlSourceWhitespace(inner)}${close}`;
  // Each block is swapped for a placeholder so the text around it is cleaned in
  // one pass with the right context (a newline on either side of a block is
  // judged against its neighbours, not cut off at the marker) and so markup
  // inside a block never decides whether the outside is laid-out HTML.
  if (/[\uE000\uE001]/.test(content)) return content.replace(codeBlock, collapseBlock);
  const blocks: string[] = [];
  const masked = content.replace(codeBlock, (...args) => {
    blocks.push(collapseBlock(...(args as [string, string, string, string])));
    return `\uE000${blocks.length - 1}\uE001`;
  });
  return collapseHtmlSourceWhitespace(masked).replace(
    /\uE000(\d+)\uE001/g,
    (_match, index: string) => blocks[Number(index)],
  );
}

/**
 * Cleans the layout whitespace out of a raw comment body before its `[code]`
 * markers are unwrapped: inside each `[code]` block when the body has any, else
 * across the whole body. See {@link collapseHtmlSourceWhitespace}.
 */
export function collapseCommentSourceWhitespace(content: string): string {
  if (!content || typeof content !== "string") return "";
  return /\[\\?\/?code\]/i.test(content)
    ? collapseCodeBlockWhitespace(content)
    : collapseHtmlSourceWhitespace(content);
}

/**
 * Cleans a comment's raw `bodyHtml` the same way `CsmCaseCommentBubble`
 * renders it — unwraps `[code]` wrapper tags into real HTML (or renders
 * bot/chatbot Markdown to HTML), drops the newlines and indentation of
 * laid-out HTML source (see {@link collapseHtmlSourceWhitespace}) and strips
 * the backend's "Customer comment added" label. The bubble calls this itself,
 * and so does any consumer that needs the same cleaned-up content without
 * rendering the bubble (e.g. the PDF report generators, which turn the result
 * into plain text via `stripHtmlTags`) — reported live when the PDF export
 * instead showed the raw, unstripped "Customer comment added" label as if it
 * were the comment's actual content.
 */
export function preprocessCommentBodyHtml(comment: CsmCaseComment): string {
  if (isMarkdownComment(comment)) return markdownToHtml(comment.bodyHtml);
  const raw = collapseCommentSourceWhitespace(comment.bodyHtml ?? "");
  const isFullCodeWrap = hasSingleCodeWrapper(raw);
  const codeBlockCount = raw.match(/\[code\]/gi)?.length ?? 0;
  const afterCode = isFullCodeWrap
    ? stripCodeWrapper(raw)
    : codeBlockCount > 1
      ? stripAllCodeBlocks(raw)
      : convertCodeTagsToHtml(raw);
  return stripCustomerCommentAddedLabel(afterCode);
}

/**
 * Replaces bare URLs in an HTML string (not already inside an href attribute)
 * with clickable anchor tags that open in a new tab.
 */
export function linkifyBareUrls(html: string): string {
  // The lookahead is wrapped in `(?=(...))\1` (an atomic-group emulation)
  // rather than matched directly, because a plain trailing negative lookahead
  // lets the greedy URL quantifier backtrack to a shorter match that
  // satisfies the lookahead — silently truncating the linkified URL (e.g.
  // matching "example.co" instead of "example.com" right before `</a>`).
  return html.replace(
    /(?<!href=["'])(?=(https?:\/\/[^\s<>"']+))\1(?!["']?\s*<\/a>)/g,
    '<a href="$1" target="_blank" rel="noopener noreferrer" style="color:inherit;text-decoration:underline;word-break:break-all;">$1</a>',
  );
}

/**
 * Whether at least one **customer-visible** comment (`internal` falsy) with
 * real displayable content exists on the case. Used to gate the "no public
 * comment yet" confirmation before a WIP case moves to Awaiting info or
 * Solution proposed — an internal-only work note, or a comment that strips
 * down to nothing renderable (see {@link hasDisplayableContent}), doesn't
 * count: the customer would still have no explanation for the transition.
 * `undefined`/empty `comments` (still loading, or a case with none yet)
 * correctly returns `false` rather than throwing.
 */
export function hasPublicComment(
  comments: CsmCaseComment[] | undefined,
): boolean {
  if (!comments) return false;
  return comments.some(
    (comment) => !comment.internal && hasDisplayableContent(comment),
  );
}
