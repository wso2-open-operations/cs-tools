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

/** Tags that mean the clipboard HTML is rendered, structured content (not a plain code view). */
const STRUCTURED_HTML = "li, ol, ul, table, h1, h2, h3, h4, h5, h6, a[href], strong, b, em, i, blockquote, img";

/**
 * Whether clipboard `text/html` is "flat": only `div`, `span`, `br`, `pre`, `code`
 * and the like, with styles, as VS Code and similar code views send. For such a
 * paste the `text/plain` is the real source (possibly Markdown). HTML with lists,
 * tables, headings, links, bold/italic, quotes or images is rendered content
 * (Word, Gmail, a web page, a chat's rendered answer), whose `text/plain` is only
 * its rendered text, so it keeps the Keep/Remove Formatting prompt; so does Word's
 * list markup (`MsoListParagraph`, `mso-` styles). Empty HTML is flat.
 */
export function isFlatClipboardHtml(html: string): boolean {
  if (!html.trim()) return true;
  const dom = new DOMParser().parseFromString(html, "text/html");
  if (dom.body.querySelector(STRUCTURED_HTML) !== null) return false;
  // Word lists are paragraphs (`class=MsoListParagraph`, `mso-list:` styles), not `li`.
  return dom.body.querySelector('[class*="MsoList"], [style*="mso-"]') === null;
}
