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

// Ported from the webapp's identical utility (features/csm-cases/utils/inlineImages.ts) — both
// apps talk to the same csm-portal backend, which embeds inline comment/description images the
// same way regardless of client.

// Shared "img tag with quoted/bare src" grammar for both extraction and
// replacement, so the two stay in sync as the pattern evolves. Capture groups:
// 1=before-src attrs, 2=double-quoted src, 3=single-quoted src, 4=bare src, 5=after-src attrs.
const IMG_TAG_SRC = /<img([^>]*?)\s+src\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s>]+))([^>]*)>/gi;

/**
 * Extracts the backing attachment id from an inline `<img>` `src` value. The
 * backing data source embeds inline images as `.iix`-suffixed references
 * (e.g. `.../<attachmentId>.iix`); this pulls the id out regardless of
 * whether it appears as a full path or a bare token.
 */
export function extractInlineImageRefId(src: string): string {
  const s = src.trim();
  const fromPath = s.match(/\/([a-f0-9]{32})\.iix(?:\?|#|$)/i);
  if (fromPath) return fromPath[1];
  const tail =
    s
      .replace(/\.iix$/i, "")
      .split("/")
      .pop()
      ?.trim() ?? "";
  if (/^[a-f0-9]{32}$/i.test(tail)) return tail;
  return s
    .replace(/^\//, "")
    .replace(/\.iix$/i, "")
    .trim();
}

/**
 * Formats a 32-character hex id (no hyphens) as a canonical UUID
 * (`8-4-4-4-12`) — the shape the backend's `/attachments/{id}/content`
 * endpoint requires. Ids already in another shape are returned unchanged.
 */
export function compactIdToUuid(id: string): string {
  if (!/^[a-f0-9]{32}$/i.test(id)) return id;
  return `${id.slice(0, 8)}-${id.slice(8, 12)}-${id.slice(12, 16)}-${id.slice(16, 20)}-${id.slice(20, 32)}`;
}

// A comment/description with an unreasonable number of distinct inline images would otherwise
// fire one parallel authenticated Blob fetch per image (see useResolvedInlineImageHtml) — this
// caps how many unique ids are ever collected, so that stays bounded regardless of how much HTML
// is thrown at it. Ids past the cap are simply never extracted, so replaceInlineImageSrcs treats
// them the same as any other unresolved reference (stripped, not left pointing at an auth-gated
// URL the WebView can't load).
const MAX_INLINE_IMAGES = 20;

/** Extracts every attachment id referenced by a `.iix` `<img>` src within an HTML string. */
export function extractIixAttachmentIds(html: string): string[] {
  const ids = new Set<string>();
  let match;
  IMG_TAG_SRC.lastIndex = 0;
  while (ids.size < MAX_INLINE_IMAGES && (match = IMG_TAG_SRC.exec(html)) !== null) {
    const src = match[2] ?? match[3] ?? match[4] ?? "";
    if (src.includes(".iix")) {
      const id = extractInlineImageRefId(src);
      if (id) ids.add(id);
    }
  }
  return Array.from(ids);
}

/**
 * Replaces every `.iix` `<img>` src in `html` with its resolved data URL from
 * `dataUrls`. A `.iix` reference with no matching entry is stripped (rather
 * than left pointing at an auth-gated URL the browser cannot fetch).
 */
export function replaceInlineImageSrcs(html: string, dataUrls: Map<string, string>): string {
  return html.replace(IMG_TAG_SRC, (fullMatch, before, doubleSrc, singleSrc, bareSrc, after) => {
    const src = (doubleSrc ?? singleSrc ?? bareSrc ?? "") as string;
    if (!src.includes(".iix")) return fullMatch;
    const refId = extractInlineImageRefId(src);
    const dataUrl = dataUrls.get(refId);
    const quote = doubleSrc !== undefined ? '"' : singleSrc !== undefined ? "'" : '"';
    if (!dataUrl) {
      return `<img${before} src=${quote}${quote} data-unresolved="true"${after}>`;
    }
    return `<img${before} src=${quote}${dataUrl}${quote}${after}>`;
  });
}
