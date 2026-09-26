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

import { useCallback, useState } from "react";
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
  BeAttachmentConfirmResponse,
  BeAttachmentCreatePayload,
  BeAttachmentCreateResponse,
  BeAttachmentSearchPayload,
  BeAttachmentSearchResponse,
  BeAttachmentShareResponse,
  BeAttachmentUploadTokenRequest,
  BeAttachmentUploadTokenResponse,
  BeDeleteAttachmentResponse,
  BeReferenceType,
} from "@api/backend/types";
import { uiAttachmentFromBe } from "@api/backend/mappers";
import { saveBlob } from "@utils/saveBlob";
import type { CaseAttachment } from "@features/csm-cases/types/csmCases";
import {
  isSafeAttachmentContentType,
  type AttachmentPreviewSource,
} from "@features/csm-cases/utils/attachmentPreview";
import { useCurrentUser } from "@context/current-user/CurrentUserContext";
import { uploadFileViaTus } from "@features/csm-cases/api/attachmentStorageTus";

/** Page size per request. Capped by the BE; see BE_MAX_PAGE_LIMIT. */
const ATTACHMENTS_PAGE_LIMIT = BE_MAX_PAGE_LIMIT;
/** Safety bound on how many pages a single entity's attachment list can page
 * through — 200 pages * BE_MAX_PAGE_LIMIT is far beyond any real case, this
 * only guards against an unbounded loop if the BE's `hasMore` were ever
 * wrong. Same convention as `useGetCsmCaseComments`'s `MAX_COMMENT_PAGES`. */
const MAX_ATTACHMENT_PAGES = 200;

/**
 * Max upload size in bytes. The BE caps the decoded file at 10 MB (the
 * request body itself is capped higher to allow base64 inflation). Validate
 * here so the user gets a clear message instead of a 413.
 */
export const MAX_ATTACHMENT_SIZE_BYTES = 10 * 1024 * 1024;

/** Read a File as a base64 data URI (`data:<mime>;base64,...`). */
function readFileAsDataUrl(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => {
      if (typeof reader.result === "string") resolve(reader.result);
      else reject(new Error("Failed to read the file."));
    };
    reader.onerror = () =>
      reject(reader.error ?? new Error("Failed to read the file."));
    reader.readAsDataURL(file);
  });
}

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
    queryFn: async (): Promise<CaseAttachment[]> => {
      if (!caseId) return [];

      const allAttachments: BeAttachment[] = [];
      let offset = 0;
      for (let page = 0; page < MAX_ATTACHMENT_PAGES; page += 1) {
        const payload: BeAttachmentSearchPayload = {
          referenceId: caseId,
          referenceType,
          pagination: { offset, limit: ATTACHMENTS_PAGE_LIMIT },
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
    },
    enabled: !!caseId,
    staleTime: 10_000,
  });
}

export interface PostCsmCaseAttachmentInput {
  caseId: string;
  file: File;
  /** Display name for the attachment; defaults to the file's own name. */
  name?: string;
  /** Optional free-text note stored with the attachment. */
  description?: string;
  /** Display name of the uploader (used by the mock; the BE sets its own). */
  uploadedBy: string;
  /** Reference entity type. Defaults to `"case"` for existing call sites. */
  referenceType?: BeReferenceType;
}

/** Return type of {@link usePostCsmCaseAttachment}: the underlying mutation
 * plus upload progress for the SFTPGo direct-upload path. */
export type PostCsmCaseAttachmentResult = UseMutationResult<
  CaseAttachment | null,
  Error,
  PostCsmCaseAttachmentInput
> & {
  /**
   * 0-100 while a direct-to-SFTPGo upload is in flight, `null` otherwise
   * (including for the whole default base64-payload path, which has no
   * granular progress). Only meaningful when
   * `sftpgoAttachmentStorageEnabled` is on.
   */
  uploadProgress: number | null;
};

/**
 * Upload a file attachment to a reference entity.
 *
 * Two mutually exclusive paths, gated on the signed-in user's
 * `sftpgoAttachmentStorageEnabled` flag (`GET /users/me`) AND on
 * `referenceType`:
 *  - Off, or `referenceType` is `"change_request"`/`"incident"`: the file is
 *    sent as a base64 data URI in a single `POST /attachments`. Direct-upload
 *    storage only supports `"case"` today — the BE authorizes but then
 *    rejects change-request/incident mint requests with a deterministic 422,
 *    so those two reference types always take this path regardless of the
 *    flag.
 *  - On, and `referenceType` is `"case"` (or absent): `POST
 *    /cases/{id}/attachments/upload-token` registers the attachment's
 *    metadata (in "pending" status) and mints a write-scoped SFTPGo share;
 *    the file's bytes then go straight from the browser to SFTPGo via that
 *    share (mirroring `uploadProgress`); finally `POST
 *    /cases/{id}/attachments/{attachmentId}/confirm` transitions the row to
 *    "complete".
 *
 * The confirm/create response is a thin ack either way, so the list is
 * refetched on success to hydrate the new entry from search.
 */
export function usePostCsmCaseAttachment(): PostCsmCaseAttachmentResult {
  const api = useBackendApi();
  const queryClient = useQueryClient();
  const { user } = useCurrentUser();
  const sftpgoEnabled = !!user?.sftpgoAttachmentStorageEnabled;
  const [uploadProgress, setUploadProgress] = useState<number | null>(null);

  const mutation = useMutation<
    CaseAttachment | null,
    Error,
    PostCsmCaseAttachmentInput
  >({
    mutationFn: async (input): Promise<CaseAttachment | null> => {
      if (input.file.size > MAX_ATTACHMENT_SIZE_BYTES) {
        throw new Error(
          `"${input.file.name}" is too large. The maximum attachment size is ${
            MAX_ATTACHMENT_SIZE_BYTES / (1024 * 1024)
          } MB.`,
        );
      }

      const referenceType = input.referenceType ?? "case";
      const name = input.name?.trim() || input.file.name;
      const type = input.file.type || "application/octet-stream";

      // Direct-upload storage only supports "case" today; change-request and
      // incident uploads always fall through to the legacy base64 path below,
      // regardless of the flag.
      if (sftpgoEnabled && referenceType === "case") {
        setUploadProgress(0);
        try {
          const tokenRequest: BeAttachmentUploadTokenRequest = {
            filename: name,
            mimeType: type,
            sizeBytes: input.file.size,
            description: input.description?.trim() || null,
            referenceType,
          };
          const tokenResponse = await api.post<
            BeAttachmentUploadTokenRequest,
            BeAttachmentUploadTokenResponse
          >(
            `/cases/${encodeURIComponent(input.caseId)}/attachments/upload-token`,
            tokenRequest,
          );

          await uploadFileViaTus({
            sftpgoBaseUrl: tokenResponse.sftpgoBaseUrl,
            shareId: tokenResponse.shareId,
            storageKey: tokenResponse.storageKey,
            file: input.file,
            onProgress: setUploadProgress,
          });

          await api.post<Record<string, never>, BeAttachmentConfirmResponse>(
            `/cases/${encodeURIComponent(input.caseId)}/attachments/${encodeURIComponent(tokenResponse.id)}/confirm`,
            {},
          );
        } finally {
          setUploadProgress(null);
        }
        // The confirm response is a thin ack; refetch hydrates the full entry.
        return null;
      }

      const dataUri = await readFileAsDataUrl(input.file);
      const payload: BeAttachmentCreatePayload = {
        referenceId: input.caseId,
        referenceType,
        name,
        type,
        file: dataUri,
        description: input.description?.trim() || null,
      };
      await api.post<BeAttachmentCreatePayload, BeAttachmentCreateResponse>(
        "/attachments",
        payload,
      );
      // The create response is a thin ack; refetch hydrates the full entry.
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

  return { ...mutation, uploadProgress };
}

/**
 * Returns a function that fetches an attachment's raw bytes via
 * `GET /attachments/{id}/content`. The content endpoint always responds with
 * `Content-Disposition: attachment` and streams behind auth, so it is fetched
 * as a `Blob` (a plain `<a href>`/`<img src>` would miss the auth headers and
 * would force a download instead of rendering inline). Shared by the download
 * action and the attachment preview dialog.
 *
 * The BE's response `Content-Type` is only forwarded as-is for an allowlist
 * of known-safe types (`safeAttachmentTypes` in the entity-service's
 * `case_handler.go`) — anything else, including every `video/*` type today,
 * is coerced to `application/octet-stream` to block a stored-XSS via a
 * crafted upstream type. That coercion is correct for the raw endpoint but
 * would make the fetched `Blob` un-renderable by `<img>`/the PDF iframe even
 * though the bytes are fine.
 *
 * The attachment's `contentType` from `/attachments/search` list metadata is
 * whatever the uploader claimed — the BE doesn't coerce it, but it also
 * doesn't verify it against the actual bytes, so it is uploader-controlled
 * and NOT automatically trustworthy. Re-labeling the blob with it would
 * re-enable exactly the coercion the backend allowlist is meant to block
 * (e.g. a file uploaded with a spoofed benign `contentType` but different
 * actual bytes). So the blob is only ever re-labeled when that metadata
 * `contentType` is itself a member of the same backend allowlist, mirrored
 * client-side as {@link isSafeAttachmentContentType} — for anything else the
 * blob is returned as-is (`application/octet-stream`, matching what the
 * backend already coerced it to), which is un-renderable by design.
 */
export function useGetCsmCaseAttachmentContent(): (
  attachment: CaseAttachment,
) => Promise<Blob> {
  const api = useBackendApi();

  return useCallback(
    async (attachment: CaseAttachment): Promise<Blob> => {
      const blob = await api.getBlob(
        `/attachments/${encodeURIComponent(attachment.id)}/content`,
      );
      if (!isSafeAttachmentContentType(attachment.contentType)) return blob;
      return blob.type === attachment.contentType
        ? blob
        : blob.slice(0, blob.size, attachment.contentType);
    },
    [api],
  );
}

/** Opens a URL for download in a new tab, rather than navigating the SPA away
 * from itself. Used for the SFTPGo share URL below, which is a plain public
 * link (not fetched as a `Blob` through this app's own auth) — a normal link
 * click, not a `fetch`, so cross-origin/CORS is a non-issue. */
function openForDownload(url: string): void {
  const a = document.createElement("a");
  a.href = url;
  a.target = "_blank";
  a.rel = "noopener noreferrer";
  document.body.appendChild(a);
  a.click();
  a.remove();
}

/**
 * Returns a function that downloads an attachment, built on
 * {@link useGetCsmCaseAttachmentContent}.
 *
 * When `sftpgoAttachmentStorageEnabled` is on, this creates a share
 * (`POST /attachments/{id}/share`) for exactly the one attachment being
 * downloaded — never eagerly for a whole list, see
 * `AttachmentStorageHandler.CreateAttachmentShare`'s doc comment on the
 * backend — and opens the returned short-lived public URL directly, instead
 * of fetching the content as an authenticated `Blob`.
 */
export function useDownloadCsmCaseAttachment(): (
  attachment: CaseAttachment,
) => Promise<void> {
  const api = useBackendApi();
  const getContent = useGetCsmCaseAttachmentContent();
  const { user } = useCurrentUser();
  const sftpgoEnabled = !!user?.sftpgoAttachmentStorageEnabled;

  return useCallback(
    async (attachment: CaseAttachment): Promise<void> => {
      if (sftpgoEnabled) {
        const { shareUrl } = await api.post<
          Record<string, never>,
          BeAttachmentShareResponse
        >(`/attachments/${encodeURIComponent(attachment.id)}/share`, {});
        openForDownload(shareUrl);
        return;
      }
      const blob = await getContent(attachment);
      saveBlob(blob, attachment.filename);
    },
    [api, getContent, sftpgoEnabled],
  );
}

/**
 * Returns a function that resolves a previewable {@link AttachmentPreviewSource}
 * for {@link AttachmentPreviewDialog}, mirroring the same
 * `sftpgoAttachmentStorageEnabled` branching {@link useDownloadCsmCaseAttachment}
 * already uses instead of always going through the authenticated content
 * endpoint (which the entity-service deliberately 503s for SFTPGo-backed
 * attachments — bytes for that data source are only reachable via a share
 * URL, never the raw content endpoint).
 *
 * When the flag is on, a read-scoped share (`POST /attachments/{id}/share`)
 * is minted and its `shareUrl` is returned as-is (`revoke: false`) — it is
 * already a directly usable public URL, so there is no reason to also fetch
 * it as a `Blob` just to re-wrap it in an object URL. Otherwise, falls back
 * to {@link useGetCsmCaseAttachmentContent} and wraps the resulting `Blob` in
 * an object URL (`revoke: true`); the caller is responsible for revoking it.
 */
export function useGetCsmCaseAttachmentPreviewSource(): (
  attachment: CaseAttachment,
) => Promise<AttachmentPreviewSource> {
  const api = useBackendApi();
  const getContent = useGetCsmCaseAttachmentContent();
  const { user } = useCurrentUser();
  const sftpgoEnabled = !!user?.sftpgoAttachmentStorageEnabled;

  return useCallback(
    async (attachment: CaseAttachment): Promise<AttachmentPreviewSource> => {
      if (sftpgoEnabled) {
        const { shareUrl } = await api.post<
          Record<string, never>,
          BeAttachmentShareResponse
        >(`/attachments/${encodeURIComponent(attachment.id)}/share`, {});
        return { url: shareUrl, revoke: false };
      }
      const blob = await getContent(attachment);
      return { url: URL.createObjectURL(blob), revoke: true };
    },
    [api, getContent, sftpgoEnabled],
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
 * Delete an attachment via `DELETE /attachments/{id}` (ServiceNow data source
 * only). On success the owning entity's attachment list is invalidated so the
 * row drops.
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
      await api.del<BeDeleteAttachmentResponse>(
        `/attachments/${encodeURIComponent(input.attachmentId)}`,
      );
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
