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
import type { AttachmentDownloadResponse } from "@features/support/types/attachments";

function getBackendBaseUrl(): string {
  const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;
  if (!baseUrl)
    throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
  return baseUrl.replace(/\/$/, "");
}

async function fetchAttachmentDataUrl(
  authFetch: (
    input: RequestInfo | URL,
    init?: RequestInit,
  ) => Promise<Response>,
  attachmentId: string,
): Promise<string | null> {
  const baseUrl = getBackendBaseUrl();
  const response = await authFetch(
    `${baseUrl}/attachments/${encodeURIComponent(attachmentId)}`,
    { method: "GET" },
  );
  if (!response.ok) return null;
  const data = (await response.json()) as AttachmentDownloadResponse;
  if (!data.content || typeof data.content !== "string") return null;
  // Reject pre-built data URLs; require raw base64.
  if (data.content.startsWith("data:")) return null;
  const stripped = data.content.replace(/\s/g, "");
  if (!stripped || !/^[A-Za-z0-9+/]+=*$/.test(stripped)) return null;
  // Validate MIME type against safe image subtypes only.
  const rawType = typeof data.type === "string" ? data.type.trim().toLowerCase() : "";
  const safeType = /^image\/(png|jpeg|jpg|gif|webp|svg\+xml|bmp|avif)$/.test(rawType)
    ? rawType
    : "image/png";
  return `data:${safeType};base64,${stripped}`;
}

/**
 * Fetches a single attachment and returns it as a data URL for display.
 *
 * @param attachmentId - Attachment id to fetch, or null/undefined to skip.
 * @returns Query result with `dataUrl` (string | null).
 */
export function useAttachmentPreview(attachmentId: string | null | undefined) {
  const authFetch = useAuthApiClient();
  return useQuery({
    queryKey: ["attachment-preview", attachmentId],
    queryFn: () => fetchAttachmentDataUrl(authFetch, attachmentId!),
    enabled: !!attachmentId,
    staleTime: 0,
    retry: 1,
  });
}

/**
 * Fetches multiple attachments in parallel and returns a map of id -> data URL.
 *
 * @param attachmentIds - List of attachment ids to fetch.
 * @returns `{ dataUrls: Map<string, string>, isLoading: boolean }`
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
  attachmentIds.forEach((id, i) => {
    const result = queries[i]?.data;
    if (result) dataUrls.set(id, result);
  });

  return { dataUrls, isLoading };
}
