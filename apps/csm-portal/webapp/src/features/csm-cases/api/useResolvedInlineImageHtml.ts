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
import { useQueries } from "@tanstack/react-query";
import { useBackendApi } from "@api/backend/client";
import { ApiQueryKeys } from "@constants/apiConstants";
import {
  extractIixAttachmentIds,
  replaceInlineImageSrcs,
  sysidToUuid,
} from "@features/csm-cases/utils/inlineImages";

const SAFE_IMAGE_SUBTYPES = /^(png|jpeg|jpg|gif|webp|svg\+xml|bmp|avif)$/i;

/** Returns the normalized image MIME type, or `null` if `raw` isn't an allowed image subtype. */
function toSafeMimeType(raw: string): string | null {
  const lower = raw.trim().toLowerCase();
  const fullMatch = lower.match(/^image\/(.+)$/);
  if (fullMatch && SAFE_IMAGE_SUBTYPES.test(fullMatch[1])) return lower;
  if (SAFE_IMAGE_SUBTYPES.test(lower)) return `image/${lower}`;
  return null;
}

function blobToDataUrl(blob: Blob): Promise<string | null> {
  return new Promise((resolve) => {
    const reader = new FileReader();
    reader.onloadend = () => {
      const result = reader.result;
      resolve(typeof result === "string" ? result : null);
    };
    reader.onerror = () => resolve(null);
    reader.readAsDataURL(blob);
  });
}

/**
 * Fetches inline-image attachments referenced within comment/description HTML
 * via the authenticated `GET /attachments/{id}/content` endpoint and resolves
 * them into `data:` URLs so `<img>` tags can render them without the browser
 * making an unauthenticated request. Mirrors the customer portal's
 * `useResolvedInlineImageHtml`.
 *
 * @param html - Sanitized HTML that may contain `.iix` `<img>` src references.
 */
export function useResolvedInlineImageHtml(html: string): {
  resolvedHtml: string;
  isLoading: boolean;
} {
  const api = useBackendApi();
  const attachmentIds = useMemo(() => extractIixAttachmentIds(html), [html]);

  const queries = useQueries({
    queries: attachmentIds.map((id) => ({
      queryKey: [ApiQueryKeys.CSM_CASE_ATTACHMENTS, "inline-preview", id],
      queryFn: async (): Promise<string | null> => {
        // The extracted id is a bare 32-char sysid; the endpoint requires the
        // canonical UUID shape (hyphens re-inserted), same as every other
        // attachment id sent to this backend.
        const blob = await api.getBlob(
          `/attachments/${encodeURIComponent(sysidToUuid(id))}/content`,
        );
        const mimeType = toSafeMimeType(blob.type);
        if (!mimeType) return null;
        return blobToDataUrl(blob);
      },
      enabled: !!id,
      // Attachment content is immutable once uploaded, so cache it
      // indefinitely rather than refetching on every window focus.
      staleTime: Infinity,
      retry: 1,
    })),
  });

  const isLoading = queries.some((q) => q.isLoading);

  const dataUrls = new Map<string, string>();
  attachmentIds.forEach((id, i) => {
    const result = queries[i]?.data;
    if (result) dataUrls.set(id, result);
  });

  // A fixed-length key derived from the resolved data URLs: useMemo's
  // dependency array must stay the same length across renders, which
  // `queries.map((q) => q.data)` cannot guarantee as attachmentIds changes.
  const dataUrlsKey = Array.from(dataUrls.entries())
    .map(([id, url]) => `${id}:${url}`)
    .join(",");

  const resolvedHtml = useMemo(
    () => replaceInlineImageSrcs(html, dataUrls),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [html, dataUrlsKey],
  );

  return { resolvedHtml, isLoading: attachmentIds.length > 0 && isLoading };
}
