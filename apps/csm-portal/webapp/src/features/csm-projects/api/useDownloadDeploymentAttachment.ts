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
import { useBackendApi } from "@api/backend/client";
import { saveBlob } from "@utils/saveBlob";
import { isSafeAttachmentContentType } from "@features/csm-cases/utils/attachmentPreview";
import type { DeploymentAttachment } from "@features/csm-projects/types/csmProjects";

/**
 * Returns a function that fetches a deployment attachment's raw bytes via
 * `GET /attachments/{id}/content`. Streams behind auth as a `Blob` — see
 * {@link isSafeAttachmentContentType} for why the fetched blob is only
 * re-labeled with the list metadata's (uploader-controlled) content type
 * when that type is itself in the backend's safe allowlist.
 */
export function useDownloadDeploymentAttachment(): (
  attachment: DeploymentAttachment,
) => Promise<void> {
  const api = useBackendApi();

  return useCallback(
    async (attachment: DeploymentAttachment): Promise<void> => {
      const blob = await api.getBlob(
        `/attachments/${encodeURIComponent(attachment.id)}/content`,
      );
      const toSave =
        isSafeAttachmentContentType(attachment.contentType) &&
        blob.type !== attachment.contentType
          ? blob.slice(0, blob.size, attachment.contentType)
          : blob;
      saveBlob(toSave, attachment.name);
    },
    [api],
  );
}
