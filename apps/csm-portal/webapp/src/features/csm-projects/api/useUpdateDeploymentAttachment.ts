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
  BeAttachmentUpdatePayload,
  BeAttachmentUpdateResponse,
} from "@api/backend/types";

export interface UpdateDeploymentAttachmentInput {
  deploymentId: string;
  attachmentId: string;
  /** At least one of `name`/`description` is required by the BE. */
  name?: string;
  description?: string | null;
}

/**
 * Update an attachment's name/description via `PATCH /attachments/{id}`
 * (`referenceType: "deployment"`). On success, invalidates the deployment's
 * attachment list so the row refreshes.
 */
export function useUpdateDeploymentAttachment(): UseMutationResult<
  BeAttachmentUpdateResponse,
  Error,
  UpdateDeploymentAttachmentInput
> {
  const api = useBackendApi();
  const queryClient = useQueryClient();

  return useMutation<
    BeAttachmentUpdateResponse,
    Error,
    UpdateDeploymentAttachmentInput
  >({
    mutationFn: ({
      deploymentId,
      attachmentId,
      name,
      description,
    }): Promise<BeAttachmentUpdateResponse> => {
      const payload: BeAttachmentUpdatePayload = {
        referenceId: deploymentId,
        referenceType: "deployment",
      };
      if (name !== undefined) payload.name = name;
      if (description !== undefined) payload.description = description;
      return api.patch<BeAttachmentUpdatePayload, BeAttachmentUpdateResponse>(
        `/attachments/${encodeURIComponent(attachmentId)}`,
        payload,
      );
    },
    onSuccess: (_res, variables) => {
      queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.DEPLOYMENT_ATTACHMENTS, variables.deploymentId],
      });
    },
  });
}
