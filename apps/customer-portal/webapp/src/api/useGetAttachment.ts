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

/** Input for downloading an attachment via its download URL. */
export interface DownloadBackendAttachmentInput {
  id: string;
  name: string;
  type?: string;
  content?: string | null;
  downloadUrl?: string | null;
}

function triggerDownloadFromBlob(blob: Blob, fileName: string): void {
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = fileName || "attachment";
  link.click();
  URL.revokeObjectURL(url);
}

async function downloadAttachmentViaDownloadUrl(
  authFetch: (
    input: RequestInfo | URL,
    init?: RequestInit,
  ) => Promise<Response>,
  input: DownloadBackendAttachmentInput,
): Promise<void> {
  if (!input.downloadUrl) {
    throw new Error("No download URL available for this attachment");
  }

  const response = await authFetch(input.downloadUrl, { method: "GET" });

  if (!response.ok) {
    throw new Error(`Failed to download attachment: ${response.statusText}`);
  }

  const blob = await response.blob();
  triggerDownloadFromBlob(blob, input.name);
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
    mutationFn: (input) => downloadAttachmentViaDownloadUrl(authFetch, input),
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
