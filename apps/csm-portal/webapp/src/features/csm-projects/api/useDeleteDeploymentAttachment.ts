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
  deleteEntityAttachment,
  entityAttachmentErrorMessage,
} from "@features/csm-cases/api/entityAttachments";

export interface DeleteDeploymentAttachmentInput {
  /** Owning deployment id; used only to invalidate the right attachment list. */
  deploymentId: string;
  attachmentId: string;
}

/**
 * Delete a deployment attachment. Only the uploader may delete it: the backend
 * answers 403 with the message to show. On success the deployment's attachment
 * list is invalidated so the row drops.
 */
export function useDeleteDeploymentAttachment(): UseMutationResult<
  void,
  Error,
  DeleteDeploymentAttachmentInput
> {
  const api = useBackendApi();
  const queryClient = useQueryClient();

  return useMutation<void, Error, DeleteDeploymentAttachmentInput>({
    mutationFn: async (input): Promise<void> => {
      try {
        await deleteEntityAttachment(api, { type: "deployment", id: input.attachmentId });
      } catch (err) {
        throw new Error(entityAttachmentErrorMessage(err, "Could not delete the attachment."), {
          cause: err,
        });
      }
    },
    onSuccess: (_void, variables) => {
      void queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.DEPLOYMENT_ATTACHMENTS, variables.deploymentId],
      });
    },
  });
}
