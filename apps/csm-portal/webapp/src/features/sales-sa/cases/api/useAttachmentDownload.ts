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

// Rewritten against useBackendApi().getBlob() (this app's existing binary-GET
// primitive, already authenticated) — replaces the source app's own
// hand-rolled fetch+Authorization-header attachment downloader, which existed
// only because its useSplApi.ts was JSON-only.
import { useCallback } from "react";
import { useBackendApi } from "@api/backend/client";

function arrayBufferToBase64(buffer: ArrayBuffer): string {
  let binary = "";
  const bytes = new Uint8Array(buffer);
  const chunkSize = 0x8000;
  for (let i = 0; i < bytes.length; i += chunkSize) {
    binary += String.fromCharCode(...bytes.subarray(i, i + chunkSize));
  }
  return btoa(binary);
}

export function useAttachmentDownload() {
  const api = useBackendApi();

  const downloadAttachment = useCallback(
    async (attachmentId: string, fileName: string) => {
      const blob = await api.getBlob(`/attachments/${encodeURIComponent(attachmentId)}/download`);
      const href = URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = href;
      link.download = fileName;
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      // Deferred, not immediate: some browsers (older Safari/Firefox) start
      // the download asynchronously, so revoking synchronously here can race
      // it and fail the download.
      setTimeout(() => URL.revokeObjectURL(href), 0);
    },
    [api],
  );

  const fetchAttachmentDataUrl = useCallback(
    async (attachmentId: string): Promise<string> => {
      const blob = await api.getBlob(`/attachments/${encodeURIComponent(attachmentId)}/download`);
      const buffer = await blob.arrayBuffer();
      const contentType = blob.type || "application/octet-stream";
      return `data:${contentType};base64,${arrayBufferToBase64(buffer)}`;
    },
    [api],
  );

  return { downloadAttachment, fetchAttachmentDataUrl };
}
