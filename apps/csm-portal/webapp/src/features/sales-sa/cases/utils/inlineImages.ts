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

// Ported from the source app's data/utils/inlineImages.ts. ServiceNow embeds
// inline comment images as <img src="/<sys_attachment sys_id>.iix">. That
// sys_id is the same identifier used by the sys_attachment table, so it can
// be resolved through the existing /attachments/{id}/download endpoint.
//
// Extraction/replacement themselves are NOT reimplemented here: they reuse
// features/csm-cases/utils/inlineImages.ts's extractIixAttachmentIds/
// extractInlineImageRefId, which parse via DOMParser rather than a regex
// over the raw markup specifically to avoid a crafted attribute (e.g. an
// `alt` containing `src="..."`-shaped text) being mistaken for the real
// `src` -- see that file's own doc comments. Case comments can come from a
// customer reply, so that's not a theoretical concern for this HTML. Only
// the placeholder rendering below is SPL's own: this hook models a fourth
// "still loading" state replaceInlineImageSrcs has no equivalent for (its
// dataUrls/deniedIds are for HTML that's already fully resolved), so it
// isn't reused as-is rather than extending that shared contract for one
// caller.
import { extractIixAttachmentIds, extractInlineImageRefId } from "@features/csm-cases/utils/inlineImages";

const RESTRICTED_PLACEHOLDER =
  '<span class="inline-attachment-placeholder inline-attachment-placeholder--restricted">' +
  '<span class="inline-attachment-placeholder-icon" aria-hidden="true">&#128274;</span>' +
  "You don't have permission to view this attachment</span>";

const ERROR_PLACEHOLDER =
  '<span class="inline-attachment-placeholder inline-attachment-placeholder--error">' +
  '<span class="inline-attachment-placeholder-icon" aria-hidden="true">&#9888;</span>' +
  "Unable to load image attachment</span>";

const LOADING_PLACEHOLDER = '<span class="inline-attachment-skeleton" aria-hidden="true"></span>';

export function extractInlineAttachmentIds(html: string): string[] {
  if (!html) return [];
  return extractIixAttachmentIds(html);
}

// Replaces every recognised inline <img> tag with its resolved data: URL, a loading
// skeleton while the fetch is still in flight, or a placeholder once it's settled -
// so the browser never issues a raw, unauthenticated request for the SN-internal
// ".iix" path against the app's own origin, and a still-loading image never
// flashes as a broken/error state.
//
// `resolved` only contains an entry once its fetch has settled: a truthy value
// means it loaded, an explicit `null` means the fetch failed. An id absent from
// the map is still in flight and renders the loading skeleton.
//
// Parses via DOMParser (see extractIixAttachmentIds's own doc comment for
// why), not a regex over the raw markup.
export function replaceInlineImageSources(
  html: string,
  resolved: Map<string, string | null>,
  unauthorized: boolean,
): string {
  if (!html) return html;
  const doc = new DOMParser().parseFromString(html, "text/html");
  for (const img of Array.from(doc.images)) {
    const src = img.getAttribute("src") ?? "";
    if (!src.includes(".iix")) continue;
    if (unauthorized) {
      img.outerHTML = RESTRICTED_PLACEHOLDER;
      continue;
    }
    const sysId = extractInlineImageRefId(src);
    if (!resolved.has(sysId)) {
      img.outerHTML = LOADING_PLACEHOLDER;
      continue;
    }
    const dataUrl = resolved.get(sysId);
    if (dataUrl) img.setAttribute("src", dataUrl);
    else img.outerHTML = ERROR_PLACEHOLDER;
  }
  return doc.body.innerHTML;
}
