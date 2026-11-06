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

const BLOCK_SELECTOR =
  "p,div,li,br,h1,h2,h3,h4,h5,h6,tr,td,th,pre,blockquote,ul,ol,table,section,article,hr";

/**
 * Plain text of an HTML article body.
 *
 * Parsed with DOMParser (an inert document: no scripts run, no images load)
 * and read back as textContent, so entities are decoded and the text of
 * <style>, <script> and similar non-content elements is dropped rather than
 * leaking into the snippet the way regex tag-stripping lets it.
 */
export function toPlainText(html: string): string {
  if (!html) return "";
  const doc = new DOMParser().parseFromString(html, "text/html");
  doc.querySelectorAll("script,style,noscript,template,head").forEach((el) => el.remove());
  // textContent glues adjacent blocks together ("<p>a</p><p>b</p>" -> "ab").
  doc.body.querySelectorAll(BLOCK_SELECTOR).forEach((el) => {
    el.after(doc.createTextNode(" "));
  });
  return (doc.body.textContent ?? "").replace(/\s+/g, " ").trim();
}

export function toSnippet(html: string, max = 180): string {
  const plain = toPlainText(html);
  if (plain.length <= max) return plain;
  // Cut on a code point boundary so an emoji is not split in half.
  const chars = Array.from(plain);
  if (chars.length <= max) return plain;
  return `${chars.slice(0, max).join("").trimEnd()}...`;
}
