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
  DOWNLOAD_STARTED,
  downloadPercent,
  isAttachmentDownloading,
  setAttachmentDownloadAbort,
  setAttachmentDownloadProgress,
} from "@features/csm-cases/api/attachmentDownloadProgress";
import { isDownloadAbort } from "@features/csm-cases/api/downloadAbort";
import {
  downloadEntityAttachment,
  type DownloadSaveOutcome,
  type EntityAttachmentType,
} from "@features/csm-cases/api/entityAttachments";

/**
 * Downloads an attachment and keeps the shared progress store up to date, so any
 * row showing it (a list, the activity feed, a deployment's files) shows the
 * percentage and a Cancel button without the page passing anything down. One
 * download of an attachment runs at a time (a second call does nothing). A
 * download the person cancels ends quietly: nothing is saved and nothing thrown.
 */
export async function downloadWithProgress(
  api: BackendApi,
  args: {
    type: EntityAttachmentType;
    id: string;
    name?: string;
    size?: number;
    /** The server's `directTransferAboveBytes`; 0 keeps the download on the backend. */
    directAboveBytes: number;
    onSaved?: (outcome: DownloadSaveOutcome) => void;
  },
): Promise<void> {
  if (isAttachmentDownloading(args.id)) return;
  const controller = new AbortController();
  setAttachmentDownloadAbort(args.id, controller);
  setAttachmentDownloadProgress(args.id, DOWNLOAD_STARTED);
  try {
    await downloadEntityAttachment(api, {
      ...args,
      signal: controller.signal,
      onProgress: (received, total) =>
        setAttachmentDownloadProgress(args.id, downloadPercent(received, total)),
    });
  } catch (err) {
    if (!isDownloadAbort(err)) throw err;
  } finally {
    setAttachmentDownloadProgress(args.id, undefined);
    setAttachmentDownloadAbort(args.id, undefined);
  }
}
