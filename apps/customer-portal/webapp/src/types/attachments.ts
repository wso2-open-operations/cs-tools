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

import type { AuditMetadata } from "@/types/common";

// Item type for inline attachment for comment images.
export type CaseCommentInlineAttachment = AuditMetadata & {
  id: string;
  fileName: string;
  contentType: string;
  downloadUrl: string;
  sys_id?: string;
  url?: string;
};

// Response type for downloading an attachment.
export type AttachmentDownloadResponse = AuditMetadata & {
  content: string;
  id: string;
  referenceId?: string;
  name: string;
  type: string;
  sizeBytes?: number;
  downloadUrl?: string | null;
  description?: string | null;
};

// Request type for patching an attachment.
export type PatchAttachmentRequest = {
  name?: string;
  description?: string;
};

// Request type for posting a case attachment.
export type PostCaseAttachmentRequest = {
  name: string;
  type: string;
  content: string;
  description?: string;
  referenceType?: string;
};
