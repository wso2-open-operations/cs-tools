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

import type { BackendApi } from "@api/backend/client";
import {
  entityAttachmentErrorMessage,
  uploadEntityAttachment,
} from "@features/csm-cases/api/entityAttachments";
import type { PendingAttachment } from "@components/attachments/pendingAttachment";

/** What is known of a file that finished uploading: kept across retries so it is never sent twice. */
export interface StoredReportDraft {
  projectId: string;
  attachmentId: string;
}

export type ReportDraftUploadResult =
  | { ok: true; ids: string[] }
  | { ok: false; cancelled: boolean; message: string };

/** Which file is uploading, how far it got (0-100), and how many wait behind it. */
export interface ReportDraftProgress {
  name: string;
  percent: number;
  waiting: number;
}

/** An upload that was cancelled: the signal fired, or the error is an AbortError. */
function isCancelled(err: unknown, signal?: AbortSignal): boolean {
  return signal?.aborted === true || (err instanceof Error && err.name === "AbortError");
}

/**
 * Uploads the files of a security report BEFORE its case exists, one after another,
 * as drafts of the project (`type: "work_item"` with `projectId`). The create request
 * then names the ids in `attachmentIds` and entity-service links them to the case it
 * creates, so a cancelled or refreshed upload leaves no case without its report.
 *
 * A file that already uploaded (its object is in `stored` for this project) is not
 * sent again, so a retry after a failed create only uploads what did not finish. The
 * first failure stops the run: the caller must not create the case. A cancel ends it
 * with `cancelled: true`, which is not an error to show.
 *
 * @param api - The authenticated backend client.
 * @param projectId - The project the case is being created in.
 * @param files - The files in the form, in order.
 * @param stored - Ids already obtained, by the file's `File` object; updated as files finish.
 * @param options - `signal` cancels; `onProgress` reports the file being sent.
 * @returns {Promise<ReportDraftUploadResult>} The attachment ids in the order of `files`, or the failure.
 */
export async function uploadReportDrafts(
  api: BackendApi,
  projectId: string,
  files: readonly PendingAttachment[],
  stored: Map<File, StoredReportDraft>,
  options: {
    signal?: AbortSignal;
    onProgress?: (progress: ReportDraftProgress | null) => void;
  } = {},
): Promise<ReportDraftUploadResult> {
  const { signal, onProgress } = options;
  const pending = files.filter((f) => stored.get(f.raw)?.projectId !== projectId);
  try {
    for (const [index, item] of pending.entries()) {
      const waiting = pending.length - index - 1;
      onProgress?.({ name: item.name, percent: 0, waiting });
      const confirmed = await uploadEntityAttachment(api, {
        type: "work_item",
        projectId,
        file: item.raw,
        name: item.name,
        signal,
        onProgress: (percent) => onProgress?.({ name: item.name, percent, waiting }),
      });
      stored.set(item.raw, { projectId, attachmentId: confirmed.id });
    }
  } catch (err) {
    if (isCancelled(err, signal)) {
      return { ok: false, cancelled: true, message: "The upload was cancelled." };
    }
    return {
      ok: false,
      cancelled: false,
      message: entityAttachmentErrorMessage(err, "The report could not be uploaded."),
    };
  } finally {
    onProgress?.(null);
  }
  return {
    ok: true,
    ids: files.map((f) => (stored.get(f.raw) as StoredReportDraft).attachmentId),
  };
}
