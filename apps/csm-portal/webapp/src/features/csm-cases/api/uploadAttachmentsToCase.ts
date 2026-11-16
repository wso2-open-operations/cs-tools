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

import type { PendingAttachment } from "@components/attachments/pendingAttachment";
import type { PostCsmCaseAttachmentInput } from "@features/csm-cases/api/useCsmCaseAttachments";

/**
 * Upload create-form attachments to an already-created case, through the same
 * chunked Azure Blob flow as the case page. Every create flow attaches files
 * this way after the record exists, rather than bundling them into the create
 * request, so a failed attachment upload never blocks or masks a successful
 * create.
 *
 * Uploads run concurrently and independently; returns the names of the files
 * that failed so the caller can warn (and name them) without blocking
 * navigation to the created case.
 *
 * @param upload      the `mutateAsync` of {@link usePostCsmCaseAttachment}
 * @param uploadedBy  display name of the uploader (signed-in engineer)
 */
export async function uploadAttachmentsToCase(
  upload: (input: PostCsmCaseAttachmentInput) => Promise<unknown>,
  caseId: string,
  attachments: PendingAttachment[],
  uploadedBy: string,
): Promise<string[]> {
  if (attachments.length === 0) return [];
  const results = await Promise.allSettled(
    attachments.map((a) => upload({ caseId, file: a.raw, name: a.name, uploadedBy })),
  );
  return attachments.filter((_, i) => results[i].status === "rejected").map((a) => a.name);
}

/**
 * The message shown when the record was created but some files did not upload.
 * It says the record exists and names the files that failed.
 */
export function failedUploadsMessage(recordLabel: string, failedNames: string[]): string {
  const count = failedNames.length;
  const names = failedNames.map((n) => `"${n}"`).join(", ");
  return `The ${recordLabel} was created, but ${count} attachment${count === 1 ? "" : "s"} failed to upload (${names}). You can add ${count === 1 ? "it" : "them"} from the ${recordLabel} page.`;
}
