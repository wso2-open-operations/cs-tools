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


/** An `<img>` whose src is a blob: URL (a local preview), up to its closing quote. */
const BLOB_IMG_SRC = /(<img\b[^>]*?\bsrc\s*=\s*)(["'])(blob:[^"']*)\2/gi;

/**
 * Replaces the local preview of each uploaded image with its reference,
 * `src="/<attachment id>"`, the shape the portal resolves through entity-service.
 * A blob: image with no uploaded id is left as it is, so the caller can tell
 * (see {@link hasBlobImageSource}) and refuse to send it.
 *
 * @param html - The comment HTML from the editor.
 * @param idsByBlobUrl - Attachment id of each preview URL that finished uploading.
 * @returns {string} The HTML with uploaded previews replaced.
 */
export function rewriteBlobImageSources(
  html: string,
  idsByBlobUrl: ReadonlyMap<string, string>,
): string {
  return html.replace(BLOB_IMG_SRC, (match, before: string, quote: string, url: string) => {
    const id = idsByBlobUrl.get(url);
    return id ? `${before}${quote}/${id.toLowerCase()}${quote}` : match;
  });
}

/** Whether the HTML still has an image that only exists in this page (a blob: src). */
export function hasBlobImageSource(html: string): boolean {
  BLOB_IMG_SRC.lastIndex = 0;
  const found = BLOB_IMG_SRC.test(html);
  BLOB_IMG_SRC.lastIndex = 0;
  return found;
}
