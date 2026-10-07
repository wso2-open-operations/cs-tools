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

// Found with sticky scans (never a backtracking pattern over the whole text), so
// the cost stays linear however many openers there are.
const THINKING_OPEN_RE = /<thinking>/gi;
const THINKING_CLOSE_RE = /<\/thinking>/gi;
// A half-written opening tag at the very end of the text (a cut-off answer).
const THINKING_PARTIAL_OPEN_RE = /<t(?:h(?:i(?:n(?:k(?:i(?:n(?:g)?)?)?)?)?)?)?$/i;
const LEADING_THINKING_RE = /^\s*<thinking>/i;

/**
 * Remove the model's `<thinking>…</thinking>` reasoning from a Novera answer.
 *
 * The agent can emit its reasoning inside the answer itself, and the customer
 * portal's backend persists the answer as-is, so a stored chat transcript can
 * contain it. Markdown rendering escapes raw HTML rather than hiding it, which
 * is why it shows up as literal text. Apply this to assistant text only — never
 * strip what a person typed.
 *
 * Removes complete blocks, everything after an opening tag that is never closed
 * (a stored answer that was cut off), and a half-written opening tag at the very
 * end. Known limits: a complete block the answer merely *mentions* (e.g. in
 * backticks) is removed too, and an unclosed mention hides the rest of the text;
 * nested blocks and a stray closing tag are left as they are.
 *
 * Mirrors `stripThinkingBlocks` in the customer portal webapp
 * (`features/support/utils/chat.ts`); keep the two in step.
 */
export function stripThinkingBlocks(text: string): string {
  if (!/<t/i.test(text)) return text;
  let kept = "";
  let pos = 0;
  for (;;) {
    THINKING_OPEN_RE.lastIndex = pos;
    const open = THINKING_OPEN_RE.exec(text);
    if (!open) {
      kept += text.slice(pos);
      break;
    }
    kept += text.slice(pos, open.index);
    THINKING_CLOSE_RE.lastIndex = open.index + open[0].length;
    const close = THINKING_CLOSE_RE.exec(text);
    if (!close) break; // never closed: the rest is reasoning
    pos = close.index + close[0].length;
  }
  const stripped = kept.replace(THINKING_PARTIAL_OPEN_RE, "");
  if (stripped === text) return text;
  // Only a block at the very start leaves a gap to tidy; whitespace anywhere
  // else is the author's (an indented code line must stay indented).
  if (!LEADING_THINKING_RE.test(text)) return stripped;
  // Drop the gap but keep the first real line's own indentation: when the gap
  // spans lines, cut up to the last newline; when it is inline, cut it all.
  const gap = /^\s*/.exec(stripped)?.[0] ?? "";
  const lastNewline = gap.lastIndexOf("\n");
  return stripped.slice(lastNewline === -1 ? gap.length : lastNewline + 1);
}
