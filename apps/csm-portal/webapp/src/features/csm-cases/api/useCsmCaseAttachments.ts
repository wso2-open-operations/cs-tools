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

import { useCallback, useRef, useState } from "react";
import {
  useMutation,
  useQuery,
  useQueryClient,
  type UseMutationResult,
  type UseQueryResult,
} from "@tanstack/react-query";
import { ApiQueryKeys, BE_MAX_PAGE_LIMIT } from "@constants/apiConstants";
import { useBackendApi } from "@api/backend/client";
import type {
  BeAttachment,
  BeAttachmentSearchPayload,
  BeAttachmentSearchResponse,
  BeReferenceType,
} from "@api/backend/types";
import { uiAttachmentFromBe } from "@api/backend/mappers";
import type { CaseAttachment } from "@features/csm-cases/types/csmCases";
import {
  isSafeAttachmentContentType,
  type AttachmentPreviewSource,
} from "@features/csm-cases/utils/attachmentPreview";
import { useAttachmentLimits } from "@api/useGetAttachmentLimits";
import { downloadWithProgress } from "@features/csm-cases/api/trackedDownload";
import { useSuccessBanner } from "@context/success-banner/SuccessBannerContext";
import {
  deleteEntityAttachment,
  type DownloadSaveOutcome,
  entityAttachmentErrorMessage,
  WORK_ITEM_ATTACHMENT_TYPE,
  fetchEntityAttachmentBlob,
  type EntityAttachmentType,
  uploadEntityAttachment,
} from "@features/csm-cases/api/entityAttachments";

/** Page size per request. Capped by the BE; see BE_MAX_PAGE_LIMIT. */
const ATTACHMENTS_PAGE_LIMIT = BE_MAX_PAGE_LIMIT;
/** Safety bound on how many pages a single entity's attachment list can page
 * through — 200 pages * BE_MAX_PAGE_LIMIT is far beyond any real case, this
 * only guards against an unbounded loop if the BE's `hasMore` were ever
 * wrong. Same convention as `useGetCsmCaseComments`'s `MAX_COMMENT_PAGES`. */
const MAX_ATTACHMENT_PAGES = 200;

/**
 * Load *every* attachment on a reference entity. Calls `POST /attachments/search`
 * scoped to `referenceType` (defaults to `"case"` for existing call sites),
 * paging through `ATTACHMENTS_PAGE_LIMIT`-sized requests until the BE reports
 * no more (`hasMore`) — a single wide page used to silently drop everything
 * past the first `ATTACHMENTS_PAGE_LIMIT` for a case with more attachments
 * than that. Same paged-loop shape as `useGetCsmCaseComments`/
 * `useGetCsmCaseActivities`: the case-detail page's activity timeline
 * (`CaseActivitiesFeed`) renders every attachment inline alongside comments
 * and audit entries, so the full list — not just a count — has to be eagerly
 * available wherever this hook is used, not just once the Attachments tab is
 * opened.
 */
export function useGetCsmCaseAttachments(
  caseId: string | undefined,
  referenceType: BeReferenceType = "case",
): UseQueryResult<CaseAttachment[], Error> {
  const api = useBackendApi();

  return useQuery<CaseAttachment[], Error>({
    queryKey: [ApiQueryKeys.CSM_CASE_ATTACHMENTS, referenceType, caseId ?? ""],
    queryFn: () => fetchAllAttachments(api, caseId, referenceType, false),
    enabled: !!caseId,
    staleTime: 10_000,
  });
}

/**
 * Load every DELETED attachment of a reference entity, most recently deleted
 * first. Only an admin gets any (the backend drops the request for anyone
 * else), and it is a query of its own so nothing is fetched until `enabled`
 * (the admin opens the Deleted tab). Its key extends the available list's, so
 * an upload or delete that invalidates that list refreshes this one too.
 */
export function useGetCsmDeletedCaseAttachments(
  caseId: string | undefined,
  referenceType: BeReferenceType = "case",
  enabled = true,
): UseQueryResult<CaseAttachment[], Error> {
  const api = useBackendApi();

  return useQuery<CaseAttachment[], Error>({
    queryKey: [ApiQueryKeys.CSM_CASE_ATTACHMENTS, referenceType, caseId ?? "", "deleted"],
    queryFn: () => fetchAllAttachments(api, caseId, referenceType, true),
    enabled: !!caseId && enabled,
    staleTime: 10_000,
  });
}

/** Pages `POST /attachments/search` to the end; `deletedOnly` asks for the deleted ones. */
async function fetchAllAttachments(
  api: ReturnType<typeof useBackendApi>,
  caseId: string | undefined,
  referenceType: BeReferenceType,
  deletedOnly: boolean,
): Promise<CaseAttachment[]> {
  if (!caseId) return [];

  const allAttachments: BeAttachment[] = [];
  let offset = 0;
  for (let page = 0; page < MAX_ATTACHMENT_PAGES; page += 1) {
    const payload: BeAttachmentSearchPayload = {
      referenceId: caseId,
      referenceType,
      pagination: { offset, limit: ATTACHMENTS_PAGE_LIMIT },
      ...(deletedOnly ? { deletedOnly: true } : {}),
    };
    const response = await api.post<
      BeAttachmentSearchPayload,
      BeAttachmentSearchResponse
    >("/attachments/search", payload);
    const rows = response.attachments ?? [];
    allAttachments.push(...rows);
    if (!response.hasMore || rows.length === 0) break;
    offset += rows.length;
  }
  return allAttachments.map(uiAttachmentFromBe);
}

export interface PostCsmCaseAttachmentInput {
  caseId: string;
  file: File;
  /** Display name for the attachment; defaults to the file's own name. */
  name?: string;
  /** Display name of the uploader (used by the mock; the BE sets its own). */
  uploadedBy: string;
  /** Reference entity type, used to refresh the right attachment list. Defaults to `"case"`. */
  referenceType?: BeReferenceType;
  /** Which attachment table the file goes in. Defaults to `"work_item"` (cases, incidents, change requests). */
  type?: EntityAttachmentType;
}

/** Return type of {@link usePostCsmCaseAttachment}: the underlying mutation
 * plus upload progress. */
export type PostCsmCaseAttachmentResult = UseMutationResult<
  CaseAttachment | null,
  Error,
  PostCsmCaseAttachmentInput
> & {
  /**
   * 0-100 while an upload through the portal backend is in flight, `null`
   * otherwise.
   */
  uploadProgress: number | null;
  /**
   * Stops the upload in flight (a large, chunked one; a small one-shot upload
   * cannot be interrupted). The mutation then fails with "The upload was
   * cancelled." and the half-made attachment is left to the server's cleanup.
   */
  cancelUpload: () => void;
};

/**
 * Upload a file attachment to a reference entity. The file goes to the portal
 * backend in chunks and is stored in Azure Blob Storage, with progress in
 * `uploadProgress`.
 *
 * The confirm response is a thin ack, so the list is refetched on success to
 * hydrate the new entry from search.
 */
export function usePostCsmCaseAttachment(): PostCsmCaseAttachmentResult {
  const api = useBackendApi();
  const queryClient = useQueryClient();
  const [uploadProgress, setUploadProgress] = useState<number | null>(null);
  const abortRef = useRef<AbortController | null>(null);
  const cancelUpload = useCallback(() => abortRef.current?.abort(), []);

  const mutation = useMutation<
    CaseAttachment | null,
    Error,
    PostCsmCaseAttachmentInput
  >({
    mutationFn: async (input): Promise<CaseAttachment | null> => {
      setUploadProgress(0);
      const controller = new AbortController();
      abortRef.current = controller;
      try {
        await uploadEntityAttachment(api, {
          type: input.type ?? WORK_ITEM_ATTACHMENT_TYPE,
          parentId: input.caseId,
          file: input.file,
          name: input.name,
          onProgress: setUploadProgress,
          signal: controller.signal,
        });
      } catch (err) {
        throw new Error(
          entityAttachmentErrorMessage(err, "Could not upload the attachment."),
          { cause: err },
        );
      } finally {
        if (abortRef.current === controller) abortRef.current = null;
        setUploadProgress(null);
      }
      return null;
    },
    onSuccess: (_created, variables) => {
      void queryClient.invalidateQueries({
        queryKey: [
          ApiQueryKeys.CSM_CASE_ATTACHMENTS,
          variables.referenceType ?? "case",
          variables.caseId,
        ],
      });
    },
  });

  return { ...mutation, uploadProgress, cancelUpload };
}

/**
 * Re-labels a fetched blob with the attachment's own metadata type, but only
 * when that type is on the safe allowlist (see {@link isSafeAttachmentContentType}).
 *
 * The backend serves every attachment as `application/octet-stream`, which would
 * make the fetched `Blob` un-renderable by `<img>` or the PDF iframe even though
 * the bytes are fine. The attachment's `contentType` from `/attachments/search`
 * is whatever the uploader claimed and is not checked against the bytes, so it
 * is NOT automatically trustworthy: re-labeling with any type would re-enable
 * the stored-XSS the backend's coercion blocks. The blob is therefore only
 * re-labeled when that type is on the same allowlist the backend uses; anything
 * else stays as served, which is un-renderable by design.
 */
function labelBlob(blob: Blob, attachment: CaseAttachment): Blob {
  if (!isSafeAttachmentContentType(attachment.contentType)) return blob;
  return blob.type === attachment.contentType
    ? blob
    : blob.slice(0, blob.size, attachment.contentType);
}

/**
 * Returns a function that downloads an attachment: it is streamed through the
 * portal backend (which checks access) and saved under its own name.
 */
export function useDownloadCsmCaseAttachment(): (
  attachment: CaseAttachment,
) => Promise<void> {
  const api = useBackendApi();
  const { directTransferAboveBytes } = useAttachmentLimits();
  const { showSuccess } = useSuccessBanner();
  const notify = useCallback(
    (name: string, outcome: DownloadSaveOutcome) =>
      showSuccess(outcome === "saved" ? `Downloaded ${name}.` : `Download of ${name} started.`),
    [showSuccess],
  );

  return useCallback(
    (attachment: CaseAttachment): Promise<void> =>
      downloadWithProgress(api, {
        type: WORK_ITEM_ATTACHMENT_TYPE,
        id: attachment.id,
        name: attachment.filename,
        size: attachment.size,
        directAboveBytes: directTransferAboveBytes,
        onSaved: (outcome) => notify(attachment.filename, outcome),
      }),
    [api, directTransferAboveBytes, notify],
  );
}

/**
 * Returns a function that resolves a previewable {@link AttachmentPreviewSource}
 * for {@link AttachmentPreviewDialog}. The file is streamed through the portal
 * backend into a `Blob` wrapped in an object URL (`revoke: true`) that the
 * caller must revoke.
 */
export function useGetCsmCaseAttachmentPreviewSource(): (
  attachment: CaseAttachment,
) => Promise<AttachmentPreviewSource> {
  const api = useBackendApi();

  return useCallback(
    async (attachment: CaseAttachment): Promise<AttachmentPreviewSource> => {
      const blob = await fetchEntityAttachmentBlob(api, {
        type: WORK_ITEM_ATTACHMENT_TYPE,
        id: attachment.id,
      });
      return { url: URL.createObjectURL(labelBlob(blob, attachment)), revoke: true };
    },
    [api],
  );
}

export interface DeleteCsmCaseAttachmentInput {
  /** Owning entity id; used only to invalidate the right attachment list. */
  caseId: string;
  attachmentId: string;
  /** Owning entity type. Defaults to `"case"` for existing call sites. */
  referenceType?: BeReferenceType;
}

/**
 * Delete an attachment. Only the uploader may delete, and not once the case is
 * closed: the backend answers 403 / 409 with the message to show. On success the
 * owning entity's attachment list is invalidated so the row drops.
 */
export function useDeleteCsmCaseAttachment(): UseMutationResult<
  void,
  Error,
  DeleteCsmCaseAttachmentInput
> {
  const api = useBackendApi();
  const queryClient = useQueryClient();

  return useMutation<void, Error, DeleteCsmCaseAttachmentInput>({
    mutationFn: async (input): Promise<void> => {
      try {
        await deleteEntityAttachment(api, { type: WORK_ITEM_ATTACHMENT_TYPE, id: input.attachmentId });
      } catch (err) {
        throw new Error(entityAttachmentErrorMessage(err, "Could not delete the attachment."), {
          cause: err,
        });
      }
    },
    onSuccess: (_void, variables) => {
      void queryClient.invalidateQueries({
        queryKey: [
          ApiQueryKeys.CSM_CASE_ATTACHMENTS,
          variables.referenceType ?? "case",
          variables.caseId,
        ],
      });
    },
  });
}
