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

// Turns a comment's text from the case activity feed into HTML the timeline can show. The feed sends
// what the author typed: plain text, or HTML, sometimes with [code] blocks. Mirrors the webapp's
// handling (apps/customer-portal/webapp/src/features/support/utils/support.ts) in a smaller form.

const ANY_HTML_TAG = /<[a-z][\s\S]*>/i;
const CODE_BLOCK = /\[\\?code\]([\s\S]*?)\[\\?\/code\]/gi;
const STRAY_CODE_TAG = /\[\\?\/?code\]/gi;
const CUSTOMER_COMMENT_ADDED = /<p>\s*Customer comment added\s*<\/p>|Customer comment added/gi;

function escapeHtml(text: string): string {
  return text
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#39;");
}

function plainTextToHtml(text: string): string {
  return escapeHtml(text)
    .split(/\r\n|\r|\n/)
    .join("<br>");
}

export function commentContentToHtml(content: string | null | undefined): string {
  if (!content) return "";
  const text = content.replace(CUSTOMER_COMMENT_ADDED, "").trim();
  if (!text) return "";

  if (/\[\\?\/?code\]/i.test(text)) {
    // Text around a [code] block is plain; the block itself is kept exactly as written.
    let html = "";
    let last = 0;
    for (const match of text.matchAll(CODE_BLOCK)) {
      html += plainTextToHtml(text.slice(last, match.index));
      html += `<code>${escapeHtml(match[1])}</code>`;
      last = match.index + match[0].length;
    }
    return (html + plainTextToHtml(text.slice(last).replace(STRAY_CODE_TAG, "\n"))).trim();
  }

  // Already HTML: shown as is (the page sanitises it when it renders).
  if (ANY_HTML_TAG.test(text)) return text;

  return plainTextToHtml(text);
}
