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

import {
  confirmStoredUpload,
  isAbortError,
  uploadStoredAttachment,
  UploadConfirmPendingError,
  type AuthFetch,
} from "@api/storedAttachments";
import { uploadPercent } from "@api/caseUploadProgress";

/** A file chosen for a security report, as the form holds it. */
export interface ReportDraftFile {
  /** Stable id of the file in the form; what a stored id is remembered under. */
  key: string;
  file: File;
  /** The name to store it under. */
  name: string;
}

/** What is known of an upload that finished: kept across retries so it is never sent twice. */
export interface StoredReportDraft {
  projectId: string;
  attachmentId: string;
}

export type ReportDraftUploadResult =
  | { ok: true; ids: string[] }
  | { ok: false; cancelled: boolean; message: string };

/** Which file is uploading, how far it got, and how many wait behind it. */
export interface ReportDraftProgress {
  name: string;
  percent: number;
  waiting: number;
}

/**
 * Uploads the files of a security report BEFORE its case exists, one after another,
 * as drafts of the project (`type: "work_item"` with `projectId`). The case request
 * then names the ids in `attachmentIds`, and entity-service links them to the case it
 * creates, so a cancelled or refreshed upload leaves no case without its report.
 *
 * A file that already uploaded (its key is in `stored` for this project) is not sent
 * again, so a retry after a failed create only uploads what did not finish. The first
 * failure stops the run: the caller must not create the case. A cancel (`signal`)
 * ends it with `cancelled: true`, which is not an error to show.
 *
 * @param authFetch - The authenticated fetch of the portal.
 * @param projectId - The project the case is being created in.
 * @param files - The files in the form, in order.
 * @param stored - Ids already obtained, by file key; updated as files finish.
 * @param options - `signal` cancels; `onProgress` reports the file being sent.
 * @returns {Promise<ReportDraftUploadResult>} The attachment ids in the order of `files`, or the failure.
 */
export async function uploadReportDrafts(
  authFetch: AuthFetch,
  projectId: string,
  files: readonly ReportDraftFile[],
  stored: Map<string, StoredReportDraft>,
  options: {
    signal?: AbortSignal;
    onProgress?: (progress: ReportDraftProgress | null) => void;
  } = {},
): Promise<ReportDraftUploadResult> {
  const { signal, onProgress } = options;
  const pending = files.filter((f) => stored.get(f.key)?.projectId !== projectId);
  try {
    for (const [index, item] of pending.entries()) {
      onProgress?.({ name: item.name, percent: 0, waiting: pending.length - index - 1 });
      let attachmentId: string;
      try {
        attachmentId = (
          await uploadStoredAttachment(authFetch, {
            type: "work_item",
            projectId,
            file: item.file,
            name: item.name,
            signal,
            onProgress: (sent, total) =>
              onProgress?.({
                name: item.name,
                percent: uploadPercent(sent, total),
                waiting: pending.length - index - 1,
              }),
          })
        ).id;
      } catch (error) {
        // The file is already in place: finish the upload instead of repeating it.
        if (!(error instanceof UploadConfirmPendingError)) throw error;
        attachmentId = (
          await confirmStoredUpload(authFetch, {
            type: "work_item",
            attachmentId: error.attachmentId,
            chunks: error.chunks,
          })
        ).id;
      }
      stored.set(item.key, { projectId, attachmentId });
    }
  } catch (error) {
    if (isAbortError(error) || signal?.aborted) {
      return { ok: false, cancelled: true, message: "The upload was cancelled." };
    }
    return {
      ok: false,
      cancelled: false,
      message:
        error instanceof Error && error.message
          ? error.message
          : "The report could not be uploaded.",
    };
  } finally {
    onProgress?.(null);
  }
  return {
    ok: true,
    ids: files.map((f) => (stored.get(f.key) as StoredReportDraft).attachmentId),
  };
}
