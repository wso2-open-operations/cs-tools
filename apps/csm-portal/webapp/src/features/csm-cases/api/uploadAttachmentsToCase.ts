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

import type { EncodedAttachment } from "@components/attachments/encodeAttachment";
import type { PostCsmCaseAttachmentInput } from "@features/csm-cases/api/useCsmCaseAttachments";

/**
 * Upload create-form attachments to an already-created case via
 * `POST /attachments` (`referenceType: "case"`). The case-create endpoint only
 * honors create-payload attachments for security reports, so standard cases and
 * service requests attach their files this way after the case exists.
 *
 * Uploads run concurrently and independently; returns the number that failed
 * so the caller can warn without blocking navigation to the created case.
 *
 * @param upload      the `mutateAsync` of {@link usePostCsmCaseAttachment}
 * @param uploadedBy  display name of the uploader (signed-in engineer)
 */
export async function uploadAttachmentsToCase(
  upload: (input: PostCsmCaseAttachmentInput) => Promise<unknown>,
  caseId: string,
  attachments: EncodedAttachment[],
  uploadedBy: string,
): Promise<number> {
  if (attachments.length === 0) return 0;
  const results = await Promise.allSettled(
    attachments.map((a) => upload({ caseId, file: a.raw, name: a.name, uploadedBy })),
  );
  return results.filter((r) => r.status === "rejected").length;
}
