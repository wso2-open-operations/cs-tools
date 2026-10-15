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

import { useCallback, useMemo, useRef, useState } from "react";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import { useAttachAnnouncementRequestFilesToCase } from "@features/csm-announcements/api/useAttachAnnouncementRequestFilesToCase";
import { useDeleteAnnouncementRequestAttachment } from "@features/csm-announcements/api/useDeleteAnnouncementRequestAttachment";
import { useDownloadAnnouncementRequestAttachment } from "@features/csm-announcements/api/useDownloadAnnouncementRequestAttachment";
import { useListAnnouncementRequestAttachments } from "@features/csm-announcements/api/useListAnnouncementRequestAttachments";
import { useUploadAnnouncementRequestAttachment } from "@features/csm-announcements/api/useUploadAnnouncementRequestAttachment";
import type { AnnouncementRequestAttachment } from "@features/csm-announcements/types/announcementRequests";
import { announcementAttachmentProblem } from "@features/csm-announcements/utils/announcementAttachments";

/**
 * One row of the attachments list.
 *  - `saved`: already on the request (and so on the server).
 *  - `staged`: picked on a form whose draft does not exist yet; uploaded by
 *    {@link UseAnnouncementAttachments.flushStaged} once it does.
 *  - `uploading`: on its way to a request that exists.
 */
export interface AnnouncementAttachmentItem {
  key: string;
  name: string;
  sizeBytes: number;
  status: "saved" | "staged" | "uploading";
  /** Set for `saved` rows: what the download and remove actions need. */
  attachment?: AnnouncementRequestAttachment;
  /** True while this row's removal is in flight. */
  removing?: boolean;
}

export interface UseAnnouncementAttachments {
  items: AnnouncementAttachmentItem[];
  /** Files saved, waiting to be saved and being uploaded: what a send would carry. */
  count: number;
  /** The saved list is still loading for the first time. */
  isLoading: boolean;
  /** Loading the saved list failed: the count above cannot be trusted. */
  loadFailed: boolean;
  /** Something is uploading or being removed. */
  busy: boolean;
  /** Checks each file, then uploads it (the request exists) or stages it (it does not yet). */
  addFiles: (files: File[]) => Promise<void>;
  remove: (item: AnnouncementAttachmentItem) => Promise<void>;
  download: (item: AnnouncementAttachmentItem) => Promise<void>;
  /**
   * Uploads every staged file to `requestId`, in order. Stops at the first one the
   * server refuses (reported through the error banner) and resolves `false`: what
   * was uploaded stays uploaded, the rest stays staged, and calling it again retries
   * only what is left. Resolves `true` when nothing is left.
   */
  flushStaged: (requestId: string) => Promise<boolean>;
  /**
   * Has the server copy the request's files onto its dry-run case, so the approver
   * sees the announcement as customers will, files included. A failure is reported
   * through the error banner (the case itself exists, and submitting again retries
   * with a fresh one); resolves whether it worked.
   */
  attachToDryRunCase: (requestId: string, caseId: string) => Promise<boolean>;
}

interface PendingFile {
  key: string;
  file: File;
}

function messageOf(error: unknown, fallback: string): string {
  return error instanceof Error && error.message.trim() ? error.message : fallback;
}

/**
 * Everything the announcement forms and the request dialog do with a request's
 * attachments, in one place so every surface behaves the same. With a
 * `requestId` the files are the request's own: picking one uploads it at once, and
 * removing one deletes it on the server (changing a request's files is a content
 * edit, so one waiting for approval goes back to draft). Without one (a create form
 * before its draft exists) picked files are only held in the form; the form then
 * creates the draft and calls `flushStaged`.
 *
 * Problems with a file (type, size, count, duplicate name) are checked up front
 * from `announcementAttachmentProblem`, which only repeats what the entity service
 * checks again; its own reason shows when it refuses one anyway.
 */
export function useAnnouncementAttachments(requestId: string | null | undefined): UseAnnouncementAttachments {
  const { showError } = useErrorBanner();
  const list = useListAnnouncementRequestAttachments(requestId ?? undefined);
  const upload = useUploadAnnouncementRequestAttachment();
  const remover = useDeleteAnnouncementRequestAttachment();
  const downloadFile = useDownloadAnnouncementRequestAttachment();
  const attachToCase = useAttachAnnouncementRequestFilesToCase();

  const [staged, setStaged] = useState<PendingFile[]>([]);
  const [uploading, setUploading] = useState<PendingFile[]>([]);
  const [removingIds, setRemovingIds] = useState<string[]>([]);
  const nextKey = useRef(0);
  const newKey = (): string => `pending-${(nextKey.current += 1)}`;

  const saved = useMemo(() => (requestId ? (list.data ?? []) : []), [requestId, list.data]);

  const items = useMemo<AnnouncementAttachmentItem[]>(
    () => [
      ...saved.map<AnnouncementAttachmentItem>((a) => ({
        key: a.id,
        name: a.name,
        sizeBytes: a.sizeBytes,
        status: "saved",
        attachment: a,
        removing: removingIds.includes(a.id),
      })),
      ...uploading.map<AnnouncementAttachmentItem>((p) => ({
        key: p.key,
        name: p.file.name,
        sizeBytes: p.file.size,
        status: "uploading",
      })),
      ...staged.map<AnnouncementAttachmentItem>((p) => ({
        key: p.key,
        name: p.file.name,
        sizeBytes: p.file.size,
        status: "staged",
      })),
    ],
    [saved, uploading, staged, removingIds],
  );

  const uploadOne = useCallback(
    async (id: string, entry: PendingFile): Promise<void> => {
      setUploading((prev) => [...prev, entry]);
      try {
        await upload.mutateAsync({ id, file: entry.file });
      } finally {
        setUploading((prev) => prev.filter((p) => p.key !== entry.key));
      }
    },
    [upload],
  );

  const addFiles = useCallback(
    async (files: File[]): Promise<void> => {
      const names = items.map((i) => i.name);
      const accepted: File[] = [];
      for (const file of files) {
        const problem = announcementAttachmentProblem(file, names);
        if (problem) {
          showError(problem);
          continue;
        }
        names.push(file.name);
        accepted.push(file);
      }
      if (accepted.length === 0) return;

      if (!requestId) {
        setStaged((prev) => [...prev, ...accepted.map((file) => ({ key: newKey(), file }))]);
        return;
      }
      // One at a time: the server counts the files and refuses names it already has,
      // so two uploads racing each other could both be judged against the same list.
      for (const file of accepted) {
        try {
          await uploadOne(requestId, { key: newKey(), file });
        } catch (error) {
          showError(messageOf(error, `Could not attach "${file.name}". Please try again.`));
        }
      }
    },
    [items, requestId, showError, uploadOne],
  );

  const remove = useCallback(
    async (item: AnnouncementAttachmentItem): Promise<void> => {
      if (item.status === "staged") {
        setStaged((prev) => prev.filter((p) => p.key !== item.key));
        return;
      }
      if (item.status !== "saved" || !requestId || !item.attachment) return;
      const attachmentId = item.attachment.id;
      setRemovingIds((prev) => [...prev, attachmentId]);
      try {
        await remover.mutateAsync({ id: requestId, attachmentId });
      } catch (error) {
        showError(messageOf(error, `Could not remove "${item.name}". Please try again.`));
      } finally {
        setRemovingIds((prev) => prev.filter((id) => id !== attachmentId));
      }
    },
    [remover, requestId, showError],
  );

  const download = useCallback(
    async (item: AnnouncementAttachmentItem): Promise<void> => {
      if (!requestId || !item.attachment) return;
      try {
        await downloadFile(requestId, item.attachment);
      } catch (error) {
        showError(messageOf(error, `Could not download "${item.name}". Please try again.`));
      }
    },
    [downloadFile, requestId, showError],
  );

  const flushStaged = useCallback(
    async (id: string): Promise<boolean> => {
      for (const entry of staged) {
        try {
          await uploadOne(id, entry);
          setStaged((prev) => prev.filter((p) => p.key !== entry.key));
        } catch (error) {
          showError(`"${entry.file.name}" could not be attached: ${messageOf(error, "please try again.")}`);
          return false;
        }
      }
      return true;
    },
    [showError, staged, uploadOne],
  );

  const attachToDryRunCase = useCallback(
    async (id: string, caseId: string): Promise<boolean> => {
      try {
        await attachToCase.mutateAsync({ id, caseId });
        return true;
      } catch (error) {
        showError(
          `The dry-run case was created, but its attachments couldn't be added: ${messageOf(
            error,
            "please try again.",
          )} Submit again to retry.`,
        );
        return false;
      }
    },
    [attachToCase, showError],
  );

  return {
    items,
    count: items.length,
    isLoading: !!requestId && list.isLoading,
    loadFailed: !!requestId && list.isError,
    busy: uploading.length > 0 || removingIds.length > 0,
    addFiles,
    remove,
    download,
    flushStaged,
    attachToDryRunCase,
  };
}
