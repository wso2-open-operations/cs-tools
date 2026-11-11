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

import { isAbortError } from "@api/storedAttachments";
import type { StoredUploadVariables } from "@api/useStoredAttachments";

export const ATTACHMENT_UPLOAD_WAIT_MS = 5_000;

export interface AttachmentUploadItem {
  id: string;
  file: File;
}

export interface UploadAttachmentsParams {
  caseId: string;
  attachments: AttachmentUploadItem[];
  attachmentNames: Map<string, string>;
  uploadAttachment: (variables: StoredUploadVariables) => Promise<unknown>;
  logger?: { error: (msg: string, err?: unknown) => void };
}

/**
 * Uploads all attachments for a created service request, one after another,
 * through the stored (Azure Blob) upload. Returns a list of file/attachment
 * names that failed to upload.
 */
export async function uploadServiceRequestAttachments({
  caseId,
  attachments,
  attachmentNames,
  uploadAttachment,
  logger,
}: UploadAttachmentsParams): Promise<string[]> {
  const failed: string[] = [];
  for (const item of attachments) {
    const attachmentName = attachmentNames.get(item.id) || item.file.name;
    try {
      await uploadAttachment({
        type: "work_item",
        parentId: caseId,
        caseId,
        file: item.file,
        body: { name: attachmentName },
      });
    } catch (error) {
      // Stopped by the person: that is not a failure to report.
      if (isAbortError(error)) continue;
      logger?.error(`Failed to upload attachment ${attachmentName}`, error);
      failed.push(attachmentName);
    }
  }
  return failed;
}
