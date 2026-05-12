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

/** Input for GET /attachments/:id or inline list payload. */
export interface DownloadBackendAttachmentInput {
  id: string;
  name: string;
  type?: string;
  content?: string | null;
  downloadUrl?: string | null;
}

function getBackendBaseUrl(): string {
  const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;
  if (!baseUrl) {
    throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
  }
  return baseUrl.replace(/\/$/, "");
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

function openExternalDownload(url: string): void {
  window.open(url, "_blank", "noopener,noreferrer");
}

async function downloadAttachmentThroughBackend(
  authFetch: (
    input: RequestInfo | URL,
    init?: RequestInit,
  ) => Promise<Response>,
  input: DownloadBackendAttachmentInput,
): Promise<void> {
  if (input.content) {
    triggerDownloadFromContentField(input.content, input.name, input.type);
    return;
  }

  const baseUrl = getBackendBaseUrl();
  const url = `${baseUrl}/attachments/${encodeURIComponent(input.id)}`;
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
    throw new Error(parseApiResponseMessage(detail, response.status, response.statusText));
  }

  const data = (await response.json()) as AttachmentDownloadResponse;
  if (!data?.content || typeof data.content !== "string") {
    if (input.downloadUrl) {
      openExternalDownload(input.downloadUrl);
      return;
    }
    throw new Error("Attachment response did not include file content");
  }

  triggerDownloadFromContentField(
    data.content,
    data.name || input.name,
    data.type || input.type,
  );
}

export interface UseGetAttachmentResult {
  /** Starts download (GET /attachments/:id or inline content). */
  downloadAttachment: (input: DownloadBackendAttachmentInput) => Promise<void>;
  /** True while a download request is in flight. */
  isDownloading: boolean;
  /** Attachment id currently being downloaded, if any. */
  downloadingId: string | null;
}

/**
 * Authenticated attachment download via GET /attachments/:id, with optional
 * inline content shortcut and ServiceNow URL fallback.
 *
 * @returns {UseGetAttachmentResult} mutateAsync, loading flag, and active id.
 */
export function useGetAttachment(): UseGetAttachmentResult {
  const authFetch = useAuthApiClient();
  const [downloadingId, setDownloadingId] = useState<string | null>(null);

  const mutation = useMutation<void, Error, DownloadBackendAttachmentInput>({
    mutationFn: (input) => downloadAttachmentThroughBackend(authFetch, input),
    onMutate: (variables) => {
      setDownloadingId(variables.id);
    },
    onSettled: () => {
      setDownloadingId(null);
    },
  });

  return {
    downloadAttachment: mutation.mutateAsync,
    isDownloading: mutation.isPending,
    downloadingId,
  };
}
