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

import { useMemo } from "react";
import { useAttachmentPreviews } from "@api/useAttachmentPreview";
import {
  extractInlineImageRefId,
  isInlineImageRefSrc,
} from "@features/support/utils/support";

/**
 * Extracts all `.iix`-style attachment IDs referenced in img src attributes within HTML.
 */
function extractIixAttachmentIds(html: string): string[] {
  const ids: string[] = [];
  const regex =
    /<img[^>]*?\s+src\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s>]+))[^>]*>/gi;
  let match;
  while ((match = regex.exec(html)) !== null) {
    const src = match[1] ?? match[2] ?? match[3] ?? "";
    if (isInlineImageRefSrc(src)) {
      const id = extractInlineImageRefId(src);
      if (id && !ids.includes(id)) ids.push(id);
    }
  }
  return ids;
}

/** What a customer reads in place of an image that could not be shown. */
export const IMAGE_REMOVED_MESSAGE = "This image was removed.";
export const IMAGE_UNAVAILABLE_MESSAGE = "This image is not available.";

/**
 * Replacement for an inline image that could not be loaded. Never carries the
 * original src (a ServiceNow URL the browser could not use anyway). The message
 * says whether the image was removed or just not available; the attachment id is
 * not shown, only kept in a data attribute for support to read.
 */
function unavailableImageHtml(refId: string, removed: boolean): string {
  const id = refId.replace(/[^a-zA-Z0-9-]/g, "");
  const message = removed ? IMAGE_REMOVED_MESSAGE : IMAGE_UNAVAILABLE_MESSAGE;
  return `<span data-unresolved="true" data-attachment-id="${id}" style="display:inline-block;padding:4px 8px;border:1px dashed currentColor;border-radius:4px;opacity:0.7;font-size:0.85em">${message}</span>`;
}

/**
 * Resolves inline image data URLs for all `.iix` img src references in an HTML string.
 * Fetches images via the authenticated backend and replaces src attributes with data URLs.
 *
 * @param html - Sanitized HTML string potentially containing `.iix` img src URLs.
 * @param caseId - The case this comment belongs to. Forwarded to the backend
 * as an authorization hint: an attachment pasted inline into a comment has
 * its own referenceId set to the comment's id, not the case's, so without
 * this hint the backend has no way to resolve it to a case at all and the
 * image 404s even when the caller can see the case.
 * @returns `{ resolvedHtml: string, isLoading: boolean }`
 */
export function useResolvedInlineImageHtml(
  html: string,
  caseId?: string | null,
): { resolvedHtml: string; isLoading: boolean } {
  const idsFromHtml = useMemo(() => extractIixAttachmentIds(html), [html]);

  const { dataUrls, removedIds, isLoading } = useAttachmentPreviews(idsFromHtml, caseId);

  const resolvedHtml = useMemo(() => {
    // Nothing to resolve. Not "no data URLs": when every fetch failed the
    // unresolved images must still be replaced below, or the original
    // (unfetchable) ServiceNow src stays in the page.
    if (!idsFromHtml.length) return html;
    return html.replace(
      /<img([^>]*?)\s+src\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s>]+))([^>]*)>/gi,
      (_match, before, doubleSrc, singleSrc, bareSrc, after) => {
        const src = (doubleSrc ?? singleSrc ?? bareSrc ?? "") as string;
        if (!isInlineImageRefSrc(src)) return _match;
        const refId = extractInlineImageRefId(src);
        // Try direct id match first, then check if any fetched id corresponds
        const dataUrl =
          dataUrls.get(refId) ??
          [...dataUrls.entries()].find(
            ([id]) => src.includes(id) || id.includes(refId),
          )?.[1];
        const quote = doubleSrc !== undefined ? '"' : singleSrc !== undefined ? "'" : '"';
        if (!dataUrl) return unavailableImageHtml(refId, removedIds?.has(refId) ?? false);
        return `<img${before} src=${quote}${dataUrl}${quote}${after}>`;
      },
    );
  }, [html, dataUrls, removedIds, idsFromHtml]);

  const hasInlineImages = idsFromHtml.length > 0;

  return { resolvedHtml, isLoading: hasInlineImages && isLoading };
}
