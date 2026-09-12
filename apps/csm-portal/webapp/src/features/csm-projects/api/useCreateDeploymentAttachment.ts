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
  useMutation,
  useQueryClient,
  type UseMutationResult,
} from "@tanstack/react-query";
import { ApiQueryKeys } from "@constants/apiConstants";
import { useBackendApi } from "@api/backend/client";
import type {
  BeAttachmentCreatePayload,
  BeAttachmentCreateResponse,
} from "@api/backend/types";

/** Max upload size in bytes — the BE caps the decoded file at 10 MB. */
export const MAX_DEPLOYMENT_ATTACHMENT_SIZE_BYTES = 10 * 1024 * 1024;

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

export interface CreateDeploymentAttachmentInput {
  deploymentId: string;
  file: File;
  /** Display name for the attachment; defaults to the file's own name. */
  name?: string;
  /** Optional free-text note stored with the attachment. */
  description?: string;
}

/**
 * Upload a file to a deployment via `POST /attachments`
 * (`referenceType: "deployment"`). The create response is a thin ack, so the
 * list is invalidated on success to hydrate the new entry from search.
 */
export function useCreateDeploymentAttachment(): UseMutationResult<
  BeAttachmentCreateResponse,
  Error,
  CreateDeploymentAttachmentInput
> {
  const api = useBackendApi();
  const queryClient = useQueryClient();

  return useMutation<
    BeAttachmentCreateResponse,
    Error,
    CreateDeploymentAttachmentInput
  >({
    mutationFn: async (input): Promise<BeAttachmentCreateResponse> => {
      if (input.file.size > MAX_DEPLOYMENT_ATTACHMENT_SIZE_BYTES) {
        throw new Error(
          `"${input.file.name}" is too large. The maximum attachment size is ${
            MAX_DEPLOYMENT_ATTACHMENT_SIZE_BYTES / (1024 * 1024)
          } MB.`,
        );
      }

      const dataUri = await readFileAsDataUrl(input.file);
      const payload: BeAttachmentCreatePayload = {
        referenceId: input.deploymentId,
        referenceType: "deployment",
        name: input.name?.trim() || input.file.name,
        type: input.file.type || "application/octet-stream",
        file: dataUri,
        description: input.description?.trim() || null,
      };
      return api.post<BeAttachmentCreatePayload, BeAttachmentCreateResponse>(
        "/attachments",
        payload,
      );
    },
    onSuccess: (_created, variables) => {
      queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.DEPLOYMENT_ATTACHMENTS, variables.deploymentId],
      });
    },
  });
}
