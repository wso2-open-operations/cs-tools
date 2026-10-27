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

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { useAuthApiClient } from "@/hooks/useAuthApiClient";
import { ApiQueryKeys } from "@constants/apiConstants";
import {
  confirmStoredUpload,
  deleteStoredAttachment,
  downloadStoredAttachment,
  uploadStoredAttachment,
  type StoredAttachmentConfirmed,
  type StoredAttachmentType,
  type StoredUploadStep,
} from "@api/storedAttachments";

/** Input for a download through entity-service. */
export interface StoredAttachmentDownloadInput {
  type: StoredAttachmentType;
  id: string;
  name?: string;
}

/**
 * Downloads an attachment through entity-service's one-use link. The link is
 * requested only when this is called (a click) and used at once.
 */
export function useStoredAttachmentDownload(): {
  downloadStoredAttachment: (input: StoredAttachmentDownloadInput) => Promise<void>;
  isDownloading: boolean;
  downloadingId: string | null;
} {
  const authFetch = useAuthApiClient();
  const [downloadingId, setDownloadingId] = useState<string | null>(null);
  const mutation = useMutation<void, Error, StoredAttachmentDownloadInput>({
    mutationFn: (input) => downloadStoredAttachment(authFetch, input),
    onMutate: (input) => setDownloadingId(input.id),
    onSettled: () => setDownloadingId(null),
  });
  return {
    downloadStoredAttachment: mutation.mutateAsync,
    isDownloading: mutation.isPending,
    downloadingId,
  };
}

/** Variables of a stored upload: a case attachment (`caseId`) or a deployment document (`deploymentId`). */
export interface StoredUploadVariables {
  type: StoredAttachmentType;
  parentId: string;
  file: File;
  /** Same shape as the older upload's variables, so the pending-upload placeholder works unchanged. */
  body: { name: string };
  caseId?: string;
  deploymentId?: string;
  onStep?: (step: StoredUploadStep) => void;
}

/**
 * Uploads a file in three steps (start, send to SFTPGo, confirm). Shares the
 * `postCaseAttachment` mutation key with the older upload so the case's pending
 * placeholder shows either way. A confirm that failed after the file arrived
 * throws UploadConfirmPendingError: use `confirm` to retry without re-uploading.
 */
export function useStoredAttachmentUpload() {
  const authFetch = useAuthApiClient();
  const queryClient = useQueryClient();

  const invalidate = (vars: { caseId?: string; deploymentId?: string }) => {
    if (vars.caseId) {
      queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.CASE_ATTACHMENTS, vars.caseId],
      });
      queryClient.invalidateQueries({ queryKey: [ApiQueryKeys.CASE_COMMENTS] });
      queryClient.invalidateQueries({ queryKey: [ApiQueryKeys.CASE_DETAILS] });
    }
    if (vars.deploymentId) {
      queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.DEPLOYMENT_ATTACHMENTS, vars.deploymentId],
      });
      queryClient.invalidateQueries({ queryKey: [ApiQueryKeys.DEPLOYMENTS] });
    }
  };

  const upload = useMutation<StoredAttachmentConfirmed, Error, StoredUploadVariables>({
    mutationKey: ["postCaseAttachment"],
    mutationFn: ({ type, parentId, file, body, onStep }) =>
      uploadStoredAttachment(authFetch, {
        type,
        parentId,
        file,
        name: body.name,
        onStep,
      }),
    onSuccess: (_data, vars) => invalidate(vars),
  });

  const confirm = useMutation<
    StoredAttachmentConfirmed,
    Error,
    { type: StoredAttachmentType; attachmentId: string; caseId?: string; deploymentId?: string }
  >({
    mutationFn: ({ type, attachmentId }) =>
      confirmStoredUpload(authFetch, { type, attachmentId }),
    onSuccess: (_data, vars) => invalidate(vars),
  });

  return { upload, confirm };
}

/** Variables of a stored delete. */
export interface StoredDeleteVariables {
  type: StoredAttachmentType;
  attachmentId: string;
  caseId?: string;
  deploymentId?: string;
}

/**
 * Deletes a stored attachment. Only its uploader may (403 otherwise) and not once
 * the case is closed (409); the server's message is the error message.
 */
export function useStoredAttachmentDelete() {
  const authFetch = useAuthApiClient();
  const queryClient = useQueryClient();
  return useMutation<void, Error, StoredDeleteVariables>({
    mutationFn: ({ type, attachmentId }) =>
      deleteStoredAttachment(authFetch, { type, id: attachmentId }),
    onSuccess: (_data, { caseId, deploymentId }) => {
      if (caseId) {
        queryClient.invalidateQueries({
          queryKey: [ApiQueryKeys.CASE_ATTACHMENTS, caseId],
        });
      }
      if (deploymentId) {
        queryClient.invalidateQueries({
          queryKey: [ApiQueryKeys.DEPLOYMENT_ATTACHMENTS, deploymentId],
        });
      }
    },
  });
}
