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
import { BackendApiError, useBackendApi } from "@api/backend/client";
import { fetchEntityInlineImage } from "@features/csm-cases/api/entityAttachments";
import { ApiQueryKeys } from "@constants/apiConstants";
import { usePortalAccess } from "@context/current-user/usePortalAccess";
import {
  extractIixAttachmentIds,
  replaceInlineImageSrcs,
  sysidToUuid,
} from "@features/csm-cases/utils/inlineImages";

/**
 * What an image's query resolves to when entity-service answers 410 (the image
 * was deleted or removed). Not a URL, so it can never be used as an `<img>` src.
 */
const INLINE_IMAGE_REMOVED = "\u0000removed";

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
 * Resolves inline-image attachments referenced within comment/description
 * HTML so `<img>` tags can render them.
 *
 * Each referenced attachment is fetched through the authenticated backend's
 * streamed comment-image route and resolved into a `data:` URL, so the browser
 * never makes an unauthenticated request. An image the backend does not know
 * (404/400) is left unresolved. Mirrors the customer portal's
 * `useResolvedInlineImageHtml`.
 *
 * A caller without the `attachment_downloader`/`cs_engineer`/`admin` role
 * (see `usePortalAccess`'s `canDownloadAttachment`) never issues a request —
 * the backend would 403 it anyway — and every referenced image is replaced
 * with a "no permission" placeholder instead of a blank `<img>`.
 *
 * Content authored before images were stored as attachments has the image
 * embedded as a raw base64 `data:` URI directly in the HTML, which
 * `GET .../comments/search` already sends to anyone holding `PermView` alone
 * (there's no separate attachment resource for `PermDownloadAttachment` to
 * gate). `replaceInlineImageSrcs`'s `denyRawBase64` parameter hides these the
 * same way — see its own doc comment for why this is a display-only
 * mitigation, not a real confidentiality fix (the bytes already reached the
 * browser by the time this runs).
 *
 * Bare attachment-id `<img>` srcs (`/<uuid>`, as found in content migrated
 * from the legacy data source) are treated exactly like `.iix` references.
 *
 * @param html - Sanitized HTML that may contain `.iix` or bare-id `<img>` src
 * references, or raw base64-embedded images.
 */
export function useResolvedInlineImageHtml(html: string): {
  resolvedHtml: string;
  isLoading: boolean;
} {
  const api = useBackendApi();
  const { canDownloadAttachment } = usePortalAccess();
  const attachmentIds = useMemo(() => extractIixAttachmentIds(html), [html]);

  const queries = useQueries({
    queries: attachmentIds.map((id) => ({
      queryKey: [
        ApiQueryKeys.CSM_CASE_ATTACHMENTS,
        "inline-preview",
        id,
      ],
      queryFn: async (): Promise<string | null> => {
        // The extracted id is a bare 32-char sysid; the endpoint requires the
        // canonical UUID shape (hyphens re-inserted), same as every other
        // attachment id sent to this backend.
        try {
          const blob = await fetchEntityInlineImage(api, sysidToUuid(id));
          return toSafeMimeType(blob.type) ? blobToDataUrl(blob) : null;
        } catch (err) {
          // 410: removed on purpose.
          if (err instanceof BackendApiError && err.status === 410) {
            return INLINE_IMAGE_REMOVED;
          }
          // Not an image the backend holds: nothing to show.
          if (err instanceof BackendApiError && (err.status === 404 || err.status === 400)) {
            return null;
          }
          throw err;
        }
      },
      // Skipping the request entirely (rather than letting it 403) avoids a
      // wasted round trip and an error the caller has no way to act on.
      enabled: !!id && canDownloadAttachment,
      // Attachment content is immutable, so it's cached indefinitely.
      staleTime: Infinity,
      retry: 1,
    })),
  });

  const isLoading = canDownloadAttachment && queries.some((q) => q.isLoading);

  const dataUrls = new Map<string, string>();
  const deniedIds = new Set<string>();
  const removedIds = new Set<string>();
  attachmentIds.forEach((id, i) => {
    if (!canDownloadAttachment) {
      deniedIds.add(id);
      return;
    }
    const result = queries[i]?.data;
    if (result === INLINE_IMAGE_REMOVED) removedIds.add(id);
    else if (result) dataUrls.set(id, result);
  });

  // A fixed-length key derived from the resolved data URLs: useMemo's
  // dependency array must stay the same length across renders, which
  // `queries.map((q) => q.data)` cannot guarantee as attachmentIds changes.
  const dataUrlsKey = Array.from(dataUrls.entries())
    .map(([id, url]) => `${id}:${url}`)
    .join(",") + `|${Array.from(removedIds).join(",")}`;

  const resolvedHtml = useMemo(
    () => replaceInlineImageSrcs(html, dataUrls, deniedIds, !canDownloadAttachment, removedIds),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [html, dataUrlsKey, canDownloadAttachment],
  );

  return { resolvedHtml, isLoading: attachmentIds.length > 0 && isLoading };
}
