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

import { useMutation } from "@tanstack/react-query";
import { useState } from "react";
import { useAuthApiClient } from "@/hooks/useAuthApiClient";
import type { AttachmentDownloadResponse } from "@features/support/types/attachments";
import { parseApiResponseMessage } from "@utils/ApiError";

/** Input for GET /attachments/:id/content (authenticated). */
export interface DownloadBackendAttachmentContentInput {
  id: string;
  name: string;
  type?: string;
  downloadUrl?: string | null;
}

function getBackendBaseUrl(): string {
  const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;
  if (!baseUrl) {
    throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
  }
  return baseUrl.replace(/\/$/, "");
}

function openExternalDownload(url: string): void {
  window.open(url, "_blank", "noopener,noreferrer");
}

function getFilenameFromContentDisposition(
  contentDisposition: string | null,
): string | null {
  if (!contentDisposition) return null;

  // Handles: filename="a.txt" and filename*=UTF-8''a%20b.txt
  const utf8Star = contentDisposition.match(/filename\*\s*=\s*UTF-8''([^;]+)/i);
  if (utf8Star?.[1]) {
    try {
      return decodeURIComponent(utf8Star[1].trim().replace(/^"|"$/g, ""));
    } catch {
      return utf8Star[1].trim().replace(/^"|"$/g, "");
    }
  }

  const plain = contentDisposition.match(/filename\s*=\s*([^;]+)/i);
  if (plain?.[1]) {
    return plain[1].trim().replace(/^"|"$/g, "");
  }

  return null;
}

function triggerDownloadFromBlob(blob: Blob, fileName: string): void {
  const objectUrl = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = objectUrl;
  link.download = fileName || "attachment";
  link.target = "_blank";
  link.rel = "noopener noreferrer";
  link.click();

  // Cleanup after the click.
  window.setTimeout(() => URL.revokeObjectURL(objectUrl), 10_000);
}

function triggerDownloadFromContentField(
  content: string,
  fileName: string,
  mimeType?: string,
): void {
  const isDataUrl = content.startsWith("data:");
  const href = isDataUrl
    ? content
    : `data:${mimeType ?? "application/octet-stream"};base64,${content}`;
  const link = document.createElement("a");
  link.href = href;
  link.download = fileName || "attachment";
  link.target = "_blank";
  link.rel = "noopener noreferrer";
  link.click();
}

async function downloadAttachmentContentThroughBackend(
  authFetch: (
    input: RequestInfo | URL,
    init?: RequestInit,
  ) => Promise<Response>,
  input: DownloadBackendAttachmentContentInput,
): Promise<void> {
  const baseUrl = getBackendBaseUrl();
  const url = `${baseUrl}/attachments/${encodeURIComponent(input.id)}/content`;

  let response: Response;
  try {
    response = await authFetch(url, { method: "GET" });
  } catch {
    if (input.downloadUrl) {
      openExternalDownload(input.downloadUrl);
      return;
    }
    throw new Error("Network error while downloading attachment");
  }

  if (!response.ok) {
    const detail = await response.text().catch(() => "");
    if (input.downloadUrl) {
      openExternalDownload(input.downloadUrl);
      return;
    }
    throw new Error(
      parseApiResponseMessage(detail, response.status, response.statusText),
    );
  }

  // Some backends might still return JSON { content, name, type }.
  const contentType = response.headers.get("content-type") ?? "";
  if (contentType.includes("application/json")) {
    const data = (await response.json()) as Partial<
      AttachmentDownloadResponse & { content?: unknown }
    >;
    if (typeof data?.content === "string") {
      triggerDownloadFromContentField(
        data.content,
        data.name || input.name,
        data.type || input.type,
      );
      return;
    }

    if (input.downloadUrl) {
      openExternalDownload(input.downloadUrl);
      return;
    }
    throw new Error("Attachment response did not include file content");
  }

  const blob = await response.blob();
  const cd = response.headers.get("content-disposition");
  const fileName = getFilenameFromContentDisposition(cd) ?? input.name;
  triggerDownloadFromBlob(blob, fileName);
}

export interface UseGetAttachmentContentResult {
  downloadAttachment: (
    input: DownloadBackendAttachmentContentInput,
  ) => Promise<void>;
  isDownloading: boolean;
  downloadingId: string | null;
}

/**
 * Authenticated attachment download via GET /attachments/:id/content.
 */
export function useGetAttachmentContent(): UseGetAttachmentContentResult {
  const authFetch = useAuthApiClient();
  const [downloadingId, setDownloadingId] = useState<string | null>(null);

  const mutation = useMutation<
    void,
    Error,
    DownloadBackendAttachmentContentInput
  >({
    mutationFn: (input) =>
      downloadAttachmentContentThroughBackend(authFetch, input),
    onMutate: (variables) => setDownloadingId(variables.id),
    onSettled: () => setDownloadingId(null),
  });

  return {
    downloadAttachment: mutation.mutateAsync,
    isDownloading: mutation.isPending,
    downloadingId,
  };
}
