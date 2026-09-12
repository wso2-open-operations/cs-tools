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
 * File name (as written in a topic's Markdown `![alt](images/<name>)`) to the
 * actual bundled asset URL Vite assigns it (hashed, under `/assets/...` in a
 * production build). A plain relative path in the raw Markdown string can't
 * resolve on its own — the string is rendered to HTML and injected via
 * `dangerouslySetInnerHTML`, so the browser would resolve it against the
 * *page's* URL (`/help#<topic>`, all topics share the one `/help` route), not
 * against this file. `resolveHelpImages`
 * below rewrites `src="images/<name>"` to this map's real URL before the HTML
 * is sanitized and rendered, so an author only ever has to add both the image
 * file and its one entry here — never hand-compute a bundled path.
 *
 * Empty today: no topic currently embeds an image. Add
 * `import x from "../content/images/<name>"` and a `"<name>": x` entry here
 * alongside any future one that does.
 */
const HELP_IMAGE_ASSETS: Record<string, string> = {};

const IMAGE_SRC_PATTERN = /src="images\/([^"]+)"/g;

/**
 * Rewrites every `images/<name>` Markdown-image reference in already-rendered
 * topic HTML to its real bundled asset URL. Run this on the `markdownToHtml`
 * output *before* `sanitizeRichTextHtml`, so sanitization always sees the
 * final `src` value.
 *
 * An unresolvable file name (typo, or an image never added to
 * `HELP_IMAGE_ASSETS`) is left as the literal `images/<name>` string rather
 * than swapped to something else — it renders as a broken image with visible
 * alt text instead of silently pointing somewhere unrelated, which is easier
 * to notice and fix in review.
 */
export function resolveHelpImages(html: string): string {
  return html.replace(IMAGE_SRC_PATTERN, (match, fileName: string) => {
    const resolved = HELP_IMAGE_ASSETS[fileName];
    return resolved ? `src="${resolved}"` : match;
  });
}
