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

import { useQuery, useQueries } from "@tanstack/react-query";
import { useAuthApiClient } from "@/hooks/useAuthApiClient";
import { fetchInlineImageBlob } from "@api/storedAttachments";

/**
 * Thrown by an image fetch when the image was removed on purpose (deleted, or
 * expired with its closed case), so a renderer can say "removed" rather than
 * "not available". Any other failure resolves to null as before.
 */
export class ImageRemovedError extends Error {
  constructor() {
    super("This image was removed.");
    this.name = "ImageRemovedError";
  }
}

const SAFE_IMAGE_SUBTYPES = /^(png|jpeg|jpg|gif|webp|svg\+xml|bmp|avif)$/i;

function toSafeMimeType(raw: string): string {
  const lower = raw.trim().toLowerCase();
  // Already a full image MIME type (e.g. "image/png")
  const fullMatch = lower.match(/^image\/(.+)$/);
  if (fullMatch && SAFE_IMAGE_SUBTYPES.test(fullMatch[1])) return lower;
  // Bare subtype (e.g. "png")
  if (SAFE_IMAGE_SUBTYPES.test(lower)) return `image/${lower}`;
  return "image/png";
}

async function fetchAttachmentDataUrl(
  authFetch: (
    input: RequestInfo | URL,
    init?: RequestInit,
  ) => Promise<Response>,
  attachmentId: string,
): Promise<string | null> {
  // Entity-service checks the caller's access and the backend streams the image:
  // a comment's inline image, or a work item attachment that is an image. Anything
  // that is not an image, or cannot be shown, resolves to null.
  const inline = await fetchInlineImageBlob(authFetch, attachmentId);
  if (inline.status === "removed") throw new ImageRemovedError();
  if (inline.status !== "ok") return null;
  const mimeType = toSafeMimeType(inline.blob.type);
  if (!mimeType.startsWith("image/")) return null;
  return blobToDataUrl(inline.blob);
}

function blobToDataUrl(blob: Blob): Promise<string | null> {
  return new Promise<string | null>((resolve) => {
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
 * Fetches a single attachment and returns it as a data URL for display.
 *
 * @param attachmentId - Attachment id to fetch, or null/undefined to skip.
 * @returns Query result with `dataUrl` (string | null).
 */
export function useAttachmentPreview(
  attachmentId: string | null | undefined,
) {
  const authFetch = useAuthApiClient();
  return useQuery({
    queryKey: ["attachment-preview", attachmentId],
    queryFn: () => fetchAttachmentDataUrl(authFetch, attachmentId!),
    enabled: !!attachmentId,
    staleTime: 0,
    // Each fetch of an attachment preview streams the whole file, so a refocus
    // of the window must not fetch it again.
    refetchOnWindowFocus: false,
    retry: (count: number, error: unknown) =>
      !(error instanceof ImageRemovedError) && count < 1,
  });
}

/**
 * Fetches multiple attachments in parallel and returns a map of id -> data URL.
 *
 * @param attachmentIds - List of attachment ids to fetch.
 * @returns `{ dataUrls, removedIds, isLoading }`: `removedIds` holds the ids
 * whose image was removed (deleted), as opposed to merely failing to load.
 */
export function useAttachmentPreviews(attachmentIds: string[]) {
  const authFetch = useAuthApiClient();
  const queries = useQueries({
    queries: attachmentIds.map((id) => ({
      queryKey: ["attachment-preview", id],
      queryFn: () => fetchAttachmentDataUrl(authFetch, id),
      enabled: !!id,
      staleTime: 0,
      retry: false,
    })),
  });

  const isLoading = queries.some((q) => q.isLoading);
  const dataUrls = new Map<string, string>();
  const removedIds = new Set<string>();
  attachmentIds.forEach((id, i) => {
    const result = queries[i]?.data;
    if (result) dataUrls.set(id, result);
    else if (queries[i]?.error instanceof ImageRemovedError) removedIds.add(id);
  });

  return { dataUrls, removedIds, isLoading };
}
