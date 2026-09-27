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

// Ported verbatim from the source app's data/utils/inlineImages.ts.
// ServiceNow embeds inline comment images as <img src="/<sys_attachment sys_id>.iix">.
// That sys_id is the same identifier used by the sys_attachment table, so it can be
// resolved through the existing /attachments/{id}/download endpoint.
const INLINE_IMG_TAG_REGEX = /<img\b[^>]*\bsrc="[^"]*\/([a-f0-9]{32})\.iix[^"]*"[^>]*>/gi;

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
  const ids = new Set<string>();
  const regex = new RegExp(INLINE_IMG_TAG_REGEX);
  let match: RegExpExecArray | null;
  while ((match = regex.exec(html)) !== null) {
    ids.add(match[1]);
  }
  return Array.from(ids);
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
export function replaceInlineImageSources(
  html: string,
  resolved: Map<string, string | null>,
  unauthorized: boolean,
): string {
  if (!html) return html;
  return html.replace(INLINE_IMG_TAG_REGEX, (fullTag, sysId) => {
    if (unauthorized) return RESTRICTED_PLACEHOLDER;
    if (!resolved.has(sysId)) return LOADING_PLACEHOLDER;
    const dataUrl = resolved.get(sysId);
    if (dataUrl) return fullTag.replace(/\ssrc="[^"]*"/i, ` src="${dataUrl}"`);
    return ERROR_PLACEHOLDER;
  });
}
