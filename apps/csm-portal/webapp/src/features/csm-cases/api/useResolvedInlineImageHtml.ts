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
import type { BeAttachmentShareResponse } from "@api/backend/types";
import { useCurrentUser } from "@context/current-user/CurrentUserContext";
import { usePortalAccess } from "@context/current-user/usePortalAccess";
import {
  extractIixAttachmentIds,
  replaceInlineImageSrcs,
  sysidToUuid,
} from "@features/csm-cases/utils/inlineImages";

/**
 * How long a resolved share URL is treated as fresh before React Query would
 * refetch it. SFTPGo shares expire 5 minutes after creation (see the
 * backend's `shareTTL`); this is kept comfortably under that so a stale
 * cached URL is never handed to an `<img>` past its expiry, while still
 * avoiding a share-creation call on every re-render/remount within the
 * window. A fixed shorter `staleTime` is a deliberate, simple first pass —
 * not a full "detect the 403 and mint a fresh share" retry loop, which is
 * unnecessary complexity for an inline preview image.
 */
const INLINE_IMAGE_SHARE_STALE_TIME_MS = 3 * 60_000;

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
 * Two mutually exclusive paths, gated on the signed-in user's
 * `sftpgoAttachmentStorageEnabled` flag (`GET /users/me`):
 *  - Off (default, unchanged): fetches each attachment via the authenticated
 *    `GET /attachments/{id}/content` endpoint and resolves it into a `data:`
 *    URL, so the browser never makes an unauthenticated request. Mirrors the
 *    customer portal's `useResolvedInlineImageHtml`.
 *  - On: creates a short-lived public share (`POST /attachments/{id}/share`)
 *    for each referenced attachment and uses its `shareUrl` directly as the
 *    `<img>` src.
 *
 * A caller without the `attachment_downloader`/`cs_engineer`/`admin` role
 * (see `usePortalAccess`'s `canDownloadAttachment`) never issues either
 * request — the backend would 403 both anyway — and every referenced image is
 * replaced with a "no permission" placeholder instead of a blank `<img>`.
 *
 * @param html - Sanitized HTML that may contain `.iix` `<img>` src references.
 */
export function useResolvedInlineImageHtml(html: string): {
  resolvedHtml: string;
  isLoading: boolean;
} {
  const api = useBackendApi();
  const { user } = useCurrentUser();
  const { canDownloadAttachment } = usePortalAccess();
  const sftpgoEnabled = !!user?.sftpgoAttachmentStorageEnabled;
  const attachmentIds = useMemo(() => extractIixAttachmentIds(html), [html]);

  const queries = useQueries({
    queries: attachmentIds.map((id) => ({
      queryKey: [
        ApiQueryKeys.CSM_CASE_ATTACHMENTS,
        "inline-preview",
        id,
        sftpgoEnabled,
      ],
      queryFn: async (): Promise<string | null> => {
        // The extracted id is a bare 32-char sysid; the endpoint requires the
        // canonical UUID shape (hyphens re-inserted), same as every other
        // attachment id sent to this backend.
        const attachmentId = encodeURIComponent(sysidToUuid(id));

        if (sftpgoEnabled) {
          // A share is a real SFTPGo object with its own lifecycle — only
          // create one when this specific inline image is actually being
          // resolved for render, never eagerly for a whole comment thread.
          const { shareUrl } = await api.post<
            Record<string, never>,
            BeAttachmentShareResponse
          >(`/attachments/${attachmentId}/share`, {});
          // The share URL is a public, unauthenticated download link, so it
          // can be set directly as the <img> src rather than fetched and
          // re-encoded as a data URL.
          return shareUrl;
        }

        const blob = await api.getBlob(`/attachments/${attachmentId}/content`);
        const mimeType = toSafeMimeType(blob.type);
        if (!mimeType) return null;
        return blobToDataUrl(blob);
      },
      // Skipping the request entirely (rather than letting it 403) avoids a
      // wasted round trip and an error the caller has no way to act on.
      enabled: !!id && canDownloadAttachment,
      // The default (non-SFTPGo) path resolves immutable attachment content,
      // so it's cached indefinitely. The SFTPGo share path resolves to a
      // URL that itself expires — see INLINE_IMAGE_SHARE_STALE_TIME_MS.
      staleTime: sftpgoEnabled ? INLINE_IMAGE_SHARE_STALE_TIME_MS : Infinity,
      retry: 1,
    })),
  });

  const isLoading = canDownloadAttachment && queries.some((q) => q.isLoading);

  const dataUrls = new Map<string, string>();
  const deniedIds = new Set<string>();
  attachmentIds.forEach((id, i) => {
    if (!canDownloadAttachment) {
      deniedIds.add(id);
      return;
    }
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
    () => replaceInlineImageSrcs(html, dataUrls, deniedIds),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [html, dataUrlsKey, canDownloadAttachment],
  );

  return { resolvedHtml, isLoading: attachmentIds.length > 0 && isLoading };
}
