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
import { isEntityAttachmentsEnabled } from "@/config/entityAttachmentsConfig";
import { fetchInlineImageBlob } from "@api/storedAttachments";

function getBackendBaseUrl(): string {
  const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;
  if (!baseUrl)
    throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
  return baseUrl.replace(/\/$/, "");
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
  caseId?: string | null,
): Promise<string | null> {
  // Entity-service streams a comment's image (and checks the caller's access) when
  // it holds it. A 404 means this id is not one of its stored images, so fall
  // through to the existing flow below, which still serves older attachments.
  if (isEntityAttachmentsEnabled()) {
    const inline = await fetchInlineImageBlob(authFetch, attachmentId);
    if (inline.status === "ok") {
      const mimeType = toSafeMimeType(inline.blob.type);
      if (!mimeType.startsWith("image/")) return null;
      return blobToDataUrl(inline.blob);
    }
    if (inline.status === "error") return null;
  }
  const baseUrl = getBackendBaseUrl();
  // Resolved against window.location.href so a relative
  // CUSTOMER_PORTAL_BACKEND_BASE_URL still works: the base is ignored when
  // baseUrl is already absolute (every deployment seen so far), so this
  // preserves existing behavior and only changes anything for a relative one.
  const url = new URL(
    `${baseUrl}/attachments/${encodeURIComponent(attachmentId)}/content`,
    window.location.href,
  );
  // caseId is a hint only: it lets the backend authorize an attachment
  // pasted inline into a case comment (whose own referenceId is the
  // comment's id, not the case's, so it can't otherwise be resolved to a
  // case at all) by re-verifying the caller can see this case and that the
  // attachment genuinely belongs to one of its comments. Omitted for
  // attachment-tab previews, where the attachment's own referenceId already
  // resolves directly.
  if (caseId) url.searchParams.set("caseId", caseId);
  const response = await authFetch(url, { method: "GET" });
  if (!response.ok) return null;

  const blob = await response.blob();
  const rawType = blob.type || response.headers.get("content-type") || "";
  const mimeType = toSafeMimeType(rawType);
  if (!mimeType.startsWith("image/")) return null;

  return blobToDataUrl(blob);
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
 * @param caseId - Optional case-id hint, for an attachment pasted inline
 * into a case comment (see fetchAttachmentDataUrl's own doc comment).
 * @returns Query result with `dataUrl` (string | null).
 */
export function useAttachmentPreview(
  attachmentId: string | null | undefined,
  caseId?: string | null,
) {
  const authFetch = useAuthApiClient();
  return useQuery({
    queryKey: ["attachment-preview", attachmentId, caseId],
    queryFn: () => fetchAttachmentDataUrl(authFetch, attachmentId!, caseId),
    enabled: !!attachmentId,
    staleTime: 0,
    retry: 1,
  });
}

/**
 * Fetches multiple attachments in parallel and returns a map of id -> data URL.
 *
 * @param attachmentIds - List of attachment ids to fetch.
 * @param caseId - Optional case-id hint, forwarded to every fetch (see
 * fetchAttachmentDataUrl's own doc comment).
 * @returns `{ dataUrls: Map<string, string>, isLoading: boolean }`
 */
export function useAttachmentPreviews(
  attachmentIds: string[],
  caseId?: string | null,
) {
  const authFetch = useAuthApiClient();
  const queries = useQueries({
    queries: attachmentIds.map((id) => ({
      queryKey: ["attachment-preview", id, caseId],
      queryFn: () => fetchAttachmentDataUrl(authFetch, id, caseId),
      enabled: !!id,
      staleTime: 0,
      retry: false,
    })),
  });

  const isLoading = queries.some((q) => q.isLoading);
  const dataUrls = new Map<string, string>();
  attachmentIds.forEach((id, i) => {
    const result = queries[i]?.data;
    if (result) dataUrls.set(id, result);
  });

  return { dataUrls, isLoading };
}
