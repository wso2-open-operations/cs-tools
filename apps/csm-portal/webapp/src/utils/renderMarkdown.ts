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

import MarkdownIt from "markdown-it";

/**
 * Render Markdown (e.g. Novera chat answers, which arrive as Markdown with
 * headings, tables, and fenced code) to HTML.
 *
 * `html: false` escapes any raw HTML in the source, so the only markup produced
 * comes from Markdown syntax — but always pass the result through
 * `sanitizeRichTextHtml` before injecting into the DOM, as defence in depth and
 * for consistency with how the rest of the app renders backend rich text.
 * `linkify` auto-links bare URLs; `breaks` turns single newlines into `<br>` so
 * chat messages keep their line breaks.
 */
const md: MarkdownIt = new MarkdownIt({
  html: false,
  linkify: true,
  breaks: true,
});

// Wrap tables in a scrollable container instead of making the <table> itself a
// scroll box: `display: block` on a <table> drops its implicit table/row/cell
// roles for assistive tech. The wrapper scrolls; the table keeps native display.
md.renderer.rules.table_open = () => '<div class="md-table-wrap">\n<table>\n';
md.renderer.rules.table_close = () => "</table>\n</div>\n";

export function markdownToHtml(source: string | undefined | null): string {
  return md.render(source ?? "");
}
