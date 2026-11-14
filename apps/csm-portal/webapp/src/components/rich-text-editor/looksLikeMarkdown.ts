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
 * Whether pasted plain text is clearly Markdown source, so the editor may offer
 * "Paste as Markdown". Deliberately conservative: it must be wrong on the side of
 * "no", because a false positive interrupts a paste with a dialog while a false
 * negative only pastes plain text, as before. Any ONE of these is enough:
 *
 * - a fenced code block (``` or ~~~ on a line of its own)
 * - an ATX heading (`#` to `######`, a space, then text)
 * - two or more consecutive list lines (`- `, `* `, `+ `, `1. `, `1) `; blank
 *   lines between items are fine, a prose line breaks the run)
 * - a blockquote: two or more consecutive `> text` lines, or one followed by another
 *   Markdown signal (a lone `> app@1.0.0 build` from npm, or `>30 days`, is not one)
 * - a GFM table (a header row with pipes, then a `|---|---|` row)
 * - `**bold**` (not glued to a word, so `2**3**4` is not bold) or `__bold text__`
 *   (a space inside, so a Python `__init__` is not bold)
 * - `[text](url)` where the url is http(s), mailto:, `./`, `../`, `/` or `#anchor`
 * - `~~strike~~`
 *
 * Not enough on their own: a lone `*italic*`, a single backtick span, `#123`, a
 * single "1. step" line, a Windows path with underscores, e-mail addresses, bare
 * URLs, `foo[0](x/y)`, stack traces and log lines.
 *
 * Every rule is linear or bounded: only the first {@link MAX_SCANNED_CHARS}
 * characters are judged (a Markdown signal shows up early), lines are cut at
 * {@link MAX_SCANNED_LINE} and every open-ended quantifier has an upper bound.
 */
const MAX_SCANNED_CHARS = 20_000;
const MAX_SCANNED_LINE = 1_000;

const FENCE = /^ {0,3}(?:`{3,}|~{3,})[\w+#.-]*[ \t]*$/;
const HEADING = /^ {0,3}#{1,6}[ \t]+\S/;
const LIST_LINE = /^ {0,6}(?:[-*+]|\d{1,3}[.)])[ \t]+\S/;
const BLOCKQUOTE = /^ {0,3}>[ \t]?\S/;
const TABLE_SEPARATOR = /^[ \t]*\|?[ \t]*:?-{3,}:?[ \t]*(?:\|[ \t]*:?-{3,}:?[ \t]*)+\|?[ \t]*$/;
// Not glued to a word or a slash: `**/node_modules/**` and `src/**/test/**` are globs, not bold.
const STAR_BOLD = /(?<![\w*/])\*\*(?=[^\s*/])[^*\n]{1,200}?(?<=[^\s*/])\*\*(?![\w*/])/;
// Words (no whitespace, no underscore) separated by blanks: unambiguous, so no backtracking blow-up.
const UNDERSCORE_BOLD = /(?<![\w_])__[^\s_]{1,100}(?:[ \t]+[^\s_]{1,100}){1,20}__(?![\w_])/;
const LINK = /\[[^\][\n]{1,200}\]\((?:https?:\/\/|mailto:|\.{1,2}\/|\/|#)[^)\s]{0,500}\)/;
const STRIKE = /~~[^\s~][^~\n]{0,200}(?<=[^\s~])~~/;

export function looksLikeMarkdown(input: string): boolean {
  const text = input.slice(0, MAX_SCANNED_CHARS);
  if (!text.trim()) return false;
  if (STAR_BOLD.test(text) || UNDERSCORE_BOLD.test(text) || STRIKE.test(text) || LINK.test(text)) {
    return true;
  }

  let listRun = 0;
  let quoteRun = 0;
  let previous = "";
  let quoteSeen = false;
  let listSeen = false;
  for (const raw of text.split(/\r?\n/)) {
    const line = raw.slice(0, MAX_SCANNED_LINE);
    if (FENCE.test(line) || HEADING.test(line)) return true;
    if (BLOCKQUOTE.test(line)) {
      if (++quoteRun >= 2 || listSeen) return true;
      quoteSeen = true;
    } else {
      quoteRun = 0;
    }
    if (previous.includes("|") && TABLE_SEPARATOR.test(line)) return true;
    if (LIST_LINE.test(line)) {
      if (quoteSeen) return true; // a quote line plus a list line
      listSeen = true;
      if (++listRun >= 2) return true;
    } else if (line.trim() !== "") {
      listRun = 0;
    }
    previous = line;
  }
  return false;
}
