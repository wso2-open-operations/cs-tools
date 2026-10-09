// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License. You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

import type { PostAttachmentsVariables } from "@features/support/types/supportApi";

export const ATTACHMENT_UPLOAD_WAIT_MS = 5_000;

export interface AttachmentUploadItem {
  id: string;
  file: File;
}

export function fileToBase64(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => {
      const s = typeof reader.result === "string" ? reader.result : "";
      const i = s.indexOf(",");
      resolve(i >= 0 ? s.slice(i + 1) : s);
    };
    reader.onerror = () => reject(new Error(`Failed to read ${file.name}`));
    reader.readAsDataURL(file);
  });
}

export interface UploadAttachmentsParams {
  caseId: string;
  attachments: AttachmentUploadItem[];
  attachmentNames: Map<string, string>;
  uploadAttachment: (variables: PostAttachmentsVariables) => Promise<unknown>;
  encodeFile?: (file: File) => Promise<string>;
  logger?: { error: (msg: string, err?: unknown) => void };
}

/**
 * Uploads all attachments for a created service request.
 * Returns a list of file/attachment names that failed to upload.
 */
export async function uploadServiceRequestAttachments({
  caseId,
  attachments,
  attachmentNames,
  uploadAttachment,
  encodeFile = fileToBase64,
  logger,
}: UploadAttachmentsParams): Promise<string[]> {
  const failed: string[] = [];
  for (const item of attachments) {
    const attachmentName = attachmentNames.get(item.id) || item.file.name;
    try {
      const content = await encodeFile(item.file);
      await uploadAttachment({
        caseId,
        body: {
          name: attachmentName,
          type: item.file.type || "application/octet-stream",
          content,
        },
      });
    } catch (error) {
      logger?.error(`Failed to upload attachment ${attachmentName}`, error);
      failed.push(attachmentName);
    }
  }
  return failed;
}
