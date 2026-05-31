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
import { extractInlineImageRefId } from "@features/support/utils/support";
import type { InlineAttachment } from "@features/support/utils/support";

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
    if (src.includes(".iix")) {
      const id = extractInlineImageRefId(src);
      if (id && !ids.includes(id)) ids.push(id);
    }
  }
  return ids;
}

/**
 * Resolves inline image data URLs for all `.iix` img src references in an HTML string.
 * Fetches images via the authenticated backend and replaces src attributes with data URLs.
 *
 * @param html - Sanitized HTML string potentially containing `.iix` img src URLs.
 * @param inlineAttachments - Inline attachment metadata list from the comment.
 * @returns `{ resolvedHtml: string, isLoading: boolean }`
 */
export function useResolvedInlineImageHtml(
  html: string,
  _inlineAttachments?: InlineAttachment[] | null,
): { resolvedHtml: string; isLoading: boolean } {
  const idsFromHtml = useMemo(() => extractIixAttachmentIds(html), [html]);

  const { dataUrls, isLoading } = useAttachmentPreviews(idsFromHtml);

  const resolvedHtml = useMemo(() => {
    if (!dataUrls.size) return html;
    return html.replace(
      /<img([^>]*?)\s+src\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s>]+))([^>]*)>/gi,
      (_match, before, doubleSrc, singleSrc, bareSrc, after) => {
        const src = (doubleSrc ?? singleSrc ?? bareSrc ?? "") as string;
        if (!src.includes(".iix")) return _match;
        const refId = extractInlineImageRefId(src);
        // Try direct id match first, then check if any fetched id corresponds
        const dataUrl =
          dataUrls.get(refId) ??
          [...dataUrls.entries()].find(
            ([id]) => src.includes(id) || id.includes(refId),
          )?.[1];
        const quote = doubleSrc !== undefined ? '"' : singleSrc !== undefined ? "'" : '"';
        if (!dataUrl) {
          // Strip the unresolved .iix src so the browser does not attempt an unauthenticated request.
          return `<img${before} src=${quote}${quote} data-unresolved="true"${after}>`;
        }
        return `<img${before} src=${quote}${dataUrl}${quote}${after}>`;
      },
    );
  }, [html, dataUrls]);

  const hasInlineImages = idsFromHtml.length > 0;

  return { resolvedHtml, isLoading: hasInlineImages && isLoading };
}
