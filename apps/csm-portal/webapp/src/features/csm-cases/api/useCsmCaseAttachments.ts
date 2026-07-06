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

import { useCallback } from "react";
import {
  useMutation,
  useQuery,
  useQueryClient,
  type UseMutationResult,
  type UseQueryResult,
} from "@tanstack/react-query";
import { ApiQueryKeys } from "@constants/apiConstants";
import { useBackendApi } from "@api/backend/client";
import type {
  BeAttachmentCreatePayload,
  BeAttachmentCreateResponse,
  BeAttachmentSearchPayload,
  BeAttachmentSearchResponse,
  BeDeleteAttachmentResponse,
} from "@api/backend/types";
import { uiAttachmentFromBe } from "@api/backend/mappers";
import type { CaseAttachment } from "@features/csm-cases/types/csmCases";

/**
 * Page size used by the attachments list. A single wide page is enough for the
 * case-detail view. If a case ever exceeds this, switch to an explicit
 * pagination wrapper rather than chasing pages.
 */
const ATTACHMENTS_PAGE_LIMIT = 50;

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

/** Trigger a browser "save as" for a fetched blob. */
function saveBlob(blob: Blob, filename: string): void {
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(url);
}

/**
 * Load all attachments on a case. Calls `POST /attachments/search` scoped to the
 * case (`referenceType: "case"`) with a single wide page.
 */
export function useGetCsmCaseAttachments(
  caseId: string | undefined,
): UseQueryResult<CaseAttachment[], Error> {
  const api = useBackendApi();

  return useQuery<CaseAttachment[], Error>({
    queryKey: [ApiQueryKeys.CSM_CASE_ATTACHMENTS, caseId ?? ""],
    queryFn: async (): Promise<CaseAttachment[]> => {
      if (!caseId) return [];

      const payload: BeAttachmentSearchPayload = {
        referenceId: caseId,
        referenceType: "case",
        pagination: { offset: 0, limit: ATTACHMENTS_PAGE_LIMIT },
      };
      const response = await api.post<
        BeAttachmentSearchPayload,
        BeAttachmentSearchResponse
      >("/attachments/search", payload);
      return response.attachments.map(uiAttachmentFromBe);
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
}

/**
 * Upload a file attachment to a case via `POST /attachments` (scoped with
 * `referenceType: "case"`). The file is sent as a base64 data URI. The create
 * response is a thin ack, so the list is refetched on success to hydrate the
 * new entry from search.
 */
export function usePostCsmCaseAttachment(): UseMutationResult<
  CaseAttachment | null,
  Error,
  PostCsmCaseAttachmentInput
> {
  const api = useBackendApi();
  const queryClient = useQueryClient();

  return useMutation<CaseAttachment | null, Error, PostCsmCaseAttachmentInput>({
    mutationFn: async (input): Promise<CaseAttachment | null> => {
      if (input.file.size > MAX_ATTACHMENT_SIZE_BYTES) {
        throw new Error(
          `"${input.file.name}" is too large. The maximum attachment size is ${
            MAX_ATTACHMENT_SIZE_BYTES / (1024 * 1024)
          } MB.`,
        );
      }

      const dataUri = await readFileAsDataUrl(input.file);
      const payload: BeAttachmentCreatePayload = {
        referenceId: input.caseId,
        referenceType: "case",
        name: input.name?.trim() || input.file.name,
        type: input.file.type || "application/octet-stream",
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
        queryKey: [ApiQueryKeys.CSM_CASE_ATTACHMENTS, variables.caseId],
      });
    },
  });
}

/**
 * Returns a function that downloads an attachment's content and saves it via
 * `GET /attachments/{id}/content`. The content endpoint streams raw bytes
 * behind auth, so it is fetched as a blob (a plain `<a href>` would miss the
 * auth headers) and handed to the browser.
 */
export function useDownloadCsmCaseAttachment(): (
  attachment: CaseAttachment,
) => Promise<void> {
  const api = useBackendApi();

  return useCallback(
    async (attachment: CaseAttachment): Promise<void> => {
      const blob = await api.getBlob(
        `/attachments/${encodeURIComponent(attachment.id)}/content`,
      );
      saveBlob(blob, attachment.filename);
    },
    [api],
  );
}

export interface DeleteCsmCaseAttachmentInput {
  /** Owning case id; used only to invalidate the right attachment list. */
  caseId: string;
  attachmentId: string;
}

/**
 * Delete an attachment via `DELETE /attachments/{id}` (ServiceNow data source
 * only). On success the case's attachment list is invalidated so the row drops.
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
        queryKey: [ApiQueryKeys.CSM_CASE_ATTACHMENTS, variables.caseId],
      });
    },
  });
}
