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
import {
  entityAttachmentErrorMessage,
  uploadEntityAttachment,
} from "@features/csm-cases/api/entityAttachments";

export interface CreateDeploymentAttachmentInput {
  deploymentId: string;
  file: File;
  /** Display name for the attachment; defaults to the file's own name. */
  name?: string;
}

/**
 * Upload a file to a deployment: it goes to the portal backend in chunks and is
 * stored in Azure Blob Storage (attachment type `deployment`). The confirm
 * response is a thin ack, so the list is invalidated on success to hydrate the
 * new entry from search.
 */
export function useCreateDeploymentAttachment(): UseMutationResult<
  void,
  Error,
  CreateDeploymentAttachmentInput
> {
  const api = useBackendApi();
  const queryClient = useQueryClient();

  return useMutation<void, Error, CreateDeploymentAttachmentInput>({
    mutationFn: async (input): Promise<void> => {
      try {
        await uploadEntityAttachment(api, {
          type: "deployment",
          parentId: input.deploymentId,
          file: input.file,
          name: input.name?.trim() || input.file.name,
        });
      } catch (err) {
        throw new Error(entityAttachmentErrorMessage(err, "Could not upload the attachment."), {
          cause: err,
        });
      }
    },
    onSuccess: (_created, variables) => {
      void queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.DEPLOYMENT_ATTACHMENTS, variables.deploymentId],
      });
    },
  });
}
